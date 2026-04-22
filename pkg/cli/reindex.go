package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"uniam/internal/core"

	"github.com/spf13/cobra"
)

var reindexAll bool

var reindexCmd = &cobra.Command{
	Use:   "reindex",
	Short: "Rebuild vector index with current embedding provider",
	//nolint:revive
	Run: func(cmd *cobra.Command, args []string) {
		svc, err := core.NewService("")
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		defer func() { _ = svc.Close() }()

		var project *string
		if !reindexAll {
			dir, _ := os.Getwd()
			projectName := filepath.Base(dir)
			project = &projectName
			fmt.Printf("Reindexing notes for project %s...\n", projectName)
		} else {
			fmt.Println("Reindexing notes for all projects...")
		}

		progressCallback := func(current, total int) {
			fmt.Printf("  %d/%d\r", current, total)

			if current == total {
				fmt.Println()
			}
		}

		result, err := svc.Reindex(project, progressCallback)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Reindex skipped: %v\n", err)

			return
		}

		fmt.Printf("Re-indexed %v notes with %v (%v dims); removed %v DB-only notes\n",
			result["count"], result["model"], result["dim"], result["deleted"])
		if skipped, ok := result["skipped_projects"]; ok {
			fmt.Printf("Skipped DB-vs-shelves deletion for projects with incomplete shelf IDs: %v\n", skipped)
		}
	},
}

func init() {
	reindexCmd.Flags().BoolVarP(&reindexAll, "all", "a", false, "Reindex all projects instead of only the current project")
}
