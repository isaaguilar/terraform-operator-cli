package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// Execute executes the root command.
var version string

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version of this bin",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Fprintf(os.Stderr, "ik-")
		fmt.Printf("%s\n", version)
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
