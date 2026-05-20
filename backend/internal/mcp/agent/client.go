package agent

import (
	"context"
	"encoding/json"
	"fmt"

	mcpclient "github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/mcp"
)

// MCPClient is the interface used by the agent provider (Task 2) to invoke MCP tools.
// Each agent worker holds its own independent MCPClient instance.
type MCPClient interface {
	CallTool(ctx context.Context, name string, input json.RawMessage) (json.RawMessage, error)
}

// mcpClientAdapter wraps mark3labs/mcp-go *client.Client to satisfy MCPClient.
type mcpClientAdapter struct {
	c *mcpclient.Client
}

// CallTool calls the named tool with the given JSON input and returns the first text
// content item of the result as raw JSON, or an error envelope if the tool reports
// an error.
func (a *mcpClientAdapter) CallTool(ctx context.Context, name string, input json.RawMessage) (json.RawMessage, error) {
	var args any
	if len(input) > 0 {
		if err := json.Unmarshal(input, &args); err != nil {
			return nil, fmt.Errorf("mcp CallTool %s: unmarshal input: %w", name, err)
		}
	}

	result, err := a.c.CallTool(ctx, mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name:      name,
			Arguments: args,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("mcp CallTool %s: %w", name, err)
	}

	for _, c := range result.Content {
		if tc, ok := c.(mcp.TextContent); ok {
			return json.RawMessage(tc.Text), nil
		}
	}
	// No text content — return null JSON rather than error so callers can distinguish.
	return json.RawMessage("null"), nil
}
