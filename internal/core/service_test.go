package core

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"uniam/internal/models"
)

type testEmbedder struct{}

func (e *testEmbedder) Embed(_ context.Context, _ string) ([]float32, error) {
	return []float32{0.1, 0.2, 0.3}, nil
}

func TestNewService(t *testing.T) {
	tmpDir := t.TempDir()

	svc, err := NewService(tmpDir)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	defer svc.Close()

	if svc == nil {
		t.Fatal("NewService() returned nil")
	}

	if svc.uniamHome != tmpDir {
		t.Errorf("NewService() uniamHome = %q, want %q", svc.uniamHome, tmpDir)
	}
}

func TestService_Store(t *testing.T) {
	tmpDir := t.TempDir()

	svc, err := NewService(tmpDir)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	defer svc.Close()

	raw := models.RawItemInput{
		Title: "Test Item",
		What:  "This is a test item",
		Tags:  []string{"test"},
	}

	result, err := svc.Store(raw, "test-project")
	if err != nil {
		t.Fatalf("Store() error = %v", err)
	}

	id, _ := result["id"].(string)
	if id == "" {
		t.Error("Store() should return item ID")
	}

	action, _ := result["action"].(string)
	if action != "created" {
		t.Errorf("Store() action = %q, want %q", result["action"], "created")
	}
}

func TestService_Search(t *testing.T) {
	tmpDir := t.TempDir()

	svc, err := NewService(tmpDir)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	defer svc.Close()

	// Store an item first
	raw := models.RawItemInput{
		Title: "Search Test",
		What:  "This is searchable content",
	}

	_, err = svc.Store(raw, "test-project")
	if err != nil {
		t.Fatalf("Store() error = %v", err)
	}

	// Search for it
	results, err := svc.Search("searchable", 5, nil, nil, false)
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}

	if len(results) == 0 {
		t.Error("Search() should return at least one result")
	}
}

func TestService_GetDetails(t *testing.T) {
	tmpDir := t.TempDir()

	svc, err := NewService(tmpDir)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	defer svc.Close()

	// Store an item with details
	details := "Full details here"
	raw := models.RawItemInput{
		Title:   "Details Test",
		What:    "Test item",
		Details: &details,
	}

	result, err := svc.Store(raw, "test-project")
	if err != nil {
		t.Fatalf("Store() error = %v", err)
	}

	// Retrieve details
	id, _ := result["id"].(string)

	detail, err := svc.GetDetails(id)
	if err != nil {
		t.Fatalf("GetDetails() error = %v", err)
	}

	if detail == nil {
		t.Fatal("GetDetails() returned nil")
	}

	if detail.Body != details {
		t.Errorf("GetDetails() Body = %q, want %q", detail.Body, details)
	}
}

func TestService_Remove(t *testing.T) {
	tmpDir := t.TempDir()

	svc, err := NewService(tmpDir)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	defer svc.Close()

	// Store an item
	raw := models.RawItemInput{
		Title: "Delete Test",
		What:  "This will be deleted",
	}

	result, err := svc.Store(raw, "test-project")
	if err != nil {
		t.Fatalf("Store() error = %v", err)
	}

	resultID, _ := result["id"].(string)

	// Delete it
	deleted, err := svc.Remove(resultID)
	if err != nil {
		t.Fatalf("Remove() error = %v", err)
	}

	if !deleted {
		t.Error("Remove() should return true for existing item")
	}

	// Try to delete again (should return false)
	deleted, err = svc.Remove(resultID)
	if err != nil {
		t.Fatalf("Remove() error = %v", err)
	}

	if deleted {
		t.Error("Remove() should return false for non-existent item")
	}
}

func TestService_Archive_HidesItemFromSearch(t *testing.T) {
	tmpDir := t.TempDir()

	svc, err := NewService(tmpDir)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	defer svc.Close()

	raw := models.RawItemInput{
		Title: "Archive Me",
		What:  "archive-specific-token",
	}

	result, err := svc.Store(raw, "test-project")
	if err != nil {
		t.Fatalf("Store() error = %v", err)
	}

	resultID, _ := result["id"].(string)
	if _, err := svc.Archive(resultID); err != nil {
		t.Fatalf("Archive() error = %v", err)
	}

	results, err := svc.Search("archive-specific-token", 5, nil, nil, false)
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}

	if len(results) != 0 {
		t.Fatalf("expected archived item to be hidden from active search, got %d results", len(results))
	}
}

func TestService_Update_ChangesContent(t *testing.T) {
	tmpDir := t.TempDir()

	svc, err := NewService(tmpDir)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	defer svc.Close()

	raw := models.RawItemInput{
		Title: "Update Me",
		What:  "old content",
	}

	result, err := svc.Store(raw, "test-project")
	if err != nil {
		t.Fatalf("Store() error = %v", err)
	}

	resultID, _ := result["id"].(string)
	newWhat := "new content token"

	if _, err := svc.Update(resultID, models.ItemUpdateInput{What: &newWhat}); err != nil {
		t.Fatalf("Update() error = %v", err)
	}

	results, err := svc.Search("new content token", 5, nil, nil, false)
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}

	if len(results) == 0 {
		t.Fatal("expected updated item to be searchable by new content")
	}
}

