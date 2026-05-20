package agent

import "context"

// Provider is the agent decision-making abstraction. Implementations handle
// tool-use loops, cost accounting, and latency tracking internally.
type Provider interface {
	// Decide runs the agent loop for one trigger and returns the final structured
	// decision. Implementations are responsible for connecting to the MCP server,
	// executing tool calls, and bounding the loop (max iterations, timeout).
	Decide(ctx context.Context, req DecisionRequest) (*Decision, error)
	Name() string
	Model() string
}
