package cmd

import (
	"os"

	"github.com/spf13/cobra"

	"github.com/EarthmanMuons/herosync/internal/logging"
)

var combineCmd = &cobra.Command{
	Use:     "combine",
	Aliases: []string{"merge"},
	Short:   "Merge video clips into full recordings",
	RunE:    runCombine,
}

func runCombine(cmd *cobra.Command, args []string) error {
	log := logging.GetLogger()

	// cfg, err := getConfigWithFlags(cmd)
	// if err != nil {
	// 	return err
	// }

	log.Error("UNIMPLEMENTED", "command", cmd.Use)
	os.Exit(1)

	return nil
}
