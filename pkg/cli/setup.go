package cli

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/spf13/cobra"
)

var (
	setupConfigDir string
	setupProject   bool
)

type agentFunc func(configDir string, project bool, fastContext bool) (map[string]string, error)

func runAgentCmd(agent string, handlers map[string]agentFunc, configDir string, project bool, isSetup bool) {
	fn, ok := handlers[agent]
	if !ok {
		fmt.Fprintf(os.Stderr, "Error: unknown agent: %s\n", agent)
		fmt.Fprintf(os.Stderr, "Supported agents: claude, cursor, windsurf, antigravity, codex, codex-cli, opencode, roocode, copilot, gemini-cli\n")
		os.Exit(1)
	}

	fastContext := false
	if isSetup && agent != "windsurf" && agent != "opencode" {
		fmt.Print("install fast context? yes/no (default no): ")
		reader := bufio.NewReader(os.Stdin)
		response, _ := reader.ReadString('\n')
		response = strings.TrimSpace(strings.ToLower(response))
		if response == "yes" || response == "y" {
			fastContext = true
		}
	}

	result, err := fn(configDir, project, fastContext)
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
		runAgentCmd(args[0], map[string]agentFunc{
			"claude":      setupClaudeCode,
			"claude-code": setupClaudeCode,
			"cursor":      setupCursor,
			"windsurf":    setupWindsurf,
			"antigravity": setupAntigravity,
			"codex":       setupCodex,
			"codex-cli":   setupCodex,
			"copilot":     setupCopilot,
			"gemini-cli":  setupGeminiCli,
			"opencode": func(configDir string, project bool, fast bool) (map[string]string, error) {
				if configDir != "" {
					return nil, errors.New("OpenCode setup does not support --config-dir. It installs only to ~/.config/opencode")
				}
				return setupOpenCode(project, fast)
			},
			"roo":     setupRooCode,
			"roocode": setupRooCode,
		}, setupConfigDir, setupProject, true)
	},
}

var uninstallCmd = &cobra.Command{
	Use:   "uninstall [agent]",
	Short: "Remove Uniam hooks for an agent",
	Args:  cobra.ExactArgs(1),
	//nolint:revive
	Run: func(cmd *cobra.Command, args []string) {
		runAgentCmd(args[0], map[string]agentFunc{
			"claude":      uninstallClaudeCode,
			"claude-code": uninstallClaudeCode,
			"cursor":      uninstallCursor,
			"windsurf":    uninstallWindsurf,
			"antigravity": uninstallAntigravity,
			"codex":       uninstallCodex,
			"codex-cli":   uninstallCodex,
			"copilot":     uninstallCopilot,
			"gemini-cli":  uninstallGeminiCli,
			"opencode": func(configDir string, project bool, _ bool) (map[string]string, error) {
				if configDir != "" {
					return nil, errors.New("OpenCode uninstall does not support --config-dir. It removes only from ~/.config/opencode")
				}
				return uninstallOpenCode(project)
			},
			"roo":     uninstallRooCode,
			"roocode": uninstallRooCode,
		}, setupConfigDir, setupProject, false)
	},
}

