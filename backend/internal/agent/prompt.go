package agent

import (
	"crypto/sha256"
	"fmt"
	"os"
)

// Prompt holds the loaded system prompt and a stable version fingerprint.
// Version is the first 12 hex chars of sha256(content), recorded per-decision
// so prompt regressions are traceable.
type Prompt struct {
	Body    string
	Version string
}

// LoadPrompt reads the file at path and returns a Prompt. Called once at
// startup; callers should cache the result.
func LoadPrompt(path string) (*Prompt, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("agent: load prompt %q: %w", path, err)
	}
	h := sha256.Sum256(data)
	version := fmt.Sprintf("%x", h[:6]) // 12 hex chars (6 bytes)
	return &Prompt{
		Body:    string(data),
		Version: version,
	}, nil
}
