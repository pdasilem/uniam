package cli

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"uniam/internal/config"

	"github.com/spf13/cobra"
)

var (
	setupConfigDir      string
	setupProject        bool
	setupRipgrep        bool
	setupRipgrepSet     bool
	setupCodeSearch     bool
	setupCodeSet        bool
	setupContext7       bool
	setupCtx7Set        bool
	setupGitMCP         bool
	setupGitSet         bool
	setupSearXNG        bool
	setupSearXNGSet     bool
	setupBrave          bool
	setupBraveSet       bool
	setupFirecrawl      bool
	setupFirecrawlSet   bool
	currentSetupOptions setupOptions
)

type agentFunc func(configDir string, project bool, fastContext bool) (map[string]string, error)

type setupOptions struct {
	Ripgrep         bool
	CodeSearch      bool
	CodeSearchPath  string
	Context7        bool
	Context7APIKey  string
	Git             bool
	SearXNG         bool
	SearXNGURL      string
	BraveSearch     bool
	BraveAPIKey     string
	Firecrawl       bool
	FirecrawlAPIKey string
}

type optionalIntegration struct {
	key         string
	name        string
	description string
}

func optionalIntegrations() []optionalIntegration {
	return []optionalIntegration{
		{
			key:         "ripgrep",
			name:        "ripgrep MCP",
			description: "Adds fast exact text and regex search across code and config files.",
		},
		{
			key:         "code-search",
			name:        "code-search MCP",
			description: "Adds broader code discovery, symbol-oriented search, and cross-file navigation.",
		},
		{
			key:         "context7",
			name:        "Context7 MCP",
			description: "Adds up-to-date library docs, current package versions, and dependency compatibility lookups.",
		},
		{
			key:         "git",
			name:        "Git MCP",
			description: "Adds structured repository status, diff, history, and branch inspection tools.",
		},
		{
			key:         "searxng",
			name:        "SearXNG MCP",
			description: "Adds web search through your existing SearXNG instance.",
		},
		{
			key:         "brave-search",
			name:        "Brave Search MCP",
			description: "Adds web search for current external information.",
		},
		{
			key:         "firecrawl",
			name:        "Firecrawl MCP",
			description: "Adds web scraping, crawling, extraction, and live page fetch tools.",
		},
	}
}

func loadUniamConfigForSetup() (*config.Config, string, error) {
	configPath := filepath.Join(config.GetUniamHome(), "config.yaml")
	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		return nil, configPath, err
	}

	return cfg, configPath, nil
}

func resolveSetupOptions(cfg *config.Config, configPath string, promptCtx *setupPromptContext) (setupOptions, error) {
	opts := setupOptions{}

	reader := bufio.NewReader(os.Stdin)
	for _, integration := range optionalIntegrations() {
		status := inspectIntegrationPromptStatus(promptCtx, integration, cfg)
		enabled, err := promptOptionalIntegration(reader, integration, status)
		if err != nil {
			return opts, err
		}

		switch integration.key {
		case "ripgrep":
			opts.Ripgrep = enabled
			cfg.Integrations.RipgrepEnabled = enabled
		case "code-search":
			opts.CodeSearch = enabled
			cfg.Integrations.CodeSearchEnabled = enabled
			if enabled {
				entry, installErr := ensureCodeSearchInstall(cfg)
				if installErr != nil {
					fmt.Fprintf(os.Stderr, "Warning: code-search MCP install failed: %v\n", installErr)
					fmt.Fprintln(os.Stderr, "Skipping code-search MCP.")
					opts.CodeSearch = false
					cfg.Integrations.CodeSearchEnabled = false
				} else {
					opts.CodeSearchPath = entry
					cfg.Integrations.CodeSearchPath = &entry
				}
			}
		case "context7":
			opts.Context7 = enabled
			cfg.Integrations.Context7Enabled = enabled
			if enabled {
				key, reused, keyErr := resolveAPIKeyPrompt(reader, "Context7", cfg.Integrations.Context7APIKey)
				if keyErr != nil {
					return opts, keyErr
				}
				if key == "" {
					fmt.Println("Context7 was requested, but no API key is configured. Skipping Context7 MCP.")
					opts.Context7 = false
					cfg.Integrations.Context7Enabled = false
					cfg.Integrations.Context7APIKey = nil
				} else {
					opts.Context7APIKey = key
					cfg.Integrations.Context7APIKey = stringPtr(key)
					if reused {
						fmt.Println("Using saved Context7 API key from Uniam config.")
					} else {
						fmt.Println("Saved Context7 API key to Uniam config.")
					}
				}
			}
		case "git":
			opts.Git = enabled
			cfg.Integrations.GitEnabled = enabled
			if enabled {
				if _, err := exec.LookPath("uvx"); err != nil {
					fmt.Fprintln(os.Stderr, "Warning: uvx was not found in PATH. Skipping Git MCP.")
					opts.Git = false
					cfg.Integrations.GitEnabled = false
				}
			}
		case "searxng":
			opts.SearXNG = enabled
			cfg.Integrations.SearXNGEnabled = enabled
			if enabled {
				if _, err := exec.LookPath("npx"); err != nil {
					fmt.Fprintf(os.Stderr, "Warning: npx was not found in PATH. Skipping SearXNG MCP.\n")
					opts.SearXNG = false
					cfg.Integrations.SearXNGEnabled = false
					break
				}
				url, reused, detected, urlErr := resolveSearXNGURL(reader, cfg.Integrations.SearXNGURL)
				if urlErr != nil {
					return opts, urlErr
				}
				if url == "" {
					fmt.Println("SearXNG was requested, but no usable instance URL was configured or detected. Skipping SearXNG MCP.")
					opts.SearXNG = false
					cfg.Integrations.SearXNGEnabled = false
				} else {
					opts.SearXNGURL = url
					cfg.Integrations.SearXNGURL = stringPtr(url)
					opts.BraveSearch = false
					cfg.Integrations.BraveSearchEnabled = false
					if reused {
						fmt.Println("Using saved SearXNG URL from Uniam config.")
					} else if detected {
						fmt.Printf("Using detected SearXNG instance at %s.\n", url)
					} else {
						fmt.Println("Saved SearXNG URL to Uniam config.")
					}
				}
			}
		case "brave-search":
			opts.BraveSearch = enabled
			cfg.Integrations.BraveSearchEnabled = enabled
			if enabled {
				key, reused, keyErr := resolveAPIKeyPrompt(reader, "Brave Search", cfg.Integrations.BraveSearchAPIKey)
				if keyErr != nil {
					return opts, keyErr
				}
				if key == "" {
					fmt.Println("Brave Search was requested, but no API key is configured. Skipping Brave Search MCP.")
					opts.BraveSearch = false
					cfg.Integrations.BraveSearchEnabled = false
					cfg.Integrations.BraveSearchAPIKey = nil
				} else {
					opts.BraveAPIKey = key
					cfg.Integrations.BraveSearchAPIKey = stringPtr(key)
					opts.SearXNG = false
					cfg.Integrations.SearXNGEnabled = false
					if reused {
						fmt.Println("Using saved Brave Search API key from Uniam config.")
					} else {
						fmt.Println("Saved Brave Search API key to Uniam config.")
					}
				}
			}
		case "firecrawl":
			opts.Firecrawl = enabled
			cfg.Integrations.FirecrawlEnabled = enabled
			if enabled {
				key, reused, keyErr := resolveAPIKeyPrompt(reader, "Firecrawl", cfg.Integrations.FirecrawlAPIKey)
				if keyErr != nil {
					return opts, keyErr
				}
				if key == "" {
					fmt.Println("Firecrawl was requested, but no API key is configured. Skipping Firecrawl MCP.")
					opts.Firecrawl = false
					cfg.Integrations.FirecrawlEnabled = false
					cfg.Integrations.FirecrawlAPIKey = nil
				} else {
					opts.FirecrawlAPIKey = key
					cfg.Integrations.FirecrawlAPIKey = stringPtr(key)
					if reused {
						fmt.Println("Using saved Firecrawl API key from Uniam config.")
					} else {
						fmt.Println("Saved Firecrawl API key to Uniam config.")
					}
				}
			}
		}
	}

	if err := config.SaveConfig(configPath, cfg); err != nil {
		return opts, fmt.Errorf("failed to save Uniam config: %w", err)
	}

	return opts, nil
}