func init() {
	setupCmd.Flags().StringVar(&setupConfigDir, "config-dir", "", "Path to agent config directory")
	setupCmd.Flags().BoolVarP(&setupProject, "project", "p", false, "Install in current project instead of globally")
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

func readJSONMap(path string) (map[string]any, error) {
	if data, err := os.ReadFile(path); err == nil {
		var config map[string]any
		if err := json.Unmarshal(data, &config); err != nil {
			return nil, fmt.Errorf("failed to parse existing config: %w", err)
		}

		return config, nil
	}

	return make(map[string]any), nil
}

func writeJSONMap(path string, config map[string]any) error {
	data, err := json.MarshalIndent(config, "", "  ")
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

func installOpenCodeInstructions(target string) error {
	path := filepath.Join(target, openCodeInstructionsFileName)
	return os.WriteFile(path, openCodeInstructionsContent, 0644)
}

func installOpenCodePlugin(target string) error {
	pluginsDir := filepath.Join(target, "plugins")
	if err := os.MkdirAll(pluginsDir, 0755); err != nil {
		return fmt.Errorf("failed to create OpenCode plugin directory: %w", err)
	}

	path := filepath.Join(pluginsDir, openCodePluginFileName)
	return os.WriteFile(path, openCodePluginContent, 0644)
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

func addFastContextServers(mcpServers map[string]any) {
	mcpServers["ripgrep"] = map[string]any{
		"command": "npx",
		"args":    []string{"-y", "mcp-ripgrep@latest"},
	}

	fmt.Println("Installing code-search-mcp...")
	entry, err := installCodeSearch()
	if err != nil {
		fmt.Fprintf(os.Stderr, "  Warning: code-search-mcp install failed: %v\n", err)
		fmt.Fprintln(os.Stderr, "  Skipping code-search-mcp. You can retry by running 'uniam setup' again.")
		return
	}

	home, _ := os.UserHomeDir()
	mcpServers["code-search"] = map[string]any{
		"command": "node",
		"args":    []string{entry, "--allowed-workspace", home},
	}
}

func setupClaudeCode(configDir string, project bool, fastContext bool) (map[string]string, error) {
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
		if err := writeMCPJSON(configPath, mcpEntry, fastContext); err != nil {
			return nil, err
		}
	} else {
		// User scope: write to ~/.claude.json top-level mcpServers.
		home, _ := os.UserHomeDir()

		configPath = filepath.Join(home, ".claude.json")
		if err := writeClaudeJSONUserMCP(configPath, mcpEntry, fastContext); err != nil {
			return nil, err
		}
	}

	msg := "Installed Uniam MCP server in " + configPath
	if installSkill(skillTarget) {
		msg += " and skill" //nolint:goconst
	}
	if fastContext && installFastContextSkill(skillTarget) {
		msg += " with fast context"
	}

	return map[string]string{"message": msg}, nil
}

// writeMCPJSON writes an MCP server entry into a .mcp.json file (project scope).
func writeMCPJSON(configPath string, entry map[string]any, fastContext bool) error {
	var config map[string]any
	if data, err := os.ReadFile(configPath); err == nil {
		if err := json.Unmarshal(data, &config); err != nil {
			return fmt.Errorf("failed to parse existing config: %w", err)
		}
	} else {
		config = make(map[string]any)
	}

	mcpServers, _ := config["mcpServers"].(map[string]any)
	if mcpServers == nil {
		mcpServers = make(map[string]any)
		config["mcpServers"] = mcpServers
	}

	mcpServers["uniam"] = entry

	if fastContext {
		addFastContextServers(mcpServers)
	}

	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}

	return nil
}

// writeClaudeJSONUserMCP writes an MCP server entry into ~/.claude.json top-level mcpServers (user scope).
func writeClaudeJSONUserMCP(configPath string, entry map[string]any, fastContext bool) error {
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
	if fastContext {
		addFastContextServers(mcpServers)
	}

	data, err := json.MarshalIndent(root, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}

	return nil
}

func setupCursor(configDir string, project bool, fastContext bool) (map[string]string, error) {
	target := resolveConfigDir(".cursor", configDir, project)
	configPath := filepath.Join(target, "mcp.json")

	// Read existing config or create new
	var config map[string]any
	if data, err := os.ReadFile(configPath); err == nil {
		if err := json.Unmarshal(data, &config); err != nil {
			return nil, fmt.Errorf("failed to parse existing config: %w", err)
		}
	} else {
		config = make(map[string]any)
	}

	// Add MCP server config
	mcpServers, ok := config["mcpServers"].(map[string]any)
	if !ok {
		mcpServers = make(map[string]any)
		config["mcpServers"] = mcpServers
	}

	mcpServers["uniam"] = map[string]any{
		"command": "uniam",
		"args":    []string{"mcp"},
	}

	if fastContext {
		addFastContextServers(mcpServers)
	}

	// Write config
	if err := os.MkdirAll(target, 0755); err != nil {
		return nil, fmt.Errorf("failed to create config directory: %w", err)
	}

	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return nil, fmt.Errorf("failed to write config: %w", err)
	}

	msg := "Installed Uniam MCP server in " + configPath
	if installSkill(target) {
		msg += " and skill"
	}
	if fastContext && installFastContextSkill(target) {
		msg += " with fast context"
	}

	return map[string]string{"message": msg}, nil
}

