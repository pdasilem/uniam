package cli

import (
	"context"
	"fmt"
	"os"

	"uniam/internal/buildinfo"
	"uniam/internal/update"

	"github.com/spf13/cobra"
)

var checkUpdateCmd = &cobra.Command{
	Use:   "check-update",
	Short: "Check GitHub Releases for a newer uniam version",
	Run: func(cmd *cobra.Command, args []string) {
		checker := update.NewChecker(buildinfo.Version).WithProgress(func(message string) {
			fmt.Println(message)
		})

		fmt.Println("Preparing release check...")
		result, err := checker.Check(context.Background())
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("Current version: %s\n", result.CurrentVersion)
		fmt.Printf("Latest version:  %s\n", result.LatestVersion)
		if result.PublishedAt != "" {
			fmt.Printf("Published at:    %s\n", result.PublishedAt)
		}

		if result.UpdateAvailable {
			fmt.Printf("Update available: yes\n")
			if result.AssetURL != "" {
				fmt.Printf("Asset:            %s\n", result.AssetName)
			}
		} else {
			fmt.Printf("Update available: no\n")
		}
	},
}
