package cmd

import (
	"log/slog"
	"os"

	"github.com/spf13/cobra"
)

var yoloCmd = &cobra.Command{
	Use:     "yolo",
	Aliases: []string{"merge"},
	Short:   "Hands-free sync: download, combine, publish",
	RunE:    runYolo,
}

func runYolo(cmd *cobra.Command, args []string) error {
	logger := slog.Default()

	// cfg, err := getConfigWithFlags(cmd)
	// if err != nil {
	// 	return err
	// }

	logger.Error("UNIMPLEMENTED", "command", cmd.Use)
	os.Exit(1)

	return nil
}