func TestService_Supersede_HidesItemFromRecentContext(t *testing.T) {
	tmpDir := t.TempDir()

	svc, err := NewService(tmpDir)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	defer svc.Close()

	raw := models.RawItemInput{
		Title: "Old Decision",
		What:  "legacy token",
	}

	result, err := svc.Store(raw, "test-project")
	if err != nil {
		t.Fatalf("Store() error = %v", err)
	}

	resultID, _ := result["id"].(string)

	if _, err := svc.Supersede(resultID, "new-decision-id"); err != nil {
		t.Fatalf("Supersede() error = %v", err)
	}

	results, _, err := svc.GetContext(10, nil, nil, nil, "never", false)
	if err != nil {
		t.Fatalf("GetContext() error = %v", err)
	}

	if len(results) != 0 {
		t.Fatalf("expected superseded item to be hidden from active context, got %d results", len(results))
	}
}

func TestService_GetContext_TotalReflectsActiveNotes(t *testing.T) {
	tmpDir := t.TempDir()

	svc, err := NewService(tmpDir)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	defer svc.Close()

	first, err := svc.Store(models.RawItemInput{Title: "Active", What: "active token"}, "test-project")
	if err != nil {
		t.Fatalf("Store() error = %v", err)
	}

	second, err := svc.Store(models.RawItemInput{Title: "Archive Later", What: "archive token"}, "test-project")
	if err != nil {
		t.Fatalf("Store() error = %v", err)
	}

	secondID, _ := second["id"].(string)
	if _, err := svc.Archive(secondID); err != nil {
		t.Fatalf("Archive() error = %v", err)
	}

	_, total, err := svc.GetContext(10, nil, nil, nil, "never", false)
	if err != nil {
		t.Fatalf("GetContext() error = %v", err)
	}

	if total != 1 {
		t.Fatalf("expected active total 1, got %d", total)
	}

	firstID, _ := first["id"].(string)
	if firstID == "" {
		t.Fatal("expected first note id")
	}
}

func TestService_ExplainSearch_FTSMode(t *testing.T) {
	tmpDir := t.TempDir()

	svc, err := NewService(tmpDir)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	defer svc.Close()

	if _, err := svc.Store(models.RawItemInput{Title: "Explain", What: "explain token"}, "test-project"); err != nil {
		t.Fatalf("Store() error = %v", err)
	}

	explanation, results, err := svc.ExplainSearch("explain token", 5, nil, nil, false)
	if err != nil {
		t.Fatalf("ExplainSearch() error = %v", err)
	}

	if explanation.Mode != "fts_only" {
		t.Fatalf("expected fts_only mode, got %q", explanation.Mode)
	}

	if len(results) == 0 {
		t.Fatal("expected explain search to return results")
	}
}

func TestService_SearchWithMode_DebugBoostsBugNote(t *testing.T) {
	tmpDir := t.TempDir()

	svc, err := NewService(tmpDir)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	defer svc.Close()

	bugCat := "bug"
	contextCat := "context"
	if _, err := svc.Store(models.RawItemInput{Title: "General Context", What: "shared token", Category: &contextCat}, "test-project"); err != nil {
		t.Fatalf("Store() context error = %v", err)
	}
	if _, err := svc.Store(models.RawItemInput{Title: "Bug Fix", What: "shared token", Category: &bugCat}, "test-project"); err != nil {
		t.Fatalf("Store() bug error = %v", err)
	}

	results, err := svc.SearchWithMode("shared token", 5, nil, nil, false, models.RetrievalDebug)
	if err != nil {
		t.Fatalf("SearchWithMode() error = %v", err)
	}

	if len(results) == 0 || results[0].Title != "Bug Fix" {
		t.Fatalf("expected bug note first in debug mode, got %+v", results)
	}
}

