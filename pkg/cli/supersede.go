package cli

import (
	"fmt"
	"os"

	"uniam/internal/core"

	"github.com/spf13/cobra"
)

var supersedeBy string

var supersedeCmd = &cobra.Command{
	Use:   "supersede [id]",
	Short: "Mark a note as superseded by another note",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		itemID := args[0]
		if supersedeBy == "" {
			fmt.Fprintln(os.Stderr, "Error: --by is required")
			os.Exit(1)
		}

		svc, err := core.NewService("")
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		defer func() { _ = svc.Close() }()

		if _, err := svc.Supersede(itemID, supersedeBy); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("Superseded note %s by %s\n", itemID, supersedeBy)
	},
}

func init() {
	supersedeCmd.Flags().StringVar(&supersedeBy, "by", "", "ID of the note that supersedes this one")
}
