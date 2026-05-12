package postgres

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMigrationContainsFollowedSymbolsTables(t *testing.T) {
	b, err := os.ReadFile(filepath.Join("migrations", "002_followed_symbols.sql"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"CREATE TABLE IF NOT EXISTS followed_symbols", "followed_symbol_state"} {
		if !strings.Contains(string(b), want) {
			t.Fatalf("missing %q in migration", want)
		}
	}
}

func TestMigrationContainsSignalSettingsTable(t *testing.T) {
	b, err := os.ReadFile(filepath.Join("migrations", "003_signal_settings.sql"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"CREATE TABLE IF NOT EXISTS signal_settings", "move_threshold_pct", "cooldown"} {
		if !strings.Contains(string(b), want) {
			t.Fatalf("missing %q in migration", want)
		}
	}
}

func TestMigrationContainsRecentAlertsTable(t *testing.T) {
	b, err := os.ReadFile(filepath.Join("migrations", "004_recent_alerts.sql"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"CREATE TABLE IF NOT EXISTS recent_alerts", "delta_pct", "threshold_pct"} {
		if !strings.Contains(string(b), want) {
			t.Fatalf("missing %q in migration", want)
		}
	}
}
