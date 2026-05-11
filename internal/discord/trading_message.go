package discord

import (
	"fmt"
	"strings"

	"github.com/schtvr/morgans-d-stonks/internal/trading"
)

// TradingOutcomeWebhookContent renders a compact trade outcome message.
func TradingOutcomeWebhookContent(order trading.Order, decision trading.RiskDecision) string {
	status := strings.ToUpper(string(order.Status))
	msg := fmt.Sprintf("trade_outcome status=%s symbol=%s side=%s qty=%g id=%s", status, order.Symbol, order.Side, order.Quantity, order.ID)
	if len(decision.ReasonCodes) > 0 {
		msg += " reasons=" + strings.Join(decision.ReasonCodes, ",")
	}
	return msg
}
