package skills

import "embed"

// skillsFS embeds all default skill markdown files.
// These are copied from .claude/skills/ at build time.
//
//go:embed skills/*.md
var skillsFS embed.FS
