package service

import "context"

// Platform represents the game platform.
type Platform string

const (
	PlatformPC     Platform = "pc"
	PlatformPS4    Platform = "ps4"
	PlatformXbox   Platform = "xbox"
	PlatformSwitch Platform = "switch"
)

// UserStatus represents the online status of a user.
type UserStatus string

const (
	UserStatusIngame  UserStatus = "ingame"
	UserStatusOnline  UserStatus = "online"
	UserStatusOffline UserStatus = "offline"
)

// OrderType represents whether an order is a buy or sell offer.
type OrderType string

const (
	OrderTypeSell OrderType = "sell"
	OrderTypeBuy  OrderType = "buy"
)

// SortField is the field to sort orders by.
type SortField string

const (
	SortByPlatinum   SortField = "platinum"
	SortByQuantity   SortField = "quantity"
	SortByReputation SortField = "reputation"
)

// SortDirection is the direction to sort orders.
type SortDirection string

const (
	SortAsc  SortDirection = "asc"
	SortDesc SortDirection = "desc"
)

// OrdersFilter holds filters and sorting for GetOrdersByItem.
type OrdersFilter struct {
	// Platform filters orders server-side via the Platform header.
	Platform Platform
	// Crossplay filters orders server-side via the Crossplay header.
	Crossplay bool
	// Type filters orders to only "sell" or "buy". Nil means both.
	Type *OrderType
	// Status filters orders by the user's online status. Nil means all statuses.
	Status []UserStatus
	// SortBy is the field to sort by. Empty means default API ordering.
	SortBy SortField
	// SortDirection is the sort direction (asc/desc). Defaults to asc.
	SortDirection SortDirection
}

// WarframeMarketService defines the port for interacting with the Warframe Market API.
type WarframeMarketService interface {
	// GetItems returns all tradable items. Use for client-side search by name or slug.
	GetItems(ctx context.Context) ([]Item, error)

	// GetOrdersByItem returns all visible orders for an item with optional filtering and sorting.
	// The slug is the item's URL-friendly identifier (e.g. "ash_prime_set").
	GetOrdersByItem(ctx context.Context, slug string, filter OrdersFilter) ([]Order, error)

	// GetTopOrdersByItem returns the top buy and sell orders for an item (up to 5 each),
	// restricted to currently online users. The slug is the item's URL-friendly identifier.
	// Platform and Crossplay from filter are sent as request headers.
	GetTopOrdersByItem(ctx context.Context, slug string, filter OrdersFilter) (*TopOrders, error)
}

// Item represents a tradable item on Warframe Market.
type Item struct {
	ID   string `json:"id"`
	Slug string `json:"slug"`
	Name string // populated from i18n.en.name
}

// itemAPIResponse is the raw API shape for an item (i18n is locale-keyed).
type ItemAPIResponse struct {
	ID   string                    `json:"id"`
	Slug string                    `json:"slug"`
	I18n map[string]ItemI18nLocale `json:"i18n"`
}

// ItemI18nLocale holds the localised fields for an item.
type ItemI18nLocale struct {
	Name string `json:"name"`
}

// TopOrders holds the top buy and sell orders for an item from online users.
type TopOrders struct {
	Sell []Order `json:"sell"`
	Buy  []Order `json:"buy"`
}

// Order represents a single buy or sell order on Warframe Market.
type Order struct {
	ID       string    `json:"id"`
	Type     OrderType `json:"type"`
	Platinum int       `json:"platinum"`
	Quantity int       `json:"quantity"`
	User     OrderUser `json:"user"`
}

// OrderUser contains information about the user who placed an order.
type OrderUser struct {
	Slug       string     `json:"slug"`
	IngameName string     `json:"ingameName"`
	Status     UserStatus `json:"status"`
	Platform   Platform   `json:"platform"`
	Crossplay  bool       `json:"crossplay"`
	Reputation int        `json:"reputation"`
}
