package core

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"uniam/internal/config"
	"uniam/internal/db"
	"uniam/internal/embeddings"
	"uniam/internal/models"
	"uniam/internal/redaction"
	"uniam/internal/search"
	"uniam/internal/storage"
)

const (
	// DedupScoreThreshold is the minimum normalized FTS score (0–1) combined
	// with an exact title match required to treat a new store as an update.
	DedupScoreThreshold = 0.7
)

// Option is a functional option for NewService.
type Option func(*Service)

// WithStore injects a custom db.Store implementation, primarily for testing.
func WithStore(s db.Store) Option {
	return func(svc *Service) { svc.db = s }
}

// Service is the main orchestrator for uniam operations.
type Service struct {
	uniamHome      string
	shelvesDir     string
	dbPath         string
	configPath     string
	ignorePath     string
	config         *config.Config
	db             db.Store
	compiledIgnore []*regexp.Regexp // pre-compiled from .uniamignore

	// Lazy-initialized, protected by sync.Once for safety under concurrent access.
	embeddingOnce     sync.Once
	embeddingProvider embeddings.Provider
	embeddingErr      error

	vectorsOnce      sync.Once
	vectorsAvailable bool
}

// NewService creates a new uniam service. Pass Option values to override
// defaults (e.g., WithStore for testing).
func NewService(uniamHome string, opts ...Option) (*Service, error) {
	if uniamHome == "" {
		uniamHome = config.GetUniamHome()
	}

	shelvesDir := filepath.Join(uniamHome, "shelves")
	dbPath := filepath.Join(uniamHome, "index.db")
	configPath := filepath.Join(uniamHome, "config.yaml")
	ignorePath := filepath.Join(uniamHome, ".uniamignore")

	// Ensure shelves directory exists
	if err := os.MkdirAll(shelvesDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create shelves directory: %w", err)
	}

	// Load and validate configuration
	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	// Initialize database
	database, err := db.NewDB(dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize database: %w", err)
	}

	// Load ignore patterns (.uniamignore missing is fine; other errors are surfaced)
	ignorePatterns, ignoreErr := redaction.LoadUniamIgnore(ignorePath)
	if ignoreErr != nil && !os.IsNotExist(ignoreErr) {
		fmt.Fprintf(os.Stderr, "warning: failed to load .uniamignore: %v\n", ignoreErr)
	}

	svc := &Service{
		uniamHome:      uniamHome,
		shelvesDir:     shelvesDir,
		dbPath:         dbPath,
		configPath:     configPath,
		ignorePath:     ignorePath,
		config:         cfg,
		db:             database,
		compiledIgnore: redaction.CompilePatterns(ignorePatterns),
	}

	for _, o := range opts {
		o(svc)
	}

	return svc, nil
}

// GetEmbeddingProvider returns the embedding provider, lazily initializing if needed.
// Safe for concurrent use.
func (s *Service) GetEmbeddingProvider() (embeddings.Provider, error) {
	s.embeddingOnce.Do(func() {
		s.embeddingProvider, s.embeddingErr = embeddings.NewProvider(s.config.Embedding)
	})

	return s.embeddingProvider, s.embeddingErr
}

// VectorsAvailable checks if vector operations are available.
// Safe for concurrent use.
func (s *Service) VectorsAvailable() bool {
	s.vectorsOnce.Do(func() {
		s.vectorsAvailable = s.db.HasVecTable()
	})

	return s.vectorsAvailable
}

// Config returns the loaded runtime config.
func (s *Service) Config() *config.Config {
	return s.config
}

// CountItems returns the total number of stored notes, optionally filtered.
func (s *Service) CountItems(project *string, source *string) (int64, error) {
	return s.db.CountItems(project, source)
}

// Stats returns aggregated counters about the stored notes.
func (s *Service) Stats(project *string, source *string) (*models.Stats, error) {
	return s.db.Stats(project, source)
}

