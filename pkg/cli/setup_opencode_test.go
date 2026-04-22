package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
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
	assertOpenCodeConfigState(t, configPath, "./keep.md", "./uniam-instructions.md", true)

	if _, err := setupOpenCode(false, false); err != nil {
		t.Fatalf("second setupOpenCode() error = %v", err)
	}

	assertOpenCodeManagedFiles(t, target)
	assertOpenCodeConfigState(t, configPath, "./keep.md", "./uniam-instructions.md", true)
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
	assertOpenCodeConfigState(t, configPath, "./keep.md", ".opencode/uniam-instructions.md", true)

	if _, err := setupOpenCode(true, false); err != nil {
		t.Fatalf("second setupOpenCode(true, false) error = %v", err)
	}

	assertOpenCodeManagedFiles(t, target)
	assertOpenCodeConfigState(t, configPath, "./keep.md", ".opencode/uniam-instructions.md", true)
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

	assertOpenCodeConfigState(t, configPath, "./keep.md", "./uniam-instructions.md", false)

	for _, path := range []string{
		filepath.Join(target, "skills", "uniam", "SKILL.md"),
		filepath.Join(target, openCodeInstructionsFileName),
		filepath.Join(target, "plugins", openCodePluginFileName),
	} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("expected managed asset %q to be removed, stat err = %v", path, err)
		}
	}
}

func assertOpenCodeManagedFiles(t *testing.T, target string) {
	t.Helper()

	for _, path := range []string{
		filepath.Join(target, "skills", "uniam", "SKILL.md"),
		filepath.Join(target, openCodeInstructionsFileName),
		filepath.Join(target, "plugins", openCodePluginFileName),
	} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("expected managed asset %q to exist: %v", path, err)
		}
	}
}

func assertOpenCodeConfigState(t *testing.T, configPath string, keepInstruction string, uniamInstruction string, expectUniam bool) {
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

	var (
		keepCount  int
		uniamCount int
	)
	for _, instruction := range instructions {
		value, ok := instruction.(string)
		if !ok {
			t.Fatalf("instruction value type = %T, want string", instruction)
		}

		if value == keepInstruction {
			keepCount++
		}
		if value == uniamInstruction {
			uniamCount++
		}
	}

	if keepCount != 1 {
		t.Fatalf("keep instruction count = %d, want 1", keepCount)
	}

	wantUniamCount := 0
	if expectUniam {
		wantUniamCount = 1
	}
	if uniamCount != wantUniamCount {
		t.Fatalf("uniam instruction count = %d, want %d", uniamCount, wantUniamCount)
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
