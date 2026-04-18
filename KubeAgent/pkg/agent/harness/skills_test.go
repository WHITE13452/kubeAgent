package harness

import (
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"
)

// TestSkills_Register_GetRoundTrip is the simplest contract: what you
// put in is what you get out, byte-for-byte.
func TestSkills_Register_GetRoundTrip(t *testing.T) {
	s := NewSkills()
	s.Register("hello", "world")

	body, ok := s.Get("hello")
	if !ok {
		t.Fatal("expected skill to be present")
	}
	if body != "world" {
		t.Fatalf("expected %q, got %q", "world", body)
	}
}

// TestSkills_FromFS confirms the embed-based loader picks up *.md
// files at the root of the fs and ignores other extensions.
func TestSkills_FromFS(t *testing.T) {
	fsys := fstest.MapFS{
		"diagnose.md":  {Data: []byte("diagnose body")},
		"remediate.md": {Data: []byte("remediate body")},
		"README.txt":   {Data: []byte("ignored")}, // non-md must not register
	}
	s, err := NewSkillsFromFS(fsys)
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	want := map[string]string{
		"diagnose":  "diagnose body",
		"remediate": "remediate body",
	}
	for k, v := range want {
		got, ok := s.Get(k)
		if !ok {
			t.Fatalf("missing skill %q", k)
		}
		if got != v {
			t.Fatalf("skill %q: want %q got %q", k, v, got)
		}
	}
	if _, ok := s.Get("README"); ok {
		t.Fatal("non-md files must not be registered")
	}
}

// TestSkills_OverrideDirWins exercises the production override path:
// dropping a file at <dir>/<name>.md replaces the embedded version
// without code changes. This is the operator-facing contract.
func TestSkills_OverrideDirWins(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "diagnose.md"), []byte("OVERRIDDEN"), 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}

	s := NewSkills().WithOverrideDir(dir)
	s.Register("diagnose", "embedded")

	body, ok := s.Get("diagnose")
	if !ok {
		t.Fatal("expected skill present")
	}
	if body != "OVERRIDDEN" {
		t.Fatalf("override should win; got %q", body)
	}

	// A name without an override file falls back to the registered body.
	s.Register("other", "still-embedded")
	body, _ = s.Get("other")
	if body != "still-embedded" {
		t.Fatalf("missing override should fall back; got %q", body)
	}
}

// TestSkills_GetMissing returns ok=false. Callers rely on this to
// degrade gracefully (use a fallback prompt) rather than panicking.
func TestSkills_GetMissing(t *testing.T) {
	s := NewSkills()
	body, ok := s.Get("nonexistent")
	if ok {
		t.Fatal("expected ok=false for missing skill")
	}
	if body != "" {
		t.Fatalf("expected empty body for missing skill, got %q", body)
	}
}

// TestSkills_NamesSorted documents the lexicographic ordering, which
// downstream tooling (debug commands, docs generators) relies on for
// stable output.
func TestSkills_NamesSorted(t *testing.T) {
	s := NewSkills()
	s.Register("zeta", "")
	s.Register("alpha", "")
	s.Register("mu", "")

	names := s.Names()
	want := []string{"alpha", "mu", "zeta"}
	if len(names) != len(want) {
		t.Fatalf("expected %d names, got %d", len(want), len(names))
	}
	for i, n := range names {
		if n != want[i] {
			t.Fatalf("position %d: want %q got %q", i, want[i], n)
		}
	}
}