// Store stores an item in the uniam.
func (s *Service) Store(raw models.RawItemInput, project string) (map[string]any, error) {
	if project == "" {
		project = filepath.Base(getCurrentDir())
	}

	today := time.Now().UTC().Format("2006-01-02")
	projectDir := filepath.Join(s.shelvesDir, project)

	// Ensure project directory exists
	if err := os.MkdirAll(projectDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create project directory: %w", err)
	}

	// Redact all text fields using pre-compiled patterns
	raw.What = redaction.RedactCompiled(raw.What, s.compiledIgnore)
	if raw.Why != nil {
		redacted := redaction.RedactCompiled(*raw.Why, s.compiledIgnore)
		raw.Why = &redacted
	}

	if raw.Impact != nil {
		redacted := redaction.RedactCompiled(*raw.Impact, s.compiledIgnore)
		raw.Impact = &redacted
	}

	if raw.Details != nil {
		redacted := redaction.RedactCompiled(*raw.Details, s.compiledIgnore)
		raw.Details = &redacted
	}

	// Dedup check: look for similar existing item in same project
	if result, err := s.tryDedup(raw, project, today); err != nil {
		return nil, err
	} else if result != nil {
		return result, nil
	}

	// Normal save path: create new item
	filePath := filepath.Join(projectDir, today+"-notes.md")
	item := models.FromRaw(raw, project, filePath)

	// Write markdown file
	if _, err := storage.WriteNoteItem(projectDir, item, today, raw.Details); err != nil {
		return nil, fmt.Errorf("failed to write session file: %w", err)
	}

	// Insert into database
	rowid, err := s.db.InsertItem(item, raw.Details)
	if err != nil {
		return nil, fmt.Errorf("failed to insert item: %w", err)
	}

	// Generate and store embedding
	provider, err := s.GetEmbeddingProvider()
	if err == nil {
		embedText := fmt.Sprintf("%s %s %s %s %s", item.Title, item.What, getString(item.Why), getString(item.Impact), strings.Join(item.Tags, " "))

		embedding, err := provider.Embed(context.Background(), embedText)
		if err == nil {
			if err := s.db.EnsureVecTable(len(embedding)); err == nil {
				_ = s.db.InsertVector(rowid, embedding)
			}
		}
	}

	return map[string]any{
		"id":        item.ID,
		"file_path": filePath,
		"action":    "created",
	}, nil
}

// Search searches items using hybrid FTS + vector search.
func (s *Service) Search(query string, limit int, project *string, source *string, useVectors bool) ([]models.SearchResult, error) {
	return s.SearchWithMode(query, limit, project, source, useVectors, models.RetrievalSearch)
}

// SearchWithMode searches items using a retrieval ranking profile.
func (s *Service) SearchWithMode(query string, limit int, project *string, source *string, useVectors bool, mode string) ([]models.SearchResult, error) {
	provider, err := s.GetEmbeddingProvider()
	if err != nil || !useVectors || !s.VectorsAvailable() {
		// FTS-only path
		results, searchErr := s.db.FTSSearch(query, limit, project, source)
		if searchErr != nil {
			return nil, searchErr
		}

		return search.ApplyRetrievalMode(results, mode), nil
	}

	// Use tiered search: FTS first, embed only if sparse results
	results, searchErr := search.TieredSearch(context.Background(), s.db, provider, query, limit, search.DefaultMinFTSResults, project, source)
	if searchErr != nil {
		return nil, searchErr
	}

	return search.ApplyRetrievalMode(results, mode), nil
}

// ExplainSearch reports which retrieval path was taken for a query.
func (s *Service) ExplainSearch(query string, limit int, project *string, source *string, useVectors bool) (*models.SearchExplanation, []models.SearchResult, error) {
	return s.ExplainSearchWithMode(query, limit, project, source, useVectors, models.RetrievalSearch)
}

