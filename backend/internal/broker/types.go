package broker

import "time"

// Position is the canonical internal position model.
type Position struct {
	Symbol       string    `json:"symbol"`
	ConID        int64     `json:"conId"`
	Quantity     float64   `json:"quantity"`
	AvgCost      float64   `json:"avgCost"`
	MarketValue  float64   `json:"marketValue"`
	UnrealizedPL float64   `json:"unrealizedPL"`
	RealizedPL   float64   `json:"realizedPL"`
	Currency     string    `json:"currency"`
	UpdatedAt    time.Time `json:"updatedAt"`
}

// AccountSummary is account-level metrics.
type AccountSummary struct {
	AccountID      string    `json:"accountId"`
	NetLiquidation float64   `json:"netLiquidation"`
	TotalCash      float64   `json:"totalCash"`
	BuyingPower    float64   `json:"buyingPower"`
	Currency       string    `json:"currency"`
	UpdatedAt      time.Time `json:"updatedAt"`
}

// Quote is a last/bid/ask snapshot for a symbol.
type Quote struct {
	Symbol    string    `json:"symbol"`
	ConID     int64     `json:"conId"`
	Last      float64   `json:"last"`
	Bid       float64   `json:"bid"`
	Ask       float64   `json:"ask"`
	Volume    int64     `json:"volume"`
	UpdatedAt time.Time `json:"updatedAt"`
}
