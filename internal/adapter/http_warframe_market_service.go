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
	baseURLv1  string
	httpClient *http.Client
	language   string
	userAgent  string
	debug      bool
}

// NewHttpWarframeMarketService creates a new HttpWarframeMarketService.
func NewHttpWarframeMarketService(debug bool) *HttpWarframeMarketService {
	return &HttpWarframeMarketService{
		baseURL:    "https://api.warframe.market/v2",
		baseURLv1:  "https://api.warframe.market/v1",
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
	return s.makeRequestToURL(ctx, s.baseURL+endpoint, headers, out)
}

func (s *HttpWarframeMarketService) makeRequestV1(ctx context.Context, endpoint string, headers *requestHeaders, out any) error {
	return s.makeRequestToURL(ctx, s.baseURLv1+endpoint, headers, out)
}

func (s *HttpWarframeMarketService) makeRequestToURL(ctx context.Context, url string, headers *requestHeaders, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
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
		return fmt.Errorf("unexpected status %d from %s", resp.StatusCode, url)
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

// SearchAuctions searches for riven auctions with the given filter.
// GET /v1/auctions/search
func (s *HttpWarframeMarketService) SearchAuctions(ctx context.Context, filter service.AuctionFilter) ([]service.Auction, error) {
	endpoint := fmt.Sprintf("/auctions/search?type=riven&weapon_url_name=%s&sort_by=%s",
		filter.WeaponURLName, filter.SortBy)
	if len(filter.PositiveStats) > 0 {
		endpoint += "&positive_stats=" + joinStrings(filter.PositiveStats)
	}
	if filter.NegativeStats != "" {
		endpoint += "&negative_stats=" + filter.NegativeStats
	}
	if filter.ReRollsMax > 0 {
		endpoint += fmt.Sprintf("&re_rolls_max=%d", filter.ReRollsMax)
	}

	var result auctionSearchAPIResponse
	if err := s.makeRequestV1(ctx, endpoint, nil, &result); err != nil {
		return nil, fmt.Errorf("searching auctions: %w", err)
	}

	auctions := make([]service.Auction, 0, len(result.Payload.Auctions))
	for _, a := range result.Payload.Auctions {
		status := service.UserStatus(a.Owner.Status)
		if len(filter.Status) > 0 && !containsStatus(filter.Status, status) {
			continue
		}
		if filter.BuyoutOnly && !a.IsDirectSell {
			continue
		}
		if filter.BuyoutPriceMax > 0 && a.BuyoutPrice > filter.BuyoutPriceMax {
			continue
		}
		attrs := make([]service.RivenAttribute, 0, len(a.Item.Attributes))
		for _, attr := range a.Item.Attributes {
			attrs = append(attrs, service.RivenAttribute{
				URLName:  attr.URLName,
				Value:    attr.Value,
				Positive: attr.Positive,
			})
		}
		auctions = append(auctions, service.Auction{
			ID:           a.ID,
			BuyoutPrice:  a.BuyoutPrice,
			IsDirectSell: a.IsDirectSell,
			Owner: service.AuctionOwner{
				IngameName: a.Owner.IngameName,
				Slug:       a.Owner.Slug,
				Status:     status,
				Reputation: a.Owner.Reputation,
			},
			Item: service.RivenItem{
				Name:          a.Item.Name,
				WeaponURLName: a.Item.WeaponURLName,
				ModRank:       a.Item.ModRank,
				ReRolls:       a.Item.ReRolls,
				MasteryLevel:  a.Item.MasteryLevel,
				Polarity:      a.Item.Polarity,
				Attributes:    attrs,
			},
		})
	}
	return auctions, nil
}

func joinStrings(ss []string) string {
	result := ""
	for i, s := range ss {
		if i > 0 {
			result += ","
		}
		result += s
	}
	return result
}

type auctionSearchAPIResponse struct {
	Payload struct {
		Auctions []auctionAPIItem `json:"auctions"`
	} `json:"payload"`
}

type auctionAPIItem struct {
	ID           string          `json:"id"`
	BuyoutPrice  int             `json:"buyout_price"`
	IsDirectSell bool            `json:"is_direct_sell"`
	Owner        auctionOwnerAPI `json:"owner"`
	Item         rivenItemAPI    `json:"item"`
}

type auctionOwnerAPI struct {
	IngameName string `json:"ingame_name"`
	Slug       string `json:"slug"`
	Status     string `json:"status"`
	Reputation int    `json:"reputation"`
}

type rivenItemAPI struct {
	Name          string              `json:"name"`
	WeaponURLName string              `json:"weapon_url_name"`
	ModRank       int                 `json:"mod_rank"`
	ReRolls       int                 `json:"re_rolls"`
	MasteryLevel  int                 `json:"mastery_level"`
	Polarity      string              `json:"polarity"`
	Attributes    []rivenAttributeAPI `json:"attributes"`
}

type rivenAttributeAPI struct {
	URLName  string  `json:"url_name"`
	Value    float64 `json:"value"`
	Positive bool    `json:"positive"`
}