func setupWindsurf(configDir string, project bool, fastContext bool) (map[string]string, error) {
	var targets []string
	if configDir != "" {
		targets = append(targets, configDir)
	} else {
		var baseDir string
		if project {
			baseDir, _ = os.Getwd()
		} else {
			baseDir, _ = os.UserHomeDir()
		}

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
		var config map[string]any
		if data, err := os.ReadFile(configPath); err == nil {
			if err := json.Unmarshal(data, &config); err != nil {
				return nil, fmt.Errorf("failed to parse existing config for %s: %w", target, err)
			}
		} else {
			config = make(map[string]any)
		}

		// Add MCP server config
		mcpServers, ok := config["mcpServers"].(map[string]any)
		if !ok {
			mcpServers = make(map[string]any)
			config["mcpServers"] = mcpServers
		}

		mcpServers["uniam"] = map[string]any{
			"command": "uniam",
			"args":    []string{"mcp"},
		}

		// Write config
		if err := os.MkdirAll(target, 0755); err != nil {
			return nil, fmt.Errorf("failed to create config directory %s: %w", target, err)
		}

		data, err := json.MarshalIndent(config, "", "  ")
		if err != nil {
			return nil, fmt.Errorf("failed to marshal config for %s: %w", target, err)
		}

		if err := os.WriteFile(configPath, data, 0644); err != nil {
			return nil, fmt.Errorf("failed to write config for %s: %w", target, err)
		}

		msg := "Installed Uniam MCP server in " + configPath
		if installSkill(target) {
			msg += " and skill"
		}
		installed = append(installed, msg)
	}

	return map[string]string{"message": strings.Join(installed, "\n")}, nil
}

func setupAntigravity(configDir string, project bool, fastContext bool) (map[string]string, error) {
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
	var config map[string]any
	if data, err := os.ReadFile(configPath); err == nil {
		if err := json.Unmarshal(data, &config); err != nil {
			return nil, fmt.Errorf("failed to parse existing config: %w", err)
		}
	} else {
		config = make(map[string]any)
	}

	// Add MCP server config
	mcpServers, ok := config["mcpServers"].(map[string]any)
	if !ok {
		mcpServers = make(map[string]any)
		config["mcpServers"] = mcpServers
	}

	mcpServers["uniam"] = map[string]any{
		"command": "uniam",
		"args":    []string{"mcp"},
	}

	if fastContext {
		addFastContextServers(mcpServers)
	}

	// Write config
	if err := os.MkdirAll(target, 0755); err != nil {
		return nil, fmt.Errorf("failed to create config directory: %w", err)
	}

	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return nil, fmt.Errorf("failed to write config: %w", err)
	}

	msg := "Installed Uniam MCP server in " + configPath
	if installSkill(target) {
		msg += " and skill"
	}

	return map[string]string{"message": msg}, nil
}

