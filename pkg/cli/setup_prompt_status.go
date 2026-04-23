package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"uniam/internal/config"
)

type setupTargetFormat string

const (
	targetJSONMCPServers setupTargetFormat = "json_mcp_servers"
	targetOpenCodeMCP    setupTargetFormat = "opencode_mcp"
	targetCodexTOML      setupTargetFormat = "codex_toml"
)

type setupPromptContext struct {
	agent   string
	project bool
	targets []setupPromptTarget
}

type setupPromptTarget struct {
	configPath string
	format     setupTargetFormat
}

type normalizedMCPSpec struct {
	exec []string
	env  map[string]string
}

type integrationPromptStatus struct {
	summary string
	details []string
}

func buildSetupPromptContext(agent string, configDir string, project bool) *setupPromptContext {
	targets := resolveSetupPromptTargets(agent, configDir, project)
	if len(targets) == 0 {
		return &setupPromptContext{agent: agent, project: project}
	}

	return &setupPromptContext{
		agent:   agent,
		project: project,
		targets: targets,
	}
}

func resolveSetupPromptTargets(agent string, configDir string, project bool) []setupPromptTarget {
	switch agent {
	case "claude-code":
		if project {
			cwd, err := os.Getwd()
			if err != nil {
				return nil
			}
			return []setupPromptTarget{{configPath: filepath.Join(cwd, ".mcp.json"), format: targetJSONMCPServers}}
		}
		home, err := os.UserHomeDir()
		if err != nil {
			return nil
		}
		return []setupPromptTarget{{configPath: filepath.Join(home, ".claude.json"), format: targetJSONMCPServers}}
	case "cursor":
		target := resolveConfigDir(".cursor", configDir, project)
		return []setupPromptTarget{{configPath: filepath.Join(target, "mcp.json"), format: targetJSONMCPServers}}
	case "windsurf":
		var targets []setupPromptTarget
		if configDir != "" {
			return []setupPromptTarget{{configPath: filepath.Join(configDir, "mcp_config.json"), format: targetJSONMCPServers}}
		}
		home, err := os.UserHomeDir()
		if err != nil {
			return nil
		}
		for _, dir := range []string{
			filepath.Join(home, ".codeium", "windsurf"),
			filepath.Join(home, ".codeium"),
		} {
			if info, err := os.Stat(dir); err == nil && info.IsDir() {
				targets = append(targets, setupPromptTarget{
					configPath: filepath.Join(dir, "mcp_config.json"),
					format:     targetJSONMCPServers,
				})
			}
		}
		return targets
	case "antigravity":
		var target string
		if configDir != "" {
			target = configDir
		} else if project {
			cwd, err := os.Getwd()
			if err != nil {
				return nil
			}
			target = filepath.Join(cwd, ".gemini", "antigravity")
		} else {
			home, err := os.UserHomeDir()
			if err != nil {
				return nil
			}
			target = filepath.Join(home, ".gemini", "antigravity")
		}
		return []setupPromptTarget{{configPath: filepath.Join(target, "mcp_config.json"), format: targetJSONMCPServers}}
	case "codex":
		target := resolveConfigDir(".codex", configDir, project)
		return []setupPromptTarget{{configPath: filepath.Join(target, "config.toml"), format: targetCodexTOML}}
	case "opencode":
		if configDir != "" {
			return nil
		}
		_, configPath, _, err := openCodeInstallPaths(project)
		if err != nil {
			return nil
		}
		return []setupPromptTarget{{configPath: configPath, format: targetOpenCodeMCP}}
	case "copilot":
		var configPath string
		if project {
			cwd, err := os.Getwd()
			if err != nil {
				return nil
			}
			configPath = filepath.Join(cwd, ".mcp.json")
		} else {
			path, err := getCopilotConfigPath()
			if err != nil {
				return nil
			}
			configPath = path
		}
		return []setupPromptTarget{{configPath: configPath, format: targetJSONMCPServers}}
	case "gemini-cli":
		configPath, err := getGeminiCliConfigPath(project)
		if err != nil {
			return nil
		}
		return []setupPromptTarget{{configPath: configPath, format: targetJSONMCPServers}}
	default:
		return nil
	}
}

