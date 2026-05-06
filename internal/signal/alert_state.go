package signal

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// AlertState tracks the last seen price and last alert time per symbol.
type AlertState struct {
	path    string
	mu      sync.Mutex
	Symbols map[string]AlertStateEntry `json:"symbols"`
}

// AlertStateEntry stores the baseline needed for threshold checks.
type AlertStateEntry struct {
	LastPrice   float64   `json:"lastPrice"`
	LastSeenAt  time.Time `json:"lastSeenAt"`
	LastAlertAt time.Time `json:"lastAlertAt,omitempty"`
}

// AlertDecision reports the outcome of one threshold evaluation.
type AlertDecision struct {
	PreviousPrice float64
	CurrentPrice  float64
	DeltaAmount   float64
	DeltaPct      float64
	Alert         bool
}

// NewAlertState loads state from disk, creating the directory if needed.
func NewAlertState(path string) (*AlertState, error) {
	if dir := filepath.Dir(path); dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, err
		}
	}
	s := &AlertState{path: path, Symbols: map[string]AlertStateEntry{}}
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return s, nil
		}
		return nil, err
	}
	if len(b) == 0 {
		return s, nil
	}
	if err := json.Unmarshal(b, &s.Symbols); err != nil {
		return nil, err
	}
	return s, nil
}

// Evaluate updates the baseline and determines whether an alert should fire.
func (s *AlertState) Evaluate(symbol string, currentPrice, thresholdPct float64, cooldown time.Duration, now time.Time) (AlertDecision, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	decision := AlertDecision{CurrentPrice: currentPrice}
	if symbol == "" || currentPrice <= 0 {
		return decision, nil
	}

	entry, ok := s.Symbols[symbol]
	if !ok || entry.LastPrice <= 0 {
		entry.LastPrice = currentPrice
		entry.LastSeenAt = now
		s.Symbols[symbol] = entry
		return decision, s.persistLocked()
	}

	decision.PreviousPrice = entry.LastPrice
	decision.DeltaAmount = currentPrice - entry.LastPrice
	if entry.LastPrice != 0 {
		decision.DeltaPct = (decision.DeltaAmount / entry.LastPrice) * 100
	}
	entry.LastPrice = currentPrice
	entry.LastSeenAt = now

	if thresholdPct < 0 {
		thresholdPct = -thresholdPct
	}
	move := decision.DeltaPct
	if move < 0 {
		move = -move
	}
	if move >= thresholdPct {
		if entry.LastAlertAt.IsZero() || now.Sub(entry.LastAlertAt) >= cooldown {
			decision.Alert = true
			entry.LastAlertAt = now
		}
	}

	s.Symbols[symbol] = entry
	return decision, s.persistLocked()
}

func (s *AlertState) persistLocked() error {
	b, err := json.MarshalIndent(s.Symbols, "", "  ")
	if err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return err
	}
	if err := os.Rename(tmp, s.path); err != nil {
		return fmt.Errorf("signal: persist state: %w", err)
	}
	return nil
}
