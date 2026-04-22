package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRenderUniamSkillContent_ConditionalIntegrationGuidance(t *testing.T) {
	without := renderUniamSkillContent(setupOptions{})
	for _, fragment := range []string{
		"exact text matches",
		"broader code discovery",
		"dependency compatibility details",
		"repository status, diffs, history",
		"current external web information",
	} {
		if strings.Contains(without, fragment) {
			t.Fatalf("expected %q guidance to be absent without optional MCPs", fragment)
		}
	}

	with := renderUniamSkillContent(setupOptions{
		Ripgrep:     true,
		CodeSearch:  true,
		Context7:    true,
		Git:         true,
		BraveSearch: true,
	})
	for _, fragment := range []string{
		"exact text matches",
		"broader code discovery",
		"dependency compatibility details",
		"repository status, diffs, history",
		"current external web information",
	} {
		if !strings.Contains(with, fragment) {
			t.Fatalf("expected %q guidance to be present with optional MCPs", fragment)
		}
	}
}

func TestSetupCodexUpdatesContext7KeyOnRepeatedSetup(t *testing.T) {
	target := t.TempDir()

	prev := currentSetupOptions
	t.Cleanup(func() {
		currentSetupOptions = prev
	})

	currentSetupOptions = setupOptions{
		Context7:       true,
		Context7APIKey: "ctx7sk-old",
	}

	if _, err := setupCodex(target, false, false); err != nil {
		t.Fatalf("setupCodex() first error = %v", err)
	}

	currentSetupOptions = setupOptions{
		Context7:       true,
		Context7APIKey: "ctx7sk-new",
	}

	if _, err := setupCodex(target, false, false); err != nil {
		t.Fatalf("setupCodex() second error = %v", err)
	}

	configPath := filepath.Join(target, "config.toml")
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("os.ReadFile() error = %v", err)
	}

	text := string(data)
	if !strings.Contains(text, "ctx7sk-new") {
		t.Fatal("expected updated Context7 API key to be written to Codex config")
	}
	if strings.Contains(text, "ctx7sk-old") {
		t.Fatal("expected old Context7 API key to be replaced in Codex config")
	}
}
