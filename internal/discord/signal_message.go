package discord

import (
	"encoding/json"
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

// CryptoAlertWebhookContent renders the machine-readable crypto alert payload.
func CryptoAlertWebhookContent(payload signal.CryptoAlert) (string, error) {
	b, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	return string(b), nil
}
