package adapter

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/ulikunitz/xz/lzma"

	"github.com/benkeil/warframe-market-operator/internal/domain/service"
)

const exportIndexURL = "https://origin.warframe.com/PublicExport/index_en.txt.lzma"
const exportBaseURL = "https://content.warframe.com/PublicExport/Manifest"

// HttpWarframeExportService implements service.WarframeExportService using the
// official Warframe public export data (https://content.warframe.com/PublicExport).
type HttpWarframeExportService struct {
	httpClient *http.Client
	userAgent  string
}

// NewHttpWarframeExportService creates a new HttpWarframeExportService.
func NewHttpWarframeExportService() *HttpWarframeExportService {
	return &HttpWarframeExportService{
		httpClient: &http.Client{},
		userAgent:  "warframe-market-operator/1.0",
	}
}

// GetRecipes returns all crafting recipes from the public export.
func (s *HttpWarframeExportService) GetRecipes(ctx context.Context) ([]service.Recipe, error) {
	filename, err := s.resolveFilename(ctx, "ExportRecipes_en.json")
	if err != nil {
		return nil, err
	}

	var raw exportRecipesResponse
	if err := s.fetchJSON(ctx, fmt.Sprintf("%s/%s", exportBaseURL, filename), &raw); err != nil {
		return nil, fmt.Errorf("fetching recipes: %w", err)
	}

	recipes := make([]service.Recipe, 0, len(raw.ExportRecipes))
	for _, r := range raw.ExportRecipes {
		ingredients := make([]service.RecipeIngredient, 0, len(r.Ingredients))
		for _, ing := range r.Ingredients {
			ingredients = append(ingredients, service.RecipeIngredient{
				ItemType:  ing.ItemType,
				ItemCount: ing.ItemCount,
			})
		}
		recipes = append(recipes, service.Recipe{
			UniqueName:  r.UniqueName,
			ResultType:  r.ResultType,
			Ingredients: ingredients,
		})
	}
	return recipes, nil
}

// GetRecipeByResultType returns the recipe that produces the given item uniqueName.
func (s *HttpWarframeExportService) GetRecipeByResultType(ctx context.Context, resultType string) (*service.Recipe, error) {
	recipes, err := s.GetRecipes(ctx)
	if err != nil {
		return nil, err
	}
	for _, r := range recipes {
		if r.ResultType == resultType {
			return &r, nil
		}
	}
	return nil, nil
}

// resolveFilename fetches the LZMA-compressed export index and returns the
// versioned filename for the given base name
// (e.g. "ExportRecipes_en.json" → "ExportRecipes_en.json!00_xxxx").
func (s *HttpWarframeExportService) resolveFilename(ctx context.Context, base string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, exportIndexURL, nil)
	if err != nil {
		return "", fmt.Errorf("creating index request: %w", err)
	}
	req.Header.Set("User-Agent", s.userAgent)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("fetching export index: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("export index returned status %d", resp.StatusCode)
	}

	r, err := lzma.NewReader(resp.Body)
	if err != nil {
		return "", fmt.Errorf("creating lzma reader: %w", err)
	}
	body, err := io.ReadAll(r)
	if err != nil {
		return "", fmt.Errorf("reading export index: %w", err)
	}

	for _, line := range strings.Split(string(body), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, base+"!") {
			return line, nil
		}
	}
	return "", fmt.Errorf("file %q not found in export index", base)
}

func (s *HttpWarframeExportService) fetchJSON(ctx context.Context, url string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("User-Agent", s.userAgent)
	req.Header.Set("Accept", "application/json")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("executing request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status %d from %s", resp.StatusCode, url)
	}

	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("decoding response: %w", err)
	}
	return nil
}

type exportRecipesResponse struct {
	ExportRecipes []exportRecipe `json:"ExportRecipes"`
}

type exportRecipe struct {
	UniqueName  string             `json:"uniqueName"`
	ResultType  string             `json:"resultType"`
	Ingredients []exportIngredient `json:"ingredients"`
}

type exportIngredient struct {
	ItemType  string `json:"ItemType"`
	ItemCount int    `json:"ItemCount"`
}

// GetWeaponByGameRef returns weapon metadata for a given gameRef (uniqueName) from the DE public export.
func (s *HttpWarframeExportService) GetWeaponByGameRef(ctx context.Context, gameRef string) (*service.WeaponInfo, error) {
	filename, err := s.resolveFilename(ctx, "ExportWeapons_en.json")
	if err != nil {
		return nil, err
	}
	var raw exportWeaponsResponse
	if err := s.fetchJSON(ctx, fmt.Sprintf("%s/%s", exportBaseURL, filename), &raw); err != nil {
		return nil, fmt.Errorf("fetching weapons export: %w", err)
	}
	for _, w := range raw.ExportWeapons {
		if w.UniqueName == gameRef {
			return &service.WeaponInfo{
				Disposition: w.OmegaAttenuation,
				Category:    w.ProductCategory,
			}, nil
		}
	}
	return nil, nil
}
func (s *HttpWarframeExportService) GetWeaponByName(ctx context.Context, name string) (*service.WeaponInfo, error) {
	filename, err := s.resolveFilename(ctx, "ExportWeapons_en.json")
	if err != nil {
		return nil, err
	}
	var raw exportWeaponsResponse
	if err := s.fetchJSON(ctx, fmt.Sprintf("%s/%s", exportBaseURL, filename), &raw); err != nil {
		return nil, fmt.Errorf("fetching weapons export: %w", err)
	}
	nameLower := strings.ToLower(name)
	for _, w := range raw.ExportWeapons {
		if strings.ToLower(w.Name) == nameLower {
			return &service.WeaponInfo{
				Name:        w.Name,
				Disposition: w.OmegaAttenuation,
				Category:    w.ProductCategory,
			}, nil
		}
	}
	return nil, nil
}

type exportWeaponsResponse struct {
	ExportWeapons []exportWeaponDE `json:"ExportWeapons"`
}

type exportWeaponDE struct {
	Name             string  `json:"name"`
	UniqueName       string  `json:"uniqueName"`
	ProductCategory  string  `json:"productCategory"`
	OmegaAttenuation float64 `json:"omegaAttenuation"`
}
