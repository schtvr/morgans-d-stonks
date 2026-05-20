package signal

import (
	"strings"
	"time"
)

// RuleCooldown returns the suppression window for a fired rule+symbol pair.
func RuleCooldown(rule Rule, fallback time.Duration) time.Duration {
	if d := strings.TrimSpace(rule.Cooldown); d != "" {
		if parsed, err := time.ParseDuration(d); err == nil && parsed > 0 {
			return parsed
		}
	}
	return fallback
}
