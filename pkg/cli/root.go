package cli

import (
	"fmt"
	"os"
	"strings"

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

type mcpToolHelp struct {
	Name        string
	Description string
}

var mcpTools = []mcpToolHelp{
	{"uniam_context", "Get recent notes for the current project scope."},
	{"uniam_search", "Search notes inside the current project scope."},
	{"uniam_retrieve", "Retrieve full note contents from the current project scope."},
	{"uniam_store", "Store a note for future sessions in the current project scope."},
	{"uniam_archive", "Archive a note in the current project scope."},
	{"uniam_supersede", "Mark a note as superseded by another note in the current project scope."},
	{"uniam_update_note", "Explicitly update a note in the current project scope."},
	{"uniam_compact", "Create a canonical summary note and archive matched notes in the current project scope."},
	{"uniam_explain_search", "Explain retrieval behavior for the current project scope."},
}

// Execute runs the root command.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func init() {
	defaultHelpFunc := rootCmd.HelpFunc()
	rootCmd.SetHelpFunc(func(cmd *cobra.Command, args []string) {
		if cmd != rootCmd {
			defaultHelpFunc(cmd, args)
			return
		}
		fmt.Fprint(cmd.OutOrStdout(), cmd.Long)
		fmt.Fprint(cmd.OutOrStdout(), "\n\nUsage:\n")
		fmt.Fprintln(cmd.OutOrStdout(), "  uniam [command]")
		fmt.Fprintln(cmd.OutOrStdout(), "  uniam [flags]")
		fmt.Fprintln(cmd.OutOrStdout(), "  uniam [command] [flags]")
		fmt.Fprintln(cmd.OutOrStdout())

		fmt.Fprintln(cmd.OutOrStdout(), "Available Commands:")
		for _, child := range cmd.Commands() {
			if !child.IsAvailableCommand() || child.Hidden {
				continue
			}
			fmt.Fprintf(cmd.OutOrStdout(), "  %-12s %s\n", child.Name(), child.Short)
		}

		fmt.Fprintln(cmd.OutOrStdout(), "\nMCP Tools:")
		for _, tool := range mcpTools {
			fmt.Fprintf(cmd.OutOrStdout(), "  %-18s %s\n", tool.Name, tool.Description)
		}

		fmt.Fprintln(cmd.OutOrStdout(), "\nFlags:")
		fmt.Fprintln(cmd.OutOrStdout(), "  -h, --help      help for uniam")
		fmt.Fprintln(cmd.OutOrStdout(), "  -v, --version   version for uniam")
		fmt.Fprintln(cmd.OutOrStdout(), "\nSetup Flags:")
		fmt.Fprintln(cmd.OutOrStdout(), "  -p, --project        install or uninstall in the current project root when the agent supports project scope")
		fmt.Fprintln(cmd.OutOrStdout(), "      --ripgrep        add the optional ripgrep MCP server during setup")
		fmt.Fprintln(cmd.OutOrStdout(), "      --code-search    add the optional code-search MCP server during setup")
		fmt.Fprintln(cmd.OutOrStdout(), "      --context7       add optional Context7 MCP server during setup")
		fmt.Fprintln(cmd.OutOrStdout(), "      --git-mcp        add the optional Git MCP server during setup")
		fmt.Fprintln(cmd.OutOrStdout(), "      --brave-search   add the optional Brave Search MCP server during setup")
		fmt.Fprintln(cmd.OutOrStdout(), "      --config-dir     override the target config directory for supported agents")
		fmt.Fprintf(cmd.OutOrStdout(), "\nUse %q for more information about a command.\n", strings.Join([]string{cmd.CommandPath(), "[command]", "--help"}, " "))
	})

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
