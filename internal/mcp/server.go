package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"uniam/internal/buildinfo"
	"uniam/internal/config"
	"uniam/internal/core"
	"uniam/internal/models"
	"uniam/internal/update"
)

// uniamService is the subset of core.Service used by MCP tool handlers.
// Defining it here allows tests to inject stubs without depending on core.Service.
type uniamService interface {
	Store(raw models.RawItemInput, project string) (map[string]any, error)
	Search(query string, limit int, project *string, source *string, useVectors bool) ([]models.SearchResult, error)
	GetContext(limit int, project *string, source *string, query *string, semanticMode string, topupRecent bool) ([]models.SearchResult, int64, error)
	Retrieve(itemID string, project string) (map[string]any, error)
	ArchiveInProject(itemID string, project string) (map[string]any, error)
	SupersedeInProject(itemID string, project string, supersededBy string) (map[string]any, error)
	UpdateInProject(itemID string, project string, input models.ItemUpdateInput) (map[string]any, error)
	Compact(summary models.RawItemInput, project string, query string, source *string, limit int, category *string) (map[string]any, error)
	ExplainSearchWithMode(query string, limit int, project *string, source *string, useVectors bool, mode string) (*models.SearchExplanation, []models.SearchResult, error)
	Config() *config.Config
	Close() error
}

// RunServer starts the MCP server with stdio transport.
func RunServer() error {
	svc, err := core.NewService("")
	if err != nil {
		return fmt.Errorf("failed to create service: %w", err)
	}

	defer func() { _ = svc.Close() }()

	cfg := svc.Config()
	if cfg.Updates.CheckOnMCPStart {
		checker := update.NewChecker(buildinfo.Version).WithCheckTTL(time.Duration(cfg.Updates.CheckIntervalHours) * time.Hour)
		if result, checkErr := checker.Check(context.Background(), false); checkErr == nil {
			if result.UpdateAvailable {
				if cfg.Updates.AutoApply && runtime.GOOS != "windows" {
					if applyErr := checker.Apply(context.Background(), result); applyErr == nil {
						fmt.Fprintf(os.Stderr, "uniam: applied update %s -> %s; restart the agent to use the new binary\n", result.CurrentVersion, result.LatestVersion)
					} else {
						fmt.Fprintf(os.Stderr, "uniam: update available %s -> %s (auto-apply failed: %v)\n", result.CurrentVersion, result.LatestVersion, applyErr)
					}
				} else if cfg.Updates.AutoApply && runtime.GOOS == "windows" {
					fmt.Fprintf(os.Stderr, "uniam: update available %s -> %s; auto-apply is disabled on windows, run `uniam update`\n", result.CurrentVersion, result.LatestVersion)
				} else {
					fmt.Fprintf(os.Stderr, "uniam: update available %s -> %s; run `uniam update`\n", result.CurrentVersion, result.LatestVersion)
				}
			}
		}
	}

	// Create MCP server
	mcpServer := mcpsdk.NewServer(&mcpsdk.Implementation{
		Name:    "uniam",
		Version: buildinfo.Version,
	}, nil)

	// Register tools
	if err := registerTools(mcpServer, svc); err != nil {
		return fmt.Errorf("failed to register tools: %w", err)
	}

	// Run server with stdio transport
	return mcpServer.Run(context.Background(), &mcpsdk.StdioTransport{})
}

