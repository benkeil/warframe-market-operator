package usecase

import (
	"context"
	"fmt"
	"slices"

	"github.com/benkeil/warframe-market-operator/internal/domain/service"
)

// BuyOrderLine is one line in the buy plan for a set part: buy Quantity units from Seller at Price each.
type BuyOrderLine struct {
	Seller   service.OrderUser
	Quantity int
	Price    int
	Subtotal int
}

// PartPrice holds the buy plan for one set part and the total cost.
type PartPrice struct {
	Part      SetPart
	BuyOrders []BuyOrderLine
	Total     int
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

// Execute prices all components of the given set and returns a buy plan per part.
func (uc *GetSetComponentPriceUseCase) Execute(ctx context.Context, slug string, filter service.OrdersFilter) (*SetComponentPrice, error) {
	info, err := uc.getSetInfo.Execute(ctx, slug)
	if err != nil {
		return nil, err
	}

	sellType := service.OrderTypeSell
	partFilter := service.OrdersFilter{
		Platform:      filter.Platform,
		Crossplay:     filter.Crossplay,
		Type:          &sellType,
		Status:        []service.UserStatus{service.UserStatusIngame, service.UserStatusOnline},
		SortBy:        service.SortByPlatinum,
		SortDirection: service.SortAsc,
	}

	result := &SetComponentPrice{SetName: info.Name, SetSlug: info.Slug}

	for _, part := range info.Parts {
		orders, err := uc.marketService.GetOrdersByItem(ctx, part.Slug, partFilter)
		if err != nil {
			return nil, fmt.Errorf("fetching orders for %q: %w", part.Slug, err)
		}
		lines := buildBuyPlan(orders, part.Count)
		total := 0
		for _, l := range lines {
			total += l.Subtotal
		}
		result.Parts = append(result.Parts, PartPrice{
			Part:      part,
			BuyOrders: lines,
			Total:     total,
		})
		result.TotalPlatinum += total
	}

	return result, nil
}

// buildBuyPlan greedily fills `need` units from the cheapest sell orders.
func buildBuyPlan(orders []service.Order, need int) []BuyOrderLine {
	sorted := slices.SortedFunc(slices.Values(orders), func(a, b service.Order) int {
		return a.Platinum - b.Platinum
	})
	var lines []BuyOrderLine
	for _, o := range sorted {
		if need <= 0 {
			break
		}
		take := o.Quantity
		if take > need {
			take = need
		}
		lines = append(lines, BuyOrderLine{
			Seller:   o.User,
			Quantity: take,
			Price:    o.Platinum,
			Subtotal: take * o.Platinum,
		})
		need -= take
	}
	return lines
}
