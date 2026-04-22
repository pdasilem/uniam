package db

import (
	"path/filepath"
	"testing"
	"time"

	"uniam/internal/models"
)

func stringPtr(s string) *string {
	return &s
}

// newTestDB creates a fresh in-memory-backed DB in a temp directory.
func newTestDB(t *testing.T) *DB {
	t.Helper()
	dir := t.TempDir()

	database, err := NewDB(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("NewDB() error = %v", err)
	}

	t.Cleanup(func() { _ = database.Close() })

	return database
}

// makeItem returns a minimal valid Item for testing.
func makeItem(title, project string) models.Item {
	now := time.Now().UTC().Format(time.RFC3339)

	return models.Item{
		ID:        title + "-id",
		Title:     title,
		What:      "what for " + title,
		Tags:      []string{"tag1", "tag2"},
		Project:   project,
		FilePath:  "/tmp/" + title + ".md",
		CreatedAt: now,
		UpdatedAt: now,
	}
}

// --- InsertItem / GetItem ---

func TestInsertItem_GetItem_Roundtrip(t *testing.T) {
	d := newTestDB(t)
	item := makeItem("Test Item", "myproject")
	why := "because why"
	item.Why = &why

	rowid, err := d.InsertItem(item, nil)
	if err != nil {
		t.Fatalf("InsertItem() error = %v", err)
	}

	if rowid <= 0 {
		t.Errorf("InsertItem() rowid = %d, want > 0", rowid)
	}

	got, hasDetails, err := d.GetItem(item.ID)
	if err != nil {
		t.Fatalf("GetItem() error = %v", err)
	}

	if got == nil {
		t.Fatal("GetItem() returned nil")
	}

	if got.Title != item.Title {
		t.Errorf("Title = %q, want %q", got.Title, item.Title)
	}

	if got.What != item.What {
		t.Errorf("What = %q, want %q", got.What, item.What)
	}

	if got.Why == nil || *got.Why != why {
		t.Errorf("Why = %v, want %q", got.Why, why)
	}

	if len(got.Tags) != 2 || got.Tags[0] != "tag1" {
		t.Errorf("Tags = %v, want [tag1 tag2]", got.Tags)
	}

	if hasDetails {
		t.Error("hasDetails should be false when no details stored")
	}
}

func TestInsertItem_NotFound(t *testing.T) {
	d := newTestDB(t)

	got, _, err := d.GetItem("nonexistent-id")
	if err != nil {
		t.Fatalf("GetItem() error = %v", err)
	}

	if got != nil {
		t.Error("GetItem() should return nil for nonexistent id")
	}
}

// --- Details ---

func TestInsertItem_GetDetails_Roundtrip(t *testing.T) {
	d := newTestDB(t)
	item := makeItem("Detailed Item", "proj")
	details := "This is the full detail body"

	_, err := d.InsertItem(item, &details)
	if err != nil {
		t.Fatalf("InsertItem() error = %v", err)
	}

	_, hasDetails, err := d.GetItem(item.ID)
	if err != nil {
		t.Fatalf("GetItem() error = %v", err)
	}

	if !hasDetails {
		t.Error("hasDetails should be true when details stored")
	}

	d2, err := d.GetDetails(item.ID)
	if err != nil {
		t.Fatalf("GetDetails() error = %v", err)
	}

	if d2 == nil {
		t.Fatal("GetDetails() returned nil")
	}

	if d2.Body != details {
		t.Errorf("Body = %q, want %q", d2.Body, details)
	}
}

func TestGetDetails_NotFound(t *testing.T) {
	d := newTestDB(t)

	detail, err := d.GetDetails("nonexistent")
	if err != nil {
		t.Fatalf("GetDetails() error = %v", err)
	}

	if detail != nil {
		t.Error("GetDetails() should return nil for nonexistent item")
	}
}

// --- FTSSearch ---

func TestFTSSearch_Match(t *testing.T) {
	d := newTestDB(t)
	item := makeItem("FTS Match Test", "proj")
	item.What = "unique searchable keyword xyzzy"

	_, err := d.InsertItem(item, nil)
	if err != nil {
		t.Fatalf("InsertItem() error = %v", err)
	}

	results, err := d.FTSSearch("xyzzy", 5, nil, nil)
	if err != nil {
		t.Fatalf("FTSSearch() error = %v", err)
	}

	if len(results) == 0 {
		t.Error("FTSSearch() should find item with matching keyword")
	}

	if results[0].Title != item.Title {
		t.Errorf("FTSSearch() result title = %q, want %q", results[0].Title, item.Title)
	}
}

