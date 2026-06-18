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
	got, err := CryptoAlertWebhookContent("", payload)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "### BTC-USD") {
		t.Fatalf("missing symbol heading: %s", got)
	}
	if !strings.Contains(got, "**price:** 65000") || !strings.Contains(got, "**delta:** 1.25%") || !strings.Contains(got, "**threshold:** 1.00%") {
		t.Fatalf("missing human summary fields: %s", got)
	}
	if !strings.Contains(got, "```json") {
		t.Fatalf("missing fenced json: %s", got)
	}
	if !strings.Contains(got, `"schemaVersion":"crypto_signal_v1"`) || !strings.Contains(got, `"id":"btc_usd-19700101T000000Z"`) {
		t.Fatalf("unexpected payload in fence: %s", got)
	}
}

func TestCryptoAlertWebhookContent_botMention(t *testing.T) {
	t.Parallel()
	fixed := time.Unix(0, 0).UTC()
	payload := signal.CryptoAlert{
		SchemaVersion: signal.CryptoSignalSchemaVersion,
		ID:            "x",
		Type:          "crypto_price_move",
		Symbol:        "ETH-USD",
		CurrentPrice:  100,
		DeltaPct:      -2.5,
		ThresholdPct:  1.0,
		FiredAt:       fixed,
	}
	got, err := CryptoAlertWebhookContent("<@123456789>", payload)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(got, "<@123456789> ### ETH-USD") {
		t.Fatalf("want mention prefix, got %q", got)
	}
	if !strings.Contains(got, "**delta:** -2.50%") {
		t.Fatalf("delta percent: %s", got)
	}
}
