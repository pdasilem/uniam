package cli

import (
	_ "embed"
	"os"
	"path/filepath"
)

const (
	ripgrepInstructionLine     = "- If ripgrep MCP is installed, use it for exact text matches, literals, identifiers, config keys, and regex-based narrowing.\n"
	codeSearchInstructionLine  = "- If code-search MCP is installed, use it for broader code discovery, symbol relationships, and cross-file navigation.\n"
	context7InstructionLine    = "- If Context7 MCP is installed, use it for up-to-date library and framework documentation, current package versions, and dependency compatibility details.\n"
	gitInstructionLine         = "- If Git MCP is installed, use it for structured repository status, diffs, history, and branch inspection.\n"
	braveSearchInstructionLine = "- If Brave Search MCP is installed, use it for current external web information.\n"
)

//go:embed skills/uniam/SKILL.md
var skillContent []byte

// installSkill installs the Uniam SKILL.md into an agent's skills directory.
// agentHome: path to the agent's config directory (e.g. ~/.claude, ~/.cursor, ~/.codex).
// Returns true if skill was installed, false if already present.
func installSkill(agentHome string) bool {
	skillDir := filepath.Join(agentHome, "skills", "uniam")
	skillPath := filepath.Join(skillDir, "SKILL.md")

	if err := os.MkdirAll(skillDir, 0755); err != nil {
		return false
	}

	content := renderUniamSkillContent(currentSetupOptions)
	if err := os.WriteFile(skillPath, []byte(content), 0644); err != nil {
		return false
	}

	return true
}

func renderUniamSkillContent(opts setupOptions) string {
	content := string(skillContent)
	anchor := "- Use `uniam_explain_search` only when retrieval behavior needs debugging.\n"
	content = insertInstructionLines(content, anchor, integrationInstructionLines(opts))

	return content
}

// uninstallSkill removes the Uniam skill from an agent's skills directory.
// Returns true if skill was removed, false if not found.
func uninstallSkill(agentHome string) bool {
	skillDir := filepath.Join(agentHome, "skills", "uniam")

	info, err := os.Stat(skillDir)
	if err != nil {
		return false
	}

	if info.IsDir() {
		if err := os.RemoveAll(skillDir); err != nil {
			return false
		}
	} else {
		// Symlink
		if err := os.Remove(skillDir); err != nil {
			return false
		}
	}

	// Remove the parent skills/ dir if now empty.
	skillsDir := filepath.Join(agentHome, "skills")

	entries, err := os.ReadDir(skillsDir)

	if err == nil && len(entries) == 0 {
		_ = os.Remove(skillsDir)
	}

	return true
}
