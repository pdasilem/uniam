package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"uniam/internal/buildinfo"
	"uniam/internal/config"
	"uniam/internal/core"
	"uniam/internal/redaction"
	"uniam/internal/update"

	"github.com/spf13/cobra"
)

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Check uniam health and capabilities",
	//nolint:revive
	Run: func(cmd *cobra.Command, args []string) {
		ok := true
		pass := func(label, detail string) {
			fmt.Printf("  \u2713 %-28s %s\n", label, detail)
		}
		fail := func(label, detail string) {
			fmt.Printf("  \u2717 %-28s %s\n", label, detail)

			ok = false
		}
		warn := func(label, detail string) {
			fmt.Printf("  ! %-28s %s\n", label, detail)
		}

		home := config.GetUniamHome()
		fmt.Printf("\nUniam home: %s\n\n", home)

		exePath, exeErr := os.Executable()

		// --- Filesystem ---
		fmt.Println("Filesystem:")

		if info, err := os.Stat(home); err != nil || !info.IsDir() {
			fail("uniam home", "directory missing — run `uniam init`")
		} else {
			pass("uniam home", home)
		}

		dbPath := filepath.Join(home, "index.db")
		if _, err := os.Stat(dbPath); err != nil {
			fail("index.db", "missing — run `uniam init`")
		} else {
			pass("index.db", dbPath)
		}

		shelvesDir := filepath.Join(home, "shelves")
		if _, err := os.Stat(shelvesDir); err != nil {
			fail("shelves/", "missing — run `uniam init`")
		} else {
			pass("shelves/", shelvesDir)
		}

		configPath := filepath.Join(home, "config.yaml")
		if _, err := os.Stat(configPath); err != nil {
			warn("config.yaml", "not found, using defaults")
		} else {
			pass("config.yaml", configPath)
		}

		ignorePath := filepath.Join(home, ".uniamignore")
		if _, err := os.Stat(ignorePath); err != nil {
			warn(".uniamignore", "not found (optional)")
		} else {
			pass(".uniamignore", ignorePath)
		}

		if exeErr != nil {
			warn("binary path", exeErr.Error())
		} else {
			pass("binary path", exePath)
			if isPathWritable(exePath) {
				pass("self-update", "binary path is writable")
			} else {
				warn("self-update", "binary path is not writable — update may require sudo or manual replacement")
			}
		}

		// --- Configuration ---
		fmt.Println("\nConfiguration:")
		pass("version", buildinfo.Version)

		cfg, err := config.LoadConfig(configPath)
		if err != nil {
			fail("load config", err.Error())
		} else {
			pass("load config", "ok")

			if err := cfg.Validate(); err != nil {
				fail("validate config", err.Error())
			} else {
				pass("validate config", "ok")
			}

			baseURL := "(default)"
			if cfg.Embedding.BaseURL != nil {
				baseURL = *cfg.Embedding.BaseURL
			}

			pass("embedding provider", fmt.Sprintf("%s / %s @ %s", cfg.Embedding.Provider, cfg.Embedding.Model, baseURL))
			pass("context.semantic", cfg.Context.Semantic)
			if cfg.Integrations.RipgrepEnabled {
				pass("ripgrep MCP", "enabled in Uniam config")
			} else {
				warn("ripgrep MCP", "not enabled in Uniam config")
			}
			if cfg.Integrations.CodeSearchEnabled {
				detail := "enabled in Uniam config"
				if cfg.Integrations.CodeSearchPath != nil && strings.TrimSpace(*cfg.Integrations.CodeSearchPath) != "" {
					detail += " (" + strings.TrimSpace(*cfg.Integrations.CodeSearchPath) + ")"
				}
				pass("code-search MCP", detail)
			} else {
				warn("code-search MCP", "not enabled in Uniam config")
			}
			if cfg.Integrations.GitEnabled {
				pass("Git MCP", "enabled in Uniam config")
			} else {
				warn("Git MCP", "not enabled in Uniam config")
			}
			if cfg.Integrations.Context7APIKey != nil && strings.TrimSpace(*cfg.Integrations.Context7APIKey) != "" {
				pass("context7 api key", "configured in Uniam config")
			} else {
				warn("context7 api key", "not configured")
			}
			if cfg.Integrations.BraveSearchAPIKey != nil && strings.TrimSpace(*cfg.Integrations.BraveSearchAPIKey) != "" {
				pass("brave api key", "configured in Uniam config")
			} else {
				warn("brave api key", "not configured")
			}
		}

		svc, err := core.NewService(home)
		if err == nil {
			defer func() { _ = svc.Close() }()

			provider, providerErr := svc.GetEmbeddingProvider()
			if providerErr != nil {
				fail("initialize provider", providerErr.Error())
			} else {
				pass("initialize provider", "ok")

				embedding, embedErr := provider.Embed(context.Background(), "uniam doctor probe")
				if embedErr != nil {
					fail("live probe", embedErr.Error())
					warn("", "check that your embedding service is running and reachable")
				} else {
					pass("live probe", fmt.Sprintf("ok — %d dimensions", len(embedding)))
				}
			}
		}

		// --- Sensitive data filtering ---
		fmt.Println("\nSensitive data filtering:")
		pass("built-in patterns", fmt.Sprintf("%d secret-masking patterns (Stripe, GitHub, AWS, Slack, private keys, JWT, password/secret/api key fields)", len(redaction.SensitivePatterns)))

		if patterns, err := redaction.LoadUniamIgnore(ignorePath); err != nil && !os.IsNotExist(err) {
			fail(".uniamignore patterns", err.Error())
		} else {
			pass(".uniamignore patterns", fmt.Sprintf("%d custom patterns", len(patterns)))
		}

		// --- Agent integrations ---
		fmt.Println("\nAgent integrations:")
		homeDir, _ := os.UserHomeDir()
		if homeDir == "" {
			warn("integrations", "home directory is unavailable")
		} else {
			for _, integration := range globalIntegrationStatuses(homeDir) {
				switch integration.status {
				case "configured":
					pass(integration.name, integration.detail)
				case "not-supported":
					warn(integration.name, integration.detail)
				default:
					warn(integration.name, integration.detail)
				}
			}
		}

		// --- Database & search ---
		fmt.Println("\nDatabase & search:")
		if svc == nil {
			fail("database connection", err.Error())
			fmt.Println("\nFix the issues above and re-run `uniam doctor`.")
			os.Exit(1)
		}

		pass("database connection", "ok")

		stats, err := svc.Stats(nil, nil)
		if err != nil {
			fail("note stats", err.Error())
		} else {
			pass("project count", fmt.Sprintf("%d projects", len(stats.ByProject)))
			pass("note count", fmt.Sprintf("%d notes stored", stats.Total))
			pass("active note count", fmt.Sprintf("%d active / %d total", stats.Active, stats.Total))
		}

		pass("FTS5 search", "always available")

		if svc.VectorsAvailable() {
			pass("vector search", "available (sqlite-vec loaded, table exists)")
		} else {
			warn("vector search", "not available — run `uniam reindex` after configuring embeddings")
		}

		// --- Updates ---
		fmt.Println("\nUpdates:")
		checker := update.NewChecker(buildinfo.Version)
		if release, err := checker.Check(context.Background()); err != nil {
			warn("update check", err.Error())
		} else if release.UpdateAvailable {
			warn("update available", fmt.Sprintf("%s -> %s", release.CurrentVersion, release.LatestVersion))
		} else {
			pass("update status", "up to date")
		}

		// --- Summary ---
		fmt.Println()

		if ok {
			fmt.Println("All checks passed.")
		} else {
			fmt.Println("Some checks failed. Fix the issues above.")
			os.Exit(1)
		}
	},
}

