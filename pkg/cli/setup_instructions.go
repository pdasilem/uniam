package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	compactUniamLegacyPrefix = "## Uniam\n\nUse Uniam for cross-session memory.\n\nRequired:\n"
	verboseUniamLegacyPrefix = "## Uniam\n\nUse Uniam for cross-session memory.\n\nRequired workflow:\n"
)

func compactUniamSection(opts setupOptions) string {
	section := "## Uniam\n\nUse Uniam for cross-session memory.\n\n" +
		"Required:\n" +
		"- Retrieve before meaningful work.\n" +
		"- Checkpoint during long or decision-heavy work.\n" +
		"- Store again before finishing meaningful work.\n" +
		"- Curate stale or repetitive memory when needed.\n"
	section += integrationInstructionLines(opts)
	section += "- Never operate outside the current project or folder scope.\n"

	return section
}

func verboseUniamSection(opts setupOptions) string {
	section := "## Uniam\n\n" +
		"Use Uniam for cross-session memory.\n\n" +
		"Required workflow:\n" +
		"- Before meaningful work, retrieve with `uniam_context`, `uniam_search`, or `uniam_retrieve`.\n" +
		"- During long or decision-heavy work, checkpoint with `uniam_store`.\n" +
		"- Before finishing meaningful work, store a final note with `uniam_store`.\n" +
		"- Curate memory with `uniam_archive`, `uniam_supersede`, `uniam_update_note`, and `uniam_compact`.\n" +
		"- Use `uniam_explain_search` only when retrieval behavior needs debugging.\n"
	section = insertInstructionLines(section, "- Use `uniam_explain_search` only when retrieval behavior needs debugging.\n", integrationInstructionLines(opts))
	section += "\nCurrent scope is only the current project or folder. Cross-project access is not allowed.\n\n" +
		"Store decisions, bugs, root causes, constraints, and non-obvious context.\n" +
		"Do not store trivial edits, obvious code facts, secrets, or duplicates.\n"

	return section
}

func claudeManagedBlock(opts setupOptions) string {
	return "<!-- uniam:begin claude -->\n" +
		verboseUniamSection(opts) +
		"<!-- uniam:end claude -->\n"
}

func sharedAgentsManagedBlock(opts setupOptions) string {
	return "<!-- uniam:begin agents -->\n" +
		compactUniamSection(opts) +
		"<!-- uniam:end agents -->\n"
}

func codexManagedBlock(opts setupOptions) string {
	return "<!-- uniam:begin codex -->\n" +
		compactUniamSection(opts) +
		"<!-- uniam:end codex -->\n"
}

func geminiManagedBlock(opts setupOptions) string {
	return "<!-- uniam:begin gemini -->\n" +
		compactUniamSection(opts) +
		"<!-- uniam:end gemini -->\n"
}

func projectRootFile(name string) (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("failed to resolve current directory: %w", err)
	}

	return filepath.Join(cwd, name), nil
}

func claudeMemoryPath(project bool) (string, error) {
	if project {
		return projectRootFile("CLAUDE.md")
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to resolve home directory: %w", err)
	}

	return filepath.Join(home, ".claude", "CLAUDE.md"), nil
}

func geminiAgentsPath(project bool) (string, error) {
	if project {
		return projectRootFile("AGENTS.md")
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to resolve home directory: %w", err)
	}

	return filepath.Join(home, ".gemini", "AGENTS.md"), nil
}

func upsertManagedBlockWithLegacy(path string, marker string, block string, legacyPrefix string) error {
	existing, _ := os.ReadFile(path)
	text := string(existing)

	begin := "<!-- uniam:begin " + marker + " -->"
	if strings.Contains(text, begin) {
		return upsertManagedBlock(path, marker, block)
	}

	if legacyPrefix != "" {
		if replaced, ok := replaceLegacyUniamSection(text, legacyPrefix, block); ok {
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				return err
			}
			return os.WriteFile(path, []byte(replaced), 0o644)
		}
	}

	return upsertManagedBlock(path, marker, block)
}

func replaceLegacyUniamSection(text string, legacyPrefix string, replacement string) (string, bool) {
	start := strings.Index(text, legacyPrefix)
	if start == -1 {
		return "", false
	}

	searchFrom := start + len(legacyPrefix)
	nextHeading := strings.Index(text[searchFrom:], "\n## ")
	end := len(text)
	if nextHeading != -1 {
		end = searchFrom + nextHeading
	}

	if start > 0 && text[start-1] == '\n' && !strings.HasSuffix(replacement, "\n") {
		replacement += "\n"
	}

	return text[:start] + replacement + text[end:], true
}

func ensureGeminiContextFilenames(configPath string) error {
	root, err := readJSONMap(configPath)
	if err != nil {
		return err
	}

	contextMap, _ := root["context"].(map[string]any)
	if contextMap == nil {
		contextMap = make(map[string]any)
		root["context"] = contextMap
	}

	var names []string
	switch value := contextMap["fileName"].(type) {
	case string:
		if strings.TrimSpace(value) != "" {
			names = append(names, strings.TrimSpace(value))
		}
	case []any:
		for _, item := range value {
			name, ok := item.(string)
			if ok && strings.TrimSpace(name) != "" {
				names = append(names, strings.TrimSpace(name))
			}
		}
	}

	ensureName := func(name string) {
		for _, existing := range names {
			if existing == name {
				return
			}
		}
		names = append(names, name)
	}

	ensureName("AGENTS.md")
	ensureName("GEMINI.md")

	contextMap["fileName"] = names

	data, err := json.MarshalIndent(root, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal gemini settings: %w", err)
	}

	return os.WriteFile(configPath, data, 0o644)
}
