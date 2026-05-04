package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/rgarcia/amazon-cli/pkg/amazon"
	"github.com/urfave/cli/v3"
)

var ordersCmd = cli.Command{
	Name:  "orders",
	Usage: "List and inspect Amazon orders",
	Commands: []*cli.Command{
		&ordersListCmd,
		&ordersGetCmd,
	},
	HideHelpCommand: true,
}

var orderPageFlags = []cli.Flag{
	&cli.IntFlag{
		Name:  "page",
		Usage: "Amazon order history page number",
		Value: 1,
	},
	&cli.IntFlag{
		Name:  "page-size",
		Usage: "Expected Amazon page size used to convert page to start index",
		Value: 10,
	},
	&cli.IntFlag{
		Name:  "start-index",
		Usage: "Amazon order history startIndex value; overrides --page",
		Value: -1,
	},
	&cli.StringFlag{
		Name:  "time-filter",
		Usage: "Amazon order history time filter, e.g. year-2026, year-2025, last30",
		Value: "year-2026",
	},
}

var ordersListCmd = cli.Command{
	Name:            "list",
	Usage:           "List Amazon orders",
	Flags:           orderPageFlags,
	Action:          handleOrdersList,
	HideHelpCommand: true,
}

func handleOrdersList(ctx context.Context, cmd *cli.Command) error {
	api, cleanup, err := newAmazonClient(ctx, cmd)
	if err != nil {
		return err
	}
	defer cleanup()

	opts := amazon.ListOrdersOptions{
		Page:       cmd.Int("page"),
		PageSize:   cmd.Int("page-size"),
		StartIndex: cmd.Int("start-index"),
		TimeFilter: cmd.String("time-filter"),
	}
	page, err := api.ListOrders(ctx, opts)
	if err != nil {
		return err
	}

	format := cmd.Root().String("output")
	if format != "auto" {
		return ShowAny(os.Stdout, page, format, cmd.Root().String("transform"))
	}
	if len(page.Orders) == 0 {
		fmt.Fprintln(os.Stderr, "No orders found.")
		return nil
	}

	table := NewTableWriter(os.Stdout, "ORDER", "DATE", "TOTAL", "STATUS", "ITEMS")
	table.TruncOrder = []int{4, 3}
	for _, o := range page.Orders {
		table.AddRow(o.ID, o.OrderPlaced, o.Total, o.Status, itemSummary(o.Items))
	}
	table.Render()
	if page.NextStartIndex > 0 {
		fmt.Fprintf(os.Stderr, "\nNext page: amzn orders list --time-filter %s --start-index %d\n", page.TimeFilter, page.NextStartIndex)
	}
	return nil
}

var ordersGetCmd = cli.Command{
	Name:            "get",
	Usage:           "Get Amazon order details",
	ArgsUsage:       "<order-id>",
	Action:          handleOrdersGet,
	HideHelpCommand: true,
}

func handleOrdersGet(ctx context.Context, cmd *cli.Command) error {
	args := cmd.Args().Slice()
	if len(args) < 1 {
		return fmt.Errorf("order ID required")
	}
	api, cleanup, err := newAmazonClient(ctx, cmd)
	if err != nil {
		return err
	}
	defer cleanup()

	order, err := api.GetOrder(ctx, args[0])
	if err != nil {
		return err
	}
	return ShowAny(os.Stdout, order, cmd.Root().String("output"), cmd.Root().String("transform"))
}

func newAmazonClient(ctx context.Context, cmd *cli.Command) (*amazon.Client, func(), error) {
	apiKey, err := resolveKernelAPIKey(cmd)
	if err != nil {
		return nil, func() {}, err
	}
	cfg := resolveProfileConfig(cmd)
	browserID := cmd.Root().String("browser-id")
	if browserID == "" && cfg.KernelProfileID == "" && cfg.KernelProfileName == "" {
		return nil, func() {}, fmt.Errorf("no Kernel browser profile configured. Run 'amzn config init', set kernel_profile_id/kernel_profile_name, or pass --browser-id")
	}
	opts := amazon.Options{
		KernelAPIKey:      apiKey,
		KernelBaseURL:     resolveKernelBaseURL(cmd),
		KernelProfileID:   cfg.KernelProfileID,
		KernelProfileName: cfg.KernelProfileName,
		AmazonBaseURL:     cfg.AmazonBaseURL,
		BrowserID:         browserID,
		KeepBrowser:       cmd.Root().Bool("keep-browser"),
		BrowserTimeout:    cmd.Root().Int("browser-timeout"),
		RequestTimeout:    cmd.Root().Int("request-timeout"),
		Debug:             cmd.Root().Bool("debug"),
	}
	api, err := amazon.NewClient(ctx, opts)
	if err != nil {
		return nil, func() {}, err
	}
	return api, func() {
		if err := api.Close(context.Background()); err != nil && opts.Debug {
			fmt.Fprintf(os.Stderr, "delete browser: %s\n", err)
		}
	}, nil
}

func itemSummary(items []amazon.OrderItem) string {
	if len(items) == 0 {
		return ""
	}
	if len(items) == 1 {
		return items[0].Title
	}
	return fmt.Sprintf("%s (+%d more)", items[0].Title, len(items)-1)
}
