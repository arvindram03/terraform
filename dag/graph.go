package dag

import (
	"bytes"
	"fmt"
	"sort"
	"sync"
)

// Graph is used to represent a dependency graph.
type Graph struct {
	vertices  *Set
	edges     *Set
	downEdges map[Vertex]*Set
	upEdges   map[Vertex]*Set
	once      sync.Once
}

// Vertex of the graph.
type Vertex interface{}

// NamedVertex is an optional interface that can be implemented by Vertex
// to give it a human-friendly name that is used for outputting the graph.
type NamedVertex interface {
	Vertex
	Name() string
}

// Vertices returns the list of all the vertices in the graph.
func (g *Graph) Vertices() []Vertex {
	list := g.vertices.List()
	result := make([]Vertex, len(list))
	for i, v := range list {
		result[i] = v.(Vertex)
	}

	return result
}

// Edges returns the list of all the edges in the graph.
func (g *Graph) Edges() []Edge {
	list := g.edges.List()
	result := make([]Edge, len(list))
	for i, v := range list {
		result[i] = v.(Edge)
	}

	return result
}

// Add adds a vertex to the graph. This is safe to call multiple time with
// the same Vertex.
func (g *Graph) Add(v Vertex) Vertex {
	g.once.Do(g.init)
	g.vertices.Add(v)
	return v
}

// Remove removes a vertex from the graph. This will also remove any
// edges with this vertex as a source or target.
func (g *Graph) Remove(v Vertex) Vertex {
	// Delete the vertex itself
	g.vertices.Delete(v)

	// Delete the edges to non-existent things
	for _, target := range g.DownEdges(v).List() {
		g.RemoveEdge(BasicEdge(v, target))
	}
	for _, source := range g.UpEdges(v).List() {
		g.RemoveEdge(BasicEdge(source, v))
	}

	return nil
}

// RemoveEdge removes an edge from the graph.
func (g *Graph) RemoveEdge(edge Edge) {
	g.once.Do(g.init)

	// Delete the edge from the set
	g.edges.Delete(edge)

	// Delete the up/down edges
	if s, ok := g.downEdges[edge.Source()]; ok {
		s.Delete(edge.Target())
	}
	if s, ok := g.upEdges[edge.Target()]; ok {
		s.Delete(edge.Source())
	}
}

// DownEdges returns the outward edges from the source Vertex v.
func (g *Graph) DownEdges(v Vertex) *Set {
	g.once.Do(g.init)
	return g.downEdges[v]
}

// UpEdges returns the inward edges to the destination Vertex v.
func (g *Graph) UpEdges(v Vertex) *Set {
	g.once.Do(g.init)
	return g.upEdges[v]
}

// Connect adds an edge with the given source and target. This is safe to
// call multiple times with the same value. Note that the same value is
// verified through pointer equality of the vertices, not through the
// value of the edge itself.
func (g *Graph) Connect(edge Edge) {
	g.once.Do(g.init)

	source := edge.Source()
	target := edge.Target()

	// Do we have this already? If so, don't add it again.
	if s, ok := g.downEdges[source]; ok && s.Include(target) {
		return
	}

	// Add the edge to the set
	g.edges.Add(edge)

	// Add the down edge
	s, ok := g.downEdges[source]
	if !ok {
		s = new(Set)
		g.downEdges[source] = s
	}
	s.Add(target)

	// Add the up edge
	s, ok = g.upEdges[target]
	if !ok {
		s = new(Set)
		g.upEdges[target] = s
	}
	s.Add(source)
}

// String outputs some human-friendly output for the graph structure.
func (g *Graph) String() string {
	var buf bytes.Buffer

	// Build the list of node names and a mapping so that we can more
	// easily alphabetize the output to remain deterministic.
	vertices := g.Vertices()
	names := make([]string, 0, len(vertices))
	mapping := make(map[string]Vertex, len(vertices))
	for _, v := range vertices {
		name := VertexName(v)
		names = append(names, name)
		mapping[name] = v
	}
	sort.Strings(names)

	// Write each node in order...
	for _, name := range names {
		v := mapping[name]
		targets := g.downEdges[v]

		buf.WriteString(fmt.Sprintf("%s\n", name))

		// Alphabetize dependencies
		deps := make([]string, 0, targets.Len())
		for _, target := range targets.List() {
			deps = append(deps, VertexName(target))
		}
		sort.Strings(deps)

		// Write dependencies
		for _, d := range deps {
			buf.WriteString(fmt.Sprintf("  %s\n", d))
		}
	}

	return buf.String()
}

func (g *Graph) init() {
	g.vertices = new(Set)
	g.edges = new(Set)
	g.downEdges = make(map[Vertex]*Set)
	g.upEdges = make(map[Vertex]*Set)
}

// VertexName returns the name of a vertex.
func VertexName(raw Vertex) string {
	switch v := raw.(type) {
	case NamedVertex:
		return v.Name()
	case fmt.Stringer:
		return fmt.Sprintf("%s", v)
	default:
		return fmt.Sprintf("%v", v)
	}
}