func TestFTSSearch_NoMatch(t *testing.T) {
	d := newTestDB(t)
	item := makeItem("No Match Test", "proj")

	_, err := d.InsertItem(item, nil)
	if err != nil {
		t.Fatalf("InsertItem() error = %v", err)
	}

	results, err := d.FTSSearch("zzznomatch999", 5, nil, nil)
	if err != nil {
		t.Fatalf("FTSSearch() error = %v", err)
	}

	if len(results) != 0 {
		t.Errorf("FTSSearch() should return 0 results for non-matching query, got %d", len(results))
	}
}

func TestFTSSearch_ProjectFilter(t *testing.T) {
	d := newTestDB(t)

	item1 := makeItem("Project A Item", "projectA")
	item1.What = "unique qwerty content"
	item2 := makeItem("Project B Item", "projectB")
	item2.What = "unique qwerty content"

	if _, err := d.InsertItem(item1, nil); err != nil {
		t.Fatalf("InsertItem() error = %v", err)
	}

	if _, err := d.InsertItem(item2, nil); err != nil {
		t.Fatalf("InsertItem() error = %v", err)
	}

	results, err := d.FTSSearch("qwerty", 10, stringPtr("projectA"), nil)
	if err != nil {
		t.Fatalf("FTSSearch() error = %v", err)
	}

	if len(results) != 1 {
		t.Errorf("FTSSearch() with project filter should return 1 result, got %d", len(results))
	}

	if results[0].Project != "projectA" {
		t.Errorf("FTSSearch() result project = %q, want projectA", results[0].Project)
	}
}

// --- UpdateItem ---

func TestUpdateItem(t *testing.T) {
	d := newTestDB(t)
	item := makeItem("Update Target", "proj")

	_, err := d.InsertItem(item, nil)
	if err != nil {
		t.Fatalf("InsertItem() error = %v", err)
	}

	newWhat := "updated what field"
	newTags := []string{"newtag"}

	err = d.UpdateItem(item.ID, &newWhat, nil, nil, newTags, nil)
	if err != nil {
		t.Fatalf("UpdateItem() error = %v", err)
	}

	got, _, err := d.GetItem(item.ID)
	if err != nil {
		t.Fatalf("GetItem() after update error = %v", err)
	}

	if got.What != newWhat {
		t.Errorf("What after update = %q, want %q", got.What, newWhat)
	}
}

func TestUpdateItem_DetailsAppend(t *testing.T) {
	d := newTestDB(t)
	item := makeItem("Details Append Test", "proj")
	original := "original details"

	_, err := d.InsertItem(item, &original)
	if err != nil {
		t.Fatalf("InsertItem() error = %v", err)
	}

	err = d.UpdateItem(item.ID, nil, nil, nil, nil, stringPtr("new appended content"))
	if err != nil {
		t.Fatalf("UpdateItem() error = %v", err)
	}

	detail, err := d.GetDetails(item.ID)
	if err != nil {
		t.Fatalf("GetDetails() error = %v", err)
	}

	if detail == nil {
		t.Fatal("GetDetails() returned nil after append")
	}

	if detail.Body == original {
		t.Error("Details body should have been appended to")
	}
}

func TestUpdateItem_NotFound(t *testing.T) {
	d := newTestDB(t)

	err := d.UpdateItem("nonexistent", nil, nil, nil, nil, nil)
	if err == nil {
		t.Error("UpdateItem() should return error for nonexistent item")
	}
}

func TestUpdateStatus_ArchiveHidesFromActiveRetrieval(t *testing.T) {
	d := newTestDB(t)
	item := makeItem("Archive Target", "proj")

	if _, err := d.InsertItem(item, nil); err != nil {
		t.Fatalf("InsertItem() error = %v", err)
	}

	now := time.Now().UTC().Format(time.RFC3339)
	if err := d.UpdateStatus(item.ID, models.StatusArchived, nil, &now, nil); err != nil {
		t.Fatalf("UpdateStatus() error = %v", err)
	}

	results, err := d.ListRecent(10, nil, nil)
	if err != nil {
		t.Fatalf("ListRecent() error = %v", err)
	}

	if len(results) != 0 {
		t.Fatalf("expected archived item to be hidden from active recent list, got %d results", len(results))
	}
}

