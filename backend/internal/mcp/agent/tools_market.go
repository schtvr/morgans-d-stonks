package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"

	"github.com/schtvr/morgans-d-stonks/internal/broker/coinbase"
	"github.com/schtvr/morgans-d-stonks/internal/portfolio"
)

// window → (lookback duration, Coinbase granularity string, human label)
var windowConfig = map[string]struct {
	lookback    time.Duration
	granularity string
}{
	"1h":  {time.Hour, "ONE_MINUTE"},
	"24h": {24 * time.Hour, "FIVE_MINUTE"},
	"7d":  {7 * 24 * time.Hour, "ONE_HOUR"},
	"30d": {30 * 24 * time.Hour, "SIX_HOUR"},
}

const mcpCandlesCap = 200 // LLM token budget cap (dashboard uses 480)

func registerMarketTools(s *mcpserver.MCPServer, cb *coinbase.Client) {
	s.AddTool(
		mcp.NewTool("get_market_candles",
			mcp.WithDescription("OHLCV candles for a Coinbase symbol over the requested window. Returns up to 200 points."),
			mcp.WithString("symbol",
				mcp.Required(),
				mcp.Description("Coinbase product_id, e.g. BTC-USD"),
			),
			mcp.WithString("window",
				mcp.Required(),
				mcp.Description("Lookback window: 1h | 24h | 7d | 30d"),
				mcp.Enum("1h", "24h", "7d", "30d"),
			),
		),
		makeGetMarketCandlesHandler(cb),
	)
}

func makeGetMarketCandlesHandler(cb *coinbase.Client) mcpserver.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args := req.GetArguments()
		symbol, _ := args["symbol"].(string)
		window, _ := args["window"].(string)

		if symbol == "" {
			return mcp.NewToolResultError(`{"error":{"code":"bad_request","message":"symbol is required"}}`), nil
		}
		cfg, ok := windowConfig[window]
		if !ok {
			return mcp.NewToolResultError(`{"error":{"code":"bad_request","message":"window must be one of 1h|24h|7d|30d"}}`), nil
		}

		now := time.Now().UTC()
		since := now.Add(-cfg.lookback)

		bars, err := cb.FetchProductCandles(ctx, symbol, since, now)
		if err != nil {
			return mcp.NewToolResultError(errJSON("upstream_failure", fmt.Sprintf("coinbase candles: %v", err))), nil
		}

		pts := make([]portfolio.MarketCandlePoint, 0, len(bars))
		for _, b := range bars {
			pts = append(pts, portfolio.MarketCandlePoint{
				AsOf:   b.Start,
				Open:   b.Open,
				High:   b.High,
				Low:    b.Low,
				Close:  b.Close,
				Volume: b.Volume,
			})
		}
		pts = portfolio.DownsampleMarketCandlePoints(pts, mcpCandlesCap)

		resp := struct {
			Symbol      string                        `json:"symbol"`
			Window      string                        `json:"window"`
			Granularity string                        `json:"granularity"`
			From        time.Time                     `json:"from"`
			To          time.Time                     `json:"to"`
			Points      []portfolio.MarketCandlePoint `json:"points"`
		}{
			Symbol:      symbol,
			Window:      window,
			Granularity: cfg.granularity,
			From:        since,
			To:          now,
			Points:      pts,
		}

		b, err := json.Marshal(resp)
		if err != nil {
			return mcp.NewToolResultError(errJSON("upstream_failure", "marshal response: "+err.Error())), nil
		}
		return mcp.NewToolResultText(string(b)), nil
	}
}

// errJSON returns a compact JSON error envelope string.
func errJSON(code, message string) string {
	b, _ := json.Marshal(map[string]any{
		"error": map[string]string{
			"code":    code,
			"message": message,
		},
	})
	return string(b)
}
