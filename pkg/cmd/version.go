package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/urfave/cli/v3"
)

var Version = "dev"

var versionCmd = cli.Command{
	Name:            "version",
	Usage:           "Print the version",
	Action:          handleVersion,
	HideHelpCommand: true,
}

func handleVersion(_ context.Context, _ *cli.Command) error {
	fmt.Fprintf(os.Stdout, "amzn version %s\n", Version)
	return nil
}