func TestUpdateStatus_SupersededHidesFromActiveSearch(t *testing.T) {
	d := newTestDB(t)
	item := makeItem("Supersede Target", "proj")
	item.What = "unique supersede search token"

	if _, err := d.InsertItem(item, nil); err != nil {
		t.Fatalf("InsertItem() error = %v", err)
	}

	byID := "new-note-id"
	if err := d.UpdateStatus(item.ID, models.StatusSuperseded, &byID, nil, nil); err != nil {
		t.Fatalf("UpdateStatus() error = %v", err)
	}

	results, err := d.FTSSearch("supersede", 10, nil, nil)
	if err != nil {
		t.Fatalf("FTSSearch() error = %v", err)
	}

	if len(results) != 0 {
		t.Fatalf("expected superseded item to be hidden from active search, got %d results", len(results))
	}
}

// --- DeleteItem ---

func TestDeleteItem_ExistingItem(t *testing.T) {
	d := newTestDB(t)
	item := makeItem("Delete Me", "proj")

	_, err := d.InsertItem(item, nil)
	if err != nil {
		t.Fatalf("InsertItem() error = %v", err)
	}

	deleted, err := d.DeleteItem(item.ID)
	if err != nil {
		t.Fatalf("DeleteItem() error = %v", err)
	}

	if !deleted {
		t.Error("DeleteItem() should return true for existing item")
	}

	// Confirm item is gone
	got, _, err := d.GetItem(item.ID)
	if err != nil {
		t.Fatalf("GetItem() after delete error = %v", err)
	}

	if got != nil {
		t.Error("GetItem() should return nil after deletion")
	}
}

func TestDeleteItem_NonExistent(t *testing.T) {
	d := newTestDB(t)

	deleted, err := d.DeleteItem("does-not-exist")
	if err != nil {
		t.Fatalf("DeleteItem() error = %v", err)
	}

	if deleted {
		t.Error("DeleteItem() should return false for nonexistent item")
	}
}

// --- ListRecent ---

func TestListRecent_OrderByCreatedAtDesc(t *testing.T) {
	d := newTestDB(t)

	// Insert items with different timestamps (sleep ensures distinct ordering)
	for _, title := range []string{"First", "Second", "Third"} {
		item := makeItem(title, "proj")

		item.ID = title + "-uuid"

		if _, err := d.InsertItem(item, nil); err != nil {
			t.Fatalf("InsertItem(%s) error = %v", title, err)
		}
	}

	results, err := d.ListRecent(10, nil, nil)
	if err != nil {
		t.Fatalf("ListRecent() error = %v", err)
	}

	if len(results) != 3 {
		t.Errorf("ListRecent() len = %d, want 3", len(results))
	}
}

func TestListRecent_LimitRespected(t *testing.T) {
	d := newTestDB(t)

	for i := range 5 {
		item := makeItem("item", "proj")

		item.ID = "item-uuid-" + string(rune('0'+i))

		if _, err := d.InsertItem(item, nil); err != nil {
			t.Fatalf("InsertItem() error = %v", err)
		}
	}

	results, err := d.ListRecent(3, nil, nil)
	if err != nil {
		t.Fatalf("ListRecent() error = %v", err)
	}

	if len(results) != 3 {
		t.Errorf("ListRecent() with limit 3 returned %d results", len(results))
	}
}

// --- CountItems ---

func TestCountItems(t *testing.T) {
	d := newTestDB(t)

	count, err := d.CountItems(nil, nil)
	if err != nil {
		t.Fatalf("CountItems() error = %v", err)
	}

	if count != 0 {
		t.Errorf("CountItems() on empty db = %d, want 0", count)
	}

	for _, title := range []string{"A", "B", "C"} {
		item := makeItem(title, "proj")

		item.ID = title + "-count-uuid"

		if _, err := d.InsertItem(item, nil); err != nil {
			t.Fatalf("InsertItem() error = %v", err)
		}
	}

	count, err = d.CountItems(nil, nil)
	if err != nil {
		t.Fatalf("CountItems() error = %v", err)
	}

	if count != 3 {
		t.Errorf("CountItems() = %d, want 3", count)
	}
}

