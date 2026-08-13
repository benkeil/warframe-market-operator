package sets

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/benkeil/warframe-market-operator/internal/adapter"
	"github.com/benkeil/warframe-market-operator/internal/domain/usecase"
	"github.com/olekukonko/tablewriter"
)

func newInfoCommand(debug *bool) *cobra.Command {
	return &cobra.Command{
		Use:   "info <slug>",
		Short: "Show set parts and required quantities",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runInfo(cmd.Context(), args[0], *debug)
		},
	}
}

func runInfo(ctx context.Context, slug string, debug bool) error {
	uc := usecase.NewGetSetInfoUseCase(
		adapter.NewHttpWarframeMarketService(debug),
		adapter.NewHttpWarframeExportService(),
	)

	info, err := uc.Execute(ctx, slug)
	if err != nil {
		return err
	}

	fmt.Printf("Set: %s (%s)\n\n", info.Name, info.Slug)

	table := tablewriter.NewWriter(os.Stdout)
	table.Header([]string{"Part", "Slug", "Count"})
	for _, p := range info.Parts {
		table.Append([]string{p.Name, p.Slug, fmt.Sprintf("%d", p.Count)})
	}
	table.Render()
	return nil
}
