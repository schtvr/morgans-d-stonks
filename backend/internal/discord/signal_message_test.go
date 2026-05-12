package discord

import (
	"strings"
	"testing"
	"time"

	"github.com/schtvr/morgans-d-stonks/internal/signal"
)

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

func TestCryptoAlertWebhookContent(t *testing.T) {
	t.Parallel()
	payload := signal.CryptoAlert{
		Type:         "crypto_alert",
		Symbol:       "BTC-USD",
		CurrentPrice: 65000,
		DeltaPct:     1.25,
		ThresholdPct: 1.0,
		FiredAt:      time.Unix(0, 0).UTC(),
	}
	got, err := CryptoAlertWebhookContent(payload)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, `"type":"crypto_alert"`) || !strings.Contains(got, `"symbol":"BTC-USD"`) {
		t.Fatalf("unexpected payload: %s", got)
	}
}