// ExplainSearchWithMode reports which retrieval path was taken for a query and applies a ranking profile.
func (s *Service) ExplainSearchWithMode(query string, limit int, project *string, source *string, useVectors bool, mode string) (*models.SearchExplanation, []models.SearchResult, error) {
	explanation := &models.SearchExplanation{
		Query:            query,
		Limit:            limit,
		UseVectors:       useVectors,
		VectorsAvailable: s.VectorsAvailable(),
		FTSWeight:        search.DefaultFTSWeight,
		VectorWeight:     search.DefaultVecWeight,
	}

	provider, providerErr := s.GetEmbeddingProvider()
	explanation.ProviderReady = providerErr == nil

	ftsResults, err := s.db.FTSSearch(query, limit*2, project, source)
	if err != nil {
		return explanation, nil, err
	}

	explanation.FTSResults = len(ftsResults)

	if providerErr != nil || !useVectors || !explanation.VectorsAvailable {
		explanation.Mode = "fts_only"
		explanation.ReturnedResults = minInt(len(ftsResults), limit)
		if len(ftsResults) > limit {
			return explanation, search.ApplyRetrievalMode(ftsResults[:limit], mode), nil
		}

		return explanation, search.ApplyRetrievalMode(ftsResults, mode), nil
	}

	if len(ftsResults) >= search.DefaultMinFTSResults {
		explanation.Mode = "fts_tier_satisfied"
		explanation.ReturnedResults = minInt(len(ftsResults), limit)
		if len(ftsResults) > limit {
			return explanation, search.ApplyRetrievalMode(ftsResults[:limit], mode), nil
		}

		return explanation, search.ApplyRetrievalMode(ftsResults, mode), nil
	}

	queryVec, err := provider.Embed(context.Background(), query)
	if err != nil {
		explanation.Mode = "fts_fallback_embed_error"
		explanation.ReturnedResults = minInt(len(ftsResults), limit)
		if len(ftsResults) > limit {
			return explanation, search.ApplyRetrievalMode(ftsResults[:limit], mode), nil
		}

		return explanation, search.ApplyRetrievalMode(ftsResults, mode), nil
	}

	explanation.Embedded = true

	vecResults, err := s.db.VectorSearch(queryVec, limit*2, project, source)
	if err != nil {
		explanation.Mode = "fts_fallback_vector_error"
		explanation.ReturnedResults = minInt(len(ftsResults), limit)
		if len(ftsResults) > limit {
			return explanation, search.ApplyRetrievalMode(ftsResults[:limit], mode), nil
		}

		return explanation, search.ApplyRetrievalMode(ftsResults, mode), nil
	}

	explanation.Mode = "hybrid"
	explanation.VectorResults = len(vecResults)

	results := search.ApplyRetrievalMode(search.MergeResults(ftsResults, vecResults, search.DefaultFTSWeight, search.DefaultVecWeight, limit), mode)
	explanation.ReturnedResults = len(results)

	return explanation, results, nil
}

// GetContext gets item pointers for context injection.
func (s *Service) GetContext(limit int, project *string, source *string, query *string, semanticMode string, topupRecent bool) ([]models.SearchResult, int64, error) {
	return s.GetContextWithMode(limit, project, source, query, semanticMode, topupRecent, models.RetrievalStartup)
}

// GetContextWithMode gets item pointers for context injection using a retrieval profile.
func (s *Service) GetContextWithMode(limit int, project *string, source *string, query *string, semanticMode string, topupRecent bool, mode string) ([]models.SearchResult, int64, error) {
	stats, err := s.db.Stats(project, source)
	if err != nil {
		return nil, 0, err
	}

	var results []models.SearchResult

	if query != nil {
		useVectors := semanticMode == "always" || (semanticMode == "auto" && s.VectorsAvailable())

		results, err = s.SearchWithMode(*query, limit, project, source, useVectors, mode)
		if err != nil {
			return nil, 0, err
		}

		if topupRecent && len(results) < limit {
			results = s.topupWithRecent(results, limit, project, source)
		}
	} else {
		results, err = s.db.ListRecent(limit, project, source)
		if err != nil {
			return nil, 0, err
		}
		results = search.ApplyRetrievalMode(results, mode)
	}

	return results, stats.Active, nil
}

// GetDetails gets full details for an item.
func (s *Service) GetDetails(itemID string) (*models.ItemDetail, error) {
	return s.db.GetDetails(itemID)
}

