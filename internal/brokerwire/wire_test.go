package brokerwire

import (
	"context"
	"testing"

	"github.com/schtvr/morgans-d-stonks/internal/broker"
)

func TestNewMock(t *testing.T) {
	b, err := New(broker.Config{Mode: "mock"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := b.Positions(context.Background()); err != nil {
		t.Fatal(err)
	}
}
