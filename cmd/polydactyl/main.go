package main

import (
	"os"

	"github.com/spf13/cobra"
)

func Command() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "polydactyl",
		Short: "Random cat generate — load generator for tekton",
	}

	cmd.AddCommand(
		generateCommand(),
		reportCommand(),
	)

	return cmd
}

func main() {
	cmd := Command()
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
