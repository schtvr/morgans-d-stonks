package mock

import (
	"context"
	"testing"
)

func TestMockBroker(t *testing.T) {
	b := New()
	ctx := context.Background()
	if _, err := b.Positions(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := b.AccountSummary(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := b.Quotes(ctx, []string{"AAPL"}); err != nil {
		t.Fatal(err)
	}
	open, err := b.IsMarketOpen(ctx)
	if err != nil || !open {
		t.Fatalf("expected open true, got %v %v", open, err)
	}
}