func inspectIntegrationPromptStatus(ctx *setupPromptContext, integration optionalIntegration, cfg *config.Config) integrationPromptStatus {
	if ctx == nil || len(ctx.targets) == 0 {
		return integrationPromptStatus{summary: "agent config not created yet"}
	}

	expected, comparable := expectedSpecForIntegration(integration.key, cfg)

	type targetState int
	const (
		stateMissing targetState = iota
		stateSame
		stateDifferent
		stateUnreadable
	)

	var (
		missingCount    int
		sameCount       int
		differentCount  int
		unreadableCount int
		detailLines     []string
	)

	for _, target := range ctx.targets {
		actual, err := readActualIntegrationSpec(target, integration.key)
		if err != nil {
			unreadableCount++
			continue
		}
		if actual == nil {
			missingCount++
			continue
		}
		if len(detailLines) == 0 {
			detailLines = detailLinesForIntegration(integration.key, actual, expected, comparable)
		}
		if comparable && specsEqual(*actual, expected) {
			sameCount++
		} else if comparable {
			differentCount++
		} else {
			sameCount++
		}
	}

	total := len(ctx.targets)
	switch {
	case unreadableCount == total:
		return integrationPromptStatus{summary: "cannot inspect", details: []string{"Current agent config could not be inspected."}}
	case sameCount == total:
		return integrationPromptStatus{summary: "already configured", details: detailLines}
	case missingCount == total:
		return integrationPromptStatus{summary: "not configured"}
	case differentCount > 0:
		status := "configured, will update"
		if total > 1 {
			status = fmt.Sprintf("configured in %d/%d targets, will update", sameCount+differentCount, total)
		}
		return integrationPromptStatus{summary: status, details: detailLines}
	default:
		if total > 1 && sameCount > 0 && missingCount > 0 {
			return integrationPromptStatus{
				summary: fmt.Sprintf("configured in %d/%d targets", sameCount, total),
				details: detailLines,
			}
		}
		return integrationPromptStatus{summary: "already configured", details: detailLines}
	}
}

func expectedSpecForIntegration(key string, cfg *config.Config) (normalizedMCPSpec, bool) {
	switch key {
	case "ripgrep":
		return normalizedMCPSpec{exec: []string{"npx", "-y", "mcp-ripgrep@latest"}, env: map[string]string{}}, true
	case "code-search":
		if cfg.Integrations.CodeSearchPath == nil || strings.TrimSpace(*cfg.Integrations.CodeSearchPath) == "" {
			return normalizedMCPSpec{}, false
		}
		home, _ := os.UserHomeDir()
		return normalizedMCPSpec{
			exec: []string{"node", strings.TrimSpace(*cfg.Integrations.CodeSearchPath), "--allowed-workspace", home},
			env:  map[string]string{},
		}, true
	case "context7":
		if cfg.Integrations.Context7APIKey == nil || strings.TrimSpace(*cfg.Integrations.Context7APIKey) == "" {
			return normalizedMCPSpec{}, false
		}
		return normalizedMCPSpec{
			exec: []string{"npx", "-y", "@upstash/context7-mcp"},
			env:  map[string]string{"CONTEXT7_API_KEY": strings.TrimSpace(*cfg.Integrations.Context7APIKey)},
		}, true
	case "git":
		return normalizedMCPSpec{exec: []string{"uvx", "mcp-server-git"}, env: map[string]string{}}, true
	case "searxng":
		if cfg.Integrations.SearXNGURL == nil || strings.TrimSpace(*cfg.Integrations.SearXNGURL) == "" {
			return normalizedMCPSpec{}, false
		}
		return normalizedMCPSpec{
			exec: []string{"npx", "-y", "mcp-searxng"},
			env:  map[string]string{"SEARXNG_URL": strings.TrimSpace(*cfg.Integrations.SearXNGURL)},
		}, true
	case "brave-search":
		if cfg.Integrations.BraveSearchAPIKey == nil || strings.TrimSpace(*cfg.Integrations.BraveSearchAPIKey) == "" {
			return normalizedMCPSpec{}, false
		}
		return normalizedMCPSpec{
			exec: []string{"npx", "-y", "@brave/brave-search-mcp-server", "--transport", "stdio"},
			env:  map[string]string{"BRAVE_API_KEY": strings.TrimSpace(*cfg.Integrations.BraveSearchAPIKey)},
		}, true
	case "firecrawl":
		if cfg.Integrations.FirecrawlAPIKey == nil || strings.TrimSpace(*cfg.Integrations.FirecrawlAPIKey) == "" {
			return normalizedMCPSpec{}, false
		}
		return normalizedMCPSpec{
			exec: []string{"npx", "-y", "firecrawl-mcp"},
			env:  map[string]string{"FIRECRAWL_API_KEY": strings.TrimSpace(*cfg.Integrations.FirecrawlAPIKey)},
		}, true
	default:
		return normalizedMCPSpec{}, false
	}
}

