package service

import "context"

// Recipe represents a craftable item and its required ingredients.
type Recipe struct {
	UniqueName  string
	ResultType  string
	Ingredients []RecipeIngredient
}

// RecipeIngredient is a single ingredient in a recipe with its required quantity.
type RecipeIngredient struct {
	ItemType  string
	ItemCount int
}

// WarframeExportService defines the port for the official Warframe public export data.
type WarframeExportService interface {
	// GetRecipes returns all crafting recipes from the public export.
	GetRecipes(ctx context.Context) ([]Recipe, error)

	// GetRecipeByResultType returns the recipe that produces the given item uniqueName.
	// Returns nil if no recipe is found.
	GetRecipeByResultType(ctx context.Context, resultType string) (*Recipe, error)
}
