package update

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"uniam/internal/config"
)

const (
	defaultRepo           = "pdasilem/uniam"
	defaultCheckTTL       = 24 * time.Hour
	defaultGitHubEndpoint = "https://api.github.com"
	defaultHTTPTimeout    = 20 * time.Second
)

// Result holds update check information.
type Result struct {
	CurrentVersion  string
	LatestVersion   string
	PublishedAt     string
	AssetName       string
	AssetURL        string
	CheckedAt       string
	UpdateAvailable bool
	Cached          bool
}

type cacheRecord struct {
	Result    Result `json:"result"`
	CheckedAt string `json:"checked_at"`
}

type githubRelease struct {
	TagName     string `json:"tag_name"`
	PublishedAt string `json:"published_at"`
	Assets      []struct {
		Name string `json:"name"`
		URL  string `json:"browser_download_url"`
	} `json:"assets"`
}

// Checker resolves latest release metadata and caches it locally.
type Checker struct {
	client         *http.Client
	repo           string
	baseURL        string
	cachePath      string
	checkTTL       time.Duration
	currentVersion string
	progress       func(string)
}

// NewChecker constructs a release checker for the current installation.
func NewChecker(currentVersion string) *Checker {
	home := config.GetUniamHome()

	return &Checker{
		client:         &http.Client{Timeout: defaultHTTPTimeout},
		repo:           defaultRepo,
		baseURL:        defaultGitHubEndpoint,
		cachePath:      filepath.Join(home, "update-check.json"),
		checkTTL:       defaultCheckTTL,
		currentVersion: currentVersion,
	}
}

// WithHTTPClient replaces the HTTP client, mainly for tests.
func (c *Checker) WithHTTPClient(client *http.Client) *Checker {
	c.client = client
	return c
}

// WithBaseURL replaces the release API base URL, mainly for tests.
func (c *Checker) WithBaseURL(baseURL string) *Checker {
	c.baseURL = strings.TrimRight(baseURL, "/")
	return c
}

// WithCheckTTL overrides the cache TTL, mainly for config and tests.
func (c *Checker) WithCheckTTL(ttl time.Duration) *Checker {
	if ttl > 0 {
		c.checkTTL = ttl
	}
	return c
}

// WithProgress installs a callback for user-visible update progress stages.
func (c *Checker) WithProgress(progress func(string)) *Checker {
	c.progress = progress
	return c
}

// Check returns cached or freshly fetched release information.
func (c *Checker) Check(ctx context.Context, force bool) (*Result, error) {
	if !force {
		c.report("Checking cached release metadata...")
		if cached, ok := c.loadCache(false); ok {
			cached.Cached = true
			c.report("Using cached release metadata.")
			return cached, nil
		}
	}

	c.report("Querying latest release metadata...")
	result, err := c.fetchLatest(ctx)
	if err != nil {
		c.report(fmt.Sprintf("Release check failed: %v", err))
		c.report("Retrying release check once...")
		result, err = c.fetchLatest(ctx)
	}
	if err != nil {
		if cached, ok := c.loadCache(true); ok {
			cached.Cached = true
			c.report("Release check failed; using cached metadata.")
			return cached, nil
		}

		return nil, fmt.Errorf("release check failed and no cached metadata is available yet: %w", err)
	}

	_ = c.storeCache(result)
	c.report("Release metadata loaded.")

	return result, nil
}

// Apply downloads and installs the latest matching release asset.
func (c *Checker) Apply(ctx context.Context, result *Result) error {
	if result == nil {
		return errors.New("nil update result")
	}
	if !result.UpdateAvailable {
		return nil
	}
	if result.AssetURL == "" {
		return errors.New("no downloadable asset found for this platform")
	}

	exePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to resolve current executable: %w", err)
	}

	if !isWritable(exePath) {
		return fmt.Errorf("binary path is not writable: %s", exePath)
	}

	c.report("Downloading latest release asset...")
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, result.AssetURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create download request: %w", err)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to download release asset: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("release download failed with status %d: %s", resp.StatusCode, string(body))
	}

	tmpPath := exePath + ".download"
	tmpFile, err := os.OpenFile(tmpPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0755)
	if err != nil {
		return fmt.Errorf("failed to create temporary download file: %w", err)
	}

	if _, err := io.Copy(tmpFile, resp.Body); err != nil {
		_ = tmpFile.Close()
		return fmt.Errorf("failed to write downloaded binary: %w", err)
	}
	if err := tmpFile.Close(); err != nil {
		return fmt.Errorf("failed to close downloaded binary: %w", err)
	}

	if info, err := os.Stat(tmpPath); err != nil || info.Size() == 0 {
		return fmt.Errorf("downloaded binary is invalid")
	}

	if runtime.GOOS != "windows" {
		if err := os.Chmod(tmpPath, 0755); err != nil {
			return fmt.Errorf("failed to mark downloaded binary executable: %w", err)
		}
	}

	c.report("Replacing current binary...")
	if err := os.Rename(tmpPath, exePath); err != nil {
		return fmt.Errorf("failed to replace current binary: %w", err)
	}

	_ = os.Remove(c.cachePath)
	c.report("Update applied successfully.")

	return nil
}