// registerTools registers all uniam tools with the MCP server.
//
//nolint:unparam
func registerTools(s *mcpsdk.Server, svc uniamService) error {
	// Register uniam_store tool
	//nolint:revive
	storeHandler := func(ctx context.Context, req *mcpsdk.CallToolRequest, input map[string]any) (*mcpsdk.CallToolResult, map[string]any, error) {
		result, err := HandleUniamStore(svc, input)
		if err != nil {
			return &mcpsdk.CallToolResult{
				Content: []mcpsdk.Content{
					&mcpsdk.TextContent{Text: fmt.Sprintf("Error: %v", err)},
				},
				IsError: true,
			}, nil, nil
		}

		return nil, result, nil
	}
	mcpsdk.AddTool(s, &mcpsdk.Tool{
		Name:        "uniam_store",
		Description: "Store a note for future sessions. You MUST call this before ending any session where you made changes, fixed bugs, made decisions, or learned something.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"title":         map[string]any{"type": "string", "description": "Short descriptive title"},
				"what":          map[string]any{"type": "string", "description": "What happened or was decided"},
				"why":           map[string]any{"type": "string", "description": "Reasoning behind it"},
				"impact":        map[string]any{"type": "string", "description": "What changed as a result"},
				"tags":          map[string]any{"type": []any{"string", "array"}, "items": map[string]any{"type": "string"}, "description": "Comma-separated string or array of tags"},
				"category":      map[string]any{"type": "string", "enum": []string{"decision", "pattern", "bug", "context", "learning"}},
				"related_files": map[string]any{"type": []any{"string", "array"}, "items": map[string]any{"type": "string"}, "description": "Comma-separated string or array of file paths"},
				"details":       map[string]any{"type": "string", "description": "Full context with all important details"},
				"source":        map[string]any{"type": "string", "description": "Source agent name"},
				"project":       map[string]any{"type": "string", "description": "Project name (defaults to current directory)"},
			},
			"required": []string{"title", "what"},
		},
	}, storeHandler)

	// Register uniam_search tool
	//nolint:revive
	searchHandler := func(ctx context.Context, req *mcpsdk.CallToolRequest, input map[string]any) (*mcpsdk.CallToolResult, map[string]any, error) {
		results, err := HandleUniamSearch(svc, input)
		if err != nil {
			return &mcpsdk.CallToolResult{
				Content: []mcpsdk.Content{
					&mcpsdk.TextContent{Text: fmt.Sprintf("Error: %v", err)},
				},
				IsError: true,
			}, nil, nil
		}

		return nil, map[string]any{"results": results}, nil
	}
	mcpsdk.AddTool(s, &mcpsdk.Tool{
		Name:        "uniam_search",
		Description: "Search notes using keyword and semantic search. Returns matching notes ranked by relevance.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"query":  map[string]any{"type": "string", "description": "Search query"},
				"limit":  map[string]any{"type": "integer", "description": "Maximum number of notes", "default": 5},
				"source": map[string]any{"type": "string", "description": "Filter by source"},
			},
			"required": []string{"query"},
		},
	}, searchHandler)

	// Register uniam_context tool
	//nolint:revive
	contextHandler := func(ctx context.Context, req *mcpsdk.CallToolRequest, input map[string]any) (*mcpsdk.CallToolResult, map[string]any, error) {
		result, err := HandleUniamContext(svc, input)
		if err != nil {
			return &mcpsdk.CallToolResult{
				Content: []mcpsdk.Content{
					&mcpsdk.TextContent{Text: fmt.Sprintf("Error: %v", err)},
				},
				IsError: true,
			}, nil, nil
		}

		return nil, result, nil
	}
	mcpsdk.AddTool(s, &mcpsdk.Tool{
		Name:        "uniam_context",
		Description: "Get notes for the current project. Returns prior decisions, bugs, and context.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"limit":  map[string]any{"type": "integer", "description": "Maximum number of notes", "default": 10},
				"source": map[string]any{"type": "string", "description": "Filter by source"},
			},
		},
	}, contextHandler)

	// Register uniam_retrieve tool
	//nolint:revive
	retrieveHandler := func(ctx context.Context, req *mcpsdk.CallToolRequest, input map[string]any) (*mcpsdk.CallToolResult, map[string]any, error) {
		result, err := HandleUniamRetrieve(svc, input)
		if err != nil {
			return &mcpsdk.CallToolResult{
				Content: []mcpsdk.Content{
					&mcpsdk.TextContent{Text: fmt.Sprintf("Error: %v", err)},
				},
				IsError: true,
			}, nil, nil
		}

		return nil, result, nil
	}
	mcpsdk.AddTool(s, &mcpsdk.Tool{
		Name:        "uniam_retrieve",
		Description: "Retrieve the full contents of a note from the current project only.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"id": map[string]any{"type": "string", "description": "Full note ID"},
			},
			"required": []string{"id"},
		},
	}, retrieveHandler)

	// Register uniam_archive tool
	//nolint:revive
	archiveHandler := func(ctx context.Context, req *mcpsdk.CallToolRequest, input map[string]any) (*mcpsdk.CallToolResult, map[string]any, error) {
		result, err := HandleUniamArchive(svc, input)
		if err != nil {
			return &mcpsdk.CallToolResult{
				Content: []mcpsdk.Content{
					&mcpsdk.TextContent{Text: fmt.Sprintf("Error: %v", err)},
				},
				IsError: true,
			}, nil, nil
		}

		return nil, result, nil
	}
	mcpsdk.AddTool(s, &mcpsdk.Tool{
		Name:        "uniam_archive",
		Description: "Archive a note from the current project only.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"id": map[string]any{"type": "string", "description": "Note ID or unique prefix in the current project"},
			},
			"required": []string{"id"},
		},
	}, archiveHandler)

	// Register uniam_supersede tool
	//nolint:revive
	supersedeHandler := func(ctx context.Context, req *mcpsdk.CallToolRequest, input map[string]any) (*mcpsdk.CallToolResult, map[string]any, error) {
		result, err := HandleUniamSupersede(svc, input)
		if err != nil {
			return &mcpsdk.CallToolResult{
				Content: []mcpsdk.Content{
					&mcpsdk.TextContent{Text: fmt.Sprintf("Error: %v", err)},
				},
				IsError: true,
			}, nil, nil
		}

		return nil, result, nil
	}
	mcpsdk.AddTool(s, &mcpsdk.Tool{
		Name:        "uniam_supersede",
		Description: "Mark a note as superseded by another note from the current project only.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"id": map[string]any{"type": "string", "description": "Note ID or unique prefix in the current project"},
				"by": map[string]any{"type": "string", "description": "Replacement note ID or unique prefix in the current project"},
			},
			"required": []string{"id", "by"},
		},
	}, supersedeHandler)

	// Register uniam_update_note tool
	//nolint:revive
	updateHandler := func(ctx context.Context, req *mcpsdk.CallToolRequest, input map[string]any) (*mcpsdk.CallToolResult, map[string]any, error) {
		result, err := HandleUniamUpdateNote(svc, input)
		if err != nil {
			return &mcpsdk.CallToolResult{
				Content: []mcpsdk.Content{
					&mcpsdk.TextContent{Text: fmt.Sprintf("Error: %v", err)},
				},
				IsError: true,
			}, nil, nil
		}

		return nil, result, nil
	}
	mcpsdk.AddTool(s, &mcpsdk.Tool{
		Name:        "uniam_update_note",
		Description: "Update a note in the current project only.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"id":      map[string]any{"type": "string", "description": "Note ID or unique prefix in the current project"},
				"what":    map[string]any{"type": "string", "description": "Replace the What field"},
				"why":     map[string]any{"type": "string", "description": "Replace the Why field"},
				"impact":  map[string]any{"type": "string", "description": "Replace the Impact field"},
				"tags":    map[string]any{"type": []any{"string", "array"}, "items": map[string]any{"type": "string"}, "description": "Replace tags with a comma-separated string or array"},
				"details": map[string]any{"type": "string", "description": "Append or create details content"},
			},
			"required": []string{"id"},
		},
	}, updateHandler)

	// Register uniam_compact tool
	//nolint:revive
	compactHandler := func(ctx context.Context, req *mcpsdk.CallToolRequest, input map[string]any) (*mcpsdk.CallToolResult, map[string]any, error) {
		result, err := HandleUniamCompact(svc, input)
		if err != nil {
			return &mcpsdk.CallToolResult{
				Content: []mcpsdk.Content{
					&mcpsdk.TextContent{Text: fmt.Sprintf("Error: %v", err)},
				},
				IsError: true,
			}, nil, nil
		}

		return nil, result, nil
	}
	mcpsdk.AddTool(s, &mcpsdk.Tool{
		Name:        "uniam_compact",
		Description: "Create a canonical summary note and archive matched notes inside the current project only.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"title":    map[string]any{"type": "string", "description": "Title of the canonical summary note"},
				"what":     map[string]any{"type": "string", "description": "Summary statement for the canonical note"},
				"why":      map[string]any{"type": "string", "description": "Why the compacted summary matters"},
				"impact":   map[string]any{"type": "string", "description": "Impact of the compacted summary"},
				"details":  map[string]any{"type": "string", "description": "Optional details to prepend before the generated covered-note list"},
				"query":    map[string]any{"type": "string", "description": "Search query used to select notes for compaction"},
				"source":   map[string]any{"type": "string", "description": "Restrict compaction to notes from a given source"},
				"category": map[string]any{"type": "string", "description": "Restrict compaction to a given category"},
				"limit":    map[string]any{"type": "integer", "description": "Maximum number of matching notes to compact", "default": 20},
			},
			"required": []string{"title", "what", "query"},
		},
	}, compactHandler)

	// Register uniam_explain_search tool
	//nolint:revive
	explainHandler := func(ctx context.Context, req *mcpsdk.CallToolRequest, input map[string]any) (*mcpsdk.CallToolResult, map[string]any, error) {
		result, err := HandleUniamExplainSearch(svc, input)
		if err != nil {
			return &mcpsdk.CallToolResult{
				Content: []mcpsdk.Content{
					&mcpsdk.TextContent{Text: fmt.Sprintf("Error: %v", err)},
				},
				IsError: true,
			}, nil, nil
		}

		return nil, result, nil
	}
	mcpsdk.AddTool(s, &mcpsdk.Tool{
		Name:        "uniam_explain_search",
		Description: "Explain retrieval behavior for a search query inside the current project only.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"query":   map[string]any{"type": "string", "description": "Search query"},
				"limit":   map[string]any{"type": "integer", "description": "Maximum number of results", "default": 5},
				"vectors": map[string]any{"type": "boolean", "description": "Allow vector search when available", "default": true},
				"mode":    map[string]any{"type": "string", "description": "Retrieval mode: startup, search, debug, architecture, maintenance", "default": "search"},
				"source":  map[string]any{"type": "string", "description": "Filter by source"},
			},
			"required": []string{"query"},
		},
	}, explainHandler)

	return nil
}

