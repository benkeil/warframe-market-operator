package items

import (
	"context"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/benkeil/warframe-market-operator/internal/adapter"
)

func newInfoCommand(debug *bool) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "info <slug>",
		Short: "Show detailed information about an item including set parts and recipe",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runInfo(cmd.Context(), args[0], *debug)
		},
	}
	return cmd
}

func runInfo(ctx context.Context, slug string, debug bool) error {
	marketSvc := adapter.NewHttpWarframeMarketService(debug)
	exportSvc := adapter.NewHttpWarframeExportService()

	item, err := marketSvc.GetItemBySlug(ctx, slug)
	if err != nil {
		return fmt.Errorf("fetching item: %w", err)
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintf(w, "Name:\t%s\n", item.Name)
	_, _ = fmt.Fprintf(w, "Slug:\t%s\n", item.Slug)
	_, _ = fmt.Fprintf(w, "Tags:\t%s\n", strings.Join(item.Tags, ", "))
	_, _ = fmt.Fprintf(w, "Mastery Rank:\t%d\n", item.ReqMasteryRank)
	_, _ = fmt.Fprintf(w, "Ducats:\t%d\n", item.Ducats)
	_, _ = fmt.Fprintf(w, "Trading Tax:\t%d\n", item.TradingTax)
	if err := w.Flush(); err != nil {
		return err
	}

	// Fetch all items to resolve set part IDs to names.
	if item.SetRoot && len(item.SetParts) > 0 {
		allItems, err := marketSvc.GetItems(ctx)
		if err != nil {
			return fmt.Errorf("fetching items for set parts: %w", err)
		}
		byID := make(map[string]string, len(allItems))
		slugByID := make(map[string]string, len(allItems))
		for _, i := range allItems {
			byID[i.ID] = i.Name
			slugByID[i.ID] = i.Slug
		}

		fmt.Println("\nSet Parts:")
		pw := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		_, _ = fmt.Fprintln(pw, "  NAME\tSLUG")
		for _, partID := range item.SetParts {
			name := byID[partID]
			slugVal := slugByID[partID]
			if name == "" {
				name = partID
			}
			_, _ = fmt.Fprintf(pw, "  %s\t%s\n", name, slugVal)
		}
		_ = pw.Flush()
	}

	// Look up recipe from DE public export.
	recipe, err := exportSvc.GetRecipeByResultType(ctx, item.GameRef)
	if err != nil {
		return fmt.Errorf("fetching recipe: %w", err)
	}

	if recipe != nil {
		fmt.Println("\nRecipe Ingredients:")
		rw := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		_, _ = fmt.Fprintln(rw, "  ITEM\tCOUNT")
		for _, ing := range recipe.Ingredients {
			// Use last path segment as readable name fallback.
			parts := strings.Split(ing.ItemType, "/")
			name := parts[len(parts)-1]
			_, _ = fmt.Fprintf(rw, "  %s\t%d\n", name, ing.ItemCount)
		}
		_ = rw.Flush()
	}

	return nil
}
