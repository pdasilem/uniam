package models

import (
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
)

// ValidCategories defines the allowed categories for items.
var ValidCategories = []string{"decision", "pattern", "bug", "context", "learning"}

const (
	StatusActive     = "active"
	StatusSuperseded = "superseded"
	StatusArchived   = "archived"
	StatusStale      = "stale"
	RetrievalStartup = "startup"
	RetrievalSearch  = "search"
	RetrievalDebug   = "debug"
	RetrievalArch    = "architecture"
	RetrievalMaint   = "maintenance"
)

// ValidStatuses defines the allowed lifecycle states for items.
var ValidStatuses = []string{StatusActive, StatusSuperseded, StatusArchived, StatusStale}

// ValidRetrievalModes defines supported ranking profiles.
var ValidRetrievalModes = []string{RetrievalStartup, RetrievalSearch, RetrievalDebug, RetrievalArch, RetrievalMaint}

// CategoryHeadings maps category values to display headings.
var CategoryHeadings = map[string]string{
	"decision": "Decisions",
	"pattern":  "Patterns",
	"bug":      "Bugs Fixed",
	"context":  "Context",
	"learning": "Learnings",
}

// RawItemInput represents raw input for creating an item before processing.
type RawItemInput struct {
	Title        string
	What         string
	Why          *string
	Impact       *string
	Tags         []string
	Category     *string
	RelatedFiles []string
	Details      *string
	Source       *string
	IsCanonical  bool
}

// ItemUpdateInput represents an explicit update for an existing item.
type ItemUpdateInput struct {
	What    *string
	Why     *string
	Impact  *string
	Tags    []string
	Details *string
}

// Item represents a stored item in the uniam.
type Item struct {
	ID            string
	Title         string
	What          string
	Why           *string
	Impact        *string
	Tags          []string
	Category      *string
	Project       string
	Source        *string
	RelatedFiles  []string
	FilePath      string
	SectionAnchor string
	Status        string
	SupersededBy  *string
	ArchivedAt    *string
	CoveredBy     *string
	IsCanonical   bool
	CreatedAt     string
	UpdatedAt     string
}

// FromRaw creates an Item from RawItemInput with generated fields.
func FromRaw(raw RawItemInput, project string, filePath string) Item {
	now := time.Now().UTC().Format(time.RFC3339)
	anchor := generateAnchor(raw.Title)

	return Item{
		ID:            uuid.New().String(),
		Title:         raw.Title,
		What:          raw.What,
		Why:           raw.Why,
		Impact:        raw.Impact,
		Tags:          raw.Tags,
		Category:      raw.Category,
		Project:       project,
		Source:        raw.Source,
		RelatedFiles:  raw.RelatedFiles,
		FilePath:      filePath,
		SectionAnchor: anchor,
		Status:        StatusActive,
		IsCanonical:   raw.IsCanonical,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
}

// generateAnchor creates a URL-friendly anchor from a title.
func generateAnchor(title string) string {
	// Convert to lowercase and replace non-alphanumeric with hyphens
	re := regexp.MustCompile(`[^a-z0-9]+`)
	anchor := strings.ToLower(title)
	anchor = re.ReplaceAllString(anchor, "-")
	anchor = strings.Trim(anchor, "-")

	return anchor
}

// ItemDetail represents full details/body content for an item.
type ItemDetail struct {
	ItemID string
	Body   string
}

// SearchResult represents a search result with score and metadata.
type SearchResult struct {
	ID           string
	Title        string
	What         string
	Why          *string
	Impact       *string
	Category     *string
	Tags         []string
	Project      string
	Source       *string
	Status       string
	SupersededBy *string
	CoveredBy    *string
	IsCanonical  bool
	Score        float64
	HasDetails   bool
	FilePath     string
	CreatedAt    string
}

// Stats holds aggregated note counters for observability commands.
type Stats struct {
	Total       int64
	Active      int64
	Archived    int64
	Superseded  int64
	Stale       int64
	WithDetails int64
	WithVectors int64
	ByProject   map[string]int64
	ByCategory  map[string]int64
	BySource    map[string]int64
}

// SearchExplanation describes which retrieval path was taken for a search.
type SearchExplanation struct {
	Query            string
	Limit            int
	Mode             string
	ProviderReady    bool
	VectorsAvailable bool
	UseVectors       bool
	FTSResults       int
	VectorResults    int
	Embedded         bool
	ReturnedResults  int
	FTSWeight        float64
	VectorWeight     float64
}
