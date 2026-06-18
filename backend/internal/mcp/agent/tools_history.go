// Integration tests for this tool require the Task 6 portfolio-api
// internal read endpoints to be deployed. Unit tests use HTTP stubs.
package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"
)

func registerHistoryTools(s *mcpserver.MCPServer, portfolioAPIURL, internalAPIKey string, hc *http.Client) {
	s.AddTool(
		mcp.NewTool("get_recent_signals",
			mcp.WithDescription("Recent crypto signal alerts that fired, optionally filtered by symbol."),
			mcp.WithString("symbol", mcp.Description("Filter by symbol, e.g. BTC-USD")),
			mcp.WithNumber("limit", mcp.Description("Max rows (≤50, default 20)")),
			mcp.WithString("window", mcp.Description("Lookback window (≤72h, default 24h), e.g. 24h")),
		),
		makeGetRecentSignalsHandler(portfolioAPIURL, internalAPIKey, hc),
	)

	s.AddTool(
		mcp.NewTool("get_recent_decisions",
			mcp.WithDescription("Recent agent decisions, optionally filtered by symbol."),
			mcp.WithString("symbol", mcp.Description("Filter by symbol, e.g. BTC-USD")),
			mcp.WithNumber("limit", mcp.Description("Max rows (≤20, default 10)")),
			mcp.WithString("window", mcp.Description("Lookback window (≤72h, default 24h)")),
		),
		makeGetRecentDecisionsHandler(portfolioAPIURL, internalAPIKey, hc),
	)

	s.AddTool(
		mcp.NewTool("get_decision_outcomes",
			mcp.WithDescription("Scored decision outcomes (return% vs BTC baseline) for completed horizon windows."),
			mcp.WithString("symbol", mcp.Description("Filter by symbol")),
			mcp.WithString("horizon",
				mcp.Description("Scoring horizon: 24h | 7d | 14d (default 14d)"),
				mcp.Enum("24h", "7d", "14d"),
			),
			mcp.WithNumber("limit", mcp.Description("Max rows (≤50, default 20)")),
		),
		makeGetDecisionOutcomesHandler(portfolioAPIURL, internalAPIKey, hc),
	)
}

// clampInt returns v clamped to [1, max]. If v≤0, returns def.
func clampInt(v, def, max int) int {
	if v <= 0 {
		v = def
	}
	if v > max {
		v = max
	}
	return v
}

// clampWindow returns the window string clamped to the max allowed duration.
// If w is empty or unparseable, returns def.
func clampWindow(w, def string, maxDur time.Duration) string {
	if strings.TrimSpace(w) == "" {
		return def
	}
	d, err := time.ParseDuration(w)
	if err != nil || d <= 0 {
		return def
	}
	if d > maxDur {
		d = maxDur
	}
	return d.String()
}

// doInternalGET performs an authenticated GET to a portfolio-api internal endpoint.
func doInternalGET(ctx context.Context, hc *http.Client, rawURL, internalAPIKey string, q url.Values) (json.RawMessage, error) {
	u := rawURL
	if len(q) > 0 {
		u += "?" + q.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("X-Internal-Key", internalAPIKey)

	resp, err := hc.Do(req)
	if err != nil {
		return nil, fmt.Errorf("GET %s: %w", u, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GET %s: status %d: %s", u, resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return json.RawMessage(body), nil
}

func makeGetRecentSignalsHandler(portfolioAPIURL, internalAPIKey string, hc *http.Client) mcpserver.ToolHandlerFunc {
	baseURL := strings.TrimRight(portfolioAPIURL, "/") + "/internal/recent-alerts/list"
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args := req.GetArguments()

		limit := clampInt(intArg(args, "limit"), 20, 50)
		window := clampWindow(strArg(args, "window"), "24h", 72*time.Hour)
		dur, _ := time.ParseDuration(window)
		since := time.Now().UTC().Add(-dur).Format(time.RFC3339)

		q := url.Values{}
		q.Set("limit", fmt.Sprint(limit))
		q.Set("since", since)
		if sym := strArg(args, "symbol"); sym != "" {
			q.Set("symbol", sym)
		}

		raw, err := doInternalGET(ctx, hc, baseURL, internalAPIKey, q)
		if err != nil {
			return mcp.NewToolResultError(errJSON("upstream_failure", err.Error())), nil
		}
		return mcp.NewToolResultText(string(raw)), nil
	}
}

func makeGetRecentDecisionsHandler(portfolioAPIURL, internalAPIKey string, hc *http.Client) mcpserver.ToolHandlerFunc {
	baseURL := strings.TrimRight(portfolioAPIURL, "/") + "/internal/agent-decisions/list"
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args := req.GetArguments()

		limit := clampInt(intArg(args, "limit"), 10, 20)
		window := clampWindow(strArg(args, "window"), "24h", 72*time.Hour)
		dur, _ := time.ParseDuration(window)
		since := time.Now().UTC().Add(-dur).Format(time.RFC3339)

		q := url.Values{}
		q.Set("limit", fmt.Sprint(limit))
		q.Set("since", since)
		if sym := strArg(args, "symbol"); sym != "" {
			q.Set("symbol", sym)
		}

		raw, err := doInternalGET(ctx, hc, baseURL, internalAPIKey, q)
		if err != nil {
			return mcp.NewToolResultError(errJSON("upstream_failure", err.Error())), nil
		}
		return mcp.NewToolResultText(string(raw)), nil
	}
}

func makeGetDecisionOutcomesHandler(portfolioAPIURL, internalAPIKey string, hc *http.Client) mcpserver.ToolHandlerFunc {
	baseURL := strings.TrimRight(portfolioAPIURL, "/") + "/internal/agent-decisions/outcomes"
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args := req.GetArguments()

		limit := clampInt(intArg(args, "limit"), 20, 50)
		horizon := strArg(args, "horizon")
		if horizon == "" {
			horizon = "14d"
		}
		switch horizon {
		case "24h", "7d", "14d":
		default:
			return mcp.NewToolResultError(errJSON("bad_request", "horizon must be one of 24h|7d|14d")), nil
		}

		q := url.Values{}
		q.Set("limit", fmt.Sprint(limit))
		q.Set("horizon", horizon)
		if sym := strArg(args, "symbol"); sym != "" {
			q.Set("symbol", sym)
		}

		raw, err := doInternalGET(ctx, hc, baseURL, internalAPIKey, q)
		if err != nil {
			return mcp.NewToolResultError(errJSON("upstream_failure", err.Error())), nil
		}
		return mcp.NewToolResultText(string(raw)), nil
	}
}

// strArg extracts a string from an arguments map.
func strArg(args map[string]any, key string) string {
	v, _ := args[key].(string)
	return v
}

// intArg extracts an integer from an arguments map (handles float64 from JSON decode).
func intArg(args map[string]any, key string) int {
	switch v := args[key].(type) {
	case float64:
		return int(v)
	case int:
		return v
	case int64:
		return int(v)
	}
	return 0
}