func promptOptionalIntegration(reader *bufio.Reader, integration optionalIntegration, status integrationPromptStatus) (bool, error) {
	switch integration.key {
	case "ripgrep":
		if setupRipgrepSet {
			return setupRipgrep, nil
		}
	case "code-search":
		if setupCodeSet {
			return setupCodeSearch, nil
		}
	case "context7":
		if setupCtx7Set {
			return setupContext7, nil
		}
	case "git":
		if setupGitSet {
			return setupGitMCP, nil
		}
	case "searxng":
		if setupSearXNGSet {
			return setupSearXNG, nil
		}
	case "brave-search":
		if setupBraveSet {
			return setupBrave, nil
		}
	case "firecrawl":
		if setupFirecrawlSet {
			return setupFirecrawl, nil
		}
	}

	fmt.Printf("%s: %s\n", integration.name, integration.description)
	if strings.TrimSpace(status.summary) != "" {
		fmt.Printf("Current status: %s\n", status.summary)
		for _, detail := range status.details {
			fmt.Printf("%s\n", detail)
		}
	}
	fmt.Printf("Install %s? yes/no (default no): ", integration.name)
	response, err := reader.ReadString('\n')
	if err != nil {
		return false, err
	}

	response = strings.TrimSpace(strings.ToLower(response))
	return response == "yes" || response == "y", nil
}

func resolveAPIKeyPrompt(reader *bufio.Reader, label string, saved *string) (key string, reused bool, err error) {
	existing := ""
	if saved != nil {
		existing = strings.TrimSpace(*saved)
	}

	if existing != "" {
		fmt.Printf("%s API key (press Enter to reuse saved value): ", label)
	} else {
		fmt.Printf("%s API key (leave empty to skip %s): ", label, label)
	}

	input, readErr := reader.ReadString('\n')
	if readErr != nil {
		return "", false, readErr
	}

	input = strings.TrimSpace(input)
	if input == "" {
		if existing == "" {
			return "", false, nil
		}
		return existing, true, nil
	}

	return input, false, nil
}

func resolveSearXNGURL(reader *bufio.Reader, saved *string) (url string, reused bool, detected bool, err error) {
	existing := ""
	if saved != nil {
		existing = strings.TrimSpace(*saved)
	}

	if existing != "" {
		fmt.Print("SearXNG instance URL (press Enter to reuse saved value): ")
		input, readErr := reader.ReadString('\n')
		if readErr != nil {
			return "", false, false, readErr
		}
		input = strings.TrimSpace(input)
		if input == "" {
			return existing, true, false, nil
		}
		return input, false, false, nil
	}

	if detectedURL := detectLocalSearXNGURL(); detectedURL != "" {
		fmt.Printf("SearXNG instance URL (press Enter to reuse detected %s): ", detectedURL)
		input, readErr := reader.ReadString('\n')
		if readErr != nil {
			return "", false, false, readErr
		}
		input = strings.TrimSpace(input)
		if input == "" {
			return detectedURL, false, true, nil
		}
		return input, false, false, nil
	}

	fmt.Print("SearXNG instance URL (leave empty to skip SearXNG, example http://localhost:8213): ")
	input, readErr := reader.ReadString('\n')
	if readErr != nil {
		return "", false, false, readErr
	}

	return strings.TrimSpace(input), false, false, nil
}

func detectLocalSearXNGURL() string {
	candidates := []string{
		"http://localhost:8213",
		"http://127.0.0.1:8213",
		"http://127.0.0.1:8080",
		"http://localhost:8080",
	}
	client := &http.Client{Timeout: 1500 * time.Millisecond}
	for _, baseURL := range candidates {
		req, err := http.NewRequest(http.MethodGet, strings.TrimRight(baseURL, "/")+"/search?q=uniam&format=json", nil)
		if err != nil {
			continue
		}
		resp, err := client.Do(req)
		if err != nil {
			continue
		}
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		_ = resp.Body.Close()
		if resp.StatusCode == http.StatusOK && strings.Contains(string(body), "\"results\"") {
			return baseURL
		}
	}

	return ""
}

func runAgentCmd(agent string, handlers map[string]agentFunc, configDir string, project bool, isSetup bool) {
	fn, ok := handlers[agent]
	if !ok {
		fmt.Fprintf(os.Stderr, "Error: unknown agent: %s\n", agent)
		fmt.Fprintf(os.Stderr, "Supported agents: claude-code, cursor, windsurf, antigravity, codex, opencode, copilot, gemini-cli\n")
		os.Exit(1)
	}

	currentSetupOptions = setupOptions{}

	if isSetup {
		cfg, cfgPath, err := loadUniamConfigForSetup()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to load Uniam config: %v\n", err)
			cfg = &config.Config{}
		}
		promptCtx := buildSetupPromptContext(agent, configDir, project)
		currentSetupOptions, err = resolveSetupOptions(cfg, cfgPath, promptCtx)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	}

	setupContext7 = currentSetupOptions.Context7

	result, err := fn(configDir, project, false)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Println(result["message"])
}

var setupCmd = &cobra.Command{
	Use:   "setup [agent]",
	Short: "Install Uniam hooks for an agent",
	Args:  cobra.ExactArgs(1),
	//nolint:revive
	Run: func(cmd *cobra.Command, args []string) {
		setupRipgrepSet = cmd.Flags().Changed("ripgrep")
		setupCodeSet = cmd.Flags().Changed("code-search")
		setupCtx7Set = cmd.Flags().Changed("context7")
		setupGitSet = cmd.Flags().Changed("git-mcp")
		setupSearXNGSet = cmd.Flags().Changed("searxng")
		setupBraveSet = cmd.Flags().Changed("brave-search")
		setupFirecrawlSet = cmd.Flags().Changed("firecrawl")
		runAgentCmd(args[0], map[string]agentFunc{
			"claude-code": setupClaudeCode,
			"cursor":      setupCursor,
			"windsurf":    setupWindsurf,
			"antigravity": setupAntigravity,
			"codex":       setupCodex,
			"copilot":     setupCopilot,
			"gemini-cli":  setupGeminiCli,
			"opencode": func(configDir string, project bool, fast bool) (map[string]string, error) {
				if configDir != "" {
					return nil, errors.New("OpenCode setup does not support --config-dir. It installs only to ~/.config/opencode")
				}
				return setupOpenCode(project, fast)
			},
		}, setupConfigDir, setupProject, true)
	},
}

var uninstallCmd = &cobra.Command{
	Use:   "uninstall [agent]",
	Short: "Remove Uniam hooks for an agent",
	Args:  cobra.ExactArgs(1),
	//nolint:revive
	Run: func(cmd *cobra.Command, args []string) {
		setupRipgrepSet = false
		setupCodeSet = false
		setupCtx7Set = false
		setupGitSet = false
		setupSearXNGSet = false
		setupBraveSet = false
		setupFirecrawlSet = false
		currentSetupOptions = setupOptions{}
		runAgentCmd(args[0], map[string]agentFunc{
			"claude-code": uninstallClaudeCode,
			"cursor":      uninstallCursor,
			"windsurf":    uninstallWindsurf,
			"antigravity": uninstallAntigravity,
			"codex":       uninstallCodex,
			"copilot":     uninstallCopilot,
			"gemini-cli":  uninstallGeminiCli,
			"opencode": func(configDir string, project bool, _ bool) (map[string]string, error) {
				if configDir != "" {
					return nil, errors.New("OpenCode uninstall does not support --config-dir. It removes only from ~/.config/opencode")
				}
				return uninstallOpenCode(project)
			},
		}, setupConfigDir, setupProject, false)
	},
}