// Retrieve gets a full note payload for an item within a project scope.
func (s *Service) Retrieve(itemID string, project string) (map[string]any, error) {
	item, hasDetails, err := s.getScopedItem(itemID, project)
	if err != nil {
		return nil, err
	}

	var details *models.ItemDetail
	if hasDetails {
		details, err = s.db.GetDetails(item.ID)
		if err != nil {
			return nil, err
		}
	}

	result := map[string]any{
		"id":            item.ID,
		"title":         item.Title,
		"what":          item.What,
		"why":           item.Why,
		"impact":        item.Impact,
		"tags":          item.Tags,
		"category":      item.Category,
		"project":       item.Project,
		"source":        item.Source,
		"related_files": item.RelatedFiles,
		"file_path":     item.FilePath,
		"status":        item.Status,
		"superseded_by": item.SupersededBy,
		"archived_at":   item.ArchivedAt,
		"covered_by":    item.CoveredBy,
		"is_canonical":  item.IsCanonical,
		"created_at":    item.CreatedAt,
		"updated_at":    item.UpdatedAt,
		"has_details":   hasDetails,
	}

	if details != nil {
		result["details"] = details.Body
	}

	return result, nil
}

func (s *Service) getScopedItem(itemID string, project string) (*models.Item, bool, error) {
	item, hasDetails, err := s.db.GetItem(itemID)
	if err != nil {
		return nil, false, err
	}
	if item == nil || item.Project != project {
		return nil, false, db.ErrNotFound
	}

	return item, hasDetails, nil
}

// Update applies an explicit partial update to an existing item.
func (s *Service) Update(itemID string, input models.ItemUpdateInput) (map[string]any, error) {
	if input.What == nil && input.Why == nil && input.Impact == nil && input.Tags == nil && input.Details == nil {
		return nil, errors.New("no update fields provided")
	}

	if input.What != nil {
		redacted := redaction.RedactCompiled(*input.What, s.compiledIgnore)
		input.What = &redacted
	}

	if input.Why != nil {
		redacted := redaction.RedactCompiled(*input.Why, s.compiledIgnore)
		input.Why = &redacted
	}

	if input.Impact != nil {
		redacted := redaction.RedactCompiled(*input.Impact, s.compiledIgnore)
		input.Impact = &redacted
	}

	if input.Details != nil {
		redacted := redaction.RedactCompiled(*input.Details, s.compiledIgnore)
		input.Details = &redacted
	}

	if err := s.db.UpdateItem(itemID, input.What, input.Why, input.Impact, input.Tags, input.Details); err != nil {
		return nil, err
	}

	return map[string]any{
		"id":     itemID,
		"action": "updated",
	}, nil
}

// UpdateInProject applies an explicit partial update to an existing item within a project scope.
func (s *Service) UpdateInProject(itemID string, project string, input models.ItemUpdateInput) (map[string]any, error) {
	item, _, err := s.getScopedItem(itemID, project)
	if err != nil {
		return nil, err
	}

	result, err := s.Update(item.ID, input)
	if err != nil {
		return nil, err
	}
	result["id"] = item.ID

	return result, nil
}

// Archive marks an item as archived so it no longer appears in active retrieval.
func (s *Service) Archive(itemID string) (map[string]any, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	if err := s.db.UpdateStatus(itemID, models.StatusArchived, nil, &now, nil); err != nil {
		return nil, err
	}

	return map[string]any{
		"id":     itemID,
		"action": "archived",
	}, nil
}

// ArchiveInProject archives an item only if it belongs to the given project.
func (s *Service) ArchiveInProject(itemID string, project string) (map[string]any, error) {
	item, _, err := s.getScopedItem(itemID, project)
	if err != nil {
		return nil, err
	}

	result, err := s.Archive(item.ID)
	if err != nil {
		return nil, err
	}
	result["id"] = item.ID

	return result, nil
}

// Supersede marks an item as superseded by another note.
func (s *Service) Supersede(itemID string, supersededBy string) (map[string]any, error) {
	if supersededBy == "" {
		return nil, errors.New("superseded_by must not be empty")
	}

	if err := s.db.UpdateStatus(itemID, models.StatusSuperseded, &supersededBy, nil, nil); err != nil {
		return nil, err
	}

	return map[string]any{
		"id":            itemID,
		"action":        "superseded",
		"superseded_by": supersededBy,
	}, nil
}