func setupCodex(configDir string, project bool, fastContext bool) (map[string]string, error) {
	target := resolveConfigDir(".codex", configDir, project)
	configPath := filepath.Join(target, "config.toml")
	agentsPath := filepath.Join(target, "AGENTS.md")

	if err := os.MkdirAll(target, 0755); err != nil {
		return nil, fmt.Errorf("failed to create config directory: %w", err)
	}

	// Codex uses [mcp_servers.<name>] in config.toml.
	// Only append the block if it's not already present (idempotent).
	const uniamTOML = "\n[mcp_servers.uniam]\ncommand = \"uniam\"\nargs = [\"mcp\"]\n"

	existing, _ := os.ReadFile(configPath)
	if !bytes.Contains(existing, []byte("[mcp_servers.uniam]")) {
		f, err := os.OpenFile(configPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			return nil, fmt.Errorf("failed to open config file: %w", err)
		}

		_, writeErr := f.WriteString(uniamTOML)
		closeErr := f.Close()

		if writeErr != nil {
			return nil, fmt.Errorf("failed to write config: %w", writeErr)
		}

		if closeErr != nil {
			return nil, fmt.Errorf("failed to close config file: %w", closeErr)
		}
	}

	// Add to AGENTS.md (idempotent).
	const uniamAgentsSection = "## Uniam\n\nUse Uniam for cross-session memory.\n\n" +
		"Required:\n" +
		"- Retrieve before meaningful work.\n" +
		"- Checkpoint during long or decision-heavy work.\n" +
		"- Store again before finishing meaningful work.\n" +
		"- Curate stale or repetitive memory when needed.\n" +
		"- Never operate outside the current project or folder scope.\n"

	existingAgents, _ := os.ReadFile(agentsPath)
	if !bytes.Contains(existingAgents, []byte("## Uniam")) {
		f2, err := os.OpenFile(agentsPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err == nil {
			_, _ = f2.WriteString(uniamAgentsSection)
			_ = f2.Close()
		}
	}

	msg := "Installed Uniam in " + target
	if installSkill(target) {
		msg += " (MCP + AGENTS.md + skill)"
	}
	if fastContext && installFastContextSkill(target) {
		msg += " + fast context"
	}

	return map[string]string{"message": msg}, nil
}

func setupOpenCode(project bool, fastContext bool) (map[string]string, error) {
	if project {
		return nil, errors.New("OpenCode setup is global-only. Run `uniam setup opencode` without --project")
	}

	target, err := openCodeConfigRoot()
	if err != nil {
		return nil, err
	}

	configPath := filepath.Join(target, "opencode.json")
	config, err := readJSONMap(configPath)
	if err != nil {
		return nil, err
	}

	// OpenCode uses a "mcp" key (not "mcpServers"), and command must be an array.
	mcp, _ := config["mcp"].(map[string]any)
	if mcp == nil {
		mcp = make(map[string]any)
		config["mcp"] = mcp
	}
	mcp["uniam"] = map[string]any{
		"type":    "local",
		"command": []string{"uniam", "mcp"},
	}

	instructions, _ := config["instructions"].([]any)
	if instructions == nil {
		instructions = make([]any, 0, 1)
	}
	stringInstructions := make([]string, 0, len(instructions))
	for _, instruction := range instructions {
		value, ok := instruction.(string)
		if ok {
			stringInstructions = append(stringInstructions, value)
		}
	}
	stringInstructions = appendUniqueString(stringInstructions, openCodeInstructionConfigRef)
	normalizedInstructions := make([]any, 0, len(stringInstructions))
	for _, instruction := range stringInstructions {
		normalizedInstructions = append(normalizedInstructions, instruction)
	}
	config["instructions"] = normalizedInstructions

	if err := os.MkdirAll(target, 0755); err != nil {
		return nil, fmt.Errorf("failed to create config directory: %w", err)
	}

	if err := installOpenCodeInstructions(target); err != nil {
		return nil, fmt.Errorf("failed to write OpenCode instructions: %w", err)
	}

	if err := installOpenCodePlugin(target); err != nil {
		return nil, fmt.Errorf("failed to write OpenCode plugin: %w", err)
	}

	if !installSkill(target) {
		return nil, errors.New("failed to install OpenCode skill")
	}

	if err := writeJSONMap(configPath, config); err != nil {
		return nil, err
	}

	msg := "Installed Uniam OpenCode integration in " + target
	if fastContext {
		msg += " (fast context ignored for OpenCode)"
	}
	return map[string]string{"message": msg}, nil
}

// removeServersFromMCPJSON reads a JSON config file, removes the specified keys from
// "mcpServers", and writes the result back.
func removeServersFromMCPJSON(configPath string, keysToRemove []string) error {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("failed to read config: %w", err)
	}

	var config map[string]any
	if err := json.Unmarshal(data, &config); err != nil {
		return fmt.Errorf("failed to parse config: %w", err)
	}

	if mcpServers, ok := config["mcpServers"].(map[string]any); ok {
		for _, key := range keysToRemove {
			delete(mcpServers, key)
		}
	}

	newData, err := json.MarshalIndent(config, "", "  ")
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

	if err := removeServersFromMCPJSON(configPath, []string{"uniam", "ripgrep", "code-search"}); err != nil {
		return nil, err
	}

	msg := "Removed Uniam from " + configPath
	if uninstallSkill(skillTarget) {
		msg += " and skill"
	}
	if uninstallFastContextSkill(skillTarget) {
		msg += " and fast context skill"
	}

	return map[string]string{"message": msg}, nil
}

func uninstallCursor(configDir string, project bool, _ bool) (map[string]string, error) {
	target := resolveConfigDir(".cursor", configDir, project)
	configPath := filepath.Join(target, "mcp.json")

	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return map[string]string{"message": "Uniam not found in Cursor config"}, nil
	}

	if err := removeServersFromMCPJSON(configPath, []string{"uniam", "ripgrep", "code-search"}); err != nil {
		return nil, err
	}

	msg := "Removed Uniam from " + configPath
	if uninstallSkill(target) {
		msg += " and skill"
	}
	if uninstallFastContextSkill(target) {
		msg += " and fast context skill"
	}

	return map[string]string{"message": msg}, nil
}

