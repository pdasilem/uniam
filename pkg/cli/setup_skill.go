package cli

import (
	_ "embed"
	"os"
	"path/filepath"
)

const (
	ripgrepInstructionLine     = "- Use ripgrep MCP for exact text matches, literals, identifiers, config keys, and regex-based narrowing.\n"
	codeSearchInstructionLine  = "- Use code-search MCP for broader code discovery, symbol relationships, and cross-file navigation.\n"
	context7InstructionLine    = "- Use Context7 MCP for up-to-date library and framework documentation, current package versions, and dependency compatibility details.\n"
	gitInstructionLine         = "- Use Git MCP for structured repository status, diffs, history, and branch inspection.\n"
	searxngInstructionLine     = "- Use SearXNG MCP for web search through the configured SearXNG instance.\n"
	braveSearchInstructionLine = "- Use Brave Search MCP for current web information.\n"
	firecrawlInstructionLine   = "- Use Firecrawl MCP for page fetch, scraping, crawling, and structured web extraction.\n"
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