// SupersedeInProject supersedes an item only if both notes belong to the given project.
func (s *Service) SupersedeInProject(itemID string, project string, supersededBy string) (map[string]any, error) {
	item, _, err := s.getScopedItem(itemID, project)
	if err != nil {
		return nil, err
	}
	replacement, _, err := s.getScopedItem(supersededBy, project)
	if err != nil {
		return nil, err
	}

	result, err := s.Supersede(item.ID, replacement.ID)
	if err != nil {
		return nil, err
	}
	result["id"] = item.ID
	result["superseded_by"] = replacement.ID

	return result, nil
}

// Compact creates a canonical summary note and archives matched notes under it.
func (s *Service) Compact(summary models.RawItemInput, project string, query string, source *string, limit int, category *string) (map[string]any, error) {
	if project == "" {
		project = filepath.Base(getCurrentDir())
	}
	if query == "" {
		return nil, errors.New("compact query must not be empty")
	}
	if summary.Title == "" || summary.What == "" {
		return nil, errors.New("compact summary requires title and what")
	}

	results, err := s.SearchWithMode(query, limit, &project, source, true, models.RetrievalMaint)
	if err != nil {
		return nil, err
	}

	filtered := make([]models.SearchResult, 0, len(results))
	for _, result := range results {
		if category != nil {
			if result.Category == nil || *result.Category != *category {
				continue
			}
		}
		filtered = append(filtered, result)
	}

	if len(filtered) == 0 {
		return nil, errors.New("no notes matched compact query")
	}

	summary.IsCanonical = true
	if summary.Details == nil {
		detail := buildCompactDetails(filtered)
		summary.Details = &detail
	} else {
		detail := *summary.Details + "\n\n" + buildCompactDetails(filtered)
		summary.Details = &detail
	}

	created, err := s.Store(summary, project)
	if err != nil {
		return nil, err
	}

	coveredBy, _ := created["id"].(string)
	now := time.Now().UTC().Format(time.RFC3339)
	for _, result := range filtered {
		if result.ID == coveredBy {
			continue
		}
		if statusErr := s.db.UpdateStatus(result.ID, models.StatusArchived, nil, &now, &coveredBy); statusErr != nil {
			return nil, statusErr
		}
	}

	created["covered_count"] = len(filtered)

	return created, nil
}

// Remove removes an item from uniam.
func (s *Service) Remove(itemID string) (bool, error) {
	return s.db.DeleteItem(itemID)
}

// Reindex rebuilds the vector table with current embedding provider.
func (s *Service) Reindex(progressCallback func(current, total int)) (map[string]any, error) {
	provider, err := s.GetEmbeddingProvider()
	if err != nil {
		return nil, fmt.Errorf("failed to get embedding provider: %w", err)
	}

	// Detect dimension from provider
	probe, err := provider.Embed(context.Background(), "dimension probe")
	if err != nil {
		return nil, fmt.Errorf("failed to probe embedding dimension: %w", err)
	}

	dim := len(probe)

	// Drop and recreate vec table
	if err := s.db.DropVecTable(); err != nil {
		return nil, fmt.Errorf("failed to drop vec table: %w", err)
	}

	if err := s.db.SetEmbeddingDim(dim); err != nil {
		return nil, err
	}

	if err := s.db.EnsureVecTable(dim); err != nil {
		return nil, err
	}

	// Re-embed all items
	items, err := s.db.ListAllForReindex()
	if err != nil {
		return nil, err
	}

	total := len(items)

	for i, item := range items {
		tags := ""
		if tagsVal, ok := item["tags"].([]string); ok {
			tags = strings.Join(tagsVal, " ")
		}

		embedText := fmt.Sprintf("%s %s %s %s %s",
			getStringFromMap(item, "title"),
			getStringFromMap(item, "what"),
			getStringFromMap(item, "why"),
			getStringFromMap(item, "impact"),
			tags)

		embedding, err := provider.Embed(context.Background(), embedText)
		if err != nil {
			continue
		}

		rowid, ok := item["rowid"].(int64)
		if !ok {
			continue
		}

		_ = s.db.InsertVector(rowid, embedding)

		if progressCallback != nil {
			progressCallback(i+1, total)
		}
	}

	return map[string]any{
		"count": total,
		"dim":   dim,
		"model": s.config.Embedding.Model,
	}, nil
}

