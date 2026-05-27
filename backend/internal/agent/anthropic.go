package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
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
	Action             string          `json:"action"`
	Confidence         float64         `json:"confidence"`
	Rationale          string          `json:"rationale"`
	SizingHintNotional json.RawMessage `json:"sizingHintNotional"`
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

				// record_decision is the decision output tool — parse and return.
				if tu.Name == "record_decision" {
					d, parseErr := parseDecisionJSON(string(tu.Input))
					if parseErr != nil {
						return nil, fmt.Errorf("anthropic: parse record_decision: %w", parseErr)
					}
					d.Model = p.model
					d.ToolCalls = toolCalls
					d.LatencyMS = time.Since(start).Milliseconds()
					d.CostCents = computeCostCents(p.model, totalInputTokens, totalOutputTokens)
					return d, nil
				}

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

		// StopReasonEndTurn — model chose not to call record_decision.
		// Try one forced final call requiring record_decision.
		if resp.StopReason == anthropic.StopReasonEndTurn && i < maxIterations-1 {
			messages = append(messages, anthropic.NewUserMessage(
				anthropic.NewTextBlock("Now call record_decision with your final action."),
			))
			// Force record_decision on the next iteration via tool_choice below;
			// rebuild with forced tool_choice in a targeted call.
			forced, ferr := p.client.Messages.New(ctx, anthropic.MessageNewParams{
				Model:     p.model,
				MaxTokens: 512,
				System:    []anthropic.TextBlockParam{{Text: systemPreamble}},
				Messages:  messages,
				Tools:     tools,
				ToolChoice: anthropic.ToolChoiceParamOfTool("record_decision"),
			})
			if ferr != nil {
				return nil, fmt.Errorf("anthropic: forced record_decision call: %w", ferr)
			}
			totalInputTokens += forced.Usage.InputTokens
			totalOutputTokens += forced.Usage.OutputTokens
			for _, block := range forced.Content {
				if block.Type == "tool_use" {
					tu := block.AsToolUse()
					if tu.Name == "record_decision" {
						d, parseErr := parseDecisionJSON(string(tu.Input))
						if parseErr != nil {
							return nil, fmt.Errorf("anthropic: parse forced record_decision: %w", parseErr)
						}
						d.Model = p.model
						d.ToolCalls = toolCalls
						d.LatencyMS = time.Since(start).Milliseconds()
						d.CostCents = computeCostCents(p.model, totalInputTokens, totalOutputTokens)
						return d, nil
					}
				}
			}
		}

		// No decision extracted after all attempts.
		return nil, fmt.Errorf("anthropic: no decision produced (stop_reason=%s)", resp.StopReason)
	}

	return nil, fmt.Errorf("anthropic: tool-use loop exhausted after %d iterations without a text response", maxIterations)
}

// buildSystemPrompt combines the loaded prompt body with output instructions.
func (p *AnthropicProvider) buildSystemPrompt() string {
	outputInstructions := `## How to submit your decision

When you are ready to decide, call the **record_decision** tool with your final answer.
Do NOT write prose text as your final response — always call record_decision.
You may call any of the other tools first to gather data, then call record_decision last.

Fields:
- action: "buy", "sell", or "ignore"
- confidence: float 0.0–1.0 (return "ignore" if < 0.55)
- rationale: ≤1000 chars; cite specific numbers from tool calls
- sizingHintNotional: USD notional suggestion (or omit / set null)
- toolCalls: leave as []`

	return p.prompt.Body + "\n\n" + outputInstructions
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

// placeholderTools returns the tool definitions the agent may call via MCP,
// plus the special record_decision output tool.
// MCP tools use a permissive schema; record_decision has a strict schema
// so Claude knows exactly what fields to populate.
func (p *AnthropicProvider) placeholderTools() []anthropic.ToolUnionParam {
	mcpTools := []string{
		"get_market_candles",
		"get_holdings",
		"get_position",
		"get_correlated_symbols",
		"get_recent_signals",
		"get_recent_decisions",
		"get_decision_outcomes",
	}
	tools := make([]anthropic.ToolUnionParam, 0, len(mcpTools)+1)
	for _, name := range mcpTools {
		tools = append(tools, anthropic.ToolUnionParamOfTool(
			anthropic.ToolInputSchemaParam{
				Properties: map[string]interface{}{
					"params": map[string]interface{}{
						"type":        "object",
						"description": "Tool parameters (see tool description)",
					},
				},
			},
			name,
		))
	}

	// record_decision is the structured output tool — Claude MUST call this
	// to submit its final decision instead of returning prose text.
	tools = append(tools, anthropic.ToolUnionParamOfTool(
		anthropic.ToolInputSchemaParam{
			Properties: map[string]interface{}{
				"action": map[string]interface{}{
					"type":        "string",
					"enum":        []string{"buy", "sell", "ignore"},
					"description": "The action to take",
				},
				"confidence": map[string]interface{}{
					"type":        "number",
					"description": "Confidence in the decision, 0.0–1.0",
				},
				"rationale": map[string]interface{}{
					"type":        "string",
					"description": "≤1000 chars; cite specific numbers from tool calls",
				},
				"sizingHintNotional": map[string]interface{}{
					"type":        "number",
					"description": "Suggested USD notional for the order, or null to ignore",
				},
				"toolCalls": map[string]interface{}{
					"type":        "array",
					"description": "Leave as empty array []",
					"items":       map[string]interface{}{"type": "object"},
				},
			},
		},
		"record_decision",
	))
	return tools
}

// parseDecisionJSON strictly parses the model output into a Decision.
// Unknown top-level fields cause rejection.
func parseDecisionJSON(text string) (*Decision, error) {
	// Extract first {...} block, tolerating any prose preamble.
	if start := strings.Index(text, "{"); start != -1 {
		if end := strings.LastIndex(text, "}"); end > start {
			text = text[start : end+1]
		}
	}
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

	sizing, err := parseSizingHintNotional(dj.SizingHintNotional)
	if err != nil {
		return nil, err
	}

	return &Decision{
		Action:             Action(dj.Action),
		Confidence:         dj.Confidence,
		Rationale:          dj.Rationale,
		SizingHintNotional: sizing,
	}, nil
}

// parseSizingHintNotional accepts null, number, or string from the model tool input.
func parseSizingHintNotional(raw json.RawMessage) (*float64, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return nil, nil
	}
	var f float64
	if err := json.Unmarshal(raw, &f); err == nil {
		return &f, nil
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		if s == "" || s == "null" {
			return nil, nil
		}
		var parsed float64
		if _, err := fmt.Sscanf(s, "%f", &parsed); err != nil {
			return nil, fmt.Errorf("invalid sizingHintNotional %q", s)
		}
		return &parsed, nil
	}
	return nil, fmt.Errorf("invalid sizingHintNotional")
}
