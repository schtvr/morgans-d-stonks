package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	anthropic "github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
)

// MCPClient dispatches tool calls to the in-process MCP server.
// Task 3 provides the concrete implementation; tests use a stub.
type MCPClient interface {
	CallTool(ctx context.Context, name string, input json.RawMessage) (json.RawMessage, error)
}

// Price constants (USD per million tokens) for cost estimation.
const (
	sonnetInputPerM  float64 = 3.00
	sonnetOutputPerM float64 = 15.00
	haikuInputPerM   float64 = 0.25
	haikuOutputPerM  float64 = 1.25

	maxIterations     = 8
	decisionMaxTokens = 2048
)

// pricingForModel returns (inputPerM, outputPerM) in USD.
func pricingForModel(model string) (float64, float64) {
	switch {
	case len(model) >= 12 && model[:12] == "claude-haiku":
		return haikuInputPerM, haikuOutputPerM
	default: // default: sonnet pricing
		return sonnetInputPerM, sonnetOutputPerM
	}
}

// computeCostCents converts token counts to integer US cents.
func computeCostCents(model string, inputTokens, outputTokens int64) int64 {
	inPerM, outPerM := pricingForModel(model)
	usd := float64(inputTokens)/1e6*inPerM + float64(outputTokens)/1e6*outPerM
	return int64(usd * 100)
}

// decisionJSON is the strict subset we accept from the model.
type decisionJSON struct {
	Action             string   `json:"action"`
	Confidence         float64  `json:"confidence"`
	Rationale          string   `json:"rationale"`
	SizingHintNotional *float64 `json:"sizingHintNotional"`
}

// AnthropicProvider runs the tool-use loop against the Anthropic API.
type AnthropicProvider struct {
	client    anthropic.Client
	mcpClient MCPClient
	prompt    *Prompt
	model     string
}

// NewAnthropicProvider constructs a provider. apiKey may be empty when using
// the ANTHROPIC_API_KEY env var (the SDK picks it up automatically).
func NewAnthropicProvider(apiKey string, model string, mcpClient MCPClient, prompt *Prompt) *AnthropicProvider {
	opts := []option.RequestOption{}
	if apiKey != "" {
		opts = append(opts, option.WithAPIKey(apiKey))
	}
	c := anthropic.NewClient(opts...)
	return &AnthropicProvider{
		client:    c,
		mcpClient: mcpClient,
		prompt:    prompt,
		model:     model,
	}
}

// NewAnthropicProviderWithHTTPClient is used in tests to inject a fake transport.
func NewAnthropicProviderWithHTTPClient(httpClient *http.Client, model string, mcpClient MCPClient, prompt *Prompt) *AnthropicProvider {
	c := anthropic.NewClient(
		option.WithAPIKey("test-key"),
		option.WithHTTPClient(httpClient),
	)
	return &AnthropicProvider{
		client:    c,
		mcpClient: mcpClient,
		prompt:    prompt,
		model:     model,
	}
}

func (p *AnthropicProvider) Name() string  { return "anthropic" }
func (p *AnthropicProvider) Model() string { return p.model }

func (p *AnthropicProvider) Decide(ctx context.Context, req DecisionRequest) (*Decision, error) {
	start := time.Now()

	systemPreamble := p.buildSystemPrompt()
	userMsg, err := p.buildUserMessage(req)
	if err != nil {
		return nil, fmt.Errorf("anthropic: build user message: %w", err)
	}

	tools := p.placeholderTools()
	messages := []anthropic.MessageParam{
		anthropic.NewUserMessage(anthropic.NewTextBlock(userMsg)),
	}

	var (
		totalInputTokens  int64
		totalOutputTokens int64
		toolCalls         []ToolCall
	)

	for i := 0; i < maxIterations; i++ {
		resp, err := p.client.Messages.New(ctx, anthropic.MessageNewParams{
			Model:     p.model,
			MaxTokens: decisionMaxTokens,
			System:    []anthropic.TextBlockParam{{Text: systemPreamble}},
			Messages:  messages,
			Tools:     tools,
		})
		if err != nil {
			return nil, fmt.Errorf("anthropic: api call: %w", err)
		}

		totalInputTokens += resp.Usage.InputTokens
		totalOutputTokens += resp.Usage.OutputTokens

		// Append the assistant turn to the conversation.
		contentParams := make([]anthropic.ContentBlockParamUnion, 0, len(resp.Content))
		for _, blk := range resp.Content {
			switch blk.Type {
			case "text":
				contentParams = append(contentParams, anthropic.NewTextBlock(blk.AsText().Text))
			case "tool_use":
				tu := blk.AsToolUse()
				contentParams = append(contentParams, anthropic.NewToolUseBlock(tu.ID, tu.Input, tu.Name))
			}
		}
		messages = append(messages, anthropic.NewAssistantMessage(contentParams...))

		if resp.StopReason == anthropic.StopReasonToolUse {
			// Dispatch all tool calls in this response and collect results.
			toolResultBlocks := []anthropic.ContentBlockParamUnion{}
			for _, block := range resp.Content {
				if block.Type != "tool_use" {
					continue
				}
				tu := block.AsToolUse()
				toolStart := time.Now()
				output, callErr := p.mcpClient.CallTool(ctx, tu.Name, tu.Input)
				durMS := time.Since(toolStart).Milliseconds()

				if callErr != nil {
					errOutput, _ := json.Marshal(map[string]string{"error": callErr.Error()})
					toolResultBlocks = append(toolResultBlocks,
						anthropic.NewToolResultBlock(tu.ID, string(errOutput), true))
					toolCalls = append(toolCalls, ToolCall{
						Name:       tu.Name,
						Input:      tu.Input,
						Output:     errOutput,
						DurationMS: durMS,
					})
				} else {
					toolResultBlocks = append(toolResultBlocks,
						anthropic.NewToolResultBlock(tu.ID, string(output), false))
					toolCalls = append(toolCalls, ToolCall{
						Name:       tu.Name,
						Input:      tu.Input,
						Output:     output,
						DurationMS: durMS,
					})
				}
			}
			messages = append(messages, anthropic.NewUserMessage(toolResultBlocks...))
			continue
		}

		// StopReasonEndTurn or similar — expect a text block with the Decision JSON.
		for _, block := range resp.Content {
			if block.Type != "text" {
				continue
			}
			tb := block.AsText()
			d, parseErr := parseDecisionJSON(tb.Text)
			if parseErr != nil {
				return nil, fmt.Errorf("anthropic: parse decision: %w", parseErr)
			}
			d.Model = p.model
			d.ToolCalls = toolCalls
			d.LatencyMS = time.Since(start).Milliseconds()
			d.CostCents = computeCostCents(p.model, totalInputTokens, totalOutputTokens)
			return d, nil
		}

		// No text block found in a non-tool-use response — treat as an error.
		return nil, fmt.Errorf("anthropic: response contained no text block (stop_reason=%s)", resp.StopReason)
	}

	return nil, fmt.Errorf("anthropic: tool-use loop exhausted after %d iterations without a text response", maxIterations)
}

