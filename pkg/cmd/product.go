package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/rgarcia/amazon-cli/pkg/amazon"
	"github.com/urfave/cli/v3"
)

var productCmd = cli.Command{
	Name:  "product",
	Usage: "Search and inspect Amazon products",
	Commands: []*cli.Command{
		&productSearchCmd,
		&productGetCmd,
	},
	HideHelpCommand: true,
}

var productSearchCmd = cli.Command{
	Name:      "search",
	Usage:     "Search Amazon products",
	ArgsUsage: "<query>",
	Flags: []cli.Flag{
		&cli.IntFlag{
			Name:  "page",
			Usage: "Amazon search results page number",
			Value: 1,
		},
	},
	Action:          handleProductSearch,
	HideHelpCommand: true,
}

func handleProductSearch(ctx context.Context, cmd *cli.Command) error {
	query := strings.TrimSpace(strings.Join(cmd.Args().Slice(), " "))
	if query == "" {
		return fmt.Errorf("product search query required")
	}
	api, cleanup, err := newAmazonClient(ctx, cmd)
	if err != nil {
		return err
	}
	defer cleanup()

	page, err := api.SearchProducts(ctx, amazon.SearchProductsOptions{
		Query: query,
		Page:  cmd.Int("page"),
	})
	if err != nil {
		return err
	}

	format := cmd.Root().String("output")
	if format != "auto" {
		return ShowAny(os.Stdout, page, format, cmd.Root().String("transform"))
	}
	if len(page.Products) == 0 {
		fmt.Fprintln(os.Stderr, "No products found.")
		return nil
	}

	table := NewTableWriter(os.Stdout, "ASIN", "SPONSORED", "PRICE", "RATING", "REVIEWS", "TITLE")
	table.TruncOrder = []int{5, 3}
	for _, p := range page.Products {
		table.AddRow(p.ASIN, yesNo(p.Sponsored), p.Price, p.Rating, p.Reviews, p.Title)
	}
	table.Render()
	return nil
}

var productGetCmd = cli.Command{
	Name:            "get",
	Usage:           "Get Amazon product details",
	ArgsUsage:       "<asin>",
	Action:          handleProductGet,
	HideHelpCommand: true,
}

func handleProductGet(ctx context.Context, cmd *cli.Command) error {
	args := cmd.Args().Slice()
	if len(args) < 1 {
		return fmt.Errorf("product ID required")
	}
	api, cleanup, err := newAmazonClient(ctx, cmd)
	if err != nil {
		return err
	}
	defer cleanup()

	product, err := api.GetProduct(ctx, args[0])
	if err != nil {
		return err
	}
	return ShowAny(os.Stdout, product, cmd.Root().String("output"), cmd.Root().String("transform"))
}

func yesNo(v bool) string {
	if v {
		return "yes"
	}
	return "no"
}