func (c *Checker) loadCache(ignoreTTL bool) (*Result, bool) {
	data, err := os.ReadFile(c.cachePath)
	if err != nil {
		return nil, false
	}

	var cache cacheRecord
	if err := json.Unmarshal(data, &cache); err != nil {
		return nil, false
	}

	checkedAt, err := time.Parse(time.RFC3339, cache.CheckedAt)
	if err != nil {
		return nil, false
	}

	if cache.Result.CurrentVersion != "" && c.currentVersion != "" && cache.Result.CurrentVersion != c.currentVersion {
		return nil, false
	}

	if !ignoreTTL && time.Since(checkedAt) > c.checkTTL {
		return nil, false
	}

	cache.Result.Cached = true

	return &cache.Result, true
}

func (c *Checker) storeCache(result *Result) error {
	if err := os.MkdirAll(filepath.Dir(c.cachePath), 0755); err != nil {
		return err
	}

	record := cacheRecord{
		Result:    *result,
		CheckedAt: result.CheckedAt,
	}

	data, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(c.cachePath, data, 0644)
}

func (c *Checker) fetchLatest(ctx context.Context) (*Result, error) {
	url := fmt.Sprintf("%s/repos/%s/releases/latest", c.baseURL, c.repo)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create release request: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to query latest release: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("release query failed with status %d: %s", resp.StatusCode, string(body))
	}

	var release githubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, fmt.Errorf("failed to decode release response: %w", err)
	}

	assetName := assetNameForCurrentPlatform()
	result := &Result{
		CurrentVersion:  c.currentVersion,
		LatestVersion:   release.TagName,
		PublishedAt:     release.PublishedAt,
		AssetName:       assetName,
		CheckedAt:       time.Now().UTC().Format(time.RFC3339),
		UpdateAvailable: isNewerVersion(c.currentVersion, release.TagName),
	}

	for _, asset := range release.Assets {
		if asset.Name == assetName {
			result.AssetURL = asset.URL
			break
		}
	}

	return result, nil
}

func assetNameForCurrentPlatform() string {
	osName := runtime.GOOS
	arch := runtime.GOARCH

	switch arch {
	case "amd64":
		arch = "amd64"
	case "arm64":
		arch = "arm64"
	default:
		arch = runtime.GOARCH
	}

	if osName == "windows" {
		return fmt.Sprintf("uniam-%s-%s.exe", osName, arch)
	}

	return fmt.Sprintf("uniam-%s-%s", osName, arch)
}

func isNewerVersion(current string, latest string) bool {
	if current == "" || current == "dev" || latest == "" {
		return false
	}

	cur := parseVersion(current)
	lat := parseVersion(latest)

	for i := 0; i < len(cur) || i < len(lat); i++ {
		cv := 0
		if i < len(cur) {
			cv = cur[i]
		}
		lv := 0
		if i < len(lat) {
			lv = lat[i]
		}
		if lv > cv {
			return true
		}
		if lv < cv {
			return false
		}
	}

	return false
}

func parseVersion(version string) []int {
	version = strings.TrimSpace(strings.TrimPrefix(version, "v"))
	if idx := strings.Index(version, "-"); idx >= 0 {
		version = version[:idx]
	}

	parts := strings.Split(version, ".")
	values := make([]int, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			values = append(values, 0)
			continue
		}

		n, err := strconv.Atoi(part)
		if err != nil {
			values = append(values, 0)
			continue
		}
		values = append(values, n)
	}

	return values
}

func isWritable(path string) bool {
	dir := filepath.Dir(path)
	if info, err := os.Stat(path); err == nil {
		return info.Mode().Perm()&0200 != 0
	}

	if info, err := os.Stat(dir); err == nil {
		return info.Mode().Perm()&0200 != 0
	}

	return false
}

func (c *Checker) report(message string) {
	if c.progress != nil && message != "" {
		c.progress(message)
	}
}
