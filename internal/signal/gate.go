package signal

import "math"

// GateInput is the deterministic input for health + significance filtering.
type GateInput struct {
	ProductID          string
	Return5mPct        float64
	RollingVol5mPct    float64
	SpreadBps          float64
	SpreadCapBps       float64
	QuoteVolume24h     float64
	MinQuoteVolume24h  float64
	CooldownActive     bool
	Restricted         bool
	PersistsTwoOfThree bool
	FloorPct           float64
}

// GateResult explains whether an event should be emitted and why.
type GateResult struct {
	Emit        bool
	ReasonFlags []string
}

// EvaluateGate runs health and significance gates for candidate alerts.
func EvaluateGate(in GateInput) GateResult {
	reasons := make([]string, 0, 4)
	if in.Restricted {
		return GateResult{Emit: false, ReasonFlags: []string{"restricted"}}
	}
	if in.CooldownActive {
		return GateResult{Emit: false, ReasonFlags: []string{"cooldown"}}
	}
	if in.SpreadCapBps > 0 && in.SpreadBps > in.SpreadCapBps {
		return GateResult{Emit: false, ReasonFlags: []string{"spread_too_wide"}}
	}
	if in.MinQuoteVolume24h > 0 && in.QuoteVolume24h < in.MinQuoteVolume24h {
		return GateResult{Emit: false, ReasonFlags: []string{"liquidity_too_low"}}
	}
	reasons = append(reasons, "health_ok")

	threshold := in.FloorPct
	volThresh := 2.5 * in.RollingVol5mPct
	if volThresh > threshold {
		threshold = volThresh
	}
	if math.Abs(in.Return5mPct) < threshold {
		return GateResult{Emit: false, ReasonFlags: append(reasons, "move_below_threshold")}
	}
	if !in.PersistsTwoOfThree {
		return GateResult{Emit: false, ReasonFlags: append(reasons, "not_persistent")}
	}
	reasons = append(reasons, "5m_breakout", "vol_expansion")
	if in.SpreadCapBps <= 0 || in.SpreadBps <= in.SpreadCapBps {
		reasons = append(reasons, "spread_ok")
	}
	return GateResult{Emit: true, ReasonFlags: reasons}
}