func readActualIntegrationSpec(target setupPromptTarget, key string) (*normalizedMCPSpec, error) {
	switch target.format {
	case targetJSONMCPServers:
		return readJSONServerSpec(target.configPath, "mcpServers", key)
	case targetOpenCodeMCP:
		return readJSONServerSpec(target.configPath, "mcp", key)
	case targetCodexTOML:
		return readCodexServerSpec(target.configPath, key)
	default:
		return nil, fmt.Errorf("unsupported target format %q", target.format)
	}
}

func readJSONServerSpec(path string, topKey string, serverKey string) (*normalizedMCPSpec, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errorsIsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var decoded map[string]any
	if err := json.Unmarshal(data, &decoded); err != nil {
		return nil, err
	}

	servers, _ := decoded[topKey].(map[string]any)
	if servers == nil {
		return nil, nil
	}
	server, _ := servers[serverKey].(map[string]any)
	if server == nil {
		return nil, nil
	}

	spec := &normalizedMCPSpec{
		exec: normalizeExec(server["command"], server["args"]),
		env:  normalizeEnv(server["env"], server["environment"]),
	}
	return spec, nil
}

func readCodexServerSpec(path string, serverKey string) (*normalizedMCPSpec, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errorsIsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	block := extractCodexServerBlock(string(data), serverKey)
	if block == "" {
		return nil, nil
	}

	command := parseTomlStringValue(block, "command")
	args := parseTomlStringArrayValue(block, "args")
	env := parseTomlInlineMapValue(block, "env")
	return &normalizedMCPSpec{
		exec: append([]string{command}, args...),
		env:  env,
	}, nil
}

func extractCodexServerBlock(text string, serverKey string) string {
	header := "[mcp_servers." + serverKey + "]"
	start := strings.Index(text, header)
	if start == -1 {
		return ""
	}
	rest := text[start:]
	next := strings.Index(rest[len(header):], "\n[mcp_servers.")
	if next == -1 {
		return rest
	}
	return rest[:len(header)+next]
}

func parseTomlStringValue(block string, key string) string {
	re := regexp.MustCompile(`(?m)^` + regexp.QuoteMeta(key) + `\s*=\s*"([^"]*)"\s*$`)
	matches := re.FindStringSubmatch(block)
	if len(matches) != 2 {
		return ""
	}
	return matches[1]
}

func parseTomlStringArrayValue(block string, key string) []string {
	re := regexp.MustCompile(`(?m)^` + regexp.QuoteMeta(key) + `\s*=\s*\[(.*)\]\s*$`)
	matches := re.FindStringSubmatch(block)
	if len(matches) != 2 {
		return nil
	}
	stringRe := regexp.MustCompile(`"([^"]*)"`)
	parts := stringRe.FindAllStringSubmatch(matches[1], -1)
	values := make([]string, 0, len(parts))
	for _, part := range parts {
		values = append(values, part[1])
	}
	return values
}

func parseTomlInlineMapValue(block string, key string) map[string]string {
	re := regexp.MustCompile(`(?m)^` + regexp.QuoteMeta(key) + `\s*=\s*\{(.*)\}\s*$`)
	matches := re.FindStringSubmatch(block)
	if len(matches) != 2 {
		return map[string]string{}
	}
	pairRe := regexp.MustCompile(`([A-Za-z0-9_]+)\s*=\s*"([^"]*)"`)
	pairs := pairRe.FindAllStringSubmatch(matches[1], -1)
	env := make(map[string]string, len(pairs))
	for _, pair := range pairs {
		env[pair[1]] = pair[2]
	}
	return env
}