// HandleUniamStore handles the uniam_store tool call.
func HandleUniamStore(svc uniamService, params map[string]any) (map[string]any, error) {
	title, _ := params["title"].(string)
	what, _ := params["what"].(string)
	why, _ := getStringFromMap(params, "why")
	impact, _ := getStringFromMap(params, "impact")
	tags, _ := getStringSliceFromMap(params, "tags")
	category, _ := getStringFromMap(params, "category")
	relatedFiles, _ := getStringSliceFromMap(params, "related_files")
	details, _ := getStringFromMap(params, "details")
	source, _ := getStringFromMap(params, "source")
	project, err := resolveScopedProject(params)
	if err != nil {
		return nil, err
	}

	raw := models.RawItemInput{
		Title: title,
		What:  what,
	}

	if why != "" {
		raw.Why = &why
	}

	if impact != "" {
		raw.Impact = &impact
	}

	if category != "" {
		raw.Category = &category
	}

	if source != "" {
		raw.Source = &source
	}

	if details != "" {
		raw.Details = &details
	}

	raw.Tags = tags
	raw.RelatedFiles = relatedFiles

	result, err := svc.Store(raw, project)
	if err != nil {
		return nil, err
	}

	return result, nil
}

// HandleUniamSearch handles the uniam_search tool call.
func HandleUniamSearch(svc uniamService, params map[string]any) ([]map[string]any, error) {
	query, _ := params["query"].(string)

	limit := 5
	if l, ok := params["limit"].(float64); ok {
		limit = int(l)
	}

	projectName, err := resolveScopedProject(params)
	if err != nil {
		return nil, err
	}
	project := &projectName

	results, err := svc.Search(query, limit, project, nil, true)
	if err != nil {
		return nil, err
	}

	clean := make([]map[string]any, len(results))
	for i, r := range results {
		clean[i] = map[string]any{
			"id":          r.ID,
			"title":       r.Title,
			"what":        r.What,
			"why":         r.Why,
			"impact":      r.Impact,
			"category":    r.Category,
			"tags":        r.Tags,
			"project":     r.Project,
			"source":      r.Source,
			"created_at":  r.CreatedAt[:10],
			"score":       r.Score,
			"has_details": r.HasDetails,
		}
	}

	return clean, nil
}

