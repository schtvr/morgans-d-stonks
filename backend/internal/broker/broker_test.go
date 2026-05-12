package broker

import "testing"

func TestDomainTypes(t *testing.T) {
	_ = Position{Symbol: "X"}
	_ = Quote{Symbol: "X"}
	_ = AccountSummary{AccountID: "Y"}
}
