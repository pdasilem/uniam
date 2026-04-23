package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSetupClaudeCodeWritesManagedCLAUDE(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	prev := currentSetupOptions
	t.Cleanup(func() {
		currentSetupOptions = prev
	})
	currentSetupOptions = setupOptions{Context7: true}

	if _, err := setupClaudeCode("", false, false); err != nil {
		t.Fatalf("setupClaudeCode() error = %v", err)
	}

	path := filepath.Join(home, ".claude", "CLAUDE.md")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("os.ReadFile(%q) error = %v", path, err)
	}

	text := string(data)
	if !strings.Contains(text, "<!-- uniam:begin claude -->") || !strings.Contains(text, "<!-- uniam:end claude -->") {
		t.Fatalf("CLAUDE.md missing managed Uniam block in %q", path)
	}
	if !strings.Contains(text, "Use Context7 MCP") {
		t.Fatalf("CLAUDE.md missing optional MCP guidance in %q", path)
	}
}

func TestSetupCursorProjectWritesManagedAgents(t *testing.T) {
	repo := t.TempDir()
	t.Setenv("HOME", t.TempDir())

	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("os.Getwd() error = %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(wd)
	})
	if err := os.Chdir(repo); err != nil {
		t.Fatalf("os.Chdir() error = %v", err)
	}

	prev := currentSetupOptions
	t.Cleanup(func() {
		currentSetupOptions = prev
	})
	currentSetupOptions = setupOptions{Firecrawl: true}

	if _, err := setupCursor("", true, false); err != nil {
		t.Fatalf("setupCursor(project) error = %v", err)
	}

	path := filepath.Join(repo, "AGENTS.md")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("os.ReadFile(%q) error = %v", path, err)
	}

	text := string(data)
	if !strings.Contains(text, "<!-- uniam:begin agents -->") || !strings.Contains(text, "<!-- uniam:end agents -->") {
		t.Fatalf("AGENTS.md missing managed shared block in %q", path)
	}
	if !strings.Contains(text, "Use Firecrawl MCP") {
		t.Fatalf("AGENTS.md missing Firecrawl guidance in %q", path)
	}
}

func TestSetupGeminiCliWritesAgentsAndContextFileNames(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	prev := currentSetupOptions
	t.Cleanup(func() {
		currentSetupOptions = prev
	})

	if _, err := setupGeminiCli("", false, false); err != nil {
		t.Fatalf("setupGeminiCli() error = %v", err)
	}

	agentsPath := filepath.Join(home, ".gemini", "AGENTS.md")
	agentsData, err := os.ReadFile(agentsPath)
	if err != nil {
		t.Fatalf("os.ReadFile(%q) error = %v", agentsPath, err)
	}
	if !strings.Contains(string(agentsData), "<!-- uniam:begin gemini -->") {
		t.Fatalf("Gemini AGENTS.md missing managed block in %q", agentsPath)
	}

	settingsPath := filepath.Join(home, ".gemini", "settings.json")
	settingsData, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatalf("os.ReadFile(%q) error = %v", settingsPath, err)
	}

	var settings map[string]any
	if err := json.Unmarshal(settingsData, &settings); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	contextMap, ok := settings["context"].(map[string]any)
	if !ok {
		t.Fatalf("settings.context type = %T, want map[string]any", settings["context"])
	}
	names, ok := contextMap["fileName"].([]any)
	if !ok {
		t.Fatalf("settings.context.fileName type = %T, want []any", contextMap["fileName"])
	}

	var got []string
	for _, item := range names {
		if value, ok := item.(string); ok {
			got = append(got, value)
		}
	}
	if !containsString(got, "AGENTS.md") || !containsString(got, "GEMINI.md") {
		t.Fatalf("settings.context.fileName = %v, want both AGENTS.md and GEMINI.md", got)
	}
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}

	return false
}
