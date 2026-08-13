package adapter

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"sort"
	"strconv"

	"github.com/benkeil/warframe-market-operator/internal/domain/service"
)

// HttpWarframeMarketService implements service.WarframeMarketService using the public
// Warframe Market REST API v2 (https://api.warframe.market/v2).
type HttpWarframeMarketService struct {
	baseURL    string
	httpClient *http.Client
	language   string
	userAgent  string
	debug      bool
}

// NewHttpWarframeMarketService creates a new HttpWarframeMarketService.
func NewHttpWarframeMarketService(debug bool) *HttpWarframeMarketService {
	return &HttpWarframeMarketService{
		baseURL:    "https://api.warframe.market/v2",
		httpClient: &http.Client{},
		language:   "en",
		userAgent:  "warframe-market-operator/1.0",
		debug:      debug,
	}
}

// GetItemBySlug fetches detailed information about a single item.
// GET /v2/items/{slug}
func (s *HttpWarframeMarketService) GetItemBySlug(ctx context.Context, slug string) (*service.ItemDetail, error) {
	var result apiResponse[itemDetailAPIResponse]
	if err := s.makeRequest(ctx, fmt.Sprintf("/items/%s", slug), nil, &result); err != nil {
		return nil, fmt.Errorf("getting item %q: %w", slug, err)
	}
	name := result.Data.Slug
	if locale, ok := result.Data.I18n["en"]; ok {
		name = locale.Name
	}
	return &service.ItemDetail{
		ID:             result.Data.ID,
		Slug:           result.Data.Slug,
		GameRef:        result.Data.GameRef,
		Name:           name,
		Tags:           result.Data.Tags,
		SetRoot:        result.Data.SetRoot,
		SetParts:       result.Data.SetParts,
		Ducats:         result.Data.Ducats,
		ReqMasteryRank: result.Data.ReqMasteryRank,
		TradingTax:     result.Data.TradingTax,
	}, nil
}

type itemDetailAPIResponse struct {
	ID             string                    `json:"id"`
	Slug           string                    `json:"slug"`
	GameRef        string                    `json:"gameRef"`
	Tags           []string                  `json:"tags"`
	SetRoot        bool                      `json:"setRoot"`
	SetParts       []string                  `json:"setParts"`
	Ducats         int                       `json:"ducats"`
	ReqMasteryRank int                       `json:"reqMasteryRank"`
	TradingTax     int                       `json:"tradingTax"`
	I18n           map[string]ItemI18nLocale `json:"i18n"`
}

type ItemI18nLocale struct {
	Name string `json:"name"`
}

func (s *HttpWarframeMarketService) GetItems(ctx context.Context) ([]service.Item, error) {
	var result apiResponse[[]service.ItemAPIResponse]
	if err := s.makeRequest(ctx, "/items", nil, &result); err != nil {
		return nil, fmt.Errorf("getting items: %w", err)
	}

	items := make([]service.Item, 0, len(result.Data))
	for _, raw := range result.Data {
		name := raw.Slug
		if locale, ok := raw.I18n["en"]; ok {
			name = locale.Name
		}
		items = append(items, service.Item{ID: raw.ID, Slug: raw.Slug, Name: name})
	}
	return items, nil
}

// GetOrdersByItem fetches all visible orders for an item.
// Platform and crossplay are sent as per-request headers. Type and status are filtered client-side.
// GET /v2/orders/item/{slug}
func (s *HttpWarframeMarketService) GetOrdersByItem(ctx context.Context, slug string, filter service.OrdersFilter) ([]service.Order, error) {
	h := &requestHeaders{platform: filter.Platform, crossplay: filter.Crossplay}
	var result apiResponse[[]service.Order]
	if err := s.makeRequest(ctx, fmt.Sprintf("/orders/item/%s", slug), h, &result); err != nil {
		return nil, fmt.Errorf("getting orders for %q: %w", slug, err)
	}

	orders := applyFilter(result.Data, filter)
	applySort(orders, filter.SortBy, filter.SortDirection)
	return orders, nil
}

// GetTopOrdersByItem fetches the top buy and sell orders from online users (up to 5 each).
// Platform and Crossplay from filter are sent as request headers.
// GET /v2/orders/item/{slug}/top
func (s *HttpWarframeMarketService) GetTopOrdersByItem(ctx context.Context, slug string, filter service.OrdersFilter) (*service.TopOrders, error) {
	h := &requestHeaders{platform: filter.Platform, crossplay: filter.Crossplay}
	var result apiResponse[service.TopOrders]
	if err := s.makeRequest(ctx, fmt.Sprintf("/orders/item/%s/top", slug), h, &result); err != nil {
		return nil, fmt.Errorf("getting top orders for %q: %w", slug, err)
	}
	return &result.Data, nil
}

func applyFilter(orders []service.Order, f service.OrdersFilter) []service.Order {
	if f.Type == nil && len(f.Status) == 0 {
		return orders
	}
	result := make([]service.Order, 0, len(orders))
	for _, o := range orders {
		if f.Type != nil && o.Type != *f.Type {
			continue
		}
		if len(f.Status) > 0 && !containsStatus(f.Status, o.User.Status) {
			continue
		}
		result = append(result, o)
	}
	return result
}

func containsStatus(statuses []service.UserStatus, s service.UserStatus) bool {
	for _, v := range statuses {
		if v == s {
			return true
		}
	}
	return false
}

func applySort(orders []service.Order, by service.SortField, dir service.SortDirection) {
	if by == "" {
		return
	}
	sort.SliceStable(orders, func(i, j int) bool {
		var less bool
		switch by {
		case service.SortByQuantity:
			less = orders[i].Quantity < orders[j].Quantity
		case service.SortByReputation:
			less = orders[i].User.Reputation < orders[j].User.Reputation
		default:
			less = orders[i].Platinum < orders[j].Platinum
		}
		if dir == service.SortDesc {
			return !less
		}
		return less
	})
}

// requestHeaders holds per-request header overrides for platform-aware endpoints.
type requestHeaders struct {
	platform  service.Platform
	crossplay bool
}

func (s *HttpWarframeMarketService) makeRequest(ctx context.Context, endpoint string, headers *requestHeaders, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.baseURL+endpoint, nil)
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("language", s.language)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", s.userAgent)

	if headers != nil {
		if headers.platform != "" {
			req.Header.Set("platform", string(headers.platform))
		}
		req.Header.Set("crossplay", strconv.FormatBool(headers.crossplay))
	}

	if s.debug {
		fmt.Fprintf(os.Stderr, "> %s %s\n", req.Method, req.URL)
		keys := make([]string, 0, len(req.Header))
		for k := range req.Header {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			fmt.Fprintf(os.Stderr, "> %s: %s\n", k, req.Header.Get(k))
		}
		fmt.Fprintln(os.Stderr)
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("executing request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status %d from %s", resp.StatusCode, endpoint)
	}

	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("decoding response: %w", err)
	}
	return nil
}

type apiResponse[T any] struct {
	APIVersion string `json:"apiVersion"`
	Data       T      `json:"data"`
}
