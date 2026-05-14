package discord

import (
	"strings"
	"unicode/utf8"
)

// WebhookContentMaxRunes is Discord's limit for webhook message content.
const WebhookContentMaxRunes = 2000

// WebhookChunk is one POST body to Discord plus how many source alerts complete in this chunk.
type WebhookChunk struct {
	Content       string
	AlertsApplied int
}

// WebhookChunks packs whole alert payloads into Discord-sized chunks.
// Each element of segments must be one full signal body (e.g. from CryptoAlertWebhookContent).
// It joins multiple signals with betweenAlerts only when they fit in one message; otherwise it
// starts a new message (e.g. 4 signals then 1). A single signal that exceeds maxRunes is split
// on JSON structure via SplitOversizedCryptoAlertDiscord.
func WebhookChunks(segments []string, betweenAlerts string, maxRunes int) []WebhookChunk {
	if len(segments) == 0 || maxRunes < 1 {
		return nil
	}
	sepRunes := utf8.RuneCountInString(betweenAlerts)

	var out []WebhookChunk
	var batch []string
	batchRunes := 0

	flushBatch := func() {
		if len(batch) == 0 {
			return
		}
		out = append(out, WebhookChunk{
			Content:       strings.Join(batch, betweenAlerts),
			AlertsApplied: len(batch),
		})
		batch = batch[:0]
		batchRunes = 0
	}

	for _, seg := range segments {
		if utf8.RuneCountInString(seg) > maxRunes {
			flushBatch()
			parts := SplitOversizedCryptoAlertDiscord(seg, maxRunes)
			for i, p := range parts {
				applied := 0
				if i == len(parts)-1 {
					applied = 1
				}
				out = append(out, WebhookChunk{Content: p, AlertsApplied: applied})
			}
			continue
		}

		add := utf8.RuneCountInString(seg)
		if len(batch) > 0 {
			add += sepRunes
		}
		if len(batch) > 0 && batchRunes+add > maxRunes {
			flushBatch()
			add = utf8.RuneCountInString(seg)
		}
		batch = append(batch, seg)
		batchRunes += add
	}
	flushBatch()
	return out
}
