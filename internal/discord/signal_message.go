package discord

import "strings"

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
