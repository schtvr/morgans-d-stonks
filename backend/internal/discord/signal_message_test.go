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
	fixed := time.Unix(0, 0).UTC()
	payload := signal.CryptoAlert{
		SchemaVersion: signal.CryptoSignalSchemaVersion,
		ID:            "btc_usd-19700101T000000Z",
		Type:          "crypto_price_move",
		Symbol:        "BTC-USD",
		CurrentPrice:  65000,
		DeltaPct:      1.25,
		ThresholdPct:  1.0,
		FiredAt:       fixed,
	}
	got, err := CryptoAlertWebhookContent(payload)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "crypto_price_move BTC-USD") {
		t.Fatalf("missing summary line: %s", got)
	}
	if !strings.Contains(got, "```json") {
		t.Fatalf("missing fenced json: %s", got)
	}
	if !strings.Contains(got, `"schemaVersion":"crypto_signal_v1"`) || !strings.Contains(got, `"id":"btc_usd-19700101T000000Z"`) {
		t.Fatalf("unexpected payload in fence: %s", got)
	}
}
