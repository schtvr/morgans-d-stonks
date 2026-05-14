package portfolio

import (
	"encoding/json"
	"time"

	"github.com/schtvr/morgans-d-stonks/internal/broker"
)

// IngestSnapshotRequest is the JSON body for POST /internal/snapshots (ingest job).
type IngestSnapshotRequest struct {
	TakenAt   time.Time             `json:"takenAt"`
	Positions []broker.Position     `json:"positions"`
	Summary   broker.AccountSummary `json:"summary"`
}

// PositionsResponse is returned by GET /api/portfolio/positions.
type PositionsResponse struct {
	Positions []PositionView `json:"positions"`
	AsOf      *time.Time     `json:"asOf,omitempty"`
}

// PositionView is a dashboard row for the positions table.
type PositionView struct {
	Symbol      string  `json:"symbol"`
	Quantity    float64 `json:"quantity"`
	AvgCost     float64 `json:"avgCost"`
	LastPrice   float64 `json:"lastPrice"`
	MarketValue float64 `json:"marketValue"`
	DayPL       float64 `json:"dayPL"`
	TotalPL     float64 `json:"totalPL"`
	Currency    string  `json:"currency"`
}

// SummaryResponse is returned by GET /api/portfolio/summary.
type SummaryResponse struct {
	AccountID      string    `json:"accountId"`
	NetLiquidation float64   `json:"netLiquidation"`
	TotalCash      float64   `json:"totalCash"`
	BuyingPower    float64   `json:"buyingPower"`
	Currency       string    `json:"currency"`
	AsOf           time.Time `json:"asOf"`
}

// LoginRequest is POST /api/auth/login.
type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// LoginResponse returns a bearer token for non-browser API clients when applicable.
// Same-origin browser logins omit token and rely on the HttpOnly session cookie only.
type LoginResponse struct {
	Token string `json:"token,omitempty"`
}