// buildSystemPrompt combines the loaded prompt body with a schema preamble.
func (p *AnthropicProvider) buildSystemPrompt() string {
	schema := `## Decision output schema (STRICT — return only this JSON, no markdown fences)

{
  "action": "buy" | "sell" | "ignore",
  "confidence": 0.0..1.0,
  "rationale": "≤ 1000 chars; cite specific numbers from your tool calls",
  "sizingHintNotional": <number in USD> | null,
  "toolCalls": []
}

Unknown fields will cause the decision to be rejected. The "toolCalls" field is managed by the system; set it to [].`

	return p.prompt.Body + "\n\n" + schema
}

// buildUserMessage serializes the EagerContext and optional Signal into the
// initial user turn.
func (p *AnthropicProvider) buildUserMessage(req DecisionRequest) (string, error) {
	type userPayload struct {
		TriggerKind  TriggerKind  `json:"triggerKind"`
		TriggerAt    time.Time    `json:"triggerAt"`
		EagerContext EagerContext `json:"eagerContext"`
		Signal       interface{}  `json:"signal,omitempty"`
	}
	payload := userPayload{
		TriggerKind:  req.TriggerKind,
		TriggerAt:    req.TriggerAt,
		EagerContext: req.EagerContext,
		Signal:       req.Signal,
	}
	b, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// placeholderTools returns the tool definitions the agent may call via MCP.
// The MCP client resolves them at dispatch time; the input schemas here are
// intentionally permissive (freeform JSON object) — the MCP server validates.
func (p *AnthropicProvider) placeholderTools() []anthropic.ToolUnionParam {
	names := []string{
		"get_market_candles",
		"get_holdings",
		"get_position",
		"get_correlated_symbols",
		"get_recent_signals",
		"get_recent_decisions",
		"get_decision_outcomes",
	}
	tools := make([]anthropic.ToolUnionParam, 0, len(names))
	for _, name := range names {
		tools = append(tools, anthropic.ToolUnionParamOfTool(
			anthropic.ToolInputSchemaParam{
				Properties: map[string]interface{}{},
			},
			name,
		))
	}
	return tools
}

// parseDecisionJSON strictly parses the model output into a Decision.
// Unknown top-level fields cause rejection.
func parseDecisionJSON(text string) (*Decision, error) {
	// Strict decode: DisallowUnknownFields.
	var raw map[string]json.RawMessage
	if err := json.Unmarshal([]byte(text), &raw); err != nil {
		return nil, fmt.Errorf("not valid JSON: %w", err)
	}
	allowed := map[string]bool{
		"action": true, "confidence": true, "rationale": true,
		"sizingHintNotional": true, "toolCalls": true,
	}
	for k := range raw {
		if !allowed[k] {
			return nil, fmt.Errorf("unknown field %q in decision JSON", k)
		}
	}

	var dj decisionJSON
	if err := json.Unmarshal([]byte(text), &dj); err != nil {
		return nil, fmt.Errorf("unmarshal decision: %w", err)
	}

	switch Action(dj.Action) {
	case ActionBuy, ActionSell, ActionIgnore:
	default:
		return nil, fmt.Errorf("invalid action %q; must be buy, sell, or ignore", dj.Action)
	}
	if dj.Confidence < 0 || dj.Confidence > 1 {
		return nil, fmt.Errorf("confidence %v out of range [0, 1]", dj.Confidence)
	}

	return &Decision{
		Action:             Action(dj.Action),
		Confidence:         dj.Confidence,
		Rationale:          dj.Rationale,
		SizingHintNotional: dj.SizingHintNotional,
	}, nil
}