// HandleUniamRetrieve handles the uniam_retrieve tool call.
func HandleUniamRetrieve(svc uniamService, params map[string]any) (map[string]any, error) {
	itemID, _ := getStringFromMap(params, "id")
	if itemID == "" {
		return nil, errors.New("id is required")
	}

	project, err := resolveScopedProject(params)
	if err != nil {
		return nil, err
	}

	return svc.Retrieve(itemID, project)
}

// HandleUniamArchive handles the uniam_archive tool call.
func HandleUniamArchive(svc uniamService, params map[string]any) (map[string]any, error) {
	itemID, _ := getStringFromMap(params, "id")
	if itemID == "" {
		return nil, errors.New("id is required")
	}

	project, err := resolveScopedProject(params)
	if err != nil {
		return nil, err
	}

	return svc.ArchiveInProject(itemID, project)
}

// HandleUniamSupersede handles the uniam_supersede tool call.
func HandleUniamSupersede(svc uniamService, params map[string]any) (map[string]any, error) {
	itemID, _ := getStringFromMap(params, "id")
	if itemID == "" {
		return nil, errors.New("id is required")
	}
	by, _ := getStringFromMap(params, "by")
	if by == "" {
		return nil, errors.New("by is required")
	}

	project, err := resolveScopedProject(params)
	if err != nil {
		return nil, err
	}

	return svc.SupersedeInProject(itemID, project, by)
}

