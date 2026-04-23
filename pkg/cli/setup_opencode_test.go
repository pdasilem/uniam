package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSetupOpenCodeInstallsGlobalAssetsAndIsIdempotent(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	target := filepath.Join(home, ".config", "opencode")
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatalf("os.MkdirAll() error = %v", err)
	}

	configPath := filepath.Join(target, "opencode.json")
	existing := map[string]any{
		"mcp": map[string]any{
			"other": map[string]any{
				"type": "local",
			},
		},
		"instructions": []string{"./keep.md"},
	}
	writeJSONFixture(t, configPath, existing)

	if _, err := setupOpenCode(false, false); err != nil {
		t.Fatalf("setupOpenCode() error = %v", err)
	}

	assertOpenCodeManagedFiles(t, target)
	assertOpenCodeConfigState(t, configPath, "./keep.md", true)
	assertOpenCodeAgentsManaged(t, filepath.Join(target, "AGENTS.md"), "opencode")

	if _, err := setupOpenCode(false, false); err != nil {
		t.Fatalf("second setupOpenCode() error = %v", err)
	}

	assertOpenCodeManagedFiles(t, target)
	assertOpenCodeConfigState(t, configPath, "./keep.md", true)
	assertOpenCodeAgentsManaged(t, filepath.Join(target, "AGENTS.md"), "opencode")
}

func TestSetupOpenCodeInstallsProjectAssetsAndIsIdempotent(t *testing.T) {
	repo := t.TempDir()
	t.Setenv("HOME", t.TempDir())

	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("os.Getwd() error = %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(wd)
	})
	if err := os.Chdir(repo); err != nil {
		t.Fatalf("os.Chdir() error = %v", err)
	}

	target := filepath.Join(repo, ".opencode")
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatalf("os.MkdirAll() error = %v", err)
	}

	configPath := filepath.Join(repo, "opencode.json")
	existing := map[string]any{
		"mcp": map[string]any{
			"other": map[string]any{
				"type": "local",
			},
		},
		"instructions": []string{"./keep.md"},
	}
	writeJSONFixture(t, configPath, existing)

	if _, err := setupOpenCode(true, false); err != nil {
		t.Fatalf("setupOpenCode(true, false) error = %v", err)
	}

	assertOpenCodeManagedFiles(t, target)
	assertOpenCodeConfigState(t, configPath, "./keep.md", true)
	assertOpenCodeAgentsManaged(t, filepath.Join(repo, "AGENTS.md"), "agents")

	if _, err := setupOpenCode(true, false); err != nil {
		t.Fatalf("second setupOpenCode(true, false) error = %v", err)
	}

	assertOpenCodeManagedFiles(t, target)
	assertOpenCodeConfigState(t, configPath, "./keep.md", true)
	assertOpenCodeAgentsManaged(t, filepath.Join(repo, "AGENTS.md"), "agents")
}

func TestUninstallOpenCodeRemovesOnlyUniamManagedAssets(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	target := filepath.Join(home, ".config", "opencode")
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatalf("os.MkdirAll() error = %v", err)
	}

	configPath := filepath.Join(target, "opencode.json")
	existing := map[string]any{
		"mcp": map[string]any{
			"other": map[string]any{
				"type": "local",
			},
		},
		"instructions": []string{"./keep.md"},
	}
	writeJSONFixture(t, configPath, existing)

	if _, err := setupOpenCode(false, false); err != nil {
		t.Fatalf("setupOpenCode() error = %v", err)
	}

	if _, err := uninstallOpenCode(false); err != nil {
		t.Fatalf("uninstallOpenCode() error = %v", err)
	}

	assertOpenCodeConfigState(t, configPath, "./keep.md", false)

	for _, path := range []string{
		filepath.Join(target, "skills", "uniam", "SKILL.md"),
	} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("expected managed asset %q to be removed, stat err = %v", path, err)
		}
	}
	if _, err := os.Stat(filepath.Join(target, "AGENTS.md")); !os.IsNotExist(err) {
		t.Fatalf("expected managed OpenCode AGENTS.md to be removed, stat err = %v", err)
	}
}

