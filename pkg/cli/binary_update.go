package cli

import (
	"context"
	"fmt"
	"os"

	"uniam/internal/buildinfo"
	"uniam/internal/update"

	"github.com/spf13/cobra"
)

var updateCheckOnly bool

var binaryUpdateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update the uniam binary from the latest GitHub release",
	Run: func(cmd *cobra.Command, args []string) {
		checker := update.NewChecker(buildinfo.Version).WithProgress(func(message string) {
			fmt.Println(message)
		})

		fmt.Println("Preparing update...")
		result, err := checker.Check(context.Background())
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		if !result.UpdateAvailable {
			fmt.Printf("Already up to date (%s)\n", result.CurrentVersion)
			return
		}

		if updateCheckOnly {
			fmt.Printf("Update available: %s -> %s\n", result.CurrentVersion, result.LatestVersion)
			return
		}

		fmt.Printf("Update available: %s -> %s\n", result.CurrentVersion, result.LatestVersion)
		fmt.Printf("Selected asset: %s\n", result.AssetName)
		if err := checker.Apply(context.Background(), result); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("Updated uniam from %s to %s\n", result.CurrentVersion, result.LatestVersion)
	},
}

func init() {
	binaryUpdateCmd.Flags().BoolVar(&updateCheckOnly, "check-only", false, "Only check whether an update is available")
}
