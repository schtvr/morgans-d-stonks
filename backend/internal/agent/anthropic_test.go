package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"sync/atomic"
	"testing"
)

// fakeMCPClient captures CallTool invocations and returns a canned response.
type fakeMCPClient struct {
	calls  []string
	output json.RawMessage
	err    error
}

func (f *fakeMCPClient) CallTool(_ context.Context, name string, _ json.RawMessage) (json.RawMessage, error) {
	f.calls = append(f.calls, name)
	if f.err != nil {
		return nil, f.err
	}
	return f.output, nil
}

// roundTripFunc adapts a function to http.RoundTripper.
type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return fn(r) }

// apiResponse builds a minimal Anthropic Messages API response body.
// Uses large token counts so cost computation yields > 0 cents.
func apiResponse(stopReason string, contentBlocks []map[string]interface{}) string {
	resp := map[string]interface{}{
		"id":            "msg_test",
		"type":          "message",
		"role":          "assistant",
		"model":         "claude-sonnet-4-5",
		"stop_reason":   stopReason,
		"stop_sequence": nil,
		"content":       contentBlocks,
		"usage": map[string]interface{}{
			"input_tokens":  10000,
			"output_tokens": 5000,
		},
	}
	b, _ := json.Marshal(resp)
	return string(b)
}

func httpOK(body string) *http.Response {
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(bytes.NewBufferString(body)),
	}
}

// buildTestProvider wires a provider with the given round-tripper sequence.
// Responses are returned in order; panics if more calls are made than responses.
func buildTestProvider(t *testing.T, mcp MCPClient, responses []string) *AnthropicProvider {
	t.Helper()
	var idx atomic.Int32
	transport := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		i := int(idx.Add(1)) - 1
		if i >= len(responses) {
			t.Errorf("unexpected HTTP call #%d (only %d responses configured)", i+1, len(responses))
			return httpOK(`{}`), nil
		}
		return httpOK(responses[i]), nil
	})
	prompt := &Prompt{Body: "test system prompt", Version: "abcdef012345"}
	return NewAnthropicProviderWithHTTPClient(
		&http.Client{Transport: transport},
		"claude-sonnet-4-5",
		mcp,
		prompt,
	)
}

