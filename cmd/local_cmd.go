package cmd

import "github.com/spf13/cobra"

var (
	localCmd = &cobra.Command{
		Use:     "local",
		Aliases: []string{"\"kubectl tf(o)\""},
		Short:   "Use tfo with a local kubeconfig",
		Args:    cobra.MaximumNArgs(0),
	}
)

func init() {
	rootCmd.AddCommand(localCmd)
}
