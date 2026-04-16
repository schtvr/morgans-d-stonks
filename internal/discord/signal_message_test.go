package discord

import "testing"

func TestSignalWebhookContent(t *testing.T) {
	t.Parallel()
	if got := SignalWebhookContent("", "AAPL", "5% Price Drop"); got != "**AAPL** | 5% Price Drop" {
		t.Fatalf("no mention: got %q", got)
	}
	if got := SignalWebhookContent("<@123>", "MSFT", "Large"); got != "<@123> **MSFT** | Large" {
		t.Fatalf("with mention: got %q", got)
	}
	if got := SignalWebhookContent("  <@456>  ", "X", "Y"); got != "<@456> **X** | Y" {
		t.Fatalf("trim mention: got %q", got)
	}
}
