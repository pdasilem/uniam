package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"uniam/internal/core"

	"github.com/spf13/cobra"
)

var (
	explainLimit   int
	explainProject bool
	explainSource  string
	explainVectors bool
	explainMode    string
)

var explainSearchCmd = &cobra.Command{
	Use:   "explain-search [query]",
	Short: "Explain which retrieval path a search query takes",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		query := args[0]

		svc, err := core.NewService("")
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		defer func() { _ = svc.Close() }()

		var project *string
		if explainProject {
			dir, _ := os.Getwd()
			projectName := filepath.Base(dir)
			project = &projectName
		}

		var source *string
		if explainSource != "" {
			source = &explainSource
		}

		explanation, results, err := svc.ExplainSearchWithMode(query, explainLimit, project, source, explainVectors, explainMode)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		fmt.Println("Search explanation")
		fmt.Printf("  query:             %s\n", explanation.Query)
		fmt.Printf("  mode:              %s\n", explanation.Mode)
		fmt.Printf("  use_vectors:       %t\n", explanation.UseVectors)
		fmt.Printf("  provider_ready:    %t\n", explanation.ProviderReady)
		fmt.Printf("  vectors_available: %t\n", explanation.VectorsAvailable)
		fmt.Printf("  embedded:          %t\n", explanation.Embedded)
		fmt.Printf("  fts_results:       %d\n", explanation.FTSResults)
		fmt.Printf("  vector_results:    %d\n", explanation.VectorResults)
		fmt.Printf("  returned:          %d\n", explanation.ReturnedResults)
		fmt.Printf("  weights:           fts=%.2f vec=%.2f\n", explanation.FTSWeight, explanation.VectorWeight)

		if len(results) == 0 {
			fmt.Println("\nNo results.")
			return
		}

		fmt.Println("\nTop results")
		for i, result := range results {
			fmt.Printf("  [%d] %s (%s, %.2f)\n", i+1, result.Title, result.Status, result.Score)
		}
	},
}

func init() {
	explainSearchCmd.Flags().IntVarP(&explainLimit, "limit", "n", 5, "Maximum number of results to explain")
	explainSearchCmd.Flags().BoolVarP(&explainProject, "project", "p", false, "Filter to current project")
	explainSearchCmd.Flags().StringVarP(&explainSource, "source", "s", "", "Filter by source")
	explainSearchCmd.Flags().BoolVar(&explainVectors, "vectors", true, "Allow vector search when available")
	explainSearchCmd.Flags().StringVar(&explainMode, "mode", "search", "Retrieval mode: startup, search, debug, architecture, maintenance")
}