func init() {
	setupCmd.Flags().StringVar(&setupConfigDir, "config-dir", "", "Path to agent config directory")
	setupCmd.Flags().BoolVarP(&setupProject, "project", "p", false, "Install in current project instead of globally")
	setupCmd.Flags().BoolVar(&setupRipgrep, "ripgrep", false, "Also install the ripgrep MCP server")
	setupCmd.Flags().BoolVar(&setupCodeSearch, "code-search", false, "Also install the code-search MCP server")
	setupCmd.Flags().BoolVar(&setupContext7, "context7", false, "Also install the Context7 MCP server")
	setupCmd.Flags().BoolVar(&setupGitMCP, "git-mcp", false, "Also install the Git MCP server")
	setupCmd.Flags().BoolVar(&setupSearXNG, "searxng", false, "Also install the SearXNG MCP server")
	setupCmd.Flags().BoolVar(&setupBrave, "brave-search", false, "Also install the Brave Search MCP server")
	setupCmd.Flags().BoolVar(&setupFirecrawl, "firecrawl", false, "Also install the Firecrawl MCP server")
	uninstallCmd.Flags().StringVar(&setupConfigDir, "config-dir", "", "Path to agent config directory")
	uninstallCmd.Flags().BoolVarP(&setupProject, "project", "p", false, "Uninstall from current project instead of globally")
}

func resolveConfigDir(agentDotDir string, configDir string, project bool) string {
	if configDir != "" {
		return configDir
	}

	if project {
		dir, _ := os.Getwd()

		return filepath.Join(dir, agentDotDir)
	}

	home, _ := os.UserHomeDir()

	return filepath.Join(home, agentDotDir)
}

func openCodeConfigRoot() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to resolve home directory: %w", err)
	}

	return filepath.Join(home, ".config", "opencode"), nil
}

func openCodeInstallPaths(project bool) (target string, configPath string, instructionRef string, err error) {
	if project {
		cwd, cwdErr := os.Getwd()
		if cwdErr != nil {
			return "", "", "", fmt.Errorf("failed to resolve current directory: %w", cwdErr)
		}

		return filepath.Join(cwd, ".opencode"), filepath.Join(cwd, "opencode.json"), ".opencode/" + openCodeInstructionsFileName, nil
	}

	target, err = openCodeConfigRoot()
	if err != nil {
		return "", "", "", err
	}

	return target, filepath.Join(target, "opencode.json"), "./" + openCodeInstructionsFileName, nil
}

func openCodeAgentsPath(project bool, target string) (string, error) {
	if project {
		cwd, err := os.Getwd()
		if err != nil {
			return "", fmt.Errorf("failed to resolve current directory: %w", err)
		}

		return filepath.Join(cwd, "AGENTS.md"), nil
	}

	return filepath.Join(target, "AGENTS.md"), nil
}

func readJSONMap(path string) (map[string]any, error) {
	if data, err := os.ReadFile(path); err == nil {
		var decoded map[string]any
		if err := json.Unmarshal(data, &decoded); err != nil {
			return nil, fmt.Errorf("failed to parse existing config: %w", err)
		}

		return decoded, nil
	}

	return make(map[string]any), nil
}

func writeJSONMap(path string, decoded map[string]any) error {
	data, err := json.MarshalIndent(decoded, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}

	return nil
}

func appendUniqueString(values []string, target string) []string {
	for _, value := range values {
		if value == target {
			return values
		}
	}

	return append(values, target)
}

func setupMessage(agent string, location string, extras ...string) string {
	msg := "Installed Uniam " + agent + " integration in " + location
	if len(extras) > 0 {
		msg += " (" + strings.Join(extras, ", ") + ")"
	}

	return msg
}

func uninstallMessage(agent string, location string, extras ...string) string {
	msg := "Removed Uniam " + agent + " integration from " + location
	if len(extras) > 0 {
		msg += " (" + strings.Join(extras, ", ") + ")"
	}

	return msg
}

func integrationInstructionLines(opts setupOptions) string {
	lines := ""
	if opts.Ripgrep {
		lines += ripgrepInstructionLine
	}
	if opts.CodeSearch {
		lines += codeSearchInstructionLine
	}
	if opts.Context7 {
		lines += context7InstructionLine
	}
	if opts.Git {
		lines += gitInstructionLine
	}
	if opts.SearXNG {
		lines += searxngInstructionLine
	}
	if opts.BraveSearch {
		lines += braveSearchInstructionLine
	}
	if opts.Firecrawl {
		lines += firecrawlInstructionLine
	}

	return lines
}

func insertInstructionLines(content string, anchor string, lines string) string {
	if lines == "" || !strings.Contains(content, anchor) {
		return content
	}

	return strings.Replace(content, anchor, anchor+lines, 1)
}

func enabledIntegrationLabels() []string {
	var extras []string
	if currentSetupOptions.Ripgrep {
		extras = append(extras, "ripgrep MCP")
	}
	if currentSetupOptions.CodeSearch {
		extras = append(extras, "code-search MCP")
	}
	if currentSetupOptions.Context7 {
		extras = append(extras, "Context7")
	}
	if currentSetupOptions.Git {
		extras = append(extras, "Git MCP")
	}
	if currentSetupOptions.SearXNG {
		extras = append(extras, "SearXNG")
	}
	if currentSetupOptions.BraveSearch {
		extras = append(extras, "Brave Search")
	}
	if currentSetupOptions.Firecrawl {
		extras = append(extras, "Firecrawl")
	}

	return extras
}

func openCodeAgentsManagedBlock(opts setupOptions) string {
	return "<!-- uniam:begin opencode -->\n" +
		compactUniamSection(opts) +
		"<!-- uniam:end opencode -->\n"
}

func copilotRepoInstructionsManagedBlock(opts setupOptions) string {
	block := "<!-- uniam:begin copilot -->\n" +
		"## Uniam\n\n" +
		"Use Uniam for cross-session memory.\n\n" +
		"Required workflow:\n" +
		"- Before meaningful work, retrieve with `uniam_context`, `uniam_search`, or `uniam_retrieve`.\n" +
		"- During long or decision-heavy work, checkpoint with `uniam_store`.\n" +
		"- Before finishing meaningful work, store a final note with `uniam_store`.\n" +
		"- Curate memory with `uniam_archive`, `uniam_supersede`, `uniam_update_note`, and `uniam_compact`.\n"
	block += integrationInstructionLines(opts)
	block += "\n" +
		"Current scope is only the current project or folder. Cross-project access is not allowed.\n" +
		"<!-- uniam:end copilot -->\n"

	return block
}

func upsertManagedBlock(path string, marker string, block string) error {
	existing, _ := os.ReadFile(path)
	text := string(existing)

	begin := "<!-- uniam:begin " + marker + " -->"
	end := "<!-- uniam:end " + marker + " -->"

	start := strings.Index(text, begin)
	finish := strings.Index(text, end)
	if start >= 0 && finish >= 0 && finish >= start {
		finish += len(end)
		if finish < len(text) && text[finish] == '\n' {
			finish++
		}
		text = text[:start] + block + text[finish:]
	} else {
		if len(text) > 0 && !strings.HasSuffix(text, "\n") {
			text += "\n"
		}
		text += block
	}

	return os.WriteFile(path, []byte(text), 0644)
}

func removeManagedBlock(path string, marker string) error {
	existing, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	text := string(existing)
	begin := "<!-- uniam:begin " + marker + " -->"
	end := "<!-- uniam:end " + marker + " -->"
	start := strings.Index(text, begin)
	finish := strings.Index(text, end)
	if start < 0 || finish < 0 || finish < start {
		return nil
	}

	finish += len(end)
	if finish < len(text) && text[finish] == '\n' {
		finish++
	}
	text = text[:start] + text[finish:]
	text = strings.TrimSpace(text)
	if text == "" {
		if removeErr := os.Remove(path); removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
			return removeErr
		}
		return nil
	}

	text += "\n"
	return os.WriteFile(path, []byte(text), 0644)
}

func removeFileIfExists(path string) (bool, error) {
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		return false, nil
	}

	if err := os.Remove(path); err != nil {
		return false, err
	}

	return true, nil
}

func addRipgrepServer(mcpServers map[string]any) {
	mcpServers["ripgrep"] = map[string]any{
		"command": "npx",
		"args":    []string{"-y", "mcp-ripgrep@latest"},
	}
}

