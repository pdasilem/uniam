package cli

import (
	"fmt"

	"uniam/internal/buildinfo"

	"github.com/spf13/cobra"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version",
	//nolint:revive
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(buildinfo.Version)
	},
}
