package search

import (
	"context"
	"sort"
	"strings"
	"time"

	"uniam/internal/db"
	"uniam/internal/embeddings"
	"uniam/internal/models"
)

const (
	DefaultFTSWeight     = 0.3
	DefaultVecWeight     = 0.7
	DefaultMinFTSResults = 3
)

// normalizeScores scales all scores so the maximum score becomes 1.0.
// It mutates the slice in place. If the slice is empty or all scores are
// zero the slice is returned unchanged.
func normalizeScores(results []models.SearchResult) {
	if len(results) == 0 {
		return
	}

	maxScore := results[0].Score
	for _, r := range results {
		if r.Score > maxScore {
			maxScore = r.Score
		}
	}

	if maxScore <= 0 {
		return
	}

	for i := range results {
		results[i].Score /= maxScore
	}
}

// MergeResults merges FTS5 and vector search results with weighted scoring.
// Both input slices are normalized to 0–1 before weighting.
func MergeResults(ftsResults []models.SearchResult, vecResults []models.SearchResult, ftsWeight float64, vecWeight float64, limit int) []models.SearchResult {
	normalizeScores(ftsResults)
	normalizeScores(vecResults)

	// Combine with weighted scoring, dedup by ID
	scores := make(map[string]*models.SearchResult)

	for _, r := range ftsResults {
		result := r
		result.Score = ftsWeight * r.Score
		scores[r.ID] = &result
	}

	for _, r := range vecResults {
		if existing, ok := scores[r.ID]; ok {
			existing.Score += vecWeight * r.Score
		} else {
			result := r
			result.Score = vecWeight * r.Score
			scores[r.ID] = &result
		}
	}

	ranked := make([]models.SearchResult, 0, len(scores))
	for _, r := range scores {
		ranked = append(ranked, *r)
	}

	sort.Slice(ranked, func(i, j int) bool {
		return ranked[i].Score > ranked[j].Score
	})

	if len(ranked) > limit {
		return ranked[:limit]
	}

	return ranked
}

// ApplyRetrievalMode re-ranks results based on the requested retrieval profile.
func ApplyRetrievalMode(results []models.SearchResult, mode string) []models.SearchResult {
	reranked := make([]models.SearchResult, len(results))
	copy(reranked, results)

	now := time.Now().UTC()
	for i := range reranked {
		reranked[i].Score += recencyBoost(reranked[i].CreatedAt, now, mode)
		reranked[i].Score += categoryBoost(reranked[i].Category, mode)
		reranked[i].Score += canonicalBoost(reranked[i].IsCanonical, mode)
		reranked[i].Score += sourceBoost(reranked[i].Source, mode)
	}

	sort.Slice(reranked, func(i, j int) bool {
		return reranked[i].Score > reranked[j].Score
	})

	return reranked
}

func recencyBoost(createdAt string, now time.Time, mode string) float64 {
	parsed, err := time.Parse(time.RFC3339, createdAt)
	if err != nil {
		return 0
	}

	days := now.Sub(parsed).Hours() / 24
	switch mode {
	case models.RetrievalStartup:
		if days <= 3 {
			return 0.25
		}
		if days <= 14 {
			return 0.10
		}
	case models.RetrievalDebug:
		if days <= 7 {
			return 0.20
		}
	case models.RetrievalMaint:
		if days <= 30 {
			return 0.10
		}
	}

	return 0
}

func categoryBoost(category *string, mode string) float64 {
	if category == nil {
		return 0
	}

	switch mode {
	case models.RetrievalDebug:
		if *category == "bug" {
			return 0.30
		}
	case models.RetrievalArch:
		if *category == "decision" || *category == "pattern" {
			return 0.25
		}
	case models.RetrievalMaint:
		if *category == "context" || *category == "pattern" {
			return 0.15
		}
	case models.RetrievalStartup:
		if *category == "decision" || *category == "context" {
			return 0.10
		}
	}

	return 0
}

func canonicalBoost(isCanonical bool, mode string) float64 {
	if !isCanonical {
		return 0
	}

	switch mode {
	case models.RetrievalStartup, models.RetrievalArch, models.RetrievalMaint:
		return 0.25
	default:
		return 0.10
	}
}

func sourceBoost(source *string, mode string) float64 {
	if source == nil || *source == "" {
		return 0
	}

	if mode == models.RetrievalStartup && strings.EqualFold(*source, "codex") {
		return 0.05
	}

	return 0
}

// TieredSearch performs FTS-first tiered search that only calls embed when FTS results are sparse.
func TieredSearch(ctx context.Context, store db.Store, embeddingProvider embeddings.Provider, query string, limit int, minFTSResults int, project *string, source *string) ([]models.SearchResult, error) {
	ftsResults, err := store.FTSSearch(query, limit*2, project, source)
	if err != nil {
		return nil, err
	}

	normalizeScores(ftsResults)

	// If FTS has enough results, return without calling embed
	if len(ftsResults) >= minFTSResults {
		if len(ftsResults) > limit {
			return ftsResults[:limit], nil
		}

		return ftsResults, nil
	}

	// If no embedding provider, return FTS-only
	if embeddingProvider == nil {
		if len(ftsResults) > limit {
			return ftsResults[:limit], nil
		}

		return ftsResults, nil
	}

	// FTS results are sparse — fall back to hybrid (embed + vector search + merge)
	queryVec, err := embeddingProvider.Embed(ctx, query)
	if err != nil {
		// On any embedding error, return whatever FTS found
		if len(ftsResults) > limit {
			return ftsResults[:limit], nil
		}

		return ftsResults, nil
	}

	vecResults, err := store.VectorSearch(queryVec, limit*2, project, source)
	if err != nil {
		// On vector search error, return FTS results
		if len(ftsResults) > limit {
			return ftsResults[:limit], nil
		}

		return ftsResults, nil
	}

	return MergeResults(ftsResults, vecResults, DefaultFTSWeight, DefaultVecWeight, limit), nil
}

// HybridSearch runs FTS5 and optionally vector search, merges results.
func HybridSearch(ctx context.Context, store db.Store, embeddingProvider embeddings.Provider, query string, limit int, project *string, source *string) ([]models.SearchResult, error) {
	ftsResults, err := store.FTSSearch(query, limit*2, project, source)
	if err != nil {
		return nil, err
	}

	normalizeScores(ftsResults)

	if embeddingProvider == nil {
		// FTS-only mode: return directly
		if len(ftsResults) > limit {
			return ftsResults[:limit], nil
		}

		return ftsResults, nil
	}

	queryVec, err := embeddingProvider.Embed(ctx, query)
	if err != nil {
		// On embedding error, return FTS results
		if len(ftsResults) > limit {
			return ftsResults[:limit], nil
		}

		return ftsResults, nil
	}

	vecResults, err := store.VectorSearch(queryVec, limit*2, project, source)
	if err != nil {
		// On vector search error, return FTS results
		if len(ftsResults) > limit {
			return ftsResults[:limit], nil
		}

		return ftsResults, nil
	}

	return MergeResults(ftsResults, vecResults, DefaultFTSWeight, DefaultVecWeight, limit), nil
}