// TestAnthropicProvider_ToolUseThenDecision verifies the happy path:
// first response is tool_use, second is text with a valid Decision.
func TestAnthropicProvider_ToolUseThenDecision(t *testing.T) {
	t.Parallel()

	toolUseResp := apiResponse("tool_use", []map[string]interface{}{
		{
			"type":  "tool_use",
			"id":    "tu_1",
			"name":  "get_market_candles",
			"input": map[string]interface{}{"symbol": "BTC-USD", "window": "24h"},
		},
	})

	decisionText := `{"action":"buy","confidence":0.75,"rationale":"price up 5%","sizingHintNotional":null,"toolCalls":[]}`
	textResp := apiResponse("end_turn", []map[string]interface{}{
		{"type": "text", "text": decisionText},
	})

	mcp := &fakeMCPClient{output: json.RawMessage(`{"points":[]}`)}
	p := buildTestProvider(t, mcp, []string{toolUseResp, textResp})

	req := DecisionRequest{
		TriggerKind:    TriggerSignal,
		IdempotencyKey: "test-1",
	}
	d, err := p.Decide(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if d.Action != ActionBuy {
		t.Errorf("expected buy, got %s", d.Action)
	}
	if d.Confidence != 0.75 {
		t.Errorf("expected 0.75, got %v", d.Confidence)
	}
	if len(mcp.calls) != 1 || mcp.calls[0] != "get_market_candles" {
		t.Errorf("expected one MCP call to get_market_candles, got %v", mcp.calls)
	}
	if len(d.ToolCalls) != 1 {
		t.Errorf("expected 1 tool call recorded, got %d", len(d.ToolCalls))
	}
	if d.CostCents == 0 {
		t.Error("expected non-zero cost cents")
	}
}

// TestAnthropicProvider_MalformedJSON verifies that a parse error is returned
// when the final text block is not valid Decision JSON.
func TestAnthropicProvider_MalformedJSON(t *testing.T) {
	t.Parallel()

	textResp := apiResponse("end_turn", []map[string]interface{}{
		{"type": "text", "text": `not valid json`},
	})

	mcp := &fakeMCPClient{}
	p := buildTestProvider(t, mcp, []string{textResp})

	req := DecisionRequest{TriggerKind: TriggerDaily}
	_, err := p.Decide(context.Background(), req)
	if err == nil {
		t.Fatal("expected error for malformed JSON, got nil")
	}
}

// TestAnthropicProvider_UnknownField verifies that unknown fields in the
// decision JSON cause rejection.
func TestAnthropicProvider_UnknownField(t *testing.T) {
	t.Parallel()

	bad := `{"action":"buy","confidence":0.8,"rationale":"ok","sizingHintNotional":null,"toolCalls":[],"extraField":"oops"}`
	textResp := apiResponse("end_turn", []map[string]interface{}{
		{"type": "text", "text": bad},
	})

	mcp := &fakeMCPClient{}
	p := buildTestProvider(t, mcp, []string{textResp})

	_, err := p.Decide(context.Background(), req(TriggerDaily))
	if err == nil {
		t.Fatal("expected error for unknown field, got nil")
	}
}

// TestAnthropicProvider_LoopExhausted verifies that 8 consecutive tool_use
// responses result in an error.
func TestAnthropicProvider_LoopExhausted(t *testing.T) {
	t.Parallel()

	toolUseResp := apiResponse("tool_use", []map[string]interface{}{
		{"type": "tool_use", "id": "tu_1", "name": "get_holdings", "input": map[string]interface{}{}},
	})
	responses := make([]string, maxIterations)
	for i := range responses {
		responses[i] = toolUseResp
	}

	mcp := &fakeMCPClient{output: json.RawMessage(`{}`)}
	p := buildTestProvider(t, mcp, responses)

	_, err := p.Decide(context.Background(), req(TriggerDaily))
	if err == nil {
		t.Fatal("expected error after loop exhaustion, got nil")
	}
}

// TestAnthropicProvider_InvalidAction checks rejection of bad action values.
func TestAnthropicProvider_InvalidAction(t *testing.T) {
	t.Parallel()

	bad := `{"action":"hold","confidence":0.8,"rationale":"ok","sizingHintNotional":null,"toolCalls":[]}`
	textResp := apiResponse("end_turn", []map[string]interface{}{
		{"type": "text", "text": bad},
	})

	mcp := &fakeMCPClient{}
	p := buildTestProvider(t, mcp, []string{textResp})

	_, err := p.Decide(context.Background(), req(TriggerDaily))
	if err == nil {
		t.Fatal("expected error for invalid action, got nil")
	}
}

// TestAnthropicProvider_MCPError verifies that an MCP tool error is surfaced
// as an error result block but the loop continues to the next response.
func TestAnthropicProvider_MCPError(t *testing.T) {
	t.Parallel()

	toolUseResp := apiResponse("tool_use", []map[string]interface{}{
		{"type": "tool_use", "id": "tu_1", "name": "get_holdings", "input": map[string]interface{}{}},
	})
	decisionText := `{"action":"ignore","confidence":0.3,"rationale":"mcp error fallback","sizingHintNotional":null,"toolCalls":[]}`
	textResp := apiResponse("end_turn", []map[string]interface{}{
		{"type": "text", "text": decisionText},
	})

	mcp := &fakeMCPClient{err: errors.New("mcp: unavailable")}
	p := buildTestProvider(t, mcp, []string{toolUseResp, textResp})

	d, err := p.Decide(context.Background(), req(TriggerSignal))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if d.Action != ActionIgnore {
		t.Errorf("expected ignore, got %s", d.Action)
	}
	// The tool call should still be recorded (with error output).
	if len(d.ToolCalls) != 1 {
		t.Errorf("expected 1 tool call recorded even on error, got %d", len(d.ToolCalls))
	}
}

func req(kind TriggerKind) DecisionRequest {
	return DecisionRequest{TriggerKind: kind, IdempotencyKey: "test"}
}