func ensureCodeSearchInstall(cfg *config.Config) (string, error) {
	if cfg.Integrations.CodeSearchPath != nil {
		existing := strings.TrimSpace(*cfg.Integrations.CodeSearchPath)
		if existing != "" {
			if _, err := os.Stat(existing); err == nil {
				return existing, nil
			}
		}
	}

	fmt.Println("Installing code-search MCP...")
	entry, err := installCodeSearch()
	if err != nil {
		return "", err
	}

	return entry, nil
}

func addCodeSearchServer(mcpServers map[string]any, entry string) {
	if strings.TrimSpace(entry) == "" {
		return
	}

	home, _ := os.UserHomeDir()
	mcpServers["code-search"] = map[string]any{
		"command": "node",
		"args":    []string{entry, "--allowed-workspace", home},
	}
}

func addContext7Server(mcpServers map[string]any, apiKey string) {
	if strings.TrimSpace(apiKey) == "" {
		return
	}

	mcpServers["context7"] = map[string]any{
		"command": "npx",
		"args":    []string{"-y", "@upstash/context7-mcp"},
		"env": map[string]any{
			"CONTEXT7_API_KEY": apiKey,
		},
	}
}

func addGitServer(mcpServers map[string]any) {
	mcpServers["git"] = map[string]any{
		"command": "uvx",
		"args":    []string{"mcp-server-git"},
	}
}

func addBraveSearchServer(mcpServers map[string]any, apiKey string) {
	if strings.TrimSpace(apiKey) == "" {
		return
	}

	mcpServers["brave-search"] = map[string]any{
		"command": "npx",
		"args":    []string{"-y", "@brave/brave-search-mcp-server", "--transport", "stdio"},
		"env": map[string]any{
			"BRAVE_API_KEY": apiKey,
		},
	}
}

func addSearXNGServer(mcpServers map[string]any, baseURL string) {
	if strings.TrimSpace(baseURL) == "" {
		return
	}

	mcpServers["searxng"] = map[string]any{
		"command": "npx",
		"args":    []string{"-y", "mcp-searxng"},
		"env": map[string]any{
			"SEARXNG_URL": baseURL,
		},
	}
}

func addFirecrawlServer(mcpServers map[string]any, apiKey string) {
	if strings.TrimSpace(apiKey) == "" {
		return
	}

	mcpServers["firecrawl"] = map[string]any{
		"command": "npx",
		"args":    []string{"-y", "firecrawl-mcp"},
		"env": map[string]any{
			"FIRECRAWL_API_KEY": apiKey,
		},
	}
}

func addOpenCodeContext7Server(mcp map[string]any, apiKey string) {
	if strings.TrimSpace(apiKey) == "" {
		return
	}

	mcp["context7"] = map[string]any{
		"type":    "local",
		"command": []string{"npx", "-y", "@upstash/context7-mcp"},
		"environment": map[string]any{
			"CONTEXT7_API_KEY": apiKey,
		},
	}
}

func addOpenCodeRipgrepServer(mcp map[string]any) {
	mcp["ripgrep"] = map[string]any{
		"type":    "local",
		"command": []string{"npx", "-y", "mcp-ripgrep@latest"},
	}
}

func addOpenCodeCodeSearchServer(mcp map[string]any, entry string) {
	if strings.TrimSpace(entry) == "" {
		return
	}

	home, _ := os.UserHomeDir()
	mcp["code-search"] = map[string]any{
		"type":    "local",
		"command": []string{"node", entry, "--allowed-workspace", home},
	}
}

func addOpenCodeGitServer(mcp map[string]any) {
	mcp["git"] = map[string]any{
		"type":    "local",
		"command": []string{"uvx", "mcp-server-git"},
	}
}

func addOpenCodeBraveSearchServer(mcp map[string]any, apiKey string) {
	if strings.TrimSpace(apiKey) == "" {
		return
	}

	mcp["brave-search"] = map[string]any{
		"type":    "local",
		"command": []string{"npx", "-y", "@brave/brave-search-mcp-server", "--transport", "stdio"},
		"environment": map[string]any{
			"BRAVE_API_KEY": apiKey,
		},
	}
}

func addOpenCodeSearXNGServer(mcp map[string]any, baseURL string) {
	if strings.TrimSpace(baseURL) == "" {
		return
	}

	mcp["searxng"] = map[string]any{
		"type":    "local",
		"command": []string{"npx", "-y", "mcp-searxng"},
		"environment": map[string]any{
			"SEARXNG_URL": baseURL,
		},
	}
}

func addOpenCodeFirecrawlServer(mcp map[string]any, apiKey string) {
	if strings.TrimSpace(apiKey) == "" {
		return
	}

	mcp["firecrawl"] = map[string]any{
		"type":    "local",
		"command": []string{"npx", "-y", "firecrawl-mcp"},
		"environment": map[string]any{
			"FIRECRAWL_API_KEY": apiKey,
		},
	}
}

func addOptionalServers(mcpServers map[string]any) {
	if currentSetupOptions.Ripgrep {
		addRipgrepServer(mcpServers)
	}
	if currentSetupOptions.CodeSearch {
		addCodeSearchServer(mcpServers, currentSetupOptions.CodeSearchPath)
	}
	if currentSetupOptions.Context7 {
		addContext7Server(mcpServers, currentSetupOptions.Context7APIKey)
	}
	if currentSetupOptions.Git {
		addGitServer(mcpServers)
	}
	if currentSetupOptions.SearXNG {
		addSearXNGServer(mcpServers, currentSetupOptions.SearXNGURL)
	}
	if currentSetupOptions.BraveSearch {
		addBraveSearchServer(mcpServers, currentSetupOptions.BraveAPIKey)
	}
	if currentSetupOptions.Firecrawl {
		addFirecrawlServer(mcpServers, currentSetupOptions.FirecrawlAPIKey)
	}
}

func addOpenCodeOptionalServers(mcp map[string]any) {
	if currentSetupOptions.Ripgrep {
		addOpenCodeRipgrepServer(mcp)
	}
	if currentSetupOptions.CodeSearch {
		addOpenCodeCodeSearchServer(mcp, currentSetupOptions.CodeSearchPath)
	}
	if currentSetupOptions.Context7 {
		addOpenCodeContext7Server(mcp, currentSetupOptions.Context7APIKey)
	}
	if currentSetupOptions.Git {
		addOpenCodeGitServer(mcp)
	}
	if currentSetupOptions.SearXNG {
		addOpenCodeSearXNGServer(mcp, currentSetupOptions.SearXNGURL)
	}
	if currentSetupOptions.BraveSearch {
		addOpenCodeBraveSearchServer(mcp, currentSetupOptions.BraveAPIKey)
	}
	if currentSetupOptions.Firecrawl {
		addOpenCodeFirecrawlServer(mcp, currentSetupOptions.FirecrawlAPIKey)
	}
}

func optionalMCPServerKeys() []string {
	return []string{"context7", "ripgrep", "code-search", "git", "searxng", "brave-search", "firecrawl"}
}

func setupClaudeCode(configDir string, project bool, _ bool) (map[string]string, error) {
	skillTarget := resolveConfigDir(".claude", configDir, project)

	mcpEntry := map[string]any{
		"type":    "stdio",
		"command": "uniam",
		"args":    []string{"mcp"},
		"env":     map[string]any{},
	}

	var configPath string

	if project {
		// Project scope: write to .mcp.json in the current directory.
		// This is checked into source control and shared with the team.
		cwd, _ := os.Getwd()

		configPath = filepath.Join(cwd, ".mcp.json")
		if err := writeMCPJSON(configPath, mcpEntry); err != nil {
			return nil, err
		}
	} else {
		// User scope: write to ~/.claude.json top-level mcpServers.
		home, _ := os.UserHomeDir()

		configPath = filepath.Join(home, ".claude.json")
		if err := writeClaudeJSONUserMCP(configPath, mcpEntry); err != nil {
			return nil, err
		}
	}

	var extras []string
	if installSkill(skillTarget) {
		extras = append(extras, "skill")
	}
	claudePath, err := claudeMemoryPath(project)
	if err != nil {
		return nil, err
	}
	if err := upsertManagedBlockWithLegacy(claudePath, "claude", claudeManagedBlock(currentSetupOptions), verboseUniamLegacyPrefix); err != nil {
		return nil, fmt.Errorf("failed to write CLAUDE.md: %w", err)
	}
	extras = append(extras, "CLAUDE.md")
	extras = append(extras, enabledIntegrationLabels()...)

	return map[string]string{"message": setupMessage("Claude Code", configPath, extras...)}, nil
}

