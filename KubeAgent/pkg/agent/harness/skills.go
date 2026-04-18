package harness

import (
	"fmt"
	"io/fs"
	"os"
	"path"
	"strings"
	"sync"
)

// Skills is the Inferential Guide registry. It owns the LLM-facing
// instructions ("how to diagnose a CrashLoopBackOff", "how to write a
// safe remediation plan") that used to live as inline string constants
// scattered across agents.
//
// Why this exists:
//   - Inline prompt strings are invisible to version control diffs once
//     they grow past a few lines, and impossible to A/B test.
//   - Skills live in pkg/agent/skills/*.md, are embedded into the
//     binary at build time, and are addressable by name from any agent.
//   - Operators can override an embedded skill by providing a file at
//     runtime via OverrideDir, without recompiling.
type Skills struct {
	mu          sync.RWMutex
	registry    map[string]string
	overrideDir string
}

// SkillsFS is the embedded filesystem for the default skill set.
// It is populated by NewDefaultSkills using go:embed at the package
// that owns the skill files (pkg/agent/skills). Keeping the embed
// directive at the consumer side (skills_embed.go below) avoids
// pulling skill files into this generic registry file.
type SkillsFS = fs.FS

// NewSkills builds an empty registry. Callers normally use
// NewSkillsFromFS to seed defaults.
func NewSkills() *Skills {
	return &Skills{registry: make(map[string]string)}
}

// NewSkillsFromFS loads every *.md file at the root of fsys (recursive)
// and registers it under its filename (without extension). Subdirectory
// entries become "<dir>/<name>" keys.
func NewSkillsFromFS(fsys SkillsFS) (*Skills, error) {
	s := NewSkills()
	if fsys == nil {
		return s, nil
	}
	err := fs.WalkDir(fsys, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if !strings.HasSuffix(d.Name(), ".md") {
			return nil
		}
		data, rerr := fs.ReadFile(fsys, p)
		if rerr != nil {
			return fmt.Errorf("read skill %s: %w", p, rerr)
		}
		key := strings.TrimSuffix(p, path.Ext(p))
		// Normalise leading "./".
		key = strings.TrimPrefix(key, "./")
		s.registry[key] = string(data)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return s, nil
}

// WithOverrideDir lets operators drop a file at <dir>/<name>.md and have
// it transparently replace the embedded version. Useful for tweaking
// prompts in production without a rebuild. Pass empty string to disable.
func (s *Skills) WithOverrideDir(dir string) *Skills {
	s.mu.Lock()
	s.overrideDir = dir
	s.mu.Unlock()
	return s
}

// Register adds (or replaces) a skill at runtime. Mainly useful in
// tests, or for plugins that ship their own prompts.
func (s *Skills) Register(name, body string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.registry[name] = body
}

// Get returns the skill body for the given name. The lookup order is:
//   1. OverrideDir on disk (if configured and file exists).
//   2. Registered/embedded body.
// When neither yields a result, ok=false and body is "".
//
// Get returns the body verbatim. Callers that want to interpolate
// variables should pass the result through their own template engine;
// keeping templating outside this registry avoids leaking a particular
// template syntax into every skill file.
func (s *Skills) Get(name string) (string, bool) {
	s.mu.RLock()
	override := s.overrideDir
	body, ok := s.registry[name]
	s.mu.RUnlock()

	if override != "" {
		if disk, found := readOverride(override, name); found {
			return disk, true
		}
	}
	return body, ok
}

// MustGet is the convenience companion. It returns an empty string for
// missing skills, which is intentional: a missing skill should degrade
// the LLM prompt gracefully, not panic the agent.
func (s *Skills) MustGet(name string) string {
	body, _ := s.Get(name)
	return body
}

// Names returns all registered skill names, sorted lexicographically.
// Useful for debug commands ("which skills are loaded?").
func (s *Skills) Names() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]string, 0, len(s.registry))
	for k := range s.registry {
		out = append(out, k)
	}
	// Tiny helper sort instead of importing sort just for one call.
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && out[j-1] > out[j]; j-- {
			out[j-1], out[j] = out[j], out[j-1]
		}
	}
	return out
}

// readOverride attempts to read <dir>/<name>.md from disk. Failures
// (missing file, permission error) silently fall through to the
// embedded version — overrides are advisory, not required.
func readOverride(dir, name string) (string, bool) {
	if dir == "" || name == "" {
		return "", false
	}
	full := path.Join(dir, name+".md")
	data, err := os.ReadFile(full)
	if err != nil {
		return "", false
	}
	return string(data), true
}