func TestSetupOpenCodeAddsContext7WhenEnabled(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	target := filepath.Join(home, ".config", "opencode")
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatalf("os.MkdirAll() error = %v", err)
	}

	configPath := filepath.Join(target, "opencode.json")
	writeJSONFixture(t, configPath, map[string]any{})

	prevOptions := currentSetupOptions
	t.Cleanup(func() {
		currentSetupOptions = prevOptions
	})

	currentSetupOptions = setupOptions{
		Context7:       true,
		Context7APIKey: "ctx7sk-test",
	}

	if _, err := setupOpenCode(false, false); err != nil {
		t.Fatalf("setupOpenCode() error = %v", err)
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("os.ReadFile() error = %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	mcp, ok := decoded["mcp"].(map[string]any)
	if !ok {
		t.Fatalf("config mcp type = %T, want map[string]any", decoded["mcp"])
	}

	context7, ok := mcp["context7"].(map[string]any)
	if !ok {
		t.Fatal("expected mcp.context7 to be configured")
	}

	env, ok := context7["environment"].(map[string]any)
	if !ok {
		t.Fatalf("context7 environment type = %T, want map[string]any", context7["environment"])
	}

	if got := env["CONTEXT7_API_KEY"]; got != "ctx7sk-test" {
		t.Fatalf("CONTEXT7_API_KEY = %v, want %q", got, "ctx7sk-test")
	}
}

func TestSetupOpenCodeAddsSearXNGAndFirecrawlWhenEnabled(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	target := filepath.Join(home, ".config", "opencode")
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatalf("os.MkdirAll() error = %v", err)
	}

	configPath := filepath.Join(target, "opencode.json")
	writeJSONFixture(t, configPath, map[string]any{})

	prevOptions := currentSetupOptions
	t.Cleanup(func() {
		currentSetupOptions = prevOptions
	})

	currentSetupOptions = setupOptions{
		SearXNG:         true,
		SearXNGURL:      "http://127.0.0.1:8080",
		Firecrawl:       true,
		FirecrawlAPIKey: "fc-test",
	}

	if _, err := setupOpenCode(false, false); err != nil {
		t.Fatalf("setupOpenCode() error = %v", err)
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("os.ReadFile() error = %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	mcp, ok := decoded["mcp"].(map[string]any)
	if !ok {
		t.Fatalf("config mcp type = %T, want map[string]any", decoded["mcp"])
	}

	searxng, ok := mcp["searxng"].(map[string]any)
	if !ok {
		t.Fatal("expected mcp.searxng to be configured")
	}
	command, ok := searxng["command"].([]any)
	if !ok || len(command) != 3 || command[0] != "npx" || command[1] != "-y" || command[2] != "mcp-searxng" {
		t.Fatalf("searxng command = %v, want %v", searxng["command"], []string{"npx", "-y", "mcp-searxng"})
	}
	searxEnv, ok := searxng["environment"].(map[string]any)
	if !ok || searxEnv["SEARXNG_URL"] != "http://127.0.0.1:8080" {
		t.Fatalf("SEARXNG_URL = %v, want %q", searxEnv["SEARXNG_URL"], "http://127.0.0.1:8080")
	}

	firecrawl, ok := mcp["firecrawl"].(map[string]any)
	if !ok {
		t.Fatal("expected mcp.firecrawl to be configured")
	}
	firecrawlEnv, ok := firecrawl["environment"].(map[string]any)
	if !ok || firecrawlEnv["FIRECRAWL_API_KEY"] != "fc-test" {
		t.Fatalf("FIRECRAWL_API_KEY = %v, want %q", firecrawlEnv["FIRECRAWL_API_KEY"], "fc-test")
	}
}

func assertOpenCodeManagedFiles(t *testing.T, target string) {
	t.Helper()

	for _, path := range []string{
		filepath.Join(target, "skills", "uniam", "SKILL.md"),
	} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("expected managed asset %q to exist: %v", path, err)
		}
	}

	if _, err := os.Stat(filepath.Join(target, openCodeInstructionsFileName)); !os.IsNotExist(err) {
		t.Fatalf("expected legacy instructions file to be absent, stat err = %v", err)
	}
}

func assertOpenCodeConfigState(t *testing.T, configPath string, keepInstruction string, expectUniam bool) {
	t.Helper()

	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("os.ReadFile() error = %v", err)
	}

	var config map[string]any
	if err := json.Unmarshal(data, &config); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	mcp, ok := config["mcp"].(map[string]any)
	if !ok {
		t.Fatalf("config mcp type = %T, want map[string]any", config["mcp"])
	}

	if _, ok := mcp["other"]; !ok {
		t.Fatal("existing mcp.other entry was removed")
	}

	_, hasUniam := mcp["uniam"]
	if hasUniam != expectUniam {
		t.Fatalf("mcp.uniam presence = %v, want %v", hasUniam, expectUniam)
	}

	instructions, ok := config["instructions"].([]any)
	if !ok {
		t.Fatalf("config instructions type = %T, want []any", config["instructions"])
	}

	var keepCount int
	for _, instruction := range instructions {
		value, ok := instruction.(string)
		if !ok {
			t.Fatalf("instruction value type = %T, want string", instruction)
		}

		if value == keepInstruction {
			keepCount++
		}
	}

	if keepCount != 1 {
		t.Fatalf("keep instruction count = %d, want 1", keepCount)
	}

	for _, instruction := range instructions {
		value := instruction.(string)
		if value == "./uniam-instructions.md" || value == ".opencode/uniam-instructions.md" {
			t.Fatalf("legacy OpenCode instruction reference still present: %q", value)
		}
	}
}

func assertOpenCodeAgentsManaged(t *testing.T, path string, marker string) {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("os.ReadFile(%q) error = %v", path, err)
	}

	text := string(data)
	begin := "<!-- uniam:begin " + marker + " -->"
	end := "<!-- uniam:end " + marker + " -->"
	if !strings.Contains(text, begin) {
		t.Fatalf("OpenCode AGENTS.md missing managed block begin marker in %q", path)
	}
	if !strings.Contains(text, end) {
		t.Fatalf("OpenCode AGENTS.md missing managed block end marker in %q", path)
	}
	if !strings.Contains(text, "## Uniam") {
		t.Fatalf("OpenCode AGENTS.md missing Uniam heading in %q", path)
	}
	if !strings.Contains(text, "Required:") {
		t.Fatalf("OpenCode AGENTS.md missing compact Required section in %q", path)
	}
}

func writeJSONFixture(t *testing.T, path string, value map[string]any) {
	t.Helper()

	data, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("os.WriteFile() error = %v", err)
	}
}
