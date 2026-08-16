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

	// GetWeaponByGameRef returns weapon metadata (disposition, category) for a given gameRef.
	// Returns nil if the weapon is not found.
	GetWeaponByGameRef(ctx context.Context, gameRef string) (*WeaponInfo, error)

	// GetWeaponByName returns weapon metadata (disposition, category) by display name (case-insensitive).
	// Returns nil if the weapon is not found.
	GetWeaponByName(ctx context.Context, name string) (*WeaponInfo, error)
}

// WeaponInfo holds riven-relevant metadata for a weapon.
type WeaponInfo struct {
	// The name of the weapon as used in the Warframe public export (e.g. "MK1-Paris").
	Name string
	// Disposition is the riven disposition multiplier (omegaAttenuation), 0.5–1.55.
	Disposition float64
	// Category is the product category (e.g. "Melee", "Pistols", "Primary", "Shotguns").
	Category string
}
