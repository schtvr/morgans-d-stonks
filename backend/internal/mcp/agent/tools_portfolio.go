package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"

	"github.com/schtvr/morgans-d-stonks/internal/portfolio"
)

func registerPortfolioTools(
	s *mcpserver.MCPServer,
	portfolioAPIURL, internalAPIKey string,
	ingestIntervalSec int,
	hc *http.Client,
) {
	s.AddTool(
		mcp.NewTool("get_holdings",
			mcp.WithDescription("All portfolio positions and account summary from the latest ingest snapshot."),
		),
		makeGetHoldingsHandler(portfolioAPIURL, internalAPIKey, ingestIntervalSec, hc),
	)

	s.AddTool(
		mcp.NewTool("get_position",
			mcp.WithDescription("One position from the latest ingest snapshot by symbol."),
			mcp.WithString("symbol",
				mcp.Required(),
				mcp.Description("Coinbase product_id, e.g. BTC-USD"),
			),
		),
		makeGetPositionHandler(portfolioAPIURL, internalAPIKey, ingestIntervalSec, hc),
	)

	s.AddTool(
		mcp.NewTool("get_correlated_symbols",
			mcp.WithDescription("Returns a deduped set of symbols correlated to the input: itself, BTC-USD, ETH-USD, and the top-3 positions by market value."),
			mcp.WithString("symbol",
				mcp.Required(),
				mcp.Description("Symbol to correlate from"),
			),
		),
		makeGetCorrelatedSymbolsHandler(portfolioAPIURL, internalAPIKey, ingestIntervalSec, hc),
	)
}

// fetchLatestSnapshot calls GET /internal/snapshot/latest and returns the parsed body.
func fetchLatestSnapshot(ctx context.Context, portfolioAPIURL, internalAPIKey string, hc *http.Client) (*portfolio.IngestSnapshotRequest, error) {
	url := strings.TrimRight(portfolioAPIURL, "/") + "/internal/snapshot/latest"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("X-Internal-Key", internalAPIKey)

	resp, err := hc.Do(req)
	if err != nil {
		return nil, fmt.Errorf("GET snapshot/latest: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read snapshot/latest body: %w", err)
	}
	if resp.StatusCode == http.StatusNotFound {
		return nil, nil // no snapshot yet
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GET snapshot/latest: status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var snap portfolio.IngestSnapshotRequest
	if err := json.Unmarshal(body, &snap); err != nil {
		return nil, fmt.Errorf("decode snapshot/latest: %w", err)
	}
	return &snap, nil
}

// snapshotStale returns true when the snapshot is older than ingestIntervalSec × 3.
func snapshotStale(takenAt time.Time, ingestIntervalSec int) bool {
	ageSec := int(time.Since(takenAt).Seconds())
	return ageSec > ingestIntervalSec*3
}

func makeGetHoldingsHandler(portfolioAPIURL, internalAPIKey string, ingestIntervalSec int, hc *http.Client) mcpserver.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		snap, err := fetchLatestSnapshot(ctx, portfolioAPIURL, internalAPIKey, hc)
		if err != nil {
			return mcp.NewToolResultError(errJSON("upstream_failure", err.Error())), nil
		}
		if snap == nil {
			return mcp.NewToolResultError(errJSON("not_found", "no snapshot available")), nil
		}

		ageSec := int(time.Since(snap.TakenAt).Seconds())
		if ageSec < 0 {
			ageSec = 0
		}

		resp := struct {
			Source         string      `json:"source"`
			Timestamp      time.Time   `json:"timestamp"`
			SnapshotAgeSec int         `json:"snapshotAgeSec"`
			Stale          bool        `json:"stale"`
			Positions      interface{} `json:"positions"`
			Summary        interface{} `json:"summary"`
			SchemaVersion  string      `json:"schemaVersion"`
		}{
			Source:         "ingest_snapshot",
			Timestamp:      snap.TakenAt,
			SnapshotAgeSec: ageSec,
			Stale:          snapshotStale(snap.TakenAt, ingestIntervalSec),
			Positions:      snap.Positions,
			Summary:        snap.Summary,
			SchemaVersion:  "holdings_v1",
		}

		b, err := json.Marshal(resp)
		if err != nil {
			return mcp.NewToolResultError(errJSON("upstream_failure", "marshal: "+err.Error())), nil
		}
		return mcp.NewToolResultText(string(b)), nil
	}
}

