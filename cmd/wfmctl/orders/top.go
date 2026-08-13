package orders

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/benkeil/warframe-market-operator/internal/adapter"
	"github.com/benkeil/warframe-market-operator/internal/domain/service"
)

func newTopCommand(debug *bool) *cobra.Command {
	var platform string
	var noCrossplay bool

	cmd := &cobra.Command{
		Use:   "top <slug>",
		Short: "Show top sell orders for an item from online users (up to 5)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			crossplay := !noCrossplay
			return runTop(cmd.Context(), args[0], platform, crossplay, *debug)
		},
	}

	cmd.Flags().StringVarP(&platform, "platform", "p", "pc", "Platform: pc, ps4, xbox, switch")
	cmd.Flags().BoolVar(&noCrossplay, "no-crossplay", false, "Exclude crossplay users from other platforms")

	return cmd
}

func runTop(ctx context.Context, slug string, platform string, crossplay bool, debug bool) error {
	svc := adapter.NewHttpWarframeMarketService(debug)

	filter := service.OrdersFilter{
		Platform:  service.Platform(platform),
		Crossplay: crossplay,
	}

	top, err := svc.GetTopOrdersByItem(ctx, slug, filter)
	if err != nil {
		return fmt.Errorf("fetching top orders: %w", err)
	}

	if len(top.Sell) == 0 {
		fmt.Println("No sell orders found.")
		return nil
	}

	return renderOrdersTable(os.Stdout, top.Sell)
}
