package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"uniam/internal/config"
)

func TestInspectIntegrationPromptStatusNotConfigured(t *testing.T) {
	ctx := &setupPromptContext{
		agent: "claude-code",
		targets: []setupPromptTarget{{
			configPath: filepath.Join(t.TempDir(), ".mcp.json"),
			format:     targetJSONMCPServers,
		}},
	}

	status := inspectIntegrationPromptStatus(ctx, optionalIntegration{key: "ripgrep", name: "ripgrep MCP"}, &config.Config{})
	if status.summary != "not configured" {
		t.Fatalf("summary = %q, want %q", status.summary, "not configured")
	}
}

func TestInspectIntegrationPromptStatusJSONAlreadyConfiguredMaskedKey(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, ".claude.json")
	writePromptJSONFixture(t, configPath, map[string]any{
		"mcpServers": map[string]any{
			"brave-search": map[string]any{
				"command": "npx",
				"args":    []string{"-y", "@brave/brave-search-mcp-server", "--transport", "stdio"},
				"env": map[string]any{
					"BRAVE_API_KEY": "BSA123456XYZ",
				},
			},
		},
	})

	cfg := &config.Config{}
	cfg.Integrations.BraveSearchAPIKey = stringPtr("BSA123456XYZ")

	ctx := &setupPromptContext{
		agent: "claude-code",
		targets: []setupPromptTarget{{
			configPath: configPath,
			format:     targetJSONMCPServers,
		}},
	}

	status := inspectIntegrationPromptStatus(ctx, optionalIntegration{key: "brave-search", name: "Brave Search MCP"}, cfg)
	if status.summary != "already configured" {
		t.Fatalf("summary = %q, want %q", status.summary, "already configured")
	}
	if len(status.details) == 0 || status.details[0] != "API key: BSA...XYZ" {
		t.Fatalf("details = %v, want masked API key detail", status.details)
	}
}

func TestInspectIntegrationPromptStatusCodexWillUpdate(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.toml")
	data := `
[mcp_servers.code-search]
command = "node"
args = ["/tmp/code-search/dist/index.js", "--allowed-workspace", "/tmp/other"]
`
	if err := os.WriteFile(configPath, []byte(data), 0o644); err != nil {
		t.Fatalf("os.WriteFile() error = %v", err)
	}

	cfg := &config.Config{}
	cfg.Integrations.CodeSearchPath = stringPtr("/tmp/code-search/dist/index.js")

	ctx := &setupPromptContext{
		agent: "codex",
		targets: []setupPromptTarget{{
			configPath: configPath,
			format:     targetCodexTOML,
		}},
	}

	status := inspectIntegrationPromptStatus(ctx, optionalIntegration{key: "code-search", name: "code-search MCP"}, cfg)
	if status.summary != "configured, will update" {
		t.Fatalf("summary = %q, want %q", status.summary, "configured, will update")
	}
	if len(status.details) < 2 {
		t.Fatalf("details = %v, want workspace details", status.details)
	}
	if !strings.Contains(status.details[0], "Workspace: /tmp/other") {
		t.Fatalf("details[0] = %q, want workspace detail", status.details[0])
	}
	if !strings.Contains(status.details[1], "Workspace path differs") {
		t.Fatalf("details[1] = %q, want differs detail", status.details[1])
	}
}

func TestInspectIntegrationPromptStatusOpenCodeAlreadyConfigured(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "opencode.json")
	writePromptJSONFixture(t, configPath, map[string]any{
		"mcp": map[string]any{
			"searxng": map[string]any{
				"type":    "local",
				"command": []string{"npx", "-y", "mcp-searxng"},
				"environment": map[string]any{
					"SEARXNG_URL": "http://localhost:8213",
				},
			},
		},
	})

	cfg := &config.Config{}
	cfg.Integrations.SearXNGURL = stringPtr("http://localhost:8213")

	ctx := &setupPromptContext{
		agent: "opencode",
		targets: []setupPromptTarget{{
			configPath: configPath,
			format:     targetOpenCodeMCP,
		}},
	}

	status := inspectIntegrationPromptStatus(ctx, optionalIntegration{key: "searxng", name: "SearXNG MCP"}, cfg)
	if status.summary != "already configured" {
		t.Fatalf("summary = %q, want %q", status.summary, "already configured")
	}
	if len(status.details) == 0 || status.details[0] != "URL: http://localhost:8213" {
		t.Fatalf("details = %v, want URL detail", status.details)
	}
}

func TestInspectIntegrationPromptStatusMultiTargetSummary(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "mcp_config.json")
	writePromptJSONFixture(t, configPath, map[string]any{
		"mcpServers": map[string]any{
			"git": map[string]any{
				"command": "uvx",
				"args":    []string{"mcp-server-git"},
			},
		},
	})

	ctx := &setupPromptContext{
		agent: "windsurf",
		targets: []setupPromptTarget{
			{configPath: configPath, format: targetJSONMCPServers},
			{configPath: filepath.Join(dir, "missing.json"), format: targetJSONMCPServers},
		},
	}

	status := inspectIntegrationPromptStatus(ctx, optionalIntegration{key: "git", name: "Git MCP"}, &config.Config{})
	if status.summary != "configured in 1/2 targets" {
		t.Fatalf("summary = %q, want %q", status.summary, "configured in 1/2 targets")
	}
}

func writePromptJSONFixture(t *testing.T, path string, value map[string]any) {
	t.Helper()

	data, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("os.WriteFile() error = %v", err)
	}
}
