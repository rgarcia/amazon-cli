package cmd

import (
	"fmt"
	"os"
	"slices"
	"strings"

	"github.com/urfave/cli/v3"
)

var Command *cli.Command

func init() {
	cli.VersionPrinter = func(cmd *cli.Command) {
		fmt.Fprintf(os.Stdout, "amzn version %s\n", cmd.Root().Version)
	}

	Command = &cli.Command{
		Name:    "amzn",
		Usage:   "CLI for Amazon.com through Kernel cloud browsers",
		Version: Version,
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "profile",
				Aliases: []string{"p"},
				Usage:   "Config profile to use",
			},
			&cli.IntFlag{
				Name:  "browser-timeout",
				Usage: "Kernel browser inactivity timeout in seconds",
				Value: 600,
			},
			&cli.IntFlag{
				Name:  "request-timeout",
				Usage: "Browser curl timeout in seconds",
				Value: 30,
			},
			&cli.BoolFlag{
				Name:  "debug",
				Usage: "Print request URLs and browser session lifecycle messages to stderr",
			},
			&cli.StringFlag{
				Name:    "output",
				Aliases: []string{"o"},
				Usage:   "Output format (one of: " + strings.Join(OutputFormats, ", ") + ")",
				Value:   "auto",
				Validator: func(format string) error {
					if !slices.Contains(OutputFormats, strings.ToLower(format)) {
						return fmt.Errorf("output must be one of: %s", strings.Join(OutputFormats, ", "))
					}
					return nil
				},
			},
			&cli.StringFlag{
				Name:  "transform",
				Usage: "GJSON expression to transform JSON output",
			},
		},
		Commands: []*cli.Command{
			&ordersCmd,
			&productCmd,
			&configCmd,
			&versionCmd,
		},
		EnableShellCompletion:      true,
		ShellCompletionCommandName: "@completion",
		HideHelpCommand:            true,
	}
}