// writeMCPJSON writes an MCP server entry into a .mcp.json file (project scope).
func writeMCPJSON(configPath string, entry map[string]any) error {
	var decoded map[string]any
	if data, err := os.ReadFile(configPath); err == nil {
		if err := json.Unmarshal(data, &decoded); err != nil {
			return fmt.Errorf("failed to parse existing config: %w", err)
		}
	} else {
		decoded = make(map[string]any)
	}

	mcpServers, _ := decoded["mcpServers"].(map[string]any)
	if mcpServers == nil {
		mcpServers = make(map[string]any)
		decoded["mcpServers"] = mcpServers
	}

	mcpServers["uniam"] = entry
	addOptionalServers(mcpServers)

	data, err := json.MarshalIndent(decoded, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}

	return nil
}

// writeClaudeJSONUserMCP writes an MCP server entry into ~/.claude.json top-level mcpServers (user scope).
func writeClaudeJSONUserMCP(configPath string, entry map[string]any) error {
	var root map[string]any
	if data, err := os.ReadFile(configPath); err == nil {
		if err := json.Unmarshal(data, &root); err != nil {
			return fmt.Errorf("failed to parse existing config: %w", err)
		}
	} else {
		root = make(map[string]any)
	}

	mcpServers, _ := root["mcpServers"].(map[string]any)
	if mcpServers == nil {
		mcpServers = make(map[string]any)
		root["mcpServers"] = mcpServers
	}

	mcpServers["uniam"] = entry
	addOptionalServers(mcpServers)

	data, err := json.MarshalIndent(root, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}

	return nil
}

func setupCursor(configDir string, project bool, _ bool) (map[string]string, error) {
	target := resolveConfigDir(".cursor", configDir, project)
	configPath := filepath.Join(target, "mcp.json")

	// Read existing config or create new
	var decoded map[string]any
	if data, err := os.ReadFile(configPath); err == nil {
		if err := json.Unmarshal(data, &decoded); err != nil {
			return nil, fmt.Errorf("failed to parse existing config: %w", err)
		}
	} else {
		decoded = make(map[string]any)
	}

	// Add MCP server config
	mcpServers, ok := decoded["mcpServers"].(map[string]any)
	if !ok {
		mcpServers = make(map[string]any)
		decoded["mcpServers"] = mcpServers
	}

	mcpServers["uniam"] = map[string]any{
		"command": "uniam",
		"args":    []string{"mcp"},
	}

	addOptionalServers(mcpServers)

	// Write config
	if err := os.MkdirAll(target, 0755); err != nil {
		return nil, fmt.Errorf("failed to create config directory: %w", err)
	}

	data, err := json.MarshalIndent(decoded, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return nil, fmt.Errorf("failed to write config: %w", err)
	}

	var extras []string
	if installSkill(target) {
		extras = append(extras, "skill")
	}
	if project {
		agentsPath, pathErr := projectRootFile("AGENTS.md")
		if pathErr != nil {
			return nil, pathErr
		}
		if err := upsertManagedBlockWithLegacy(agentsPath, "agents", sharedAgentsManagedBlock(currentSetupOptions), compactUniamLegacyPrefix); err != nil {
			return nil, fmt.Errorf("failed to write project AGENTS.md: %w", err)
		}
		extras = append(extras, "AGENTS.md")
	}
	extras = append(extras, enabledIntegrationLabels()...)

	return map[string]string{"message": setupMessage("Cursor", configPath, extras...)}, nil
}

func setupWindsurf(configDir string, project bool, _ bool) (map[string]string, error) {
	if project {
		return nil, errors.New("Windsurf setup does not support --project. Install globally or use --config-dir for an explicit target")
	}

	var targets []string
	if configDir != "" {
		targets = append(targets, configDir)
	} else {
		baseDir, _ := os.UserHomeDir()

		appTarget := filepath.Join(baseDir, ".codeium", "windsurf")
		if info, err := os.Stat(appTarget); err == nil && info.IsDir() {
			targets = append(targets, appTarget)
		}

		pluginTarget := filepath.Join(baseDir, ".codeium")
		if info, err := os.Stat(pluginTarget); err == nil && info.IsDir() {
			targets = append(targets, pluginTarget)
		}
	}

	if len(targets) == 0 {
		return map[string]string{"message": "Windsurf/Cascade installation directories not found"}, nil
	}

	var installed []string
	for _, target := range targets {
		configPath := filepath.Join(target, "mcp_config.json")

		// Read existing config or create new
		var decoded map[string]any
		if data, err := os.ReadFile(configPath); err == nil {
			if err := json.Unmarshal(data, &decoded); err != nil {
				return nil, fmt.Errorf("failed to parse existing config for %s: %w", target, err)
			}
		} else {
			decoded = make(map[string]any)
		}

		// Add MCP server config
		mcpServers, ok := decoded["mcpServers"].(map[string]any)
		if !ok {
			mcpServers = make(map[string]any)
			decoded["mcpServers"] = mcpServers
		}

		mcpServers["uniam"] = map[string]any{
			"command": "uniam",
			"args":    []string{"mcp"},
		}

		addOptionalServers(mcpServers)

		// Write config
		if err := os.MkdirAll(target, 0755); err != nil {
			return nil, fmt.Errorf("failed to create config directory %s: %w", target, err)
		}

		data, err := json.MarshalIndent(decoded, "", "  ")
		if err != nil {
			return nil, fmt.Errorf("failed to marshal config for %s: %w", target, err)
		}

		if err := os.WriteFile(configPath, data, 0644); err != nil {
			return nil, fmt.Errorf("failed to write config for %s: %w", target, err)
		}

		var extras []string
		if installSkill(target) {
			extras = append(extras, "skill")
		}
		extras = append(extras, enabledIntegrationLabels()...)
		msg := setupMessage("Windsurf", configPath, extras...)
		installed = append(installed, msg)
	}

	return map[string]string{"message": strings.Join(installed, "\n")}, nil
}

func setupAntigravity(configDir string, project bool, _ bool) (map[string]string, error) {
	var target string
	if configDir != "" {
		target = configDir
	} else if project {
		cwd, _ := os.Getwd()
		target = filepath.Join(cwd, ".gemini", "antigravity")
	} else {
		home, _ := os.UserHomeDir()
		target = filepath.Join(home, ".gemini", "antigravity")
	}

	configPath := filepath.Join(target, "mcp_config.json")

	// Read existing config or create new
	var decoded map[string]any
	if data, err := os.ReadFile(configPath); err == nil {
		if err := json.Unmarshal(data, &decoded); err != nil {
			return nil, fmt.Errorf("failed to parse existing config: %w", err)
		}
	} else {
		decoded = make(map[string]any)
	}

	// Add MCP server config
	mcpServers, ok := decoded["mcpServers"].(map[string]any)
	if !ok {
		mcpServers = make(map[string]any)
		decoded["mcpServers"] = mcpServers
	}

	mcpServers["uniam"] = map[string]any{
		"command": "uniam",
		"args":    []string{"mcp"},
	}

	addOptionalServers(mcpServers)

	// Write config
	if err := os.MkdirAll(target, 0755); err != nil {
		return nil, fmt.Errorf("failed to create config directory: %w", err)
	}

	data, err := json.MarshalIndent(decoded, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return nil, fmt.Errorf("failed to write config: %w", err)
	}

	var extras []string
	if installSkill(target) {
		extras = append(extras, "skill")
	}
	extras = append(extras, enabledIntegrationLabels()...)

	return map[string]string{"message": setupMessage("Antigravity", configPath, extras...)}, nil
}

