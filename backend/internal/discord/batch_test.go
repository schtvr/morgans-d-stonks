package discord

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestWebhookChunks_empty(t *testing.T) {
	if got := WebhookChunks(nil, "\n", 100); got != nil {
		t.Fatalf("got %#v", got)
	}
	if got := WebhookChunks([]string{}, "\n", 100); got != nil {
		t.Fatalf("got %#v", got)
	}
}

func TestWebhookChunks_packsWholeSignals(t *testing.T) {
	// Five 10-rune payloads; sep "|" (1 rune). Max 45 runes: four fit (10+1+10+1+10+1+10=43), fifth starts new chunk.
	const unit = "0123456789"
	segs := []string{unit, unit, unit, unit, unit}
	ch := WebhookChunks(segs, "|", 45)
	if len(ch) != 2 {
		t.Fatalf("want 2 chunks, got %d: %#v", len(ch), ch)
	}
	if ch[0].AlertsApplied != 4 || ch[1].AlertsApplied != 1 {
		t.Fatalf("unexpected AlertsApplied: %#v", ch)
	}
	if ch[0].Content != strings.Join(segs[:4], "|") {
		t.Fatalf("chunk0: %q", ch[0].Content)
	}
	if ch[1].Content != segs[4] {
		t.Fatalf("chunk1: %q", ch[1].Content)
	}
}

func TestWebhookChunks_singleFits(t *testing.T) {
	ch := WebhookChunks([]string{"a", "b", "c"}, "---", 10)
	if len(ch) != 1 {
		t.Fatalf("want 1 chunk got %d: %#v", len(ch), ch)
	}
	if ch[0].Content != "a---b---c" {
		t.Fatalf("got %q", ch[0].Content)
	}
	if ch[0].AlertsApplied != 3 {
		t.Fatalf("AlertsApplied=%d", ch[0].AlertsApplied)
	}
}

func TestWebhookChunks_splitsHugeSegmentAtJSON(t *testing.T) {
	// Multi-line summary + fence + JSON; maxRunes must exceed head+suffix overhead (~90 runes)
	// or splitting falls back to raw runes and shell parsing breaks.
	const maxRunes = 120
	s := "### BTC-USD\n**price:** 1   |    **delta:** 1.00   |   **threshold:** 1.00%\n```json\n" +
		`{"a":1,"b":2,"c":3,"d":4,"e":5,"f":6,"g":7,"h":8}` + "\n```"
	ch := SplitOversizedCryptoAlertDiscord(s, maxRunes)
	if len(ch) < 2 {
		t.Fatalf("want split into multiple parts, got %d: %#v", len(ch), ch)
	}
	for _, c := range ch {
		if utf8.RuneCountInString(c) > maxRunes {
			t.Fatalf("chunk over limit: %d runes", utf8.RuneCountInString(c))
		}
	}
	// Reassemble inner JSON from chunks (strip markdown shells) and parse.
	combined, ok := rejoinSplitCryptoDiscordJSON(ch)
	if !ok {
		t.Fatal("could not rejoin chunks")
	}
	if combined != `{"a":1,"b":2,"c":3,"d":4,"e":5,"f":6,"g":7,"h":8}` {
		t.Fatalf("rejoined JSON: %q", combined)
	}
}

// rejoinSplitCryptoDiscordJSON concatenates inner JSON fragments from split crypto Discord chunks.
func rejoinSplitCryptoDiscordJSON(chunks []string) (string, bool) {
	var b strings.Builder
	for _, c := range chunks {
		_, body, _, ok := parseCryptoAlertDiscordShell(c)
		if !ok {
			return "", false
		}
		b.WriteString(body)
	}
	return b.String(), true
}
