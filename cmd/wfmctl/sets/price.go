package sets

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/benkeil/warframe-market-operator/internal/adapter"
	"github.com/benkeil/warframe-market-operator/internal/domain/service"
	"github.com/benkeil/warframe-market-operator/internal/domain/usecase"
	"github.com/olekukonko/tablewriter"
)

func newPriceCommand(debug *bool) *cobra.Command {
	var platform string
	var noCrossplay bool

	cmd := &cobra.Command{
		Use:   "price <slug>",
		Short: "Show cheapest component prices for a set bought individually",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			crossplay := !noCrossplay
			return runPrice(cmd.Context(), args[0], platform, crossplay, *debug)
		},
	}

	cmd.Flags().StringVarP(&platform, "platform", "p", "pc", "Platform: pc, ps4, xbox, switch")
	cmd.Flags().BoolVar(&noCrossplay, "no-crossplay", false, "Exclude crossplay users from other platforms")

	return cmd
}

func runPrice(ctx context.Context, slug string, platform string, crossplay bool, debug bool) error {
	marketSvc := adapter.NewHttpWarframeMarketService(debug)
	exportSvc := adapter.NewHttpWarframeExportService()

	infoUC := usecase.NewGetSetInfoUseCase(marketSvc, exportSvc)
	priceUC := usecase.NewGetSetComponentPriceUseCase(infoUC, marketSvc)

	filter := service.OrdersFilter{
		Platform:  service.Platform(platform),
		Crossplay: crossplay,
	}

	result, err := priceUC.Execute(ctx, slug, filter)
	if err != nil {
		return err
	}

	fmt.Printf("Set: %s (%s)\n", result.SetName, result.SetSlug)

	for _, p := range result.Parts {
		fmt.Println()
		if len(p.BuyOrders) == 0 {
			fmt.Printf("  %s  [%s]  —  no sell orders found\n", p.Part.Name, p.Part.Slug)
			continue
		}
		fmt.Printf("  %s  [%s]  (x%d)\n", p.Part.Name, p.Part.Slug, p.Part.Count)
		table := tablewriter.NewWriter(os.Stdout)
		table.Header([]string{"Status", "Player", "Price (p)", "Qty", "Subtotal (p)"})
		for _, l := range p.BuyOrders {
			_ = table.Append([]string{
				string(l.Seller.Status),
				l.Seller.IngameName,
				fmt.Sprintf("%d", l.Price),
				fmt.Sprintf("%d", l.Quantity),
				fmt.Sprintf("%d", l.Subtotal),
			})
		}
		_ = table.Render()
		if p.Part.Count > 1 {
			fmt.Printf("  Part total: %d platinum\n", p.Total)
		}
	}

	fmt.Printf("\nTotal: %d platinum\n", result.TotalPlatinum)
	return nil
}
