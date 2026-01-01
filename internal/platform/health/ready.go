package health

import "context"

// NewReadyGraph returns a root node for readiness dependencies.
// Callers add named deps via ready.Add("db", CheckDB(...)), etc.
func NewReadyGraph() *Node {
	return &Node{Name: "ready"}
}

// CheckAlwaysReady returns a readiness check that always succeeds.
// Useful as a placeholder dependency that you can later replace with a real check.
func CheckAlwaysReady() Check {
	return func(ctx context.Context) error {
		return nil
	}
}
