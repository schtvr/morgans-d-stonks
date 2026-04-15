package signal

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Dedup tracks cooldowns per rule+symbol on disk.
type Dedup struct {
	path  string
	mu    sync.Mutex
	state map[string]time.Time
}

// NewDedup loads state from path (if present).
func NewDedup(path string) (*Dedup, error) {
	if dir := filepath.Dir(path); dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, err
		}
	}
	d := &Dedup{path: path, state: map[string]time.Time{}}
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return d, nil
		}
		return nil, err
	}
	if len(b) == 0 {
		return d, nil
	}
	if err := json.Unmarshal(b, &d.state); err != nil {
		return nil, err
	}
	return d, nil
}

func (d *Dedup) key(ruleID, symbol string) string {
	return ruleID + "|" + symbol
}

// ShouldFire returns whether enough time passed since last fire for this key.
func (d *Dedup) ShouldFire(ruleID, symbol string, cooldown time.Duration, now time.Time) bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	k := d.key(ruleID, symbol)
	if last, ok := d.state[k]; ok {
		if now.Sub(last) < cooldown {
			return false
		}
	}
	d.state[k] = now
	_ = d.persistLocked()
	return true
}

func (d *Dedup) persistLocked() error {
	b, err := json.MarshalIndent(d.state, "", "  ")
	if err != nil {
		return err
	}
	tmp := d.path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, d.path)
}
