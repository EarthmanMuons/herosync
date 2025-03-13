package cmd

import (
	"os"

	"github.com/spf13/cobra"
)

// newPublishCmd constructs the "publish" subcommand.
func newPublishCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "publish",
		Aliases: []string{"pub"},
		Short:   "Upload processed videos to YouTube",
		RunE:    runCombine,
	}
	return cmd
}

// runPublish is the entry point for the "publish" subcommand.
func runPublish(cmd *cobra.Command, args []string) error {
	// logger, cfg, err := parseConfigAndLogger(cmd)
	logger, _, err := parseConfigAndLogger(cmd)
	if err != nil {
		return err
	}

	logger.Error("UNIMPLEMENTED", "command", cmd.Use)
	os.Exit(1)

	return nil
}
