package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// codeSearchInstallDir returns the path where code-search-mcp will be installed.
func codeSearchInstallDir() string {
	switch runtime.GOOS {
	case "windows":
		local := os.Getenv("LOCALAPPDATA")
		if local == "" {
			local = filepath.Join(os.Getenv("USERPROFILE"), "AppData", "Local")
		}
		return filepath.Join(local, "uniam", "code-search-mcp")
	default:
		home, _ := os.UserHomeDir()
		return filepath.Join(home, ".local", "share", "uniam", "code-search-mcp")
	}
}

// codeSearchEntryPoint returns the absolute path to dist/index.js.
func codeSearchEntryPoint() string {
	return filepath.Join(codeSearchInstallDir(), "dist", "index.js")
}

// checkBin returns an error if the binary is not found in PATH.
func checkBin(name string) error {
	if _, err := exec.LookPath(name); err != nil {
		return fmt.Errorf("%s not found in PATH", name)
	}
	return nil
}

// ctagsHint returns a human-readable hint for installing universal-ctags.
func ctagsHint() string {
	switch runtime.GOOS {
	case "darwin":
		return "brew install universal-ctags"
	case "windows":
		return "choco install universal-ctags"
	default:
		// Detect package manager
		if _, err := exec.LookPath("apt-get"); err == nil {
			return "sudo apt-get install universal-ctags"
		}
		if _, err := exec.LookPath("pacman"); err == nil {
			return "sudo pacman -S universal-ctags"
		}
		return "install universal-ctags via your package manager"
	}
}

// ripgrepHint returns a human-readable hint for installing ripgrep.
func ripgrepHint() string {
	switch runtime.GOOS {
	case "darwin":
		return "brew install ripgrep"
	case "windows":
		return "choco install ripgrep"
	default:
		// Detect package manager
		if _, err := exec.LookPath("apt-get"); err == nil {
			return "sudo apt-get install ripgrep"
		}
		if _, err := exec.LookPath("pacman"); err == nil {
			return "sudo pacman -S ripgrep"
		}
		return "install ripgrep via your package manager"
	}
}

// installCodeSearch clones (or updates) and builds the code-search-mcp server.
// It returns the entry point path on success.
func installCodeSearch() (string, error) {
	// Check required tools
	for _, bin := range []string{"git", "node", "npm"} {
		if err := checkBin(bin); err != nil {
			return "", fmt.Errorf("required dependency missing: %w\n  Install it and run 'uniam setup' again", err)
		}
	}

	// Warn about universal-ctags (optional but recommended for symbol search)
	if _, err := exec.LookPath("ctags"); err != nil {
		fmt.Printf("  Note: universal-ctags not found (symbol search disabled).\n  Install with: %s\n\n", ctagsHint())
	} else {
		fmt.Println("  Found universal-ctags. Symbol search enabled.")
	}

	// Warn about ripgrep (required for text search)
	if _, err := exec.LookPath("rg"); err != nil {
		fmt.Printf("  Note: ripgrep not found (text search disabled).\n  Install with: %s\n\n", ripgrepHint())
	} else {
		fmt.Println("  Found ripgrep. Text search enabled.")
	}

	dir := codeSearchInstallDir()
	entry := codeSearchEntryPoint()
	hadEntry := fileExists(entry)
	beforeHead := gitHead(dir)

	if _, err := os.Stat(filepath.Join(dir, ".git")); os.IsNotExist(err) {
		// Fresh clone
		fmt.Printf("  Cloning code-search-mcp into %s ...\n", dir)
		if err := os.MkdirAll(filepath.Dir(dir), 0755); err != nil {
			return "", fmt.Errorf("failed to create install directory: %w", err)
		}
		cmd := exec.Command("git", "clone", "https://github.com/GhostTypes/code-search-mcp.git", dir)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return "", fmt.Errorf("git clone failed: %w", err)
		}
	} else {
		// Update existing clone
		fmt.Printf("  Updating code-search-mcp in %s ...\n", dir)
		cmd := exec.Command("git", "-C", dir, "pull", "--ff-only")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return "", fmt.Errorf("git pull failed: %w", err)
		}
	}

	afterHead := gitHead(dir)
	if hadEntry && beforeHead != "" && beforeHead == afterHead {
		fmt.Printf("  code-search-mcp already built at %s\n", entry)
		return entry, nil
	}

	// npm install
	fmt.Println("  Running npm install ...")
	npmInstall := exec.Command("npm", "install")
	npmInstall.Dir = dir
	npmInstall.Stdout = os.Stdout
	npmInstall.Stderr = os.Stderr
	if err := npmInstall.Run(); err != nil {
		return "", fmt.Errorf("npm install failed: %w", err)
	}

	// npm run build
	fmt.Println("  Building code-search-mcp ...")
	npmBuild := exec.Command("npm", "run", "build")
	npmBuild.Dir = dir
	npmBuild.Stdout = os.Stdout
	npmBuild.Stderr = os.Stderr
	if err := npmBuild.Run(); err != nil {
		return "", fmt.Errorf("npm run build failed: %w", err)
	}

	if _, err := os.Stat(entry); err != nil {
		return "", fmt.Errorf("build succeeded but entry point not found at %s", entry)
	}

	fmt.Printf("  code-search-mcp installed at %s\n", entry)
	return entry, nil
}

func gitHead(dir string) string {
	cmd := exec.Command("git", "-C", dir, "rev-parse", "HEAD")
	output, err := cmd.Output()
	if err != nil {
		return ""
	}

	return strings.TrimSpace(string(output))
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
