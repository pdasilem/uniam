package cli

import (
	"fmt"
	"os"

	"uniam/internal/buildinfo"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "uniam",
	Short: "Uniam - local notes for coding agents",
	Long: `Uniam provides local-first note storage for coding agents.
Store, search, and retrieve decisions, patterns, bugs,
and context across sessions.`,
	Version: buildinfo.Version,
}

// Execute runs the root command.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(doctorCmd)
	rootCmd.AddCommand(initCmd)
	rootCmd.AddCommand(storeCmd)
	rootCmd.AddCommand(statsCmd)
	rootCmd.AddCommand(explainSearchCmd)
	rootCmd.AddCommand(searchCmd)
	rootCmd.AddCommand(retrieveCmd)
	rootCmd.AddCommand(listCmd)
	rootCmd.AddCommand(updateCmd)
	rootCmd.AddCommand(archiveCmd)
	rootCmd.AddCommand(supersedeCmd)
	rootCmd.AddCommand(compactCmd)
	rootCmd.AddCommand(removeCmd)
	rootCmd.AddCommand(notesCmd)
	rootCmd.AddCommand(configCmd)
	rootCmd.AddCommand(checkUpdateCmd)
	rootCmd.AddCommand(binaryUpdateCmd)
	rootCmd.AddCommand(setupCmd)
	rootCmd.AddCommand(uninstallCmd)
	rootCmd.AddCommand(reindexCmd)
	rootCmd.AddCommand(mcpCmd)
}
