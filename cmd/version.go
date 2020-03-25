package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// versionCmd represents the version command
var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Display version information",
	Long: `Display acyl version, commit SHA and build date`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("version: %v\ncommit: https://github.com/dollarshaveclub/acyl/commit/%v\nbuild date: %v\n", Version, Commit, Date)
	},
}

func init() {
	RootCmd.AddCommand(versionCmd)
}
