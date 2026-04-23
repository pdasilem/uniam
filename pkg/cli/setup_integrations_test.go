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
		"current web information",
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
		SearXNG:     true,
		BraveSearch: true,
		Firecrawl:   true,
	})
	for _, fragment := range []string{
		"exact text matches",
		"broader code discovery",
		"dependency compatibility details",
		"repository status, diffs, history",
		"configured SearXNG instance",
		"current web information",
		"structured web extraction",
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

func TestSetupCodexWritesSearXNGAndFirecrawlBlocks(t *testing.T) {
	target := t.TempDir()

	prev := currentSetupOptions
	t.Cleanup(func() {
		currentSetupOptions = prev
	})

	currentSetupOptions = setupOptions{
		SearXNG:         true,
		SearXNGURL:      "http://127.0.0.1:8080",
		Firecrawl:       true,
		FirecrawlAPIKey: "fc-test",
	}

	if _, err := setupCodex(target, false, false); err != nil {
		t.Fatalf("setupCodex() error = %v", err)
	}

	configPath := filepath.Join(target, "config.toml")
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("os.ReadFile() error = %v", err)
	}

	text := string(data)
	if !strings.Contains(text, "[mcp_servers.searxng]") || !strings.Contains(text, "command = \"npx\"") || !strings.Contains(text, "args = [\"-y\", \"mcp-searxng\"]") || !strings.Contains(text, "SEARXNG_URL = \"http://127.0.0.1:8080\"") {
		t.Fatal("expected SearXNG block to be written to Codex config")
	}
	if !strings.Contains(text, "[mcp_servers.firecrawl]") || !strings.Contains(text, "FIRECRAWL_API_KEY = \"fc-test\"") {
		t.Fatal("expected Firecrawl block to be written to Codex config")
	}
}
