package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"uniam/internal/core"

	"github.com/spf13/cobra"
)

var (
	listLimit  int
	listAll    bool
	listSource string
	listQuery  string
	listMode   string
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List recent notes",
	//nolint:revive
	Run: func(cmd *cobra.Command, args []string) {
		svc, err := core.NewService("")
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		defer func() { _ = svc.Close() }()

		var project *string
		if !listAll {
			dir, _ := os.Getwd()
			projectName := filepath.Base(dir)
			project = &projectName
		}

		var source *string
		if listSource != "" {
			source = &listSource
		}

		var query *string
		if listQuery != "" {
			query = &listQuery
		}

		results, total, err := svc.GetContextWithMode(listLimit, project, source, query, "never", false, listMode)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		if len(results) == 0 {
			fmt.Println("No notes found.")

			return
		}

		fmt.Printf("Notes (%d total, showing %d):\n", total, len(results))

		for _, r := range results {
			dateStr := r.CreatedAt[:10]

			dateDisplay := dateStr
			if t, err := time.Parse("2006-01-02", dateStr); err == nil {
				dateDisplay = t.Format("Jan 02")
			}

			cat := ""
			if r.Category != nil {
				cat = fmt.Sprintf(" [%s]", *r.Category)
			}

			tags := ""
			if len(r.Tags) > 0 {
				tags = fmt.Sprintf(" [[%s]]", strings.Join(r.Tags, " "))
			}

			fmt.Printf("- %s [%s] %s%s%s\n", r.ID[:8], dateDisplay, r.Title, cat, tags)
		}

		fmt.Println("\nUse `uniam search <query>` to search notes, `uniam retrieve <id>` for full details.")
	},
}

func init() {
	listCmd.Flags().IntVarP(&listLimit, "limit", "n", 10, "Maximum number of notes")
	listCmd.Flags().BoolVarP(&listAll, "all", "a", false, "List notes from all projects instead of only the current project")
	listCmd.Flags().StringVarP(&listSource, "source", "s", "", "Filter by source")
	listCmd.Flags().StringVarP(&listQuery, "query", "q", "", "Search query for filtering")
	listCmd.Flags().StringVar(&listMode, "mode", "startup", "Retrieval mode: startup, search, debug, architecture, maintenance")
}