func isPathWritable(path string) bool {
	dir := filepath.Dir(path)
	if info, err := os.Stat(path); err == nil {
		return info.Mode().Perm()&0200 != 0
	}

	if info, err := os.Stat(dir); err == nil {
		return info.Mode().Perm()&0200 != 0
	}

	return false
}

type openCodePaths struct {
	ConfigPath string
	AgentsPath string
	SkillPath  string
	PluginPath string
}

type integrationStatus struct {
	name   string
	status string
	detail string
}

func newOpenCodePaths(homeDir string) openCodePaths {
	baseDir := filepath.Join(homeDir, ".config", "opencode")

	return openCodePaths{
		ConfigPath: filepath.Join(baseDir, "opencode.json"),
		AgentsPath: filepath.Join(baseDir, "AGENTS.md"),
		SkillPath:  filepath.Join(baseDir, "skills", "uniam", "SKILL.md"),
		PluginPath: filepath.Join(baseDir, "plugins", "uniam.js"),
	}
}

func verifyOpenCodeMCPConfig(configPath string) error {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("read failed: %w", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(data, &decoded); err != nil {
		return fmt.Errorf("invalid json: %w", err)
	}

	mcp, ok := decoded["mcp"].(map[string]any)
	if !ok {
		return fmt.Errorf("mcp.uniam missing in %s", configPath)
	}

	if _, ok := mcp["uniam"].(map[string]any); !ok {
		return fmt.Errorf("mcp.uniam missing in %s", configPath)
	}

	return nil
}

func verifyJSONMCPConfig(configPath string, topKey string, serverKey string) error {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("read failed: %w", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(data, &decoded); err != nil {
		return fmt.Errorf("invalid json: %w", err)
	}

	mcp, ok := decoded[topKey].(map[string]any)
	if !ok {
		return fmt.Errorf("%s.%s missing in %s", topKey, serverKey, configPath)
	}

	if _, ok := mcp[serverKey]; !ok {
		return fmt.Errorf("%s.%s missing in %s", topKey, serverKey, configPath)
	}

	return nil
}

func verifyCodexConfig(configPath string) error {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("read failed: %w", err)
	}

	if !strings.Contains(string(data), "[mcp_servers.uniam]") {
		return fmt.Errorf("mcp_servers.uniam missing in %s", configPath)
	}

	return nil
}

func globalIntegrationStatuses(homeDir string) []integrationStatus {
	statuses := []integrationStatus{
		checkJSONIntegration("Claude Code", filepath.Join(homeDir, ".claude.json"), "mcpServers", "uniam", []string{
			filepath.Join(homeDir, ".claude", "skills", "uniam", "SKILL.md"),
		}),
		checkJSONIntegration("Cursor", filepath.Join(homeDir, ".cursor", "mcp.json"), "mcpServers", "uniam", []string{
			filepath.Join(homeDir, ".cursor", "skills", "uniam", "SKILL.md"),
		}),
		checkWindsurfIntegration(homeDir),
		checkJSONIntegration("Antigravity", filepath.Join(homeDir, ".gemini", "antigravity", "mcp_config.json"), "mcpServers", "uniam", []string{
			filepath.Join(homeDir, ".gemini", "antigravity", "skills", "uniam", "SKILL.md"),
		}),
		checkCodexIntegration(homeDir),
		checkOpenCodeIntegration(homeDir),
		{
			name:   "RooCode",
			status: "not-supported",
			detail: "project-only integration; no global setup path",
		},
		checkJSONIntegration("GitHub Copilot", mustCopilotConfigPath(homeDir), "mcpServers", "uniam", []string{
			filepath.Join(homeDir, ".uniam", "skills", "uniam", "SKILL.md"),
		}),
		checkJSONIntegration("Gemini CLI", filepath.Join(homeDir, ".gemini", "settings.json"), "mcpServers", "uniam", []string{
			filepath.Join(homeDir, ".gemini", "skills", "uniam", "SKILL.md"),
		}),
	}

	return statuses
}

func checkJSONIntegration(name string, configPath string, topKey string, serverKey string, requiredFiles []string) integrationStatus {
	if _, err := os.Stat(configPath); err != nil {
		return integrationStatus{name: name, status: "missing", detail: "not configured"}
	}

	if err := verifyJSONMCPConfig(configPath, topKey, serverKey); err != nil {
		return integrationStatus{name: name, status: "broken", detail: err.Error()}
	}

	missing := missingPaths(requiredFiles)
	detail := fmt.Sprintf("configured (%s)", configPath)
	if len(missing) > 0 {
		return integrationStatus{name: name, status: "broken", detail: fmt.Sprintf("config ok, missing %s", strings.Join(missing, ", "))}
	}

	return integrationStatus{name: name, status: "configured", detail: detail}
}

func checkCodexIntegration(homeDir string) integrationStatus {
	configPath := filepath.Join(homeDir, ".codex", "config.toml")
	if _, err := os.Stat(configPath); err != nil {
		return integrationStatus{name: "Codex", status: "missing", detail: "not configured"}
	}

	if err := verifyCodexConfig(configPath); err != nil {
		return integrationStatus{name: "Codex", status: "broken", detail: err.Error()}
	}

	missing := missingPaths([]string{
		filepath.Join(homeDir, ".codex", "skills", "uniam", "SKILL.md"),
	})
	if len(missing) > 0 {
		return integrationStatus{name: "Codex", status: "broken", detail: fmt.Sprintf("config ok, missing %s", strings.Join(missing, ", "))}
	}

	return integrationStatus{name: "Codex", status: "configured", detail: fmt.Sprintf("configured (%s)", configPath)}
}

func checkOpenCodeIntegration(homeDir string) integrationStatus {
	paths := newOpenCodePaths(homeDir)
	if _, err := os.Stat(paths.ConfigPath); err != nil {
		return integrationStatus{name: "OpenCode", status: "missing", detail: "not configured"}
	}

	if err := verifyOpenCodeMCPConfig(paths.ConfigPath); err != nil {
		return integrationStatus{name: "OpenCode", status: "broken", detail: err.Error()}
	}

	missing := missingPaths([]string{paths.AgentsPath, paths.SkillPath, paths.PluginPath})
	if len(missing) > 0 {
		return integrationStatus{name: "OpenCode", status: "broken", detail: fmt.Sprintf("config ok, missing %s", strings.Join(missing, ", "))}
	}

	return integrationStatus{name: "OpenCode", status: "configured", detail: fmt.Sprintf("configured (%s)", paths.ConfigPath)}
}

func checkWindsurfIntegration(homeDir string) integrationStatus {
	targets := []string{
		filepath.Join(homeDir, ".codeium", "windsurf"),
		filepath.Join(homeDir, ".codeium"),
	}

	for _, target := range targets {
		configPath := filepath.Join(target, "mcp_config.json")
		if _, err := os.Stat(configPath); err != nil {
			continue
		}

		if err := verifyJSONMCPConfig(configPath, "mcpServers", "uniam"); err != nil {
			return integrationStatus{name: "Windsurf", status: "broken", detail: err.Error()}
		}

		missing := missingPaths([]string{
			filepath.Join(target, "skills", "uniam", "SKILL.md"),
		})
		if len(missing) > 0 {
			return integrationStatus{name: "Windsurf", status: "broken", detail: fmt.Sprintf("config ok, missing %s", strings.Join(missing, ", "))}
		}

		return integrationStatus{name: "Windsurf", status: "configured", detail: fmt.Sprintf("configured (%s)", configPath)}
	}

	return integrationStatus{name: "Windsurf", status: "missing", detail: "not configured"}
}

func mustCopilotConfigPath(homeDir string) string {
	path, err := getCopilotConfigPath()
	if err != nil {
		return filepath.Join(homeDir, ".config", "Code", "User", "globalStorage", "github.copilot-chat", "mcp.json")
	}

	return path
}

func missingPaths(paths []string) []string {
	var missing []string
	for _, path := range paths {
		if _, err := os.Stat(path); err != nil {
			missing = append(missing, path)
		}
	}

	return missing
}