func uninstallWindsurf(configDir string, project bool, _ bool) (map[string]string, error) {
	var targets []string
	if configDir != "" {
		targets = append(targets, configDir)
	} else {
		var baseDir string
		if project {
			baseDir, _ = os.Getwd()
		} else {
			baseDir, _ = os.UserHomeDir()
		}

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

		if err := removeServersFromMCPJSON(configPath, []string{"uniam", "ripgrep", "code-search"}); err != nil {
			return nil, err
		}

		msg := "Removed Uniam from " + configPath
		if uninstallSkill(target) {
			msg += " and skill"
		}
		if uninstallFastContextSkill(target) {
			msg += " and fast context skill"
		}
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

	if err := removeServersFromMCPJSON(configPath, []string{"uniam", "ripgrep", "code-search"}); err != nil {
		return nil, err
	}

	msg := "Removed Uniam from " + configPath
	if uninstallSkill(target) {
		msg += " and skill"
	}
	if uninstallFastContextSkill(target) {
		msg += " and fast context skill"
	}

	return map[string]string{"message": msg}, nil
}

func uninstallCodex(configDir string, project bool, _ bool) (map[string]string, error) {
	target := resolveConfigDir(".codex", configDir, project)

	msg := "Codex uninstall: manually remove Uniam entries from .codex/config.toml and AGENTS.md"

	if uninstallSkill(target) {
		msg += ". Removed skill."
	}
	if uninstallFastContextSkill(target) {
		msg += " Removed fast context skill."
	}

	return map[string]string{"message": msg}, nil
}

func uninstallOpenCode(project bool) (map[string]string, error) {
	if project {
		return nil, errors.New("OpenCode uninstall is global-only. Run `uniam uninstall opencode` without --project")
	}

	target, err := openCodeConfigRoot()
	if err != nil {
		return nil, err
	}

	configPath := filepath.Join(target, "opencode.json")
	_, statErr := os.Stat(configPath)
	configExists := !errors.Is(statErr, os.ErrNotExist)
	config := make(map[string]any)
	if configExists {
		config, err = readJSONMap(configPath)
		if err != nil {
			return nil, err
		}
	}

	if mcp, ok := config["mcp"].(map[string]any); ok {
		delete(mcp, "uniam")
		delete(mcp, "ripgrep")
		delete(mcp, "code-search")
	}

	if instructions, ok := config["instructions"].([]any); ok {
		filtered := make([]any, 0, len(instructions))
		for _, instruction := range instructions {
			value, ok := instruction.(string)
			if ok && value == openCodeInstructionConfigRef {
				continue
			}

			filtered = append(filtered, instruction)
		}
		if len(filtered) == 0 {
			delete(config, "instructions")
		} else {
			config["instructions"] = filtered
		}
	}

	if configExists {
		if err := writeJSONMap(configPath, config); err != nil {
			return nil, err
		}
	}

	_, _ = removeFileIfExists(filepath.Join(target, openCodeInstructionsFileName))
	_, _ = removeFileIfExists(filepath.Join(target, "plugins", openCodePluginFileName))
	_ = uninstallSkill(target)

	return map[string]string{
		"message": "Removed Uniam OpenCode integration from " + target,
	}, nil
}

func setupRooCode(configDir string, project bool, fastContext bool) (map[string]string, error) {
	var target string
	//nolint:gocritic
	if configDir != "" {
		target = configDir
	} else if project {
		cwd, _ := os.Getwd()
		target = filepath.Join(cwd, ".roo")
	} else {
		return nil, errors.New("RooCode global MCP config is managed via VS Code settings.\nUse --project (-p) to install in the current project's .roo/mcp.json instead")
	}

	configPath := filepath.Join(target, "mcp.json")

	var config map[string]any
	if data, err := os.ReadFile(configPath); err == nil {
		if err := json.Unmarshal(data, &config); err != nil {
			return nil, fmt.Errorf("failed to parse existing config: %w", err)
		}
	} else {
		config = make(map[string]any)
	}

	mcpServers, _ := config["mcpServers"].(map[string]any)
	if mcpServers == nil {
		mcpServers = make(map[string]any)
		config["mcpServers"] = mcpServers
	}

	mcpServers["uniam"] = map[string]any{
		"command": "uniam",
		"args":    []string{"mcp"},
	}

	if fastContext {
		addFastContextServers(mcpServers)
	}

	if err := os.MkdirAll(target, 0755); err != nil {
		return nil, fmt.Errorf("failed to create config directory: %w", err)
	}

	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return nil, fmt.Errorf("failed to write config: %w", err)
	}

	msg := "Installed Uniam MCP server in " + configPath
	if fastContext {
		msg += " with fast context"
	}
	return map[string]string{
		"message": msg,
	}, nil
}

