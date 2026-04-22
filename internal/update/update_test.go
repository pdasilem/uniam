package update

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestIsNewerVersion(t *testing.T) {
	if !isNewerVersion("1.2.0", "1.3.0") {
		t.Fatal("expected newer version to be detected")
	}

	if isNewerVersion("1.3.0", "1.2.0") {
		t.Fatal("did not expect downgrade to be treated as update")
	}

	if isNewerVersion("dev", "1.2.0") {
		t.Fatal("dev version should not auto-compare as updatable")
	}
}

func TestCheckFetchesLatestRelease(t *testing.T) {
	client := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			body := `{
				"tag_name":"v1.3.0",
				"published_at":"2026-04-22T10:00:00Z",
				"assets":[
					{"name":"` + assetNameForCurrentPlatform() + `","browser_download_url":"https://example.invalid/uniam"}
				]
			}`

			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     make(http.Header),
			}, nil
		}),
	}

	checker := NewChecker("1.2.0").WithHTTPClient(client).WithBaseURL("https://example.invalid")
	checker.cachePath = t.TempDir() + "/update-cache.json"

	result, err := checker.Check(context.Background(), true)
	if err != nil {
		t.Fatalf("Check() error = %v", err)
	}

	if !result.UpdateAvailable {
		t.Fatal("expected update to be available")
	}

	if result.AssetURL == "" {
		t.Fatal("expected asset url to be selected")
	}
}

func TestCheckInvalidatesCacheWhenCurrentVersionChanges(t *testing.T) {
	cachePath := t.TempDir() + "/update-cache.json"
	cached := cacheRecord{
		Result: Result{
			CurrentVersion:  "1.2.0",
			LatestVersion:   "v1.3.0",
			PublishedAt:     "2026-04-22T10:00:00Z",
			AssetName:       assetNameForCurrentPlatform(),
			AssetURL:        "https://example.invalid/old",
			CheckedAt:       time.Now().UTC().Format(time.RFC3339),
			UpdateAvailable: true,
		},
		CheckedAt: time.Now().UTC().Format(time.RFC3339),
	}

	data, err := json.Marshal(cached)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	if err := os.WriteFile(cachePath, data, 0644); err != nil {
		t.Fatalf("os.WriteFile() error = %v", err)
	}

	client := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			body := `{
				"tag_name":"v1.4.0",
				"published_at":"2026-04-22T11:00:00Z",
				"assets":[
					{"name":"` + assetNameForCurrentPlatform() + `","browser_download_url":"https://example.invalid/new"}
				]
			}`

			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     make(http.Header),
			}, nil
		}),
	}

	checker := NewChecker("1.3.0").WithHTTPClient(client).WithBaseURL("https://example.invalid")
	checker.cachePath = cachePath

	result, err := checker.Check(context.Background(), false)
	if err != nil {
		t.Fatalf("Check() error = %v", err)
	}

	if result.Cached {
		t.Fatal("expected stale version cache to be ignored")
	}
	if result.CurrentVersion != "1.3.0" {
		t.Fatalf("CurrentVersion = %q, want %q", result.CurrentVersion, "1.3.0")
	}
	if result.LatestVersion != "v1.4.0" {
		t.Fatalf("LatestVersion = %q, want %q", result.LatestVersion, "v1.4.0")
	}
}

func TestCheckFallsBackToCachedResultOnFetchError(t *testing.T) {
	cachePath := t.TempDir() + "/update-cache.json"
	cached := cacheRecord{
		Result: Result{
			CurrentVersion:  "1.3.0",
			LatestVersion:   "v1.4.0",
			PublishedAt:     "2026-04-22T11:00:00Z",
			AssetName:       assetNameForCurrentPlatform(),
			AssetURL:        "https://example.invalid/cached",
			CheckedAt:       time.Now().UTC().Add(-48 * time.Hour).Format(time.RFC3339),
			UpdateAvailable: true,
		},
		CheckedAt: time.Now().UTC().Add(-48 * time.Hour).Format(time.RFC3339),
	}

	data, err := json.Marshal(cached)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	if err := os.WriteFile(cachePath, data, 0644); err != nil {
		t.Fatalf("os.WriteFile() error = %v", err)
	}

	client := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return nil, context.DeadlineExceeded
		}),
	}

	checker := NewChecker("1.3.0").WithHTTPClient(client).WithBaseURL("https://example.invalid")
	checker.cachePath = cachePath
	checker.checkTTL = time.Hour

	result, err := checker.Check(context.Background(), false)
	if err != nil {
		t.Fatalf("Check() error = %v", err)
	}

	if !result.Cached {
		t.Fatal("expected cached fallback result")
	}
	if result.LatestVersion != "v1.4.0" {
		t.Fatalf("LatestVersion = %q, want %q", result.LatestVersion, "v1.4.0")
	}
}
