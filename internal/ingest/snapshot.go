package ingest

import (
	"encoding/json"
	"time"

	"github.com/schtvr/morgans-d-stonks/internal/broker"
	"github.com/schtvr/morgans-d-stonks/internal/portfolio"
)

// BuildSnapshot constructs the ingest payload from broker data.
func BuildSnapshot(takenAt time.Time, positions []broker.Position, summary *broker.AccountSummary) portfolio.IngestSnapshotRequest {
	if summary == nil {
		summary = &broker.AccountSummary{}
	}
	return portfolio.IngestSnapshotRequest{
		TakenAt:   takenAt.UTC().Truncate(time.Minute),
		Positions: positions,
		Summary:   *summary,
	}
}

// MarshalSnapshot JSON-encodes the snapshot for POST /internal/snapshots.
func MarshalSnapshot(s portfolio.IngestSnapshotRequest) ([]byte, error) {
	return json.Marshal(s)
}