func TestCountItems_ProjectFilter(t *testing.T) {
	d := newTestDB(t)
	itemA := makeItem("A", "alpha")
	itemA.ID = "alpha-uuid"
	itemB := makeItem("B", "beta")
	itemB.ID = "beta-uuid"

	if _, err := d.InsertItem(itemA, nil); err != nil {
		t.Fatalf("InsertItem() error = %v", err)
	}

	if _, err := d.InsertItem(itemB, nil); err != nil {
		t.Fatalf("InsertItem() error = %v", err)
	}

	count, err := d.CountItems(stringPtr("alpha"), nil)
	if err != nil {
		t.Fatalf("CountItems() error = %v", err)
	}

	if count != 1 {
		t.Errorf("CountItems(alpha) = %d, want 1", count)
	}
}

func TestStats_CountsLifecycleAndDetails(t *testing.T) {
	d := newTestDB(t)

	active := makeItem("Active Stats", "alpha")
	archived := makeItem("Archived Stats", "alpha")
	superseded := makeItem("Superseded Stats", "beta")

	if _, err := d.InsertItem(active, stringPtr("details")); err != nil {
		t.Fatalf("InsertItem() error = %v", err)
	}
	if _, err := d.InsertItem(archived, nil); err != nil {
		t.Fatalf("InsertItem() error = %v", err)
	}
	if _, err := d.InsertItem(superseded, nil); err != nil {
		t.Fatalf("InsertItem() error = %v", err)
	}

	now := time.Now().UTC().Format(time.RFC3339)
	if err := d.UpdateStatus(archived.ID, models.StatusArchived, nil, &now, nil); err != nil {
		t.Fatalf("UpdateStatus(archived) error = %v", err)
	}
	byID := "replacement-id"
	if err := d.UpdateStatus(superseded.ID, models.StatusSuperseded, &byID, nil, nil); err != nil {
		t.Fatalf("UpdateStatus(superseded) error = %v", err)
	}

	stats, err := d.Stats(nil, nil)
	if err != nil {
		t.Fatalf("Stats() error = %v", err)
	}

	if stats.Total != 3 || stats.Active != 1 || stats.Archived != 1 || stats.Superseded != 1 {
		t.Fatalf("unexpected stats totals: %+v", *stats)
	}

	if stats.WithDetails != 1 {
		t.Fatalf("expected WithDetails=1, got %d", stats.WithDetails)
	}

	if stats.ByProject["alpha"] != 2 || stats.ByProject["beta"] != 1 {
		t.Fatalf("unexpected project stats: %+v", stats.ByProject)
	}
}

// --- ListAllForReindex ---

func TestListAllForReindex_HasRowid(t *testing.T) {
	d := newTestDB(t)
	item := makeItem("Reindex Item", "proj")

	_, err := d.InsertItem(item, nil)
	if err != nil {
		t.Fatalf("InsertItem() error = %v", err)
	}

	items, err := d.ListAllForReindex()
	if err != nil {
		t.Fatalf("ListAllForReindex() error = %v", err)
	}

	if len(items) != 1 {
		t.Fatalf("ListAllForReindex() len = %d, want 1", len(items))
	}

	rowid, ok := items[0]["rowid"].(int64)
	if !ok || rowid <= 0 {
		t.Errorf("ListAllForReindex() rowid = %v, want int64 > 0", items[0]["rowid"])
	}

	if items[0]["title"] != item.Title {
		t.Errorf("title = %v, want %q", items[0]["title"], item.Title)
	}
}

// --- EnsureVecTable / HasVecTable ---

func TestHasVecTable_FalseByDefault(t *testing.T) {
	d := newTestDB(t)
	if d.HasVecTable() {
		t.Error("HasVecTable() should be false on fresh DB without embedding dim")
	}
}

func TestEnsureVecTable_CreatesTable(t *testing.T) {
	d := newTestDB(t)

	err := d.EnsureVecTable(384)
	if err != nil {
		t.Fatalf("EnsureVecTable(384) error = %v", err)
	}

	if !d.HasVecTable() {
		t.Error("HasVecTable() should be true after EnsureVecTable")
	}
}

func TestEnsureVecTable_DimensionMismatch(t *testing.T) {
	d := newTestDB(t)

	// First call establishes dimension
	if err := d.EnsureVecTable(384); err != nil {
		t.Fatalf("EnsureVecTable(384) error = %v", err)
	}

	// Second call with different dimension should fail
	err := d.EnsureVecTable(768)
	if err == nil {
		t.Fatal("EnsureVecTable() should fail on dimension mismatch")
	}
}