// Close closes the service and cleans up resources.
func (s *Service) Close() error {
	return s.db.Close()
}

// tryDedup checks if a matching item already exists and updates it.
// Returns (result, nil) if a duplicate was found and updated, (nil, nil) if no duplicate, or (nil, err) on failure.
func (s *Service) tryDedup(raw models.RawItemInput, project, today string) (map[string]any, error) {
	dedupQuery := fmt.Sprintf("%s %s", raw.Title, raw.What)

	candidates, err := s.db.FTSSearch(dedupQuery, 5, &project, nil)
	if err != nil || len(candidates) == 0 {
		//nolint:nilerr,nilnil
		return nil, nil
	}

	broad, _ := s.db.FTSSearch(dedupQuery, 5, nil, nil)

	maxScore := 0.0
	if len(broad) > 0 {
		maxScore = broad[0].Score
	}

	top := candidates[0]

	normalized := 0.0
	if maxScore > 0 {
		normalized = top.Score / maxScore
	}

	titleMatch := strings.EqualFold(strings.TrimSpace(raw.Title), strings.TrimSpace(top.Title))
	if normalized < DedupScoreThreshold || !titleMatch {
		return nil, nil //nolint:nilnil
	}

	mergedTags := mergeTags(top.Tags, raw.Tags)

	detailsAppend := ""
	if raw.Details != nil {
		detailsAppend = fmt.Sprintf("--- updated %s ---\n%s", today, *raw.Details)
	}

	if err := s.db.UpdateItem(top.ID, &raw.What, raw.Why, raw.Impact, mergedTags, &detailsAppend); err != nil {
		return nil, fmt.Errorf("failed to update item: %w", err)
	}

	return map[string]any{
		"id":        top.ID,
		"file_path": top.FilePath,
		"action":    "updated",
	}, nil
}

// topupWithRecent appends recent items not already in results until limit is reached.
func (s *Service) topupWithRecent(results []models.SearchResult, limit int, project *string, source *string) []models.SearchResult {
	recent, err := s.db.ListRecent(limit, project, source)
	if err != nil {
		return results
	}

	seen := make(map[string]bool, len(results))
	for _, r := range results {
		seen[r.ID] = true
	}

	for _, r := range recent {
		if !seen[r.ID] {
			results = append(results, r)
			if len(results) >= limit {
				break
			}
		}
	}

	return results
}

// Helper functions

// getCurrentDir returns the current working directory, or "unknown" if it
// cannot be determined. This prevents filepath.Base("") returning "." which
// would silently be stored as a project name.
func getCurrentDir() string {
	dir, err := os.Getwd()
	if err != nil {
		return "unknown"
	}

	return dir
}

func getString(s *string) string {
	if s == nil {
		return ""
	}

	return *s
}

func getStringFromMap(m map[string]any, key string) string {
	if val, ok := m[key]; ok {
		if str, ok := val.(string); ok {
			return str
		}
	}

	return ""
}

func minInt(a int, b int) int {
	if a < b {
		return a
	}

	return b
}

func buildCompactDetails(results []models.SearchResult) string {
	lines := []string{"Covered notes:"}
	for _, result := range results {
		lines = append(lines, fmt.Sprintf("- %s | %s | %s", result.ID, result.CreatedAt[:10], result.Title))
	}

	return strings.Join(lines, "\n")
}

func mergeTags(existing []string, extra []string) []string {
	combined := make([]string, len(existing))
	copy(combined, existing)

	existingNorm := make(map[string]bool)
	for _, t := range existing {
		existingNorm[strings.ToLower(t)] = true
	}

	for _, tag := range extra {
		if !existingNorm[strings.ToLower(tag)] {
			//nolint:makezero
			combined = append(combined, tag)
			existingNorm[strings.ToLower(tag)] = true
		}
	}

	return combined
}
