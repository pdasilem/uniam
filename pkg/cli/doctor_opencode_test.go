package cli

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewOpenCodePaths(t *testing.T) {
	t.Parallel()

	paths := newOpenCodePaths("/tmp/home")

	if got, want := paths.ConfigPath, filepath.Join("/tmp/home", ".config", "opencode", "opencode.json"); got != want {
		t.Fatalf("ConfigPath = %q, want %q", got, want)
	}

	if got, want := paths.SkillPath, filepath.Join("/tmp/home", ".config", "opencode", "skills", "uniam", "SKILL.md"); got != want {
		t.Fatalf("SkillPath = %q, want %q", got, want)
	}

	if got, want := paths.InstructionsPath, filepath.Join("/tmp/home", ".config", "opencode", "uniam-instructions.md"); got != want {
		t.Fatalf("InstructionsPath = %q, want %q", got, want)
	}

	if got, want := paths.PluginPath, filepath.Join("/tmp/home", ".config", "opencode", "plugins", "uniam.js"); got != want {
		t.Fatalf("PluginPath = %q, want %q", got, want)
	}
}

func TestVerifyOpenCodeMCPConfig(t *testing.T) {
	t.Parallel()

	t.Run("accepts configured uniam mcp", func(t *testing.T) {
		t.Parallel()

		configPath := writeTempOpenCodeConfig(t, `{"mcp":{"uniam":{"type":"local","command":["uniam","mcp"]}}}`)
		if err := verifyOpenCodeMCPConfig(configPath); err != nil {
			t.Fatalf("verifyOpenCodeMCPConfig() error = %v", err)
		}
	})

	t.Run("rejects missing uniam mcp entry", func(t *testing.T) {
		t.Parallel()

		configPath := writeTempOpenCodeConfig(t, `{"mcp":{"other":{"type":"local"}}}`)
		if err := verifyOpenCodeMCPConfig(configPath); err == nil {
			t.Fatal("verifyOpenCodeMCPConfig() error = nil, want missing mcp.uniam error")
		}
	})

	t.Run("rejects invalid json", func(t *testing.T) {
		t.Parallel()

		configPath := writeTempOpenCodeConfig(t, `{`)
		if err := verifyOpenCodeMCPConfig(configPath); err == nil {
			t.Fatal("verifyOpenCodeMCPConfig() error = nil, want invalid json error")
		}
	})
}

func writeTempOpenCodeConfig(t *testing.T, contents string) string {
	t.Helper()

	dir := t.TempDir()
	path := filepath.Join(dir, "opencode.json")
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatalf("os.WriteFile() error = %v", err)
	}

	return path
}