func setupCodex(configDir string, project bool, _ bool) (map[string]string, error) {
	target := resolveConfigDir(".codex", configDir, project)
	configPath := filepath.Join(target, "config.toml")
	agentsPath := filepath.Join(target, "AGENTS.md")

	if err := os.MkdirAll(target, 0755); err != nil {
		return nil, fmt.Errorf("failed to create config directory: %w", err)
	}

	const uniamTOML = "\n[mcp_servers.uniam]\ncommand = \"uniam\"\nargs = [\"mcp\"]\n"
	existing, _ := os.ReadFile(configPath)
	text := string(existing)
	text = upsertCodexServerBlock(text, "uniam", uniamTOML)
	for _, block := range codexOptionalBlocks() {
		text = upsertCodexServerBlock(text, block.name, block.body)
	}
	if err := os.WriteFile(configPath, []byte(text), 0644); err != nil {
		return nil, fmt.Errorf("failed to write config: %w", err)
	}

	if err := upsertManagedBlockWithLegacy(agentsPath, "codex", codexManagedBlock(currentSetupOptions), compactUniamLegacyPrefix); err != nil {
		return nil, fmt.Errorf("failed to write AGENTS.md: %w", err)
	}

	var extras []string
	if installSkill(target) {
		extras = append(extras, "skill")
	}
	extras = append(extras, enabledIntegrationLabels()...)
	extras = append([]string{"MCP", "AGENTS.md"}, extras...)

	return map[string]string{"message": setupMessage("Codex", target, extras...)}, nil
}

func upsertCodexServerBlock(existing string, name string, replacement string) string {
	header := "[mcp_servers." + name + "]\n"
	start := strings.Index(existing, "\n"+header)
	if start == -1 {
		start = strings.Index(existing, header)
	}
	if start == -1 {
		return existing + replacement
	}

	searchFrom := start + 1
	if start == 0 {
		searchFrom = len(header)
	}

	next := strings.Index(existing[searchFrom:], "\n[mcp_servers.")
	if next == -1 {
		return existing[:start] + replacement
	}

	end := searchFrom + next
	return existing[:start] + replacement + existing[end:]
}

type codexServerBlock struct {
	name string
	body string
}

func codexOptionalBlocks() []codexServerBlock {
	var blocks []codexServerBlock
	if currentSetupOptions.Ripgrep {
		blocks = append(blocks, codexServerBlock{
			name: "ripgrep",
			body: "\n[mcp_servers.ripgrep]\ncommand = \"npx\"\nargs = [\"-y\", \"mcp-ripgrep@latest\"]\n",
		})
	}
	if currentSetupOptions.CodeSearch && strings.TrimSpace(currentSetupOptions.CodeSearchPath) != "" {
		home, _ := os.UserHomeDir()
		blocks = append(blocks, codexServerBlock{
			name: "code-search",
			body: fmt.Sprintf("\n[mcp_servers.code-search]\ncommand = \"node\"\nargs = [%q, \"--allowed-workspace\", %q]\n", currentSetupOptions.CodeSearchPath, home),
		})
	}
	if currentSetupOptions.Context7 && strings.TrimSpace(currentSetupOptions.Context7APIKey) != "" {
		blocks = append(blocks, codexServerBlock{
			name: "context7",
			body: fmt.Sprintf("\n[mcp_servers.context7]\ncommand = \"npx\"\nargs = [\"-y\", \"@upstash/context7-mcp\"]\nenv = { CONTEXT7_API_KEY = %q }\n", currentSetupOptions.Context7APIKey),
		})
	}
	if currentSetupOptions.Git {
		blocks = append(blocks, codexServerBlock{
			name: "git",
			body: "\n[mcp_servers.git]\ncommand = \"uvx\"\nargs = [\"mcp-server-git\"]\n",
		})
	}
	if currentSetupOptions.SearXNG && strings.TrimSpace(currentSetupOptions.SearXNGURL) != "" {
		blocks = append(blocks, codexServerBlock{
			name: "searxng",
			body: fmt.Sprintf("\n[mcp_servers.searxng]\ncommand = \"npx\"\nargs = [\"-y\", \"mcp-searxng\"]\nenv = { SEARXNG_URL = %q }\n", currentSetupOptions.SearXNGURL),
		})
	}
	if currentSetupOptions.BraveSearch && strings.TrimSpace(currentSetupOptions.BraveAPIKey) != "" {
		blocks = append(blocks, codexServerBlock{
			name: "brave-search",
			body: fmt.Sprintf("\n[mcp_servers.brave-search]\ncommand = \"npx\"\nargs = [\"-y\", \"@brave/brave-search-mcp-server\", \"--transport\", \"stdio\"]\nenv = { BRAVE_API_KEY = %q }\n", currentSetupOptions.BraveAPIKey),
		})
	}
	if currentSetupOptions.Firecrawl && strings.TrimSpace(currentSetupOptions.FirecrawlAPIKey) != "" {
		blocks = append(blocks, codexServerBlock{
			name: "firecrawl",
			body: fmt.Sprintf("\n[mcp_servers.firecrawl]\ncommand = \"npx\"\nargs = [\"-y\", \"firecrawl-mcp\"]\nenv = { FIRECRAWL_API_KEY = %q }\n", currentSetupOptions.FirecrawlAPIKey),
		})
	}

	return blocks
}

func setupOpenCode(project bool, _ bool) (map[string]string, error) {
	target, configPath, instructionRef, err := openCodeInstallPaths(project)
	if err != nil {
		return nil, err
	}
	agentsPath, err := openCodeAgentsPath(project, target)
	if err != nil {
		return nil, err
	}

	decoded, err := readJSONMap(configPath)
	if err != nil {
		return nil, err
	}

	// OpenCode uses a "mcp" key (not "mcpServers"), and command must be an array.
	mcp, _ := decoded["mcp"].(map[string]any)
	if mcp == nil {
		mcp = make(map[string]any)
		decoded["mcp"] = mcp
	}
	mcp["uniam"] = map[string]any{
		"type":    "local",
		"command": []string{"uniam", "mcp"},
	}
	addOpenCodeOptionalServers(mcp)

	instructions, _ := decoded["instructions"].([]any)
	if instructions != nil {
		filtered := make([]any, 0, len(instructions))
		for _, instruction := range instructions {
			value, ok := instruction.(string)
			if ok && value == instructionRef {
				continue
			}

			filtered = append(filtered, instruction)
		}
		if len(filtered) == 0 {
			delete(decoded, "instructions")
		} else {
			decoded["instructions"] = filtered
		}
	}

	if err := os.MkdirAll(target, 0755); err != nil {
		return nil, fmt.Errorf("failed to create config directory: %w", err)
	}

	if _, err := removeFileIfExists(filepath.Join(target, openCodeInstructionsFileName)); err != nil {
		return nil, fmt.Errorf("failed to remove legacy OpenCode instructions: %w", err)
	}

	openCodeMarker := "opencode"
	openCodeBlock := openCodeAgentsManagedBlock(currentSetupOptions)
	if project {
		openCodeMarker = "agents"
		openCodeBlock = sharedAgentsManagedBlock(currentSetupOptions)
	}
	if err := upsertManagedBlockWithLegacy(agentsPath, openCodeMarker, openCodeBlock, compactUniamLegacyPrefix); err != nil {
		return nil, fmt.Errorf("failed to write OpenCode AGENTS.md: %w", err)
	}

	if !installSkill(target) {
		return nil, errors.New("failed to install OpenCode skill")
	}

	if err := writeJSONMap(configPath, decoded); err != nil {
		return nil, err
	}

	var extras []string
	extras = append(extras, enabledIntegrationLabels()...)
	extras = append([]string{"AGENTS.md", "skill"}, extras...)
	return map[string]string{"message": setupMessage("OpenCode", target, extras...)}, nil
}

