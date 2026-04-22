package cli

import (
	"fmt"
	"os"
	"sort"

	"uniam/internal/core"

	"github.com/spf13/cobra"
)

var statsCmd = &cobra.Command{
	Use:   "stats",
	Short: "Show note statistics and lifecycle distribution",
	Run: func(cmd *cobra.Command, args []string) {
		svc, err := core.NewService("")
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		defer func() { _ = svc.Close() }()

		stats, err := svc.Stats(nil, nil)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("Notes\n")
		fmt.Printf("  total:        %d\n", stats.Total)
		fmt.Printf("  active:       %d\n", stats.Active)
		fmt.Printf("  archived:     %d\n", stats.Archived)
		fmt.Printf("  superseded:   %d\n", stats.Superseded)
		fmt.Printf("  stale:        %d\n", stats.Stale)
		fmt.Printf("  with details: %d\n", stats.WithDetails)
		fmt.Printf("  with vectors: %d\n", stats.WithVectors)

		printCountMap("By project", stats.ByProject)
		printCountMap("By category", stats.ByCategory)
		printCountMap("By source", stats.BySource)
	},
}

func printCountMap(title string, values map[string]int64) {
	fmt.Printf("\n%s\n", title)
	if len(values) == 0 {
		fmt.Println("  (none)")
		return
	}

	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	for _, key := range keys {
		fmt.Printf("  %-20s %d\n", key+":", values[key])
	}
}
