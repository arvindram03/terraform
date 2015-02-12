package terraform

// EvalDiff is an EvalNode implementation that does a refresh for
// a resource.
type EvalDiff struct {
	Info     *InstanceInfo
	Config   EvalNode
	Provider EvalNode
	State    EvalNode
	Output   *InstanceDiff
}

func (n *EvalDiff) Args() ([]EvalNode, []EvalType) {
	return []EvalNode{n.Config, n.Provider, n.State},
		[]EvalType{EvalTypeConfig, EvalTypeResourceProvider,
			EvalTypeInstanceState}
}

// TODO: test
func (n *EvalDiff) Eval(
	ctx EvalContext, args []interface{}) (interface{}, error) {
	// Extract our arguments
	var state *InstanceState
	config := args[0].(*ResourceConfig)
	provider := args[1].(ResourceProvider)
	if args[2] != nil {
		state = args[2].(*InstanceState)
	}

	// Call pre-diff hook
	err := ctx.Hook(func(h Hook) (HookAction, error) {
		return h.PreDiff(n.Info, state)
	})
	if err != nil {
		return nil, err
	}

	// The state for the diff must never be nil
	diffState := state
	if diffState == nil {
		diffState = new(InstanceState)
	}
	diffState.init()

	// Diff!
	diff, err := provider.Diff(n.Info, diffState, config)
	if err != nil {
		return nil, err
	}
	if diff == nil {
		diff = new(InstanceDiff)
	}

	// Require a destroy if there is no ID and it requires new.
	if diff.RequiresNew() && state != nil && state.ID != "" {
		diff.Destroy = true
	}

	// If we're creating a new resource, compute its ID
	if diff.RequiresNew() || state == nil || state.ID == "" {
		var oldID string
		if state != nil {
			oldID = state.Attributes["id"]
		}

		// Add diff to compute new ID
		diff.init()
		diff.Attributes["id"] = &ResourceAttrDiff{
			Old:         oldID,
			NewComputed: true,
			RequiresNew: true,
			Type:        DiffAttrOutput,
		}
	}

	// Call post-refresh hook
	err = ctx.Hook(func(h Hook) (HookAction, error) {
		return h.PostDiff(n.Info, diff)
	})
	if err != nil {
		return nil, err
	}

	// Update our output
	*n.Output = *diff

	// Merge our state so that the state is updated with our plan
	if !diff.Empty() {
		state = state.MergeDiff(diff)
	}

	return state, nil
}

func (n *EvalDiff) Type() EvalType {
	return EvalTypeInstanceState
}

// EvalDiffDestroy is an EvalNode implementation that returns a plain
// destroy diff.
type EvalDiffDestroy struct {
	Info   *InstanceInfo
	State  EvalNode
	Output *InstanceDiff
}

func (n *EvalDiffDestroy) Args() ([]EvalNode, []EvalType) {
	return []EvalNode{n.State}, []EvalType{EvalTypeInstanceState}
}

// TODO: test
func (n *EvalDiffDestroy) Eval(
	ctx EvalContext, args []interface{}) (interface{}, error) {
	// Extract our arguments
	var state *InstanceState
	if args[0] != nil {
		state = args[0].(*InstanceState)
	}

	// Call pre-diff hook
	err := ctx.Hook(func(h Hook) (HookAction, error) {
		return h.PreDiff(n.Info, state)
	})
	if err != nil {
		return nil, err
	}

	// The diff
	diff := &InstanceDiff{Destroy: true}

	// Call post-diff hook
	err = ctx.Hook(func(h Hook) (HookAction, error) {
		return h.PostDiff(n.Info, diff)
	})
	if err != nil {
		return nil, err
	}

	// Update our output
	*n.Output = *diff

	return nil, nil
}

func (n *EvalDiffDestroy) Type() EvalType {
	return EvalTypeNull
}

// EvalWriteDiff is an EvalNode implementation that writes the diff to
// the full diff.
type EvalWriteDiff struct {
	Name string
	Diff *InstanceDiff
}

func (n *EvalWriteDiff) Args() ([]EvalNode, []EvalType) {
	return nil, nil
}

// TODO: test
func (n *EvalWriteDiff) Eval(
	ctx EvalContext, args []interface{}) (interface{}, error) {
	diff, lock := ctx.Diff()

	// Acquire the lock so that we can do this safely concurrently
	lock.Lock()
	defer lock.Unlock()

	// Write the diff
	modDiff := diff.ModuleByPath(ctx.Path())
	if modDiff == nil {
		modDiff = diff.AddModule(ctx.Path())
	}
	modDiff.Resources[n.Name] = n.Diff

	return nil, nil
}

func (n *EvalWriteDiff) Type() EvalType {
	return EvalTypeNull
}