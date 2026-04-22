package cli

import (
	"fmt"
	"os"
	"strings"

	"uniam/internal/core"
	"uniam/internal/models"

	"github.com/spf13/cobra"
)

var (
	updateWhat    string
	updateWhy     string
	updateImpact  string
	updateTags    string
	updateDetails string
)

var updateCmd = &cobra.Command{
	Use:   "update-note [id]",
	Short: "Update an existing note explicitly",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		itemID := args[0]
		input := models.ItemUpdateInput{}

		if updateWhat != "" {
			input.What = &updateWhat
		}

		if updateWhy != "" {
			input.Why = &updateWhy
		}

		if updateImpact != "" {
			input.Impact = &updateImpact
		}

		if updateDetails != "" {
			input.Details = &updateDetails
		}

		if updateTags != "" {
			tags := strings.Split(updateTags, ",")
			for i := range tags {
				tags[i] = strings.TrimSpace(tags[i])
			}

			input.Tags = tags
		}

		svc, err := core.NewService("")
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		defer func() { _ = svc.Close() }()

		result, err := svc.Update(itemID, input)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("Updated note %s (%s)\n", itemID, result["action"])
	},
}

func init() {
	updateCmd.Flags().StringVar(&updateWhat, "what", "", "Update the What field")
	updateCmd.Flags().StringVar(&updateWhy, "why", "", "Update the Why field")
	updateCmd.Flags().StringVar(&updateImpact, "impact", "", "Update the Impact field")
	updateCmd.Flags().StringVarP(&updateTags, "tags", "g", "", "Replace tags with a comma-separated list")
	updateCmd.Flags().StringVarP(&updateDetails, "details", "d", "", "Append or create details content")
}