// HandleUniamUpdateNote handles the uniam_update_note tool call.
func HandleUniamUpdateNote(svc uniamService, params map[string]any) (map[string]any, error) {
	itemID, _ := getStringFromMap(params, "id")
	if itemID == "" {
		return nil, errors.New("id is required")
	}

	project, err := resolveScopedProject(params)
	if err != nil {
		return nil, err
	}

	input := models.ItemUpdateInput{}
	if what, ok := getStringFromMap(params, "what"); ok && what != "" {
		input.What = &what
	}
	if why, ok := getStringFromMap(params, "why"); ok && why != "" {
		input.Why = &why
	}
	if impact, ok := getStringFromMap(params, "impact"); ok && impact != "" {
		input.Impact = &impact
	}
	if details, ok := getStringFromMap(params, "details"); ok && details != "" {
		input.Details = &details
	}
	if tags, ok := getStringSliceFromMap(params, "tags"); ok {
		input.Tags = tags
	}

	return svc.UpdateInProject(itemID, project, input)
}

// HandleUniamCompact handles the uniam_compact tool call.
func HandleUniamCompact(svc uniamService, params map[string]any) (map[string]any, error) {
	project, err := resolveScopedProject(params)
	if err != nil {
		return nil, err
	}

	title, _ := getStringFromMap(params, "title")
	what, _ := getStringFromMap(params, "what")
	query, _ := getStringFromMap(params, "query")
	if title == "" || what == "" || query == "" {
		return nil, errors.New("title, what, and query are required")
	}

	raw := models.RawItemInput{Title: title, What: what, IsCanonical: true}
	if why, ok := getStringFromMap(params, "why"); ok && why != "" {
		raw.Why = &why
	}
	if impact, ok := getStringFromMap(params, "impact"); ok && impact != "" {
		raw.Impact = &impact
	}
	if details, ok := getStringFromMap(params, "details"); ok && details != "" {
		raw.Details = &details
	}

	var source *string
	if sourceValue, ok := getStringFromMap(params, "source"); ok && sourceValue != "" {
		source = &sourceValue
	}

	var category *string
	if categoryValue, ok := getStringFromMap(params, "category"); ok && categoryValue != "" {
		category = &categoryValue
	}

	limit := 20
	if l, ok := params["limit"].(float64); ok {
		limit = int(l)
	}

	return svc.Compact(raw, project, query, source, limit, category)
}

