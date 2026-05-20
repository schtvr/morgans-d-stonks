package agent

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	mcpclient "github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"

	"github.com/schtvr/morgans-d-stonks/internal/broker/coinbase"
)

const defaultIngestIntervalSec = 600

// NewInProcessServer creates an independent mcp-go MCPServer with all 7 RO tools
// registered, and returns an initialised MCPClient that speaks to it in-process.
//
// Call this once per agent worker at startup — each worker gets its own server+client
// pair. mark3labs/mcp-go stdio sessions are single-caller; sharing across goroutines
// would corrupt request framing.
//
// ingestIntervalSec controls the staleness threshold (snapshot age > interval×3).
// Pass ≤0 to read from the INGEST_INTERVAL env var (Go duration; default 10m).
func NewInProcessServer(
	cb *coinbase.Client,
	portfolioAPIURL, internalAPIKey string,
	httpClient *http.Client,
	ingestIntervalSec int,
) (*mcpserver.MCPServer, MCPClient, error) {
	if ingestIntervalSec <= 0 {
		ingestIntervalSec = loadIngestIntervalSec()
	}
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 15 * time.Second}
	}

	s := mcpserver.NewMCPServer("portfolio-agent-mcp", "1.0.0")

	registerMarketTools(s, cb)
	registerPortfolioTools(s, portfolioAPIURL, internalAPIKey, ingestIntervalSec, httpClient)
	registerHistoryTools(s, portfolioAPIURL, internalAPIKey, httpClient)

	c, err := mcpclient.NewInProcessClient(s)
	if err != nil {
		return nil, nil, fmt.Errorf("mcp NewInProcessServer: create client: %w", err)
	}

	ctx := context.Background()
	if _, err := c.Initialize(ctx, mcp.InitializeRequest{
		Params: mcp.InitializeParams{
			ProtocolVersion: mcp.LATEST_PROTOCOL_VERSION,
			ClientInfo: mcp.Implementation{
				Name:    "agent-worker",
				Version: "1.0.0",
			},
		},
	}); err != nil {
		return nil, nil, fmt.Errorf("mcp NewInProcessServer: initialize: %w", err)
	}

	return s, &mcpClientAdapter{c: c}, nil
}

// loadIngestIntervalSec reads INGEST_INTERVAL from env as a Go duration and
// converts to seconds. Defaults to defaultIngestIntervalSec on any parse error.
func loadIngestIntervalSec() int {
	v := strings.TrimSpace(os.Getenv("INGEST_INTERVAL"))
	if v == "" {
		return defaultIngestIntervalSec
	}
	// Try plain integer seconds first (legacy).
	if sec, err := strconv.Atoi(v); err == nil && sec > 0 {
		return sec
	}
	// Try Go duration string (e.g. "5m", "600s").
	d, err := time.ParseDuration(v)
	if err != nil || d <= 0 {
		return defaultIngestIntervalSec
	}
	sec := int(d.Seconds())
	if sec <= 0 {
		return defaultIngestIntervalSec
	}
	return sec
}
