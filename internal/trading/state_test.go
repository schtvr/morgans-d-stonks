package trading

import "testing"

func TestTransition(t *testing.T) {
	next, err := Transition(OrderStatusNew, "accept")
	if err != nil || next != OrderStatusAccepted {
		t.Fatalf("unexpected next state: %s %v", next, err)
	}
	if _, err := Transition(OrderStatusFilled, "cancel"); err == nil {
		t.Fatal("expected invalid transition error")
	}
}
