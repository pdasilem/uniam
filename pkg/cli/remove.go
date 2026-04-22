package cli

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"uniam/internal/core"

	"github.com/spf13/cobra"
)

var removeCmd = &cobra.Command{
	Use:   "remove [id]",
	Short: "Remove a note from the uniam",
	Args:  cobra.MaximumNArgs(1),
	//nolint:revive
	Run: func(cmd *cobra.Command, args []string) {
		svc, err := core.NewService("")
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		defer func() { _ = svc.Close() }()

		if len(args) == 0 {
			dir, _ := os.Getwd()
			projectName := filepath.Base(dir)
			fmt.Printf("Warning: this will delete all Uniam notes for project %q from both index.db and shelves.\n", projectName)
			fmt.Printf("Type the project name to confirm: ")

			reader := bufio.NewReader(os.Stdin)
			confirm, _ := reader.ReadString('\n')
			if strings.TrimSpace(confirm) != projectName {
				fmt.Fprintln(os.Stderr, "Aborted.")
				os.Exit(1)
			}

			deleted, err := svc.RemoveProject(projectName)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}

			fmt.Printf("Removed %d notes and deleted shelves for project %s\n", deleted, projectName)
			return
		}

		itemID := args[0]
		deleted, err := svc.Remove(itemID)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		if deleted {
			fmt.Printf("Removed note %s\n", itemID)
		} else {
			fmt.Printf("No note found for %s\n", itemID)
		}
	},
}