// FollowedSymbol is a crypto asset the user has chosen to watch.
type FollowedSymbol struct {
	Symbol    string    `json:"symbol"`
	Source    string    `json:"source,omitempty"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// FollowedSymbolRequest is the payload for add/remove operations.
type FollowedSymbolRequest struct {
	Symbol string `json:"symbol"`
}

// FollowedSymbolsResponse lists the watched assets.
type FollowedSymbolsResponse struct {
	Symbols []FollowedSymbol `json:"symbols"`
}

// SignalSettings stores the alert thresholds used by the crypto signals loop.
type SignalSettings struct {
	MoveThresholdPct float64   `json:"moveThresholdPct"`
	Cooldown         string    `json:"cooldown"`
	UpdatedAt        time.Time `json:"updatedAt"`
}

// SignalSettingsRequest updates the persisted signal settings.
type SignalSettingsRequest struct {
	MoveThresholdPct float64 `json:"moveThresholdPct"`
	Cooldown         string  `json:"cooldown"`
}

// RecentAlert is a persisted alert that fired for a followed crypto symbol.
type RecentAlert struct {
	ID              int64     `json:"id"`
	Type            string    `json:"type"`
	Symbol          string    `json:"symbol"`
	ProductID       string    `json:"productId,omitempty"`
	Source          string    `json:"source,omitempty"`
	CurrentPrice    float64   `json:"currentPrice"`
	PreviousPrice   *float64  `json:"previousPrice,omitempty"`
	DeltaAmount     *float64  `json:"deltaAmount,omitempty"`
	DeltaPct        float64   `json:"deltaPct"`
	ThresholdPct    float64   `json:"thresholdPct"`
	Quantity        *float64  `json:"quantity,omitempty"`
	AvgCost         *float64  `json:"avgCost,omitempty"`
	CostBasis       *float64  `json:"costBasis,omitempty"`
	UnrealizedPL    *float64  `json:"unrealizedPl,omitempty"`
	UnrealizedPLPct *float64  `json:"unrealizedPlPct,omitempty"`
	FiredAt         time.Time `json:"firedAt"`
	CreatedAt       time.Time `json:"createdAt"`
	// PayloadJSON is the exact compact JSON body persisted and sent to Discord (crypto_signal_v1).
	PayloadJSON json.RawMessage `json:"payloadJson,omitempty"`
}

// RecentAlertsResponse lists the most recent fired alerts.
type RecentAlertsResponse struct {
	Alerts []RecentAlert `json:"alerts"`
}

// LabSignalEvent is the durable event record backing The Lab timeline.
type LabSignalEvent struct {
	ID            int64           `json:"id"`
	Type          string          `json:"type"`
	Symbol        string          `json:"symbol"`
	ProductID     string          `json:"productId,omitempty"`
	Source        string          `json:"source,omitempty"`
	CurrentPrice  float64         `json:"currentPrice"`
	PreviousPrice *float64        `json:"previousPrice,omitempty"`
	DeltaAmount   *float64        `json:"deltaAmount,omitempty"`
	DeltaPct      float64         `json:"deltaPct"`
	ThresholdPct  float64         `json:"thresholdPct"`
	FiredAt       time.Time       `json:"firedAt"`
	PayloadJSON   json.RawMessage `json:"payloadJson,omitempty"`
	DiscordStatus string          `json:"discordStatus"`
	CreatedAt     time.Time       `json:"createdAt"`
	Run           *LabOpenClawRun `json:"run,omitempty"`
}

type LabSignalFilter struct {
	Limit  int
	Symbol string
	From   *time.Time
	To     *time.Time
}

type LabSignalsResponse struct {
	Signals []LabSignalEvent `json:"signals"`
}

type LabOpenClawRun struct {
	RequestID      string          `json:"requestId"`
	SignalID       int64           `json:"signalId"`
	Status         string          `json:"status"`
	Attempts       int             `json:"attempts"`
	Analysis       string          `json:"analysis,omitempty"`
	Recommendation string          `json:"recommendation,omitempty"`
	Confidence     *float64        `json:"confidence,omitempty"`
	ToolNames      []string        `json:"toolNames"`
	ErrorText      string          `json:"errorText,omitempty"`
	RequestHash    string          `json:"requestHash,omitempty"`
	ResponseHash   string          `json:"responseHash,omitempty"`
	RequestJSON    json.RawMessage `json:"requestJson,omitempty"`
	ResponseJSON   json.RawMessage `json:"responseJson,omitempty"`
	StartedAt      *time.Time      `json:"startedAt,omitempty"`
	CompletedAt    *time.Time      `json:"completedAt,omitempty"`
	CreatedAt      time.Time       `json:"createdAt"`
	UpdatedAt      time.Time       `json:"updatedAt"`
}

type LabRunFilter struct {
	Limit    int
	Status   string
	SignalID int64
}

type LabRunsResponse struct {
	Runs []LabOpenClawRun `json:"runs"`
}

type LabControlState struct {
	OpenClawPaused bool      `json:"openclawPaused"`
	CircuitOpen    bool      `json:"circuitOpen"`
	CircuitReason  string    `json:"circuitReason,omitempty"`
	UpdatedAt      time.Time `json:"updatedAt"`
}

type LabOverviewResponse struct {
	Control       LabControlState  `json:"control"`
	RecentSignals []LabSignalEvent `json:"recentSignals"`
	RecentRuns    []LabOpenClawRun `json:"recentRuns"`
}

type LabOperationResponse struct {
	Control LabControlState `json:"control"`
	Message string          `json:"message"`
}

type LabNote struct {
	ID        int64     `json:"id"`
	SignalID  *int64    `json:"signalId,omitempty"`
	RequestID string    `json:"requestId,omitempty"`
	Body      string    `json:"body"`
	CreatedAt time.Time `json:"createdAt"`
}

type LabNoteRequest struct {
	SignalID  *int64 `json:"signalId,omitempty"`
	RequestID string `json:"requestId,omitempty"`
	Body      string `json:"body"`
}

type LabTelemetryPoint struct {
	Bucket       time.Time `json:"bucket"`
	Symbol       string    `json:"symbol"`
	CurrentPrice float64   `json:"currentPrice"`
	DeltaPct     float64   `json:"deltaPct"`
	ThresholdPct float64   `json:"thresholdPct"`
	SignalCount  int       `json:"signalCount"`
}

type LabTelemetryResponse struct {
	Points []LabTelemetryPoint `json:"points"`
}

type SignalSettingsVersion struct {
	ID               int64     `json:"id"`
	MoveThresholdPct float64   `json:"moveThresholdPct"`
	Cooldown         string    `json:"cooldown"`
	Reason           string    `json:"reason,omitempty"`
	CreatedAt        time.Time `json:"createdAt"`
}

type SignalSettingsHistoryResponse struct {
	Versions []SignalSettingsVersion `json:"versions"`
}

type SignalSettingsRevertRequest struct {
	VersionID int64 `json:"versionId"`
}

// MapIngestToViews converts ingest snapshot positions into API views.
func MapIngestToViews(req *IngestSnapshotRequest) PositionsResponse {
	views := make([]PositionView, 0, len(req.Positions))
	for _, p := range req.Positions {
		last := 0.0
		if p.Quantity != 0 {
			last = p.MarketValue / p.Quantity
		}
		views = append(views, PositionView{
			Symbol:      p.Symbol,
			Quantity:    p.Quantity,
			AvgCost:     p.AvgCost,
			LastPrice:   last,
			MarketValue: p.MarketValue,
			DayPL:       0,
			TotalPL:     p.UnrealizedPL,
			Currency:    p.Currency,
		})
	}
	t := req.TakenAt
	return PositionsResponse{
		Positions: views,
		AsOf:      &t,
	}
}

// MapSummary maps broker summary to API response.
func MapSummary(s *broker.AccountSummary, asOf time.Time) SummaryResponse {
	if s == nil {
		return SummaryResponse{AsOf: asOf}
	}
	return SummaryResponse{
		AccountID:      s.AccountID,
		NetLiquidation: s.NetLiquidation,
		TotalCash:      s.TotalCash,
		BuyingPower:    s.BuyingPower,
		Currency:       s.Currency,
		AsOf:           asOf,
	}
}
