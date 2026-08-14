package rivens

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/benkeil/warframe-market-operator/internal/adapter"
	"github.com/benkeil/warframe-market-operator/internal/domain/service"
	"github.com/benkeil/warframe-market-operator/internal/domain/usecase"
	"github.com/olekukonko/tablewriter"
	tw "github.com/olekukonko/tablewriter/tw"
)

func newSearchCommand(debug *bool) *cobra.Command {
	var positiveStats []string
	var negativeStats string
	var sortBy string

	var reRollsMax int
	var maxPrice int
	var statusFilter []string
	var buyoutOnly bool

	cmd := &cobra.Command{
		Use:   "search <weapon-slug>",
		Short: "Search riven auctions for a weapon",
		Example: `  wfmctl rivens search falcor --positive critical_chance,critical_damage --negative has
  wfmctl rivens search gram_prime --positive critical_chance --negative none`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSearch(
				cmd.Context(), args[0],
				positiveStats, negativeStats, sortBy, reRollsMax, maxPrice, statusFilter, buyoutOnly,
				*debug,
			)
		},
	}

	cmd.Flags().StringSliceVar(&positiveStats, "positive", nil,
		"Required positive stats (comma-separated, e.g. critical_chance,critical_damage)")
	cmd.Flags().StringVar(&negativeStats, "negative", "",
		`Negative stat filter: "has" = must have one, "none" = no negative, or a specific stat url_name`)
	cmd.Flags().StringVar(&sortBy, "sort", "price_asc", "Sort order: price_asc, price_desc")
	cmd.Flags().IntVar(&reRollsMax, "re-rolls-max", 0, "Maximum number of re-rolls (0 = no limit)")
	cmd.Flags().IntVar(&maxPrice, "max-price", 0, "Maximum buyout price in platinum (0 = no limit)")
	cmd.Flags().StringSliceVar(&statusFilter, "status", nil, "Filter by owner status: ingame, online, offline")
	cmd.Flags().BoolVar(&buyoutOnly, "buyout-only", false, "Only show direct-sell auctions (exclude bidding-only listings)")
	return cmd
}

func runSearch(
	ctx context.Context,
	weaponSlug string, positiveStats []string, negativeStats string, sortBy string, reRollsMax int, maxPrice int,
	statusFilter []string, buyoutOnly bool,
	debug bool,
) error {
	marketSvc := adapter.NewHttpWarframeMarketService(debug)
	exportSvc := adapter.NewHttpWarframeExportService()

	statuses := make([]service.UserStatus, 0, len(statusFilter))
	for _, s := range statusFilter {
		statuses = append(statuses, service.UserStatus(s))
	}

	filter := service.AuctionFilter{
		WeaponURLName:  weaponSlug,
		PositiveStats:  positiveStats,
		NegativeStats:  negativeStats,
		SortBy:         sortBy,
		ReRollsMax:     reRollsMax,
		BuyoutPriceMax: maxPrice,
		BuyoutOnly:     buyoutOnly,
		Status:         statuses,
	}

	auctions, err := marketSvc.SearchAuctions(ctx, filter)
	if err != nil {
		return err
	}

	if len(auctions) == 0 {
		fmt.Println("No auctions found.")
		return nil
	}

	// Resolve weapon info (disposition + category) for roll quality calculation.
	// Convert weapon slug (e.g. "gram_prime") to display name ("Gram Prime").
	var weaponInfo *service.WeaponInfo
	weaponName := slugToName(weaponSlug)
	weaponInfo, _ = exportSvc.GetWeaponByName(ctx, weaponName)
	calc := &usecase.RivenCalculator{}

	table := tablewriter.NewTable(os.Stdout, tablewriter.WithRendition(tw.Rendition{
		Settings: tw.Settings{
			Separators: tw.Separators{BetweenRows: tw.On},
		},
	}))
	table.Header([]string{
		"Price (p)", "Riven Name", "Rank", "Re-Rolls", "Positives", "Negative", "Roll %", "Seller", "Status",
	})

	for _, a := range auctions {
		scores := calc.ScoreAuction(a, weaponInfo)
		scoreByURL := make(map[string]usecase.RivenStatScore, len(scores))
		for _, sc := range scores {
			scoreByURL[sc.URLName] = sc
		}

		var pos, neg []string
		var rollSum float64
		var rollCount int
		for _, attr := range a.Item.Attributes {
			label := formatAttr(attr, scoreByURL[attr.URLName])
			if attr.Positive {
				pos = append(pos, label)
				if sc, ok := scoreByURL[attr.URLName]; ok && sc.Known {
					rollSum += sc.RollQuality
					rollCount++
				}
			} else {
				neg = append(neg, label)
			}
		}

		priceStr := fmt.Sprintf("%d", a.BuyoutPrice)
		if !a.IsDirectSell {
			priceStr = fmt.Sprintf("%d (bid)", a.BuyoutPrice)
		}

		rollStr := "N/A"
		if rollCount > 0 {
			rollStr = fmt.Sprintf("%.0f%%", rollSum/float64(rollCount))
		}

		_ = table.Append([]string{
			priceStr,
			a.Item.Name,
			fmt.Sprintf("%d", a.Item.ModRank),
			fmt.Sprintf("%d", a.Item.ReRolls),
			strings.Join(pos, "\n"),
			strings.Join(neg, "\n"),
			rollStr,
			a.Owner.IngameName,
			string(a.Owner.Status),
		})
	}
	_ = table.Render()
	return nil
}

// formatAttr formats a riven attribute with its roll quality if known.
func formatAttr(attr service.RivenAttribute, score usecase.RivenStatScore) string {
	sign := "+"
	if !attr.Positive {
		sign = "-"
	}
	name := strings.ReplaceAll(attr.URLName, "_", " ")
	name = strings.Title(name) //nolint:staticcheck
	base := fmt.Sprintf("%s%.1f%% %s", sign, abs(attr.Value), name)
	if score.Known {
		if attr.Positive {
			return fmt.Sprintf("%s (%.0f%%)", base, score.RollQuality)
		}
		// For negatives: invert so 100% = weakest (best), 0% = strongest (worst)
		return fmt.Sprintf("%s (%.0f%%)", base, 100-score.RollQuality)
	}
	return base
}

func abs(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}

// slugToName converts a weapon url_name slug to a display name.
// e.g. "gram_prime" → "Gram Prime", "falcor" → "Falcor"
func slugToName(slug string) string {
	words := strings.Split(slug, "_")
	for i, w := range words {
		if len(w) > 0 {
			words[i] = strings.ToUpper(w[:1]) + w[1:]
		}
	}
	return strings.Join(words, " ")
}
