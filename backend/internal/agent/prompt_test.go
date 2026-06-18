package agent

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadPrompt_Deterministic(t *testing.T) {
	t.Parallel()
	f := filepath.Join(t.TempDir(), "prompt.md")
	if err := os.WriteFile(f, []byte("# Test prompt\n\nHello world."), 0o644); err != nil {
		t.Fatal(err)
	}
	p1, err := LoadPrompt(f)
	if err != nil {
		t.Fatalf("first load: %v", err)
	}
	p2, err := LoadPrompt(f)
	if err != nil {
		t.Fatalf("second load: %v", err)
	}
	if p1.Version != p2.Version {
		t.Errorf("same file yielded different versions: %q vs %q", p1.Version, p2.Version)
	}
	if len(p1.Version) != 12 {
		t.Errorf("version should be 12 hex chars, got %d: %q", len(p1.Version), p1.Version)
	}
}

func TestLoadPrompt_DifferentContent(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	f1 := filepath.Join(dir, "a.md")
	f2 := filepath.Join(dir, "b.md")
	if err := os.WriteFile(f1, []byte("content A"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(f2, []byte("content B"), 0o644); err != nil {
		t.Fatal(err)
	}
	p1, _ := LoadPrompt(f1)
	p2, _ := LoadPrompt(f2)
	if p1.Version == p2.Version {
		t.Error("different content should yield different versions")
	}
}

func TestLoadPrompt_MissingFile(t *testing.T) {
	t.Parallel()
	_, err := LoadPrompt("/nonexistent/path/agent-prompt.md")
	if err == nil {
		t.Error("expected error for missing file")
	}
}