func makeGetPositionHandler(portfolioAPIURL, internalAPIKey string, ingestIntervalSec int, hc *http.Client) mcpserver.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args := req.GetArguments()
		symbol, _ := args["symbol"].(string)
		if symbol == "" {
			return mcp.NewToolResultError(errJSON("bad_request", "symbol is required")), nil
		}

		snap, err := fetchLatestSnapshot(ctx, portfolioAPIURL, internalAPIKey, hc)
		if err != nil {
			return mcp.NewToolResultError(errJSON("upstream_failure", err.Error())), nil
		}
		if snap == nil {
			return mcp.NewToolResultError(errJSON("not_found", "no snapshot available")), nil
		}

		sym := strings.ToUpper(symbol)
		for _, p := range snap.Positions {
			if strings.ToUpper(p.Symbol) == sym {
				resp := struct {
					Source        string      `json:"source"`
					Timestamp     time.Time   `json:"timestamp"`
					Position      interface{} `json:"position"`
					SchemaVersion string      `json:"schemaVersion"`
				}{
					Source:    "ingest_snapshot",
					Timestamp: snap.TakenAt,
					Position: map[string]interface{}{
						"symbol":       p.Symbol,
						"quantity":     p.Quantity,
						"avgCost":      p.AvgCost,
						"marketValue":  p.MarketValue,
						"unrealizedPL": p.UnrealizedPL,
					},
					SchemaVersion: "holdings_v1",
				}
				b, err := json.Marshal(resp)
				if err != nil {
					return mcp.NewToolResultError(errJSON("upstream_failure", "marshal: "+err.Error())), nil
				}
				return mcp.NewToolResultText(string(b)), nil
			}
		}

		return mcp.NewToolResultError(errJSON("not_found", fmt.Sprintf("symbol %q not found in snapshot", symbol))), nil
	}
}

func makeGetCorrelatedSymbolsHandler(portfolioAPIURL, internalAPIKey string, ingestIntervalSec int, hc *http.Client) mcpserver.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args := req.GetArguments()
		symbol, _ := args["symbol"].(string)
		if symbol == "" {
			return mcp.NewToolResultError(errJSON("bad_request", "symbol is required")), nil
		}

		snap, err := fetchLatestSnapshot(ctx, portfolioAPIURL, internalAPIKey, hc)
		if err != nil {
			return mcp.NewToolResultError(errJSON("upstream_failure", err.Error())), nil
		}

		// Seed list: input symbol first, then BTC-USD, ETH-USD.
		seed := []string{strings.ToUpper(symbol), "BTC-USD", "ETH-USD"}

		// Add top 3 positions by MarketValue from snapshot.
		if snap != nil {
			type mv struct {
				sym string
				val float64
			}
			ranked := make([]mv, 0, len(snap.Positions))
			for _, p := range snap.Positions {
				ranked = append(ranked, mv{sym: strings.ToUpper(p.Symbol), val: p.MarketValue})
			}
			sort.Slice(ranked, func(i, j int) bool { return ranked[i].val > ranked[j].val })
			added := 0
			for _, r := range ranked {
				if added >= 3 {
					break
				}
				seed = append(seed, r.sym)
				added++
			}
		}

		// Dedup while preserving order; cap at 6.
		seen := make(map[string]struct{}, 6)
		out := make([]string, 0, 6)
		for _, s := range seed {
			if _, ok := seen[s]; ok {
				continue
			}
			seen[s] = struct{}{}
			out = append(out, s)
			if len(out) == 6 {
				break
			}
		}

		resp := struct {
			Symbol     string   `json:"symbol"`
			Correlated []string `json:"correlated"`
		}{
			Symbol:     strings.ToUpper(symbol),
			Correlated: out,
		}
		b, _ := json.Marshal(resp)
		return mcp.NewToolResultText(string(b)), nil
	}
}
