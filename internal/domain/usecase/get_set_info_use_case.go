package usecase

import (
	"context"
	"fmt"

	"github.com/benkeil/warframe-market-operator/internal/domain/service"
)

// SetPart represents a single part of a set with its required quantity.
type SetPart struct {
	Name  string
	Slug  string
	Count int
}

// SetInfo contains information about a set and its component parts.
type SetInfo struct {
	Name  string
	Slug  string
	Parts []SetPart
}

// GetSetInfoUseCase resolves a set slug into its component parts with quantities
// by combining market item data with the DE public export recipes.
type GetSetInfoUseCase struct {
	marketService service.WarframeMarketService
	exportService service.WarframeExportService
}

// NewGetSetInfoUseCase creates a new GetSetInfoUseCase.
func NewGetSetInfoUseCase(marketSvc service.WarframeMarketService, exportSvc service.WarframeExportService) *GetSetInfoUseCase {
	return &GetSetInfoUseCase{
		marketService: marketSvc,
		exportService: exportSvc,
	}
}

// Execute returns set info (parts + quantities) for the given set slug.
func (uc *GetSetInfoUseCase) Execute(ctx context.Context, slug string) (*SetInfo, error) {
	item, err := uc.marketService.GetItemBySlug(ctx, slug)
	if err != nil {
		return nil, fmt.Errorf("fetching item: %w", err)
	}
	if !item.SetRoot {
		return nil, fmt.Errorf("%q is not a set", slug)
	}

	allItems, err := uc.marketService.GetItems(ctx)
	if err != nil {
		return nil, fmt.Errorf("fetching items: %w", err)
	}
	byID := make(map[string]service.Item, len(allItems))
	for _, i := range allItems {
		byID[i.ID] = i
	}

	recipe, err := uc.exportService.GetRecipeByResultType(ctx, item.GameRef)
	if err != nil {
		return nil, fmt.Errorf("fetching recipe: %w", err)
	}
	countByGameRef := make(map[string]int)
	if recipe != nil {
		for _, ing := range recipe.Ingredients {
			countByGameRef[ing.ItemType] = ing.ItemCount
		}
	}

	var parts []SetPart
	for _, partID := range item.SetParts {
		if partID == item.ID {
			continue // skip the set itself
		}
		part, ok := byID[partID]
		if !ok {
			continue
		}
		detail, err := uc.marketService.GetItemBySlug(ctx, part.Slug)
		if err != nil {
			return nil, fmt.Errorf("fetching part %q: %w", part.Slug, err)
		}
		count := countByGameRef[detail.GameRef]
		if count == 0 {
			count = 1
		}
		parts = append(parts, SetPart{Name: part.Name, Slug: part.Slug, Count: count})
	}

	return &SetInfo{Name: item.Name, Slug: item.Slug, Parts: parts}, nil
}
