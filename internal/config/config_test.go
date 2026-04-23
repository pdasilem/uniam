package config

import (
	"path/filepath"
	"testing"
)

func TestGetUniamHome(t *testing.T) {
	// Test default
	home := GetUniamHome()
	if home == "" {
		t.Error("GetUniamHome() should not return empty string")
	}

	// Test with environment variable
	t.Setenv("UNIAM_HOME", "/test/uniam")

	home = GetUniamHome()
	if home != "/test/uniam" {
		t.Errorf("GetUniamHome() = %q, want %q", home, "/test/uniam")
	}
}

func TestLoadConfig(t *testing.T) {
	// Test with non-existent file (should return defaults)
	cfg, err := LoadConfig("/nonexistent/config.yaml")
	if err != nil {
		t.Errorf("LoadConfig() error = %v, want nil", err)
	}

	if cfg == nil {
		t.Fatal("LoadConfig() returned nil config")
	}

	//nolint:goconst
	if cfg.Embedding.Provider != "ollama" {
		t.Errorf("LoadConfig() default provider = %q, want %q", cfg.Embedding.Provider, "ollama")
	}
}

func TestGetDefaultConfigTemplate(t *testing.T) {
	template := GetDefaultConfigTemplate()
	if template == "" {
		t.Error("GetDefaultConfigTemplate() should not return empty string")
	}

	if len(template) < 100 {
		t.Error("GetDefaultConfigTemplate() should return substantial template")
	}
}

func TestSaveConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	context7Key := "ctx7sk-test"
	braveKey := "brv-test"
	searxngURL := "http://127.0.0.1:8080"
	firecrawlKey := "fc-test"
	codeSearchPath := "/tmp/code-search/dist/index.js"
	cfg := &Config{
		Embedding: EmbeddingConfig{
			Provider: "ollama",
			Model:    "test-model",
		},
		Integrations: IntegrationsConfig{
			RipgrepEnabled:     true,
			CodeSearchEnabled:  true,
			CodeSearchPath:     &codeSearchPath,
			Context7Enabled:    true,
			Context7APIKey:     &context7Key,
			GitEnabled:         true,
			SearXNGEnabled:     true,
			SearXNGURL:         &searxngURL,
			BraveSearchEnabled: true,
			BraveSearchAPIKey:  &braveKey,
			FirecrawlEnabled:   true,
			FirecrawlAPIKey:    &firecrawlKey,
		},
	}

	err := SaveConfig(configPath, cfg)
	if err != nil {
		t.Errorf("SaveConfig() error = %v", err)
	}

	// Verify it can be loaded back
	loaded, err := LoadConfig(configPath)
	if err != nil {
		t.Errorf("LoadConfig() after SaveConfig error = %v", err)
	}

	if loaded.Embedding.Model != "test-model" {
		t.Errorf("LoadConfig() Model = %q, want %q", loaded.Embedding.Model, "test-model")
	}

	if loaded.Integrations.Context7APIKey == nil || *loaded.Integrations.Context7APIKey != context7Key {
		t.Errorf("LoadConfig() Context7APIKey = %v, want %q", loaded.Integrations.Context7APIKey, context7Key)
	}

	if !loaded.Integrations.RipgrepEnabled {
		t.Error("LoadConfig() RipgrepEnabled = false, want true")
	}

	if !loaded.Integrations.CodeSearchEnabled {
		t.Error("LoadConfig() CodeSearchEnabled = false, want true")
	}

	if loaded.Integrations.CodeSearchPath == nil || *loaded.Integrations.CodeSearchPath != codeSearchPath {
		t.Errorf("LoadConfig() CodeSearchPath = %v, want %q", loaded.Integrations.CodeSearchPath, codeSearchPath)
	}

	if !loaded.Integrations.Context7Enabled {
		t.Error("LoadConfig() Context7Enabled = false, want true")
	}

	if !loaded.Integrations.GitEnabled {
		t.Error("LoadConfig() GitEnabled = false, want true")
	}

	if !loaded.Integrations.BraveSearchEnabled {
		t.Error("LoadConfig() BraveSearchEnabled = false, want true")
	}

	if loaded.Integrations.BraveSearchAPIKey == nil || *loaded.Integrations.BraveSearchAPIKey != braveKey {
		t.Errorf("LoadConfig() BraveSearchAPIKey = %v, want %q", loaded.Integrations.BraveSearchAPIKey, braveKey)
	}

	if !loaded.Integrations.SearXNGEnabled {
		t.Error("LoadConfig() SearXNGEnabled = false, want true")
	}

	if loaded.Integrations.SearXNGURL == nil || *loaded.Integrations.SearXNGURL != searxngURL {
		t.Errorf("LoadConfig() SearXNGURL = %v, want %q", loaded.Integrations.SearXNGURL, searxngURL)
	}

	if !loaded.Integrations.FirecrawlEnabled {
		t.Error("LoadConfig() FirecrawlEnabled = false, want true")
	}

	if loaded.Integrations.FirecrawlAPIKey == nil || *loaded.Integrations.FirecrawlAPIKey != firecrawlKey {
		t.Errorf("LoadConfig() FirecrawlAPIKey = %v, want %q", loaded.Integrations.FirecrawlAPIKey, firecrawlKey)
	}
}
