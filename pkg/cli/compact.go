package cli

import (
	"fmt"
	"os"

	"uniam/internal/core"
	"uniam/internal/models"

	"github.com/spf13/cobra"
)

var (
	compactTitle    string
	compactWhat     string
	compactWhy      string
	compactImpact   string
	compactDetails  string
	compactProject  string
	compactQuery    string
	compactSource   string
	compactCategory string
	compactLimit    int
)

var compactCmd = &cobra.Command{
	Use:   "compact",
	Short: "Create a canonical summary note and archive matched notes under it",
	Run: func(cmd *cobra.Command, args []string) {
		raw := models.RawItemInput{
			Title: compactTitle,
			What:  compactWhat,
		}

		if compactTitle == "" || compactWhat == "" || compactQuery == "" {
			fmt.Fprintln(os.Stderr, "Error: --title, --what, and --query are required")
			os.Exit(1)
		}

		if compactWhy != "" {
			raw.Why = &compactWhy
		}
		if compactImpact != "" {
			raw.Impact = &compactImpact
		}
		if compactDetails != "" {
			raw.Details = &compactDetails
		}

		svc, err := core.NewService("")
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		defer func() { _ = svc.Close() }()

		var source *string
		if compactSource != "" {
			source = &compactSource
		}

		var category *string
		if compactCategory != "" {
			category = &compactCategory
		}

		result, err := svc.Compact(raw, compactProject, compactQuery, source, compactLimit, category)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("Created canonical note %s covering %v notes\n", result["id"], result["covered_count"])
	},
}

func init() {
	compactCmd.Flags().StringVarP(&compactTitle, "title", "t", "", "Title of the canonical summary note")
	compactCmd.Flags().StringVarP(&compactWhat, "what", "w", "", "Summary statement for the canonical note")
	compactCmd.Flags().StringVarP(&compactWhy, "why", "y", "", "Why the compacted summary matters")
	compactCmd.Flags().StringVarP(&compactImpact, "impact", "i", "", "Impact of the compacted summary")
	compactCmd.Flags().StringVarP(&compactDetails, "details", "d", "", "Optional details to prepend before the generated covered-note list")
	compactCmd.Flags().StringVarP(&compactProject, "project", "p", "", "Project name (defaults to current directory)")
	compactCmd.Flags().StringVarP(&compactQuery, "query", "q", "", "Search query used to select notes for compaction")
	compactCmd.Flags().StringVarP(&compactSource, "source", "s", "", "Restrict compaction to notes from a given source")
	compactCmd.Flags().StringVarP(&compactCategory, "category", "c", "", "Restrict compaction to a given category")
	compactCmd.Flags().IntVarP(&compactLimit, "limit", "n", 20, "Maximum number of matching notes to compact")
}
