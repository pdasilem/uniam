package mcp

import (
	"errors"
	"testing"

	"uniam/internal/models"
)

func stringPtr(s string) *string {
	return &s
}

// --- Stub implementation of uniamService ---

type stubService struct {
	storeResult    map[string]any
	storeErr       error
	searchResults  []models.SearchResult
	searchErr      error
	contextResults []models.SearchResult
	contextTotal   int64
	contextErr     error
}

//nolint:revive
func (s *stubService) Store(raw models.RawItemInput, project string) (map[string]any, error) {
	return s.storeResult, s.storeErr
}

//nolint:revive
func (s *stubService) Search(query string, limit int, project *string, source *string, useVectors bool) ([]models.SearchResult, error) {
	return s.searchResults, s.searchErr
}

//nolint:revive
func (s *stubService) GetContext(limit int, project *string, source *string, query *string, semanticMode string, topupRecent bool) ([]models.SearchResult, int64, error) {
	return s.contextResults, s.contextTotal, s.contextErr
}

func (s *stubService) Close() error { return nil }

// --- HandleUniamStore tests ---

func TestHandleUniamStore_Success(t *testing.T) {
	svc := &stubService{
		storeResult: map[string]any{
			"id":        "abc-123",
			"file_path": "/tmp/session.md",
			"action":    "created",
		},
	}

	params := map[string]any{
		"title": "My Title",
		"what":  "What happened",
	}

	result, err := HandleUniamStore(svc, params)
	if err != nil {
		t.Fatalf("HandleUniamStore() error = %v", err)
	}

	if result["id"] != "abc-123" {
		t.Errorf("id = %v, want abc-123", result["id"])
	}

	if result["action"] != "created" {
		t.Errorf("action = %v, want created", result["action"])
	}
}

func TestHandleUniamStore_PropagatesError(t *testing.T) {
	svc := &stubService{
		storeErr: errors.New("storage failure"),
	}

	params := map[string]any{
		"title": "T",
		"what":  "W",
	}

	_, err := HandleUniamStore(svc, params)
	if err == nil {
		t.Fatal("HandleUniamStore() should propagate service error")
	}
}

func TestHandleUniamStore_TagsFromCommaString(t *testing.T) {
	var capturedRaw models.RawItemInput

	svc := &stubService{}
	svc.storeResult = map[string]any{"id": "x", "file_path": "/f", "action": "created"}

	// We'll verify tag parsing by building a custom stub that captures the call
	captureSvc := &capturingStub{}
	params := map[string]any{
		"title": "T",
		"what":  "W",
		"tags":  "golang,testing,refactor",
	}

	_, err := HandleUniamStore(captureSvc, params)
	if err != nil {
		t.Fatalf("HandleUniamStore() error = %v", err)
	}

	capturedRaw = captureSvc.lastRaw
	if len(capturedRaw.Tags) != 3 {
		t.Errorf("Tags len = %d, want 3; got %v", len(capturedRaw.Tags), capturedRaw.Tags)
	}
}

func TestHandleUniamStore_TagsFromJSONArray(t *testing.T) {
	captureSvc := &capturingStub{}
	params := map[string]any{
		"title": "T",
		"what":  "W",
		"tags":  `["go","mcp"]`,
	}

	_, err := HandleUniamStore(captureSvc, params)
	if err != nil {
		t.Fatalf("HandleUniamStore() error = %v", err)
	}

	if len(captureSvc.lastRaw.Tags) != 2 {
		t.Errorf("Tags from JSON = %v, want [go mcp]", captureSvc.lastRaw.Tags)
	}
}

func TestHandleUniamStore_TagsFromNativeArray(t *testing.T) {
	captureSvc := &capturingStub{}
	params := map[string]any{
		"title": "T",
		"what":  "W",
		"tags":  []any{"alpha", "beta"},
	}

	_, err := HandleUniamStore(captureSvc, params)
	if err != nil {
		t.Fatalf("HandleUniamStore() error = %v", err)
	}

	if len(captureSvc.lastRaw.Tags) != 2 {
		t.Errorf("Tags from native array = %v, want [alpha beta]", captureSvc.lastRaw.Tags)
	}
}

// capturingStub records the last Store() call for inspection.
type capturingStub struct {
	lastRaw     models.RawItemInput
	lastProject string
}

func (c *capturingStub) Store(raw models.RawItemInput, project string) (map[string]any, error) {
	c.lastRaw = raw
	c.lastProject = project

	return map[string]any{"id": "x", "file_path": "/f", "action": "created"}, nil
}
func (c *capturingStub) Search(_ string, _ int, _ *string, _ *string, _ bool) ([]models.SearchResult, error) {
	return nil, nil
}
func (c *capturingStub) GetContext(_ int, _ *string, _ *string, _ *string, _ string, _ bool) ([]models.SearchResult, int64, error) {
	return nil, 0, nil
}
func (c *capturingStub) Close() error { return nil }

// --- HandleUniamSearch tests ---

func TestHandleUniamSearch_NoResults(t *testing.T) {
	svc := &stubService{searchResults: []models.SearchResult{}}

	params := map[string]any{
		"query": "something",
	}

	results, err := HandleUniamSearch(svc, params)
	if err != nil {
		t.Fatalf("HandleUniamSearch() error = %v", err)
	}

	if len(results) != 0 {
		t.Errorf("expected empty results, got %d", len(results))
	}
}

