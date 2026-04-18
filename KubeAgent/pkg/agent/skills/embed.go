// Package skills owns the markdown files containing LLM-facing
// instructions ("Inferential Guides" in harness terminology). Files
// here are embedded into the binary and exposed to the rest of the
// agent via harness.NewSkillsFromFS(skills.FS).
//
// Why a dedicated package:
//   - The go:embed directive must live in the same Go package as the
//     files it pulls in.
//   - Keeping skills out of /pkg/agent/harness lets operators add new
//     skill files (drop a .md, rebuild) without touching framework
//     code.
package skills

import (
	"embed"
	"io/fs"
)

//go:embed *.md
var embedded embed.FS

// FS returns the embedded skill filesystem. Pass this to
// harness.NewSkillsFromFS to populate the runtime registry.
func FS() fs.FS { return embedded }
