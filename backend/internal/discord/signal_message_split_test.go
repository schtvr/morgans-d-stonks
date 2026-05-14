package discord

import "testing"

func TestLastTopLevelJSONObjectCommaEnd(t *testing.T) {
	rest := []rune(`{"a":1,"b":{"x":2},"c":3}`)
	// Limit stops before last comma — should pick comma after "a":1
	got := lastTopLevelJSONObjectCommaEnd(rest, 10)
	want := 7 // `{"a":1,`
	if got != want {
		t.Fatalf("got %d want %d", got, want)
	}
	// Full scan: last top-level comma is after the nested object, before "c"
	got = lastTopLevelJSONObjectCommaEnd(rest, len(rest))
	if want := 19; got != want {
		t.Fatalf("full scan: got %d want %d", got, want)
	}
}
