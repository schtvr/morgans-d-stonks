package ibkr

import (
	"time"

	"github.com/schtvr/morgans-d-stonks/internal/broker"
)

// cpPosition is a subset of Client Portal portfolio position JSON.
type cpPosition struct {
	ContractID  int64   `json:"conid"`
	Symbol        string  `json:"contractDesc"`
	Position      float64 `json:"position"`
	AvgCost       float64 `json:"avgCost"`
	MarketValue   float64 `json:"mktValue"`
	Unrealized    float64 `json:"unrealizedPnl"`
	Realized      float64 `json:"realizedPnl"`
	Currency      string  `json:"currency"`
}

func mapPositions(rows []cpPosition) []broker.Position {
	out := make([]broker.Position, 0, len(rows))
	now := time.Now().UTC()
	for _, r := range rows {
		out = append(out, broker.Position{
			Symbol:       r.Symbol,
			ConID:        r.ContractID,
			Quantity:     r.Position,
			AvgCost:      r.AvgCost,
			MarketValue:  r.MarketValue,
			UnrealizedPL: r.Unrealized,
			RealizedPL:   r.Realized,
			Currency:     r.Currency,
			UpdatedAt:    now,
		})
	}
	return out
}
