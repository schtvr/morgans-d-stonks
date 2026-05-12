package signal

import "testing"

func TestEvaluateGate_HealthFailures(t *testing.T) {
	t.Parallel()
	cases := []GateInput{
		{Restricted: true, FloorPct: 1, Return5mPct: 3, PersistsTwoOfThree: true},
		{CooldownActive: true, FloorPct: 1, Return5mPct: 3, PersistsTwoOfThree: true},
		{SpreadBps: 30, SpreadCapBps: 10, FloorPct: 1, Return5mPct: 3, PersistsTwoOfThree: true},
		{QuoteVolume24h: 10, MinQuoteVolume24h: 100, FloorPct: 1, Return5mPct: 3, PersistsTwoOfThree: true},
	}
	for i, in := range cases {
		res := EvaluateGate(in)
		if res.Emit {
			t.Fatalf("case %d should not emit: %+v", i, res)
		}
	}
}

func TestEvaluateGate_SignificanceAndPersistence(t *testing.T) {
	t.Parallel()

	res := EvaluateGate(GateInput{FloorPct: 1.5, Return5mPct: 1.2, RollingVol5mPct: 0.2, PersistsTwoOfThree: true})
	if res.Emit {
		t.Fatalf("expected no emit for low move: %+v", res)
	}

	res = EvaluateGate(GateInput{FloorPct: 1.0, Return5mPct: 3.0, RollingVol5mPct: 1.5, PersistsTwoOfThree: false})
	if res.Emit {
		t.Fatalf("expected no emit for non-persistent move: %+v", res)
	}

	res = EvaluateGate(GateInput{FloorPct: 1.0, Return5mPct: 4.0, RollingVol5mPct: 1.0, PersistsTwoOfThree: true, SpreadBps: 8, SpreadCapBps: 10})
	if !res.Emit {
		t.Fatalf("expected emit: %+v", res)
	}
}