// removeServersFromMCPJSON reads a JSON config file, removes the specified keys from
// "mcpServers", and writes the result back.
func removeServersFromMCPJSON(configPath string, keysToRemove []string) error {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("failed to read config: %w", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(data, &decoded); err != nil {
		return fmt.Errorf("failed to parse config: %w", err)
	}

	if mcpServers, ok := decoded["mcpServers"].(map[string]any); ok {
		for _, key := range keysToRemove {
			delete(mcpServers, key)
		}
	}

	newData, err := json.MarshalIndent(decoded, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(configPath, newData, 0644); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}

	return nil
}

func uninstallClaudeCode(configDir string, project bool, _ bool) (map[string]string, error) {
	skillTarget := resolveConfigDir(".claude", configDir, project)

	var configPath string

	if project {
		cwd, _ := os.Getwd()

		configPath = filepath.Join(cwd, ".mcp.json")
		if _, err := os.Stat(configPath); os.IsNotExist(err) {
			return map[string]string{"message": "Uniam not found in project .mcp.json"}, nil
		}
	} else {
		home, _ := os.UserHomeDir()

		configPath = filepath.Join(home, ".claude.json")
		if _, err := os.Stat(configPath); os.IsNotExist(err) {
			return map[string]string{"message": "Uniam not found in Claude Code config"}, nil
		}
	}

	if err := removeServersFromMCPJSON(configPath, append([]string{"uniam"}, optionalMCPServerKeys()...)); err != nil {
		return nil, err
	}

	var extras []string
	if uninstallSkill(skillTarget) {
		extras = append(extras, "skill")
	}
	claudePath, pathErr := claudeMemoryPath(project)
	if pathErr != nil {
		return nil, pathErr
	}
	if err := removeManagedBlock(claudePath, "claude"); err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("failed to remove CLAUDE.md block: %w", err)
	}
	extras = append(extras, "CLAUDE.md")

	return map[string]string{"message": uninstallMessage("Claude Code", configPath, extras...)}, nil
}

func uninstallCursor(configDir string, project bool, _ bool) (map[string]string, error) {
	target := resolveConfigDir(".cursor", configDir, project)
	configPath := filepath.Join(target, "mcp.json")

	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return map[string]string{"message": "Uniam not found in Cursor config"}, nil
	}

	if err := removeServersFromMCPJSON(configPath, append([]string{"uniam"}, optionalMCPServerKeys()...)); err != nil {
		return nil, err
	}

	var extras []string
	if uninstallSkill(target) {
		extras = append(extras, "skill")
	}
	if project {
		agentsPath, pathErr := projectRootFile("AGENTS.md")
		if pathErr != nil {
			return nil, pathErr
		}
		if err := removeManagedBlock(agentsPath, "agents"); err != nil && !errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("failed to remove project AGENTS.md block: %w", err)
		}
		extras = append(extras, "AGENTS.md")
	}

	return map[string]string{"message": uninstallMessage("Cursor", configPath, extras...)}, nil
}

func uninstallWindsurf(configDir string, project bool, _ bool) (map[string]string, error) {
	if project {
		return nil, errors.New("Windsurf uninstall does not support --project. Uninstall globally or use --config-dir for an explicit target")
	}

	var targets []string
	if configDir != "" {
		targets = append(targets, configDir)
	} else {
		baseDir, _ := os.UserHomeDir()

		appTarget := filepath.Join(baseDir, ".codeium", "windsurf")
		if info, err := os.Stat(appTarget); err == nil && info.IsDir() {
			targets = append(targets, appTarget)
		}

		pluginTarget := filepath.Join(baseDir, ".codeium")
		if info, err := os.Stat(pluginTarget); err == nil && info.IsDir() {
			targets = append(targets, pluginTarget)
		}
	}

	if len(targets) == 0 {
		return map[string]string{"message": "Windsurf/Cascade installation directory not found"}, nil
	}

	var removed []string
	for _, target := range targets {
		configPath := filepath.Join(target, "mcp_config.json")

		if _, err := os.Stat(configPath); os.IsNotExist(err) {
			continue
		}

		if err := removeServersFromMCPJSON(configPath, append([]string{"uniam"}, optionalMCPServerKeys()...)); err != nil {
			return nil, err
		}

		var extras []string
		if uninstallSkill(target) {
			extras = append(extras, "skill")
		}
		msg := uninstallMessage("Windsurf", configPath, extras...)
		removed = append(removed, msg)
	}

	if len(removed) == 0 {
		return map[string]string{"message": "Uniam not found in Windsurf configs"}, nil
	}

	return map[string]string{"message": strings.Join(removed, "\n")}, nil
}

func uninstallAntigravity(configDir string, project bool, _ bool) (map[string]string, error) {
	var target string
	if configDir != "" {
		target = configDir
	} else if project {
		cwd, _ := os.Getwd()
		target = filepath.Join(cwd, ".gemini", "antigravity")
	} else {
		home, _ := os.UserHomeDir()
		target = filepath.Join(home, ".gemini", "antigravity")
	}

	configPath := filepath.Join(target, "mcp_config.json")

	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return map[string]string{"message": "Uniam not found in Antigravity config"}, nil
	}

	if err := removeServersFromMCPJSON(configPath, append([]string{"uniam"}, optionalMCPServerKeys()...)); err != nil {
		return nil, err
	}

	var extras []string
	if uninstallSkill(target) {
		extras = append(extras, "skill")
	}

	return map[string]string{"message": uninstallMessage("Antigravity", configPath, extras...)}, nil
}

func uninstallCodex(configDir string, project bool, _ bool) (map[string]string, error) {
	target := resolveConfigDir(".codex", configDir, project)

	msg := "Codex uninstall: manually remove Uniam and optional MCP entries from .codex/config.toml"

	if err := removeManagedBlock(filepath.Join(target, "AGENTS.md"), "codex"); err == nil {
		msg += ". Removed AGENTS.md block."
	}

	if uninstallSkill(target) {
		msg += ". Removed skill."
	}

	return map[string]string{"message": msg}, nil
}

func uninstallOpenCode(project bool) (map[string]string, error) {
	target, configPath, instructionRef, err := openCodeInstallPaths(project)
	if err != nil {
		return nil, err
	}
	agentsPath, err := openCodeAgentsPath(project, target)
	if err != nil {
		return nil, err
	}

	_, statErr := os.Stat(configPath)
	configExists := !errors.Is(statErr, os.ErrNotExist)
	decoded := make(map[string]any)
	if configExists {
		decoded, err = readJSONMap(configPath)
		if err != nil {
			return nil, err
		}
	}

	if mcp, ok := decoded["mcp"].(map[string]any); ok {
		delete(mcp, "uniam")
		for _, key := range optionalMCPServerKeys() {
			delete(mcp, key)
		}
	}

	if instructions, ok := decoded["instructions"].([]any); ok {
		filtered := make([]any, 0, len(instructions))
		for _, instruction := range instructions {
			value, ok := instruction.(string)
			if ok && value == instructionRef {
				continue
			}

			filtered = append(filtered, instruction)
		}
		if len(filtered) == 0 {
			delete(decoded, "instructions")
		} else {
			decoded["instructions"] = filtered
		}
	}

	if configExists {
		if err := writeJSONMap(configPath, decoded); err != nil {
			return nil, err
		}
	}

	if _, err := removeFileIfExists(filepath.Join(target, openCodeInstructionsFileName)); err != nil {
		return nil, fmt.Errorf("failed to remove OpenCode instructions: %w", err)
	}
	openCodeMarker := "opencode"
	if project {
		openCodeMarker = "agents"
	}
	if err := removeManagedBlock(agentsPath, openCodeMarker); err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("failed to remove OpenCode AGENTS.md block: %w", err)
	}
	if !uninstallSkill(target) {
		skillPath := filepath.Join(target, "skills", "uniam", "SKILL.md")
		if _, err := os.Stat(skillPath); err == nil {
			return nil, fmt.Errorf("failed to remove OpenCode skill from %s", target)
		}
	}

	return map[string]string{
		"message": uninstallMessage("OpenCode", target, "AGENTS.md", "skill"),
	}, nil
}

func getCopilotConfigPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	switch runtime.GOOS {
	case "darwin":
		return filepath.Join(home, "Library", "Application Support", "Code", "User", "globalStorage", "github.copilot-chat", "mcp.json"), nil
	case "windows":
		appData := os.Getenv("APPDATA")
		if appData == "" {
			appData = filepath.Join(home, "AppData", "Roaming")
		}
		return filepath.Join(appData, "Code", "User", "globalStorage", "github.copilot-chat", "mcp.json"), nil
	default:
		// Linux
		return filepath.Join(home, ".config", "Code", "User", "globalStorage", "github.copilot-chat", "mcp.json"), nil
	}
}

func setupCopilot(_ string, project bool, _ bool) (map[string]string, error) {
	var (
		configPath string
		agentHome  string
		err        error
	)
	if project {
		cwd, cwdErr := os.Getwd()
		if cwdErr != nil {
			return nil, fmt.Errorf("failed to resolve current directory: %w", cwdErr)
		}
		configPath = filepath.Join(cwd, ".mcp.json")
		agentHome = filepath.Join(cwd, ".github")
	} else {
		configPath, err = getCopilotConfigPath()
		if err != nil {
			return nil, fmt.Errorf("failed to get copilot config path: %w", err)
		}
		home, _ := os.UserHomeDir()
		agentHome = filepath.Join(home, ".uniam")
	}

	if err := os.MkdirAll(filepath.Dir(configPath), 0755); err != nil {
		return nil, fmt.Errorf("failed to create copilot config directory: %w", err)
	}

	mcpEntry := map[string]any{
		"command": "uniam",
		"args":    []string{"mcp"},
	}

	if err := writeMCPJSON(configPath, mcpEntry); err != nil {
		return nil, fmt.Errorf("failed to write mcp.json: %w", err)
	}

	if !installSkill(agentHome) {
		return nil, fmt.Errorf("failed to install skill in %s", agentHome)
	}
	var extras []string
	extras = append(extras, "skill")
	extras = append(extras, enabledIntegrationLabels()...)

	msg := setupMessage("GitHub Copilot", configPath, extras...)

	if project {
		instructionsPath := filepath.Join(agentHome, "copilot-instructions.md")
		if err := os.MkdirAll(agentHome, 0755); err != nil {
			return nil, fmt.Errorf("failed to create project .github directory: %w", err)
		}
		if err := upsertManagedBlock(instructionsPath, "copilot", copilotRepoInstructionsManagedBlock(currentSetupOptions)); err != nil {
			return nil, fmt.Errorf("failed to write copilot instructions: %w", err)
		}
		msg += "\nRepository instructions installed in " + instructionsPath
	} else {
		msg += "\n\033[33mIMPORTANT: VS Code Copilot does not automatically read global skill files.\033[0m\n"
		msg += "Please add the instructions from \033[36m" + filepath.Join(agentHome, "skills") + "\033[0m\n"
		msg += "directly into your VS Code Copilot extension settings (e.g. Chat Rules) to ensure proper agent behavior."
	}

	return map[string]string{"message": msg}, nil
}

func uninstallCopilot(_ string, project bool, _ bool) (map[string]string, error) {
	var (
		configPath string
		err        error
	)
	if project {
		cwd, cwdErr := os.Getwd()
		if cwdErr != nil {
			return nil, fmt.Errorf("failed to resolve current directory: %w", cwdErr)
		}
		configPath = filepath.Join(cwd, ".mcp.json")
	} else {
		configPath, err = getCopilotConfigPath()
		if err != nil {
			return nil, fmt.Errorf("failed to get copilot config path: %w", err)
		}
	}

	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return map[string]string{"message": "Uniam not found in Copilot config"}, nil
	}

	if err := removeServersFromMCPJSON(configPath, append([]string{"uniam"}, optionalMCPServerKeys()...)); err != nil {
		return nil, err
	}

	if project {
		cwd, _ := os.Getwd()
		agentHome := filepath.Join(cwd, ".github")
		if !uninstallSkill(agentHome) {
			skillPath := filepath.Join(agentHome, "skills", "uniam", "SKILL.md")
			if _, err := os.Stat(skillPath); err == nil {
				return nil, fmt.Errorf("failed to remove Copilot skill from %s", agentHome)
			}
		}
		if err := removeManagedBlock(filepath.Join(agentHome, "copilot-instructions.md"), "copilot"); err != nil && !errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("failed to remove copilot instructions: %w", err)
		}
	}

	return map[string]string{
		"message": uninstallMessage("GitHub Copilot", configPath),
	}, nil
}

func getGeminiCliConfigPath(project bool) (string, error) {
	if project {
		cwd, err := os.Getwd()
		if err != nil {
			return "", err
		}
		return filepath.Join(cwd, ".gemini", "settings.json"), nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".gemini", "settings.json"), nil
}

func setupGeminiCli(_ string, project bool, _ bool) (map[string]string, error) {
	configPath, err := getGeminiCliConfigPath(project)
	if err != nil {
		return nil, fmt.Errorf("failed to get gemini-cli config path: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(configPath), 0755); err != nil {
		return nil, fmt.Errorf("failed to create gemini-cli config directory: %w", err)
	}

	mcpEntry := map[string]any{
		"command": "uniam",
		"args":    []string{"mcp"},
	}

	// Uses same root JSON structure (mcpServers at top level) as Claude
	if err := writeClaudeJSONUserMCP(configPath, mcpEntry); err != nil {
		return nil, fmt.Errorf("failed to write settings.json: %w", err)
	}
	if err := ensureGeminiContextFilenames(configPath); err != nil {
		return nil, err
	}

	var agentHome string
	if project {
		cwd, _ := os.Getwd()
		agentHome = filepath.Join(cwd, ".gemini")
	} else {
		home, _ := os.UserHomeDir()
		agentHome = filepath.Join(home, ".gemini")
	}

	if !installSkill(agentHome) {
		return nil, fmt.Errorf("failed to install skill in %s", agentHome)
	}
	agentsPath, pathErr := geminiAgentsPath(project)
	if pathErr != nil {
		return nil, pathErr
	}
	blockMarker := "gemini"
	block := geminiManagedBlock(currentSetupOptions)
	if project {
		blockMarker = "agents"
		block = sharedAgentsManagedBlock(currentSetupOptions)
	}
	if err := upsertManagedBlockWithLegacy(agentsPath, blockMarker, block, compactUniamLegacyPrefix); err != nil {
		return nil, fmt.Errorf("failed to write Gemini CLI AGENTS.md: %w", err)
	}
	var extras []string
	extras = append(extras, "skill")
	extras = append(extras, "AGENTS.md")
	extras = append(extras, enabledIntegrationLabels()...)

	return map[string]string{"message": setupMessage("Gemini CLI", configPath, extras...)}, nil
}

func uninstallGeminiCli(_ string, project bool, _ bool) (map[string]string, error) {
	configPath, err := getGeminiCliConfigPath(project)
	if err != nil {
		return nil, fmt.Errorf("failed to get gemini-cli config path: %w", err)
	}

	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return map[string]string{"message": "Uniam not found in gemini-cli config"}, nil
	}

	if err := removeServersFromMCPJSON(configPath, append([]string{"uniam"}, optionalMCPServerKeys()...)); err != nil {
		return nil, err
	}

	var agentHome string
	if project {
		cwd, _ := os.Getwd()
		agentHome = filepath.Join(cwd, ".gemini")
	} else {
		home, _ := os.UserHomeDir()
		agentHome = filepath.Join(home, ".gemini")
	}

	if !uninstallSkill(agentHome) {
		skillPath := filepath.Join(agentHome, "skills", "uniam", "SKILL.md")
		if _, err := os.Stat(skillPath); err == nil {
			return nil, fmt.Errorf("failed to remove Gemini CLI skill from %s", agentHome)
		}
	}
	agentsPath, pathErr := geminiAgentsPath(project)
	if pathErr != nil {
		return nil, pathErr
	}
	blockMarker := "gemini"
	if project {
		blockMarker = "agents"
	}
	if err := removeManagedBlock(agentsPath, blockMarker); err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("failed to remove Gemini CLI AGENTS.md block: %w", err)
	}

	return map[string]string{
		"message": uninstallMessage("Gemini CLI", configPath, "AGENTS.md"),
	}, nil
}