func TestService_Compact_CreatesCanonicalAndArchivesCoveredNotes(t *testing.T) {
	tmpDir := t.TempDir()

	svc, err := NewService(tmpDir)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	defer svc.Close()

	patternCat := "pattern"
	if _, err := svc.Store(models.RawItemInput{Title: "Pattern One", What: "compact token", Category: &patternCat}, "test-project"); err != nil {
		t.Fatalf("Store() error = %v", err)
	}
	if _, err := svc.Store(models.RawItemInput{Title: "Pattern Two", What: "compact token", Category: &patternCat}, "test-project"); err != nil {
		t.Fatalf("Store() error = %v", err)
	}

	result, err := svc.Compact(models.RawItemInput{
		Title: "Canonical Pattern Summary",
		What:  "Summarized recurring pattern",
	}, "test-project", "compact token", nil, 10, &patternCat)
	if err != nil {
		t.Fatalf("Compact() error = %v", err)
	}

	if result["covered_count"] != 2 {
		t.Fatalf("expected covered_count=2, got %v", result["covered_count"])
	}

	results, _, err := svc.GetContextWithMode(10, nil, nil, nil, "never", false, models.RetrievalStartup)
	if err != nil {
		t.Fatalf("GetContextWithMode() error = %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("expected only canonical active note in context, got %d", len(results))
	}

	if !results[0].IsCanonical {
		t.Fatalf("expected remaining note to be canonical")
	}
}

func TestService_Compact_UsesNarrowCandidateSelection(t *testing.T) {
	tmpDir := t.TempDir()

	svc, err := NewService(tmpDir)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	defer svc.Close()

	patternCat := "pattern"
	source := "codex"
	sharedTags := []string{"pool", "db"}
	if _, err := svc.Store(models.RawItemInput{
		Title:    "Connection pool tuning",
		What:     "Raised pool size and timeout",
		Category: &patternCat,
		Source:   &source,
		Tags:     sharedTags,
	}, "test-project"); err != nil {
		t.Fatalf("Store() primary one error = %v", err)
	}
	if _, err := svc.Store(models.RawItemInput{
		Title:    "Connection pool timeout",
		What:     "Matched the same pool tuning workstream",
		Category: &patternCat,
		Source:   &source,
		Tags:     sharedTags,
	}, "test-project"); err != nil {
		t.Fatalf("Store() primary two error = %v", err)
	}

	otherCat := "context"
	if _, err := svc.Store(models.RawItemInput{
		Title:    "Connection issue elsewhere",
		What:     "Unrelated note that only shares a broad term",
		Category: &otherCat,
	}, "test-project"); err != nil {
		t.Fatalf("Store() broad-match error = %v", err)
	}

	result, err := svc.Compact(models.RawItemInput{
		Title: "Canonical connection pool summary",
		What:  "Summarized pool work",
	}, "test-project", "connection pool tuning", nil, 10, &patternCat)
	if err != nil {
		t.Fatalf("Compact() error = %v", err)
	}

	if result["covered_count"] != 2 {
		t.Fatalf("expected covered_count=2, got %v", result["covered_count"])
	}
}

func TestService_Reindex_RemovesDBEntriesMissingFromShelves(t *testing.T) {
	tmpDir := t.TempDir()

	svc, err := NewService(tmpDir)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	defer svc.Close()

	svc.embeddingOnce.Do(func() {
		svc.embeddingProvider = &testEmbedder{}
	})

	project := "test-project"
	first, err := svc.Store(models.RawItemInput{Title: "Keep Me", What: "still in shelves"}, project)
	if err != nil {
		t.Fatalf("Store() first error = %v", err)
	}
	second, err := svc.Store(models.RawItemInput{Title: "Delete Me", What: "removed from shelves"}, project)
	if err != nil {
		t.Fatalf("Store() second error = %v", err)
	}

	filePath, _ := first["file_path"].(string)
	secondID, _ := second["id"].(string)

	content, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("os.ReadFile() error = %v", err)
	}

	lines := strings.Split(string(content), "\n")
	filtered := make([]string, 0, len(lines))
	skip := false
	for _, line := range lines {
		if line == "### Delete Me" {
			skip = true
			continue
		}
		if skip && strings.HasPrefix(line, "### ") {
			skip = false
		}
		if !skip {
			filtered = append(filtered, line)
		}
	}

	if err := os.WriteFile(filePath, []byte(strings.Join(filtered, "\n")), 0644); err != nil {
		t.Fatalf("os.WriteFile() error = %v", err)
	}

	result, err := svc.Reindex(&project, nil)
	if err != nil {
		t.Fatalf("Reindex() error = %v", err)
	}

	if result["deleted"] != 1 {
		t.Fatalf("expected one DB-only note deleted, got %v", result["deleted"])
	}

	got, _, err := svc.db.GetItem(secondID)
	if err != nil {
		t.Fatalf("GetItem() error = %v", err)
	}
	if got != nil {
		t.Fatal("expected missing shelf note to be deleted from DB")
	}
}

func TestService_RemoveProject_RemovesDBAndShelves(t *testing.T) {
	tmpDir := t.TempDir()

	svc, err := NewService(tmpDir)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	defer svc.Close()

	project := "test-project"
	if _, err := svc.Store(models.RawItemInput{Title: "One", What: "first"}, project); err != nil {
		t.Fatalf("Store() error = %v", err)
	}
	if _, err := svc.Store(models.RawItemInput{Title: "Two", What: "second"}, project); err != nil {
		t.Fatalf("Store() error = %v", err)
	}

	deleted, err := svc.RemoveProject(project)
	if err != nil {
		t.Fatalf("RemoveProject() error = %v", err)
	}
	if deleted != 2 {
		t.Fatalf("RemoveProject() deleted = %d, want 2", deleted)
	}

	count, err := svc.CountItems(&project, nil)
	if err != nil {
		t.Fatalf("CountItems() error = %v", err)
	}
	if count != 0 {
		t.Fatalf("CountItems() = %d, want 0", count)
	}

	if _, err := os.Stat(filepath.Join(tmpDir, "shelves", project)); !os.IsNotExist(err) {
		t.Fatalf("expected shelves dir to be removed, got err=%v", err)
	}
}
