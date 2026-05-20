package portfolio

import (
	"context"
	"time"
)

// Repository persists snapshots and sessions.
type Repository interface {
	RunMigrations(ctx context.Context) error

	UpsertSnapshot(ctx context.Context, takenAt time.Time, payload []byte) error
	LatestSnapshot(ctx context.Context) (takenAt time.Time, payload []byte, err error)
	ListSnapshotsSince(ctx context.Context, since time.Time, limit int) ([]SnapshotRecord, error)

	ListFollowedSymbols(ctx context.Context) ([]FollowedSymbol, error)
	UpsertFollowedSymbol(ctx context.Context, symbol, source string) error
	RemoveFollowedSymbol(ctx context.Context, symbol string) error
	FollowedSymbolsSeeded(ctx context.Context) (bool, error)
	MarkFollowedSymbolsSeeded(ctx context.Context, seededAt time.Time) error
	GetSignalSettings(ctx context.Context) (*SignalSettings, error)
	UpdateSignalSettings(ctx context.Context, req SignalSettingsRequest) error
	ListRecentAlerts(ctx context.Context, limit int) ([]RecentAlert, error)
	InsertRecentAlert(ctx context.Context, alert RecentAlert) error
	InsertLabSignalEvent(ctx context.Context, alert RecentAlert) (*LabSignalEvent, error)
	ListLabSignalEvents(ctx context.Context, filter LabSignalFilter) ([]LabSignalEvent, error)
	GetLabSignalEvent(ctx context.Context, id int64) (*LabSignalEvent, error)
	UpsertLabOpenClawRun(ctx context.Context, run LabOpenClawRun) error
	ListLabOpenClawRuns(ctx context.Context, filter LabRunFilter) ([]LabOpenClawRun, error)
	GetLabOpenClawRun(ctx context.Context, requestID string) (*LabOpenClawRun, error)
	InsertLabNote(ctx context.Context, note LabNoteRequest) (*LabNote, error)
	ListLabTelemetry(ctx context.Context, symbol, window string) ([]LabTelemetryPoint, error)
	GetLabControlState(ctx context.Context) (*LabControlState, error)
	UpdateLabControlState(ctx context.Context, control LabControlState) (*LabControlState, error)
	InsertSignalSettingsVersion(ctx context.Context, req SignalSettingsRequest, reason string) (*SignalSettingsVersion, error)
	ListSignalSettingsVersions(ctx context.Context, limit int) ([]SignalSettingsVersion, error)
	RevertSignalSettings(ctx context.Context, versionID int64) (*SignalSettings, error)
	CompactLabOpenClawPayloads(ctx context.Context, olderThan time.Time) error

	CreateSession(ctx context.Context, token, username string, expiresAt time.Time) error
	SessionUser(ctx context.Context, token string) (username string, err error)
	DeleteSession(ctx context.Context, token string) error

	// Agent decision CRUD.
	InsertAgentDecision(ctx context.Context, d AgentDecision) (*AgentDecision, error)
	GetAgentDecision(ctx context.Context, id int64) (*AgentDecision, error)
	GetAgentDecisionByIdempotencyKey(ctx context.Context, key string) (*AgentDecision, error)
	ListAgentDecisions(ctx context.Context, filter AgentDecisionFilter) ([]AgentDecision, error)
	CountDecisionsForSymbolSince(ctx context.Context, symbol string, since time.Time) (int, error)
	SumCostCentsForDay(ctx context.Context, day time.Time) (int64, error)
	ListAgentCostDaily(ctx context.Context, days int) ([]AgentCostPoint, error)

	// Agent decision outcome CRUD.
	InsertAgentDecisionOutcome(ctx context.Context, o AgentDecisionOutcome) (*AgentDecisionOutcome, error)
	ListUnscoredDecisionHorizons(ctx context.Context, now time.Time, limit int) ([]UnscoredHorizon, error)
	ListAgentDecisionOutcomes(ctx context.Context, filter AgentDecisionOutcomeFilter) ([]AgentDecisionOutcome, error)
	ListBenchmarkDaily(ctx context.Context, horizon string, days int) ([]AgentBenchmarkPoint, error)
}
