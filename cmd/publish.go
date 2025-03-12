package cmd

import (
	"log/slog"
	"os"

	"github.com/spf13/cobra"
)

var publishCmd = &cobra.Command{
	Use:     "publish",
	Aliases: []string{"pub"},
	Short:   "Upload processed videos to YouTube",
	RunE:    runCombine,
}

func runPublish(cmd *cobra.Command, args []string) error {
	logger := slog.Default()

	// cfg, err := getConfigWithFlags(cmd)
	// if err != nil {
	// 	return err
	// }

	logger.Error("UNIMPLEMENTED", "command", cmd.Use)
	os.Exit(1)

	return nil
}
