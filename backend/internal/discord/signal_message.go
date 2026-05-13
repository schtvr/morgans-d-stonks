package discord

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/schtvr/morgans-d-stonks/internal/signal"
)

// SignalWebhookContent builds plain-text webhook content for a fired rule.
// mention should be the raw Discord ping substring (e.g. "<@123>") or empty.
func SignalWebhookContent(mention, symbol, ruleName string) string {
	body := "**" + symbol + "** | " + ruleName
	mention = strings.TrimSpace(mention)
	if mention == "" {
		return body
	}
	return mention + " " + body
}

// CryptoAlertWebhookContent builds a short human summary plus a fenced JSON block for OpenClaw/Discord.
func CryptoAlertWebhookContent(payload signal.CryptoAlert) (string, error) {
	b, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	summary := fmt.Sprintf(
		"%s %s price=%.8g delta=%.4g%% threshold=%.4g%%",
		payload.Type,
		payload.Symbol,
		payload.CurrentPrice,
		payload.DeltaPct,
		payload.ThresholdPct,
	)
	var sb strings.Builder
	sb.WriteString(summary)
	sb.WriteString("\n```json\n")
	sb.Write(b)
	sb.WriteString("\n```")
	return sb.String(), nil
}
