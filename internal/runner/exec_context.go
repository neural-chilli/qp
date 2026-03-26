package runner

// ExecutionContext carries shared state through the runner execution pipeline.
// For now it is a pass-through placeholder — P2 will add a flowing JSON state
// object that nodes read from and write to.
type ExecutionContext struct {
	// State will hold the shared JSON state map in P2.
	// Parallel nodes receive a snapshot; outputs are merged after completion.
	State map[string]any
}

// NewExecutionContext returns an empty execution context.
func NewExecutionContext() *ExecutionContext {
	return &ExecutionContext{
		State: map[string]any{},
	}
}