func uninstallRooCode(configDir string, project bool, _ bool) (map[string]string, error) {
	var target string
	//nolint:gocritic
	if configDir != "" {
		target = configDir
	} else if project {
		cwd, _ := os.Getwd()
		target = filepath.Join(cwd, ".roo")
	} else {
		return nil, errors.New("RooCode global MCP config is managed via VS Code settings.\nUse --project (-p) to uninstall from the current project's .roo/mcp.json instead")
	}

	configPath := filepath.Join(target, "mcp.json")

	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return map[string]string{"message": "Uniam not found in RooCode config"}, nil
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config: %w", err)
	}

	var config map[string]any
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	if mcpServers, ok := config["mcpServers"].(map[string]any); ok {
		delete(mcpServers, "uniam")
	}

	newData, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(configPath, newData, 0644); err != nil {
		return nil, fmt.Errorf("failed to write config: %w", err)
	}

	return map[string]string{
		"message": "Removed Uniam from " + configPath,
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

func setupCopilot(_ string, project bool, fastContext bool) (map[string]string, error) {
	if project {
		return nil, errors.New("GitHub Copilot only supports global installation for MCP servers.\nPlease run without the --project flag")
	}

	configPath, err := getCopilotConfigPath()
	if err != nil {
		return nil, fmt.Errorf("failed to get copilot config path: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(configPath), 0755); err != nil {
		return nil, fmt.Errorf("failed to create copilot config directory: %w", err)
	}

	mcpEntry := map[string]any{
		"command": "uniam",
		"args":    []string{"mcp"},
	}

	if err := writeMCPJSON(configPath, mcpEntry, fastContext); err != nil {
		return nil, fmt.Errorf("failed to write mcp.json: %w", err)
	}

	home, _ := os.UserHomeDir()
	agentHome := filepath.Join(home, ".uniam")

	installSkill(agentHome)
	msg := "Installed Uniam MCP server in " + configPath + "\n"

	if fastContext {
		installFastContextSkill(agentHome)
		msg += "Installed fast context MCP servers and skills.\n"
	}

	msg += "\n\033[33mIMPORTANT: VS Code Copilot does not automatically read global skill files.\033[0m\n"
	msg += "Please add the instructions from \033[36m" + filepath.Join(agentHome, "skills") + "\033[0m\n"
	msg += "directly into your VS Code Copilot extension settings (e.g. Chat Rules) to ensure proper agent behavior."

	return map[string]string{"message": msg}, nil
}

func uninstallCopilot(_ string, project bool, _ bool) (map[string]string, error) {
	if project {
		return nil, errors.New("GitHub Copilot only supports global uninstallation.\nPlease run without the --project flag")
	}

	configPath, err := getCopilotConfigPath()
	if err != nil {
		return nil, fmt.Errorf("failed to get copilot config path: %w", err)
	}

	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return map[string]string{"message": "Uniam not found in Copilot config"}, nil
	}

	if err := removeServersFromMCPJSON(configPath, []string{"uniam", "ripgrep", "code-search"}); err != nil {
		return nil, err
	}

	return map[string]string{
		"message": "Removed Uniam from " + configPath,
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

func setupGeminiCli(_ string, project bool, fastContext bool) (map[string]string, error) {
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
	if err := writeClaudeJSONUserMCP(configPath, mcpEntry, fastContext); err != nil {
		return nil, fmt.Errorf("failed to write settings.json: %w", err)
	}

	var agentHome string
	if project {
		cwd, _ := os.Getwd()
		agentHome = filepath.Join(cwd, ".gemini")
	} else {
		home, _ := os.UserHomeDir()
		agentHome = filepath.Join(home, ".gemini")
	}

	installSkill(agentHome)
	msg := "Installed Uniam MCP server in " + configPath + "\n"

	if fastContext {
		installFastContextSkill(agentHome)
		msg += "Installed fast context MCP servers and skills.\n"
	}

	return map[string]string{"message": msg}, nil
}

func uninstallGeminiCli(_ string, project bool, _ bool) (map[string]string, error) {
	configPath, err := getGeminiCliConfigPath(project)
	if err != nil {
		return nil, fmt.Errorf("failed to get gemini-cli config path: %w", err)
	}

	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return map[string]string{"message": "Uniam not found in gemini-cli config"}, nil
	}

	if err := removeServersFromMCPJSON(configPath, []string{"uniam", "ripgrep", "code-search"}); err != nil {
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

	uninstallSkill(agentHome)
	uninstallFastContextSkill(agentHome)

	return map[string]string{
		"message": "Removed Uniam from " + configPath,
	}, nil
}
