package signal

import "testing"

func TestSignalEventJSONTags(t *testing.T) {
	_ = SignalEvent{RuleID: "r", Symbol: "AAPL"}
}
