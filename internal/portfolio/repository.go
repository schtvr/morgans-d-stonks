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

	ListFollowedSymbols(ctx context.Context) ([]FollowedSymbol, error)
	UpsertFollowedSymbol(ctx context.Context, symbol, source string) error
	RemoveFollowedSymbol(ctx context.Context, symbol string) error
	FollowedSymbolsSeeded(ctx context.Context) (bool, error)
	MarkFollowedSymbolsSeeded(ctx context.Context, seededAt time.Time) error
	GetSignalSettings(ctx context.Context) (*SignalSettings, error)
	UpdateSignalSettings(ctx context.Context, req SignalSettingsRequest) error
	ListRecentAlerts(ctx context.Context, limit int) ([]RecentAlert, error)
	InsertRecentAlert(ctx context.Context, alert RecentAlert) error

	CreateSession(ctx context.Context, token, username string, expiresAt time.Time) error
	SessionUser(ctx context.Context, token string) (username string, err error)
	DeleteSession(ctx context.Context, token string) error
}
