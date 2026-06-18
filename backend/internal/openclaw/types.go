// Package openclaw is DEPRECATED as of 2026-05-18.
// The in-process Agent (internal/agent) replaces external OpenClaw integration.
// These status constants are retained only because the lab_openclaw_runs table
// still contains historical rows. Do not enqueue new rows.
package openclaw

const (
	StatusQueued    = "queued"
	StatusRetrying  = "retrying"
	StatusSkipped   = "skipped"
	StatusCompleted = "completed"
	StatusFailed    = "failed"
)