func TestHandleUniamSearch_WithResults(t *testing.T) {
	svc := &stubService{
		searchResults: []models.SearchResult{
			{
				ID:        "item-1",
				Title:     "Some Decision",
				What:      "We decided X",
				Category:  stringPtr("decision"),
				Source:    stringPtr("claude"),
				Tags:      []string{"arch"},
				Project:   "myproject",
				Score:     0.95,
				CreatedAt: "2024-01-01T00:00:00Z",
			},
		},
	}

	params := map[string]any{
		"query": "decision",
		"limit": float64(5),
	}

	results, err := HandleUniamSearch(svc, params)
	if err != nil {
		t.Fatalf("HandleUniamSearch() error = %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	if results[0]["id"] != "item-1" {
		t.Errorf("id = %v, want item-1", results[0]["id"])
	}

	if results[0]["title"] != "Some Decision" {
		t.Errorf("title = %v, want Some Decision", results[0]["title"])
	}

	if results[0]["score"] != 0.95 {
		t.Errorf("score = %v, want 0.95", results[0]["score"])
	}
}

func TestHandleUniamSearch_PropagatesError(t *testing.T) {
	svc := &stubService{searchErr: errors.New("search failed")}

	_, err := HandleUniamSearch(svc, map[string]any{"query": "x"})
	if err == nil {
		t.Fatal("HandleUniamSearch() should propagate service error")
	}
}

// --- HandleUniamContext tests ---

func TestHandleUniamContext_DefaultLimit(t *testing.T) {
	svc := &stubService{
		contextResults: []models.SearchResult{},
		contextTotal:   42,
	}

	result, err := HandleUniamContext(svc, map[string]any{})
	if err != nil {
		t.Fatalf("HandleUniamContext() error = %v", err)
	}

	if result["total"] != int64(42) {
		t.Errorf("total = %v, want 42", result["total"])
	}

	if result["showing"] != 0 {
		t.Errorf("showing = %v, want 0", result["showing"])
	}
}

func TestHandleUniamContext_LimitParam(t *testing.T) {
	called := false
	capSvc := &contextCapturingStub{onContext: func(_ int) {
		called = true
	}}

	params := map[string]any{
		"limit": float64(20),
	}

	_, err := HandleUniamContext(capSvc, params)
	if err != nil {
		t.Fatalf("HandleUniamContext() error = %v", err)
	}

	_ = called

	if capSvc.lastLimit != 20 {
		t.Errorf("limit passed to GetContext = %d, want 20", capSvc.lastLimit)
	}
}

func TestHandleUniamContext_PropagatesError(t *testing.T) {
	svc := &stubService{contextErr: errors.New("context failed")}

	_, err := HandleUniamContext(svc, map[string]any{})
	if err == nil {
		t.Fatal("HandleUniamContext() should propagate service error")
	}
}

type contextCapturingStub struct {
	lastLimit int
	onContext func(int)
}

//nolint:revive
func (c *contextCapturingStub) Store(raw models.RawItemInput, project string) (map[string]any, error) {
	return map[string]any{"id": "x", "file_path": "/f", "action": "created"}, nil
}
func (c *contextCapturingStub) Search(_ string, _ int, _ *string, _ *string, _ bool) ([]models.SearchResult, error) {
	return nil, nil
}
func (c *contextCapturingStub) GetContext(limit int, _ *string, _ *string, _ *string, _ string, _ bool) ([]models.SearchResult, int64, error) {
	c.lastLimit = limit
	if c.onContext != nil {
		c.onContext(limit)
	}

	return []models.SearchResult{}, 0, nil
}
func (c *contextCapturingStub) Close() error { return nil }

// --- getStringSliceFromMap tests ---

func TestGetStringSliceFromMap_CommaString(t *testing.T) {
	m := map[string]any{"tags": "go,testing,mcp"}

	result, ok := getStringSliceFromMap(m, "tags")
	if !ok {
		t.Fatal("getStringSliceFromMap() ok = false, want true")
	}

	if len(result) != 3 {
		t.Errorf("len = %d, want 3; got %v", len(result), result)
	}
}

func TestGetStringSliceFromMap_JSONArray(t *testing.T) {
	m := map[string]any{"tags": `["alpha","beta","gamma"]`}

	result, ok := getStringSliceFromMap(m, "tags")
	if !ok {
		t.Fatal("getStringSliceFromMap() ok = false, want true")
	}

	if len(result) != 3 || result[0] != "alpha" {
		t.Errorf("result = %v, want [alpha beta gamma]", result)
	}
}

func TestGetStringSliceFromMap_NativeSlice(t *testing.T) {
	m := map[string]any{"tags": []any{"x", "y"}}

	result, ok := getStringSliceFromMap(m, "tags")
	if !ok {
		t.Fatal("getStringSliceFromMap() ok = false, want true")
	}

	if len(result) != 2 {
		t.Errorf("len = %d, want 2", len(result))
	}
}

func TestGetStringSliceFromMap_MissingKey(t *testing.T) {
	m := map[string]any{}

	_, ok := getStringSliceFromMap(m, "tags")
	if ok {
		t.Error("getStringSliceFromMap() should return ok=false for missing key")
	}
}

func TestGetStringSliceFromMap_EmptyCommaString(t *testing.T) {
	m := map[string]any{"tags": "   "}

	_, ok := getStringSliceFromMap(m, "tags")
	if ok {
		t.Error("getStringSliceFromMap() should return ok=false for blank string")
	}
}
