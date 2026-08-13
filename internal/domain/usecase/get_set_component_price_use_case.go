package usecase

import (
	"context"
	"fmt"
	"slices"

	"github.com/benkeil/warframe-market-operator/internal/domain/service"
)

// PartPrice holds the cheapest sell price found for a set part and the total cost (price × count).
type PartPrice struct {
	Part             SetPart
	CheapestPlatinum int
	TotalPlatinum    int
}

// SetComponentPrice is the result of pricing a set via individual components.
type SetComponentPrice struct {
	SetName       string
	SetSlug       string
	Parts         []PartPrice
	TotalPlatinum int
}

// GetSetComponentPriceUseCase calculates the total platinum cost of buying all set parts
// individually at the current cheapest sell prices.
type GetSetComponentPriceUseCase struct {
	getSetInfo    *GetSetInfoUseCase
	marketService service.WarframeMarketService
}

// NewGetSetComponentPriceUseCase creates a new GetSetComponentPriceUseCase.
func NewGetSetComponentPriceUseCase(getSetInfo *GetSetInfoUseCase, marketSvc service.WarframeMarketService) *GetSetComponentPriceUseCase {
	return &GetSetComponentPriceUseCase{
		getSetInfo:    getSetInfo,
		marketService: marketSvc,
	}
}

// Execute prices all components of the given set and returns the totals.
func (uc *GetSetComponentPriceUseCase) Execute(ctx context.Context, slug string, filter service.OrdersFilter) (*SetComponentPrice, error) {
	info, err := uc.getSetInfo.Execute(ctx, slug)
	if err != nil {
		return nil, err
	}

	result := &SetComponentPrice{SetName: info.Name, SetSlug: info.Slug}

	for _, part := range info.Parts {
		orders, err := uc.marketService.GetTopOrdersByItem(ctx, part.Slug, filter)
		if err != nil {
			return nil, fmt.Errorf("fetching orders for %q: %w", part.Slug, err)
		}
		cheapest := cheapestSellPrice(orders.Sell)
		total := cheapest * part.Count
		result.Parts = append(result.Parts, PartPrice{
			Part:             part,
			CheapestPlatinum: cheapest,
			TotalPlatinum:    total,
		})
		result.TotalPlatinum += total
	}

	return result, nil
}

// cheapestSellPrice returns the lowest platinum value among the given sell orders.
// Returns 0 if the slice is empty.
func cheapestSellPrice(orders []service.Order) int {
	if len(orders) == 0 {
		return 0
	}
	return slices.MinFunc(orders, func(a, b service.Order) int {
		return a.Platinum - b.Platinum
	}).Platinum
}
