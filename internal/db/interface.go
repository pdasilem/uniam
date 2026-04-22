package db

import (
	"errors"

	"uniam/internal/models"
)

// ErrNotFound is returned when a requested item does not exist in the database.
var ErrNotFound = errors.New("item not found")

// ErrDimensionMismatch is returned when the embedding dimension stored in the
// database does not match the dimension returned by the current provider.
// The caller should advise the user to run 'uniam reindex'.
var ErrDimensionMismatch = errors.New("embedding dimension mismatch")

// Store is the persistence interface for uniam operations.
// *DB implements this interface; test code can inject a stub.
type Store interface {
	InsertItem(item models.Item, details *string) (int64, error)
	InsertVector(rowid int64, embedding []float32) error
	DeleteVector(rowid int64) error
	GetItem(itemID string) (*models.Item, bool, error)
	GetDetails(itemID string) (*models.ItemDetail, error)
	UpdateItem(itemID string, what *string, why *string, impact *string, tags []string, detailsAppend *string) error
	UpdateStatus(itemID string, status string, supersededBy *string, archivedAt *string, coveredBy *string) error
	DeleteItem(itemID string) (bool, error)
	FTSSearch(query string, limit int, project *string, source *string) ([]models.SearchResult, error)
	VectorSearch(queryEmbedding []float32, limit int, project *string, source *string) ([]models.SearchResult, error)
	ListRecent(limit int, project *string, source *string) ([]models.SearchResult, error)
	ListAllForReindex(project *string) ([]map[string]any, error)
	CountItems(project *string, source *string) (int64, error)
	Stats(project *string, source *string) (*models.Stats, error)
	HasVecTable() bool
	EnsureVecTable(dim int) error
	SetEmbeddingDim(dim int) error
	DropVecTable() error
	Close() error
}
