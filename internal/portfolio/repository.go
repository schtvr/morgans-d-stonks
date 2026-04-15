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

	CreateSession(ctx context.Context, token, username string, expiresAt time.Time) error
	SessionUser(ctx context.Context, token string) (username string, err error)
	DeleteSession(ctx context.Context, token string) error
}
