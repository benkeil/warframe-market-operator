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

func newSearchCommand(debug *bool) *cobra.Command {
	var limit int

	cmd := &cobra.Command{
		Use:   "search <query>",
		Short: "Search for tradable items by name",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSearch(cmd.Context(), args[0], limit, *debug)
		},
	}

	cmd.Flags().IntVarP(&limit, "limit", "l", 10, "Maximum number of results to display")

	return cmd
}

func runSearch(ctx context.Context, query string, limit int, debug bool) error {
	svc := adapter.NewHttpWarframeMarketService(debug)

	items, err := svc.GetItems(ctx)
	if err != nil {
		return fmt.Errorf("fetching items: %w", err)
	}

	q := strings.ToLower(query)
	var results []struct{ slug, name string }
	for _, item := range items {
		if strings.Contains(strings.ToLower(item.Name), q) || strings.Contains(item.Slug, q) {
			results = append(results, struct{ slug, name string }{item.Slug, item.Name})
			if limit > 0 && len(results) >= limit {
				break
			}
		}
	}

	if len(results) == 0 {
		fmt.Printf("No items found matching %q.\n", query)
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "SLUG\tNAME")
	fmt.Fprintln(w, "----\t----")
	for _, r := range results {
		fmt.Fprintf(w, "%s\t%s\n", r.slug, r.name)
	}
	return w.Flush()
}
