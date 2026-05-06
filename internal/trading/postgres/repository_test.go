package postgres

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMigrationContainsCoreTables(t *testing.T) {
	b, err := os.ReadFile(filepath.Join("migrations", "001_init.sql"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"CREATE TABLE IF NOT EXISTS orders", "order_events", "fills", "reconciliation", "trading_prevent_mutation"} {
		if !strings.Contains(string(b), want) {
			t.Fatalf("missing %q in migration", want)
		}
	}
}
