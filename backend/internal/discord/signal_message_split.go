package discord

import (
	"strings"
	"unicode/utf8"
)

const cryptoDiscordFenceOpen = "\n```json\n"
const cryptoDiscordFenceClose = "\n```"

// SplitOversizedCryptoAlertDiscord splits text produced by CryptoAlertWebhookContent into
// multiple Discord messages. When the layout matches summary + fenced JSON, it splits the
// inner JSON at top-level object commas so chunks rejoin into valid JSON. Otherwise it falls
// back to rune-sized pieces with a continuation prefix.
func SplitOversizedCryptoAlertDiscord(s string, maxRunes int) []string {
	if maxRunes < 1 {
		return nil
	}
	if utf8.RuneCountInString(s) <= maxRunes {
		return []string{s}
	}
	head, jsonBody, suffix, ok := parseCryptoAlertDiscordShell(s)
	if !ok {
		return splitDiscordTextRuneFallback(s, maxRunes)
	}
	rs := []rune(jsonBody)
	var out []string
	pos := 0
	first := true
	for pos < len(rs) {
		chunkHead := head
		if !first {
			chunkHead = "(continued)\n```json\n"
		}
		first = false
		overhead := utf8.RuneCountInString(chunkHead) + utf8.RuneCountInString(suffix)
		if overhead >= maxRunes {
			return splitDiscordTextRuneFallback(s, maxRunes)
		}
		budget := maxRunes - overhead
		rest := rs[pos:]
		if len(rest) <= budget {
			out = append(out, chunkHead+string(rest)+suffix)
			break
		}
		scan := min(len(rest), budget)
		split := lastTopLevelJSONObjectCommaEnd(rest, scan)
		if split <= 0 {
			split = scan
		}
		out = append(out, chunkHead+string(rest[:split])+suffix)
		pos += split
	}
	return out
}

func parseCryptoAlertDiscordShell(s string) (head, jsonBody, suffix string, ok bool) {
	idx := strings.Index(s, cryptoDiscordFenceOpen)
	if idx < 0 {
		return "", "", "", false
	}
	if !strings.HasSuffix(s, cryptoDiscordFenceClose) {
		return "", "", "", false
	}
	jsonStart := idx + len(cryptoDiscordFenceOpen)
	jsonEnd := len(s) - len(cryptoDiscordFenceClose)
	if jsonStart > jsonEnd {
		return "", "", "", false
	}
	return s[:jsonStart], s[jsonStart:jsonEnd], cryptoDiscordFenceClose, true
}

// lastTopLevelJSONObjectCommaEnd returns an exclusive end index into rest such that rest[:end]
// is at most limit runes and end falls after a comma at brace depth 1 (top-level object fields).
// Returns 0 if no suitable comma was found in rest[:limit].
func lastTopLevelJSONObjectCommaEnd(rest []rune, limit int) int {
	if limit > len(rest) {
		limit = len(rest)
	}
	if limit < 1 {
		return 0
	}
	depth := 0
	inStr := false
	esc := false
	lastEnd := 0
	for i := 0; i < limit; i++ {
		c := rest[i]
		if inStr {
			if esc {
				esc = false
				continue
			}
			if c == '\\' {
				esc = true
				continue
			}
			if c == '"' {
				inStr = false
			}
			continue
		}
		if c == '"' {
			inStr = true
			continue
		}
		switch c {
		case '{', '[':
			depth++
		case '}', ']':
			if depth > 0 {
				depth--
			}
		case ',':
			if depth == 1 {
				lastEnd = i + 1
			}
		}
	}
	return lastEnd
}

func splitDiscordTextRuneFallback(s string, maxRunes int) []string {
	const cont = "(continued)\n"
	contN := utf8.RuneCountInString(cont)
	r := []rune(s)
	var out []string
	i := 0
	for i < len(r) {
		budget := maxRunes
		if len(out) > 0 {
			budget = maxRunes - contN
			if budget < 1 {
				budget = 1
			}
		}
		end := i + min(budget, len(r)-i)
		chunk := string(r[i:end])
		if len(out) > 0 {
			chunk = cont + chunk
		}
		out = append(out, chunk)
		i = end
	}
	return out
}
