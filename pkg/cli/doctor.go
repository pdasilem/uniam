package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

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
				pass("binary writable", "yes")
			} else {
				warn("binary writable", "no")
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
		}

		// --- Redaction ---
		fmt.Println("\nRedaction:")
		pass("built-in patterns", fmt.Sprintf("%d patterns", len(redaction.SensitivePatterns)))

		if patterns, err := redaction.LoadUniamIgnore(ignorePath); err != nil && !os.IsNotExist(err) {
			fail(".uniamignore patterns", err.Error())
		} else {
			pass(".uniamignore patterns", fmt.Sprintf("%d custom patterns", len(patterns)))
		}

		// --- Agent readiness ---
		fmt.Println("\nAgent readiness:")
		rulesFiles := []string{"AGENTS.md", "CLAUDE.md", ".rules"}
		foundRule := false
		for _, name := range rulesFiles {
			if _, err := os.Stat(name); err == nil {
				pass("project rules", name)
				foundRule = true
				break
			}
		}
		if !foundRule {
			warn("project rules", "no AGENTS.md / CLAUDE.md / .rules found in cwd")
		}

		homeDir, _ := os.UserHomeDir()
		agentSkillChecks := []struct {
			label string
			path  string
		}{
			{"codex skill", filepath.Join(homeDir, ".codex", "skills", "uniam", "SKILL.md")},
			{"claude skill", filepath.Join(homeDir, ".claude", "skills", "uniam", "SKILL.md")},
			{"cursor skill", filepath.Join(homeDir, ".cursor", "skills", "uniam", "SKILL.md")},
			{"gemini skill", filepath.Join(homeDir, ".gemini", "skills", "uniam", "SKILL.md")},
		}
		for _, check := range agentSkillChecks {
			if _, err := os.Stat(check.path); err == nil {
				pass(check.label, check.path)
			}
		}

		if homeDir == "" {
			warn("opencode global", "home directory is unavailable")
		} else {
			fmt.Println("\nOpenCode global integration:")

			openCodePaths := newOpenCodePaths(homeDir)
			if _, err := os.Stat(openCodePaths.ConfigPath); err != nil {
				fail("opencode.json", fmt.Sprintf("missing — expected %s", openCodePaths.ConfigPath))
			} else {
				pass("opencode.json", openCodePaths.ConfigPath)

				if err := verifyOpenCodeMCPConfig(openCodePaths.ConfigPath); err != nil {
					fail("mcp.uniam", err.Error())
				} else {
					pass("mcp.uniam", "configured")
				}
			}

			requiredOpenCodeFiles := []struct {
				label string
				path  string
			}{
				{"opencode skill", openCodePaths.SkillPath},
				{"uniam instructions", openCodePaths.InstructionsPath},
				{"opencode plugin", openCodePaths.PluginPath},
			}
			for _, check := range requiredOpenCodeFiles {
				if _, err := os.Stat(check.path); err != nil {
					fail(check.label, fmt.Sprintf("missing — expected %s", check.path))
					continue
				}

				pass(check.label, check.path)
			}
		}

		// --- Database & search ---
		fmt.Println("\nDatabase & search:")

		svc, err := core.NewService(home)
		if err != nil {
			fail("database connection", err.Error())
			fmt.Println("\nFix the issues above and re-run `uniam doctor`.")
			os.Exit(1)
		}

		defer func() { _ = svc.Close() }()

		pass("database connection", "ok")

		total, err := svc.CountItems(nil, nil)
		if err != nil {
			fail("note count", err.Error())
		} else {
			pass("note count", fmt.Sprintf("%d notes stored", total))
		}

		stats, err := svc.Stats(nil, nil)
		if err != nil {
			fail("active note count", err.Error())
		} else {
			pass("active note count", fmt.Sprintf("%d active / %d total", stats.Active, stats.Total))
		}

		pass("FTS5 search", "always available")

		if svc.VectorsAvailable() {
			pass("vector search", "available (sqlite-vec loaded, table exists)")
		} else {
			warn("vector search", "not available — run `uniam reindex` after configuring embeddings")
		}

		checker := update.NewChecker(buildinfo.Version)
		if release, err := checker.Check(context.Background(), false); err != nil {
			warn("update check", err.Error())
		} else if release.UpdateAvailable {
			warn("update available", fmt.Sprintf("%s -> %s", release.CurrentVersion, release.LatestVersion))
		} else {
			pass("update status", "up to date")
		}

		// --- Embedding provider live test ---
		fmt.Println("\nEmbedding provider:")

		provider, err := svc.GetEmbeddingProvider()
		if err != nil {
			fail("initialize provider", err.Error())
		} else {
			pass("initialize provider", "ok")

			embedding, err := provider.Embed(context.Background(), "uniam doctor probe")
			if err != nil {
				fail("live probe", err.Error())
				warn("", "check that your embedding service is running and reachable")
			} else {
				pass("live probe", fmt.Sprintf("ok — %d dimensions", len(embedding)))
			}
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
	ConfigPath       string
	SkillPath        string
	InstructionsPath string
	PluginPath       string
}

func newOpenCodePaths(homeDir string) openCodePaths {
	baseDir := filepath.Join(homeDir, ".config", "opencode")

	return openCodePaths{
		ConfigPath:       filepath.Join(baseDir, "opencode.json"),
		SkillPath:        filepath.Join(baseDir, "skills", "uniam", "SKILL.md"),
		InstructionsPath: filepath.Join(baseDir, "uniam-instructions.md"),
		PluginPath:       filepath.Join(baseDir, "plugins", "uniam.js"),
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