// HandleUniamExplainSearch handles the uniam_explain_search tool call.
func HandleUniamExplainSearch(svc uniamService, params map[string]any) (map[string]any, error) {
	query, _ := getStringFromMap(params, "query")
	if query == "" {
		return nil, errors.New("query is required")
	}

	projectName, err := resolveScopedProject(params)
	if err != nil {
		return nil, err
	}
	project := &projectName

	limit := 5
	if l, ok := params["limit"].(float64); ok {
		limit = int(l)
	}

	useVectors := true
	if vectors, ok := params["vectors"].(bool); ok {
		useVectors = vectors
	}

	mode := models.RetrievalSearch
	if modeValue, ok := getStringFromMap(params, "mode"); ok && modeValue != "" {
		mode = modeValue
	}

	var source *string
	if sourceValue, ok := getStringFromMap(params, "source"); ok && sourceValue != "" {
		source = &sourceValue
	}

	explanation, results, err := svc.ExplainSearchWithMode(query, limit, project, source, useVectors, mode)
	if err != nil {
		return nil, err
	}

	clean := make([]map[string]any, len(results))
	for i, result := range results {
		clean[i] = map[string]any{
			"id":           result.ID,
			"title":        result.Title,
			"project":      result.Project,
			"score":        result.Score,
			"status":       result.Status,
			"is_canonical": result.IsCanonical,
			"has_details":  result.HasDetails,
		}
	}

	return map[string]any{
		"explanation": explanation,
		"results":     clean,
	}, nil
}

// HandleUniamContext handles the uniam_context tool call.
func HandleUniamContext(svc uniamService, params map[string]any) (map[string]any, error) {
	limit := 10
	if l, ok := params["limit"].(float64); ok {
		limit = int(l)
	}

	projectName, err := resolveScopedProject(params)
	if err != nil {
		return nil, err
	}
	project := &projectName

	results, total, err := svc.GetContext(limit, project, nil, nil, "never", false)
	if err != nil {
		return nil, err
	}

	notes := make([]map[string]any, len(results))

	for i, r := range results {
		dateStr := r.CreatedAt[:10]
		notes[i] = map[string]any{
			"id":       r.ID,
			"title":    r.Title,
			"category": r.Category,
			"tags":     r.Tags,
			"date":     dateStr,
		}
	}

	return map[string]any{
		"total":   total,
		"showing": len(notes),
		"notes":   notes,
	}, nil
}

// Helper functions.
//
//nolint:unparam
func getStringFromMap(m map[string]any, key string) (string, bool) {
	if val, ok := m[key]; ok {
		if str, ok := val.(string); ok {
			return str, true
		}
	}

	return "", false
}

func getStringSliceFromMap(m map[string]any, key string) ([]string, bool) {
	//nolint:nestif
	if val, ok := m[key]; ok {
		if arr, ok := val.([]any); ok {
			result := make([]string, len(arr))

			for i, v := range arr {
				if str, ok := v.(string); ok {
					result[i] = str
				}
			}

			return result, true
		}

		if str, ok := val.(string); ok {
			// Try to parse as JSON array
			var arr []string
			if err := json.Unmarshal([]byte(str), &arr); err == nil {
				return arr, true
			}
			// Fallback: comma-separated string
			parts := strings.Split(str, ",")

			result := make([]string, 0, len(parts))

			for _, p := range parts {
				if t := strings.TrimSpace(p); t != "" {
					result = append(result, t)
				}
			}

			if len(result) > 0 {
				return result, true
			}
		}
	}

	return nil, false
}

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

func currentProjectName() string {
	return filepath.Base(getCurrentDir())
}

func resolveScopedProject(params map[string]any) (string, error) {
	current := currentProjectName()
	if project, ok := params["project"].(string); ok && project != "" && project != current {
		return "", fmt.Errorf("cross-project access is not allowed: current project is %s", current)
	}

	return current, nil
}
