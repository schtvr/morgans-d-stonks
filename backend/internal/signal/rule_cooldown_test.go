package signal

import (
	"testing"
	"time"
)

func TestRuleCooldown(t *testing.T) {
	fallback := time.Hour
	if got := RuleCooldown(Rule{}, fallback); got != fallback {
		t.Fatalf("empty: got %v", got)
	}
	if got := RuleCooldown(Rule{Cooldown: "24h"}, fallback); got != 24*time.Hour {
		t.Fatalf("24h: got %v", got)
	}
	if got := RuleCooldown(Rule{Cooldown: "bogus"}, fallback); got != fallback {
		t.Fatalf("invalid: got %v", got)
	}
}
