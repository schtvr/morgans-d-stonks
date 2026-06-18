package agent

import "context"

// MockProvider is a deterministic, no-network provider for CI and local smoke tests.
// Selected when AGENT_PROVIDER=mock (the default).
type MockProvider struct{}

var _ Provider = (*MockProvider)(nil)

func NewMockProvider() *MockProvider { return &MockProvider{} }

func (m *MockProvider) Name() string  { return "mock" }
func (m *MockProvider) Model() string { return "mock" }

func (m *MockProvider) Decide(_ context.Context, req DecisionRequest) (*Decision, error) {
	if req.Signal != nil {
		if req.Signal.DeltaPct >= req.Signal.ThresholdPct*2 {
			return &Decision{
				Action:     ActionBuy,
				Confidence: 0.7,
				Rationale:  "mock: strong move",
				ToolCalls:  []ToolCall{},
				Model:      "mock",
				LatencyMS:  0,
				CostCents:  0,
			}, nil
		}
		if req.Signal.DeltaPct <= -req.Signal.ThresholdPct*2 {
			return &Decision{
				Action:     ActionSell,
				Confidence: 0.7,
				Rationale:  "mock: strong drop",
				ToolCalls:  []ToolCall{},
				Model:      "mock",
				LatencyMS:  0,
				CostCents:  0,
			}, nil
		}
	}
	return &Decision{
		Action:     ActionIgnore,
		Confidence: 0.3,
		Rationale:  "mock: insufficient signal",
		ToolCalls:  []ToolCall{},
		Model:      "mock",
		LatencyMS:  0,
		CostCents:  0,
	}, nil
}
