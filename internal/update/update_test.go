package update

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
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