func normalizeExec(commandValue any, argsValue any) []string {
	if commands, ok := anyStringSlice(commandValue); ok {
		return commands
	}
	command, _ := commandValue.(string)
	args, _ := anyStringSlice(argsValue)
	if strings.TrimSpace(command) == "" {
		return args
	}
	return append([]string{command}, args...)
}

func normalizeEnv(values ...any) map[string]string {
	env := map[string]string{}
	for _, value := range values {
		object, ok := value.(map[string]any)
		if !ok {
			continue
		}
		for key, raw := range object {
			if text, ok := raw.(string); ok {
				env[key] = text
			}
		}
	}
	return env
}

func anyStringSlice(value any) ([]string, bool) {
	switch v := value.(type) {
	case []string:
		return append([]string(nil), v...), true
	case []any:
		out := make([]string, 0, len(v))
		for _, item := range v {
			text, ok := item.(string)
			if !ok {
				return nil, false
			}
			out = append(out, text)
		}
		return out, true
	default:
		return nil, false
	}
}

func specsEqual(actual normalizedMCPSpec, expected normalizedMCPSpec) bool {
	if strings.Join(actual.exec, "\x00") != strings.Join(expected.exec, "\x00") {
		return false
	}
	if len(actual.env) != len(expected.env) {
		return false
	}
	for key, expectedValue := range expected.env {
		if actual.env[key] != expectedValue {
			return false
		}
	}
	return true
}

func detailLinesForIntegration(key string, actual *normalizedMCPSpec, expected normalizedMCPSpec, comparable bool) []string {
	if actual == nil {
		return nil
	}
	var details []string
	switch key {
	case "context7":
		if keyValue := actual.env["CONTEXT7_API_KEY"]; keyValue != "" {
			details = append(details, "API key: "+maskSecret(keyValue))
			if comparable && expected.env["CONTEXT7_API_KEY"] != keyValue {
				details = append(details, "API key in agent config differs from Uniam config.")
			}
		}
	case "brave-search":
		if keyValue := actual.env["BRAVE_API_KEY"]; keyValue != "" {
			details = append(details, "API key: "+maskSecret(keyValue))
			if comparable && expected.env["BRAVE_API_KEY"] != keyValue {
				details = append(details, "API key in agent config differs from Uniam config.")
			}
		}
	case "firecrawl":
		if keyValue := actual.env["FIRECRAWL_API_KEY"]; keyValue != "" {
			details = append(details, "API key: "+maskSecret(keyValue))
			if comparable && expected.env["FIRECRAWL_API_KEY"] != keyValue {
				details = append(details, "API key in agent config differs from Uniam config.")
			}
		}
	case "searxng":
		if url := actual.env["SEARXNG_URL"]; url != "" {
			details = append(details, "URL: "+url)
			if comparable && expected.env["SEARXNG_URL"] != url {
				details = append(details, "URL in agent config differs from Uniam config.")
			}
		}
	case "code-search":
		if workspace := codeSearchWorkspace(actual.exec); workspace != "" {
			details = append(details, "Workspace: "+workspace)
			if comparable && workspace != codeSearchWorkspace(expected.exec) {
				details = append(details, "Workspace path differs from Uniam-managed config.")
			}
		}
	}
	if comparable && len(details) == 0 && !specsEqual(*actual, expected) {
		details = append(details, "Command differs from Uniam-managed config.")
	}
	return details
}

func codeSearchWorkspace(exec []string) string {
	for i := 0; i < len(exec)-1; i++ {
		if exec[i] == "--allowed-workspace" {
			return exec[i+1]
		}
	}
	return ""
}

func maskSecret(value string) string {
	trimmed := strings.TrimSpace(value)
	if len(trimmed) < 7 {
		return "***"
	}
	return trimmed[:3] + "..." + trimmed[len(trimmed)-3:]
}

func errorsIsNotExist(err error) bool {
	return err != nil && os.IsNotExist(err)
}
