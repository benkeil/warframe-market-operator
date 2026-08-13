package orders

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/benkeil/warframe-market-operator/internal/adapter"
	"github.com/benkeil/warframe-market-operator/internal/domain/service"
)

func newListCommand(debug *bool) *cobra.Command {
	var limit int
	var orderType string
	var status []string
	var platform string
	var noCrossplay bool
	var sortBy string
	var sortDir string

	cmd := &cobra.Command{
		Use:   "list <slug>",
		Short: "List orders for an item with optional filters and sorting",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			crossplay := !noCrossplay
			return runList(cmd.Context(), args[0], limit, orderType, status, platform, crossplay, sortBy, sortDir, *debug)
		},
	}

	cmd.Flags().IntVarP(&limit, "limit", "l", 5, "Maximum number of orders to display")
	cmd.Flags().StringVarP(&orderType, "type", "t", "sell", "Order type: sell, buy")
	cmd.Flags().StringSliceVarP(&status, "status", "s",
		[]string{"ingame", "online"}, "User status filter: ingame, online, offline")
	cmd.Flags().StringVarP(&platform, "platform", "p", "pc", "Platform: pc, ps4, xbox, switch")
	cmd.Flags().BoolVar(&noCrossplay, "no-crossplay", false, "Exclude crossplay users from other platforms")
	cmd.Flags().StringVar(&sortBy, "sort-by", "platinum", "Sort field: platinum, quantity, reputation")
	cmd.Flags().StringVar(&sortDir, "sort-dir", "asc", "Sort direction: asc, desc")

	return cmd
}

func runList(
	ctx context.Context, slug string, limit int,
	orderType string, statuses []string, platform string,
	crossplay bool, sortBy string, sortDir string, debug bool,
) error {
	svc := adapter.NewHttpWarframeMarketService(debug)

	filter := service.OrdersFilter{
		Platform:      service.Platform(platform),
		Crossplay:     crossplay,
		SortBy:        service.SortField(sortBy),
		SortDirection: service.SortDirection(sortDir),
	}

	if orderType != "" {
		t := service.OrderType(orderType)
		filter.Type = &t
	}

	for _, s := range statuses {
		filter.Status = append(filter.Status, service.UserStatus(s))
	}

	orders, err := svc.GetOrdersByItem(ctx, slug, filter)
	if err != nil {
		return fmt.Errorf("fetching orders: %w", err)
	}

	if len(orders) == 0 {
		fmt.Println("No orders found.")
		return nil
	}

	if limit > 0 && len(orders) > limit {
		orders = orders[:limit]
	}

	return renderOrdersTable(os.Stdout, orders)
}
