package main

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/kernel/kernel-go-sdk"
	"github.com/rgarcia/amazon-cli/pkg/amazon"
	"github.com/rgarcia/amazon-cli/pkg/cmd"
)

func main() {
	app := cmd.Command
	if err := app.Run(context.Background(), os.Args); err != nil {
		var httpErr *amazon.HTTPError
		var kernelErr *kernel.Error
		switch {
		case errors.As(err, &httpErr):
			fmt.Fprintf(os.Stderr, "%s %q: %d\n", httpErr.Method, httpErr.URL, httpErr.Status)
			if httpErr.Body != "" {
				fmt.Fprintf(os.Stderr, "%s\n", httpErr.Body)
			}
		case errors.As(err, &kernelErr):
			fmt.Fprintf(os.Stderr, "Kernel API error: %s\n", kernelErr.Error())
		default:
			fmt.Fprintf(os.Stderr, "Error: %s\n", err.Error())
		}
		os.Exit(1)
	}
}
