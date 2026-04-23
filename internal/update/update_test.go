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

	result, err := checker.Check(context.Background())
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

func TestCheckRefreshesLatestReleaseEveryTime(t *testing.T) {
	attempts := 0
	client := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			attempts++
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

	result, err := checker.Check(context.Background())
	if err != nil {
		t.Fatalf("Check() error = %v", err)
	}

	if attempts != 1 {
		t.Fatalf("attempts = %d, want 1", attempts)
	}
	if result.CurrentVersion != "1.3.0" {
		t.Fatalf("CurrentVersion = %q, want %q", result.CurrentVersion, "1.3.0")
	}
	if result.LatestVersion != "v1.4.0" {
		t.Fatalf("LatestVersion = %q, want %q", result.LatestVersion, "v1.4.0")
	}
}

func TestCheckReturnsErrorWhenFetchFails(t *testing.T) {
	client := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return nil, context.DeadlineExceeded
		}),
	}

	checker := NewChecker("1.3.0").WithHTTPClient(client).WithBaseURL("https://example.invalid")

	_, err := checker.Check(context.Background())
	if err == nil {
		t.Fatal("Check() error = nil, want release check failure")
	}

	msg := err.Error()
	if !strings.Contains(msg, "release check failed") {
		t.Fatalf("Check() error = %q, want release failure prefix", msg)
	}
}

func TestCheckRetriesOnceBeforeSucceeding(t *testing.T) {
	attempts := 0
	client := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			attempts++
			if attempts == 1 {
				return nil, context.DeadlineExceeded
			}

			body := `{
				"tag_name":"v1.4.0",
				"published_at":"2026-04-22T11:00:00Z",
				"assets":[
					{"name":"` + assetNameForCurrentPlatform() + `","browser_download_url":"https://example.invalid/retry-success"}
				]
			}`

			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     make(http.Header),
			}, nil
		}),
	}

	var messages []string
	checker := NewChecker("1.3.0").WithHTTPClient(client).WithBaseURL("https://example.invalid").WithProgress(func(message string) {
		messages = append(messages, message)
	})

	result, err := checker.Check(context.Background())
	if err != nil {
		t.Fatalf("Check() error = %v", err)
	}

	if attempts != 2 {
		t.Fatalf("attempts = %d, want 2", attempts)
	}
	if result.LatestVersion != "v1.4.0" {
		t.Fatalf("LatestVersion = %q, want %q", result.LatestVersion, "v1.4.0")
	}

	joined := strings.Join(messages, "\n")
	if !strings.Contains(joined, "Retrying release check once") {
		t.Fatalf("progress messages = %q, want retry message", joined)
	}
}
