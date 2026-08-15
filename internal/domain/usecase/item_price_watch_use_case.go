package usecase

import (
	"cmp"
	"context"
	"fmt"
	"slices"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientgov1 "k8s.io/client-go/applyconfigurations/meta/v1"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	warframemarketv1alpha1 "github.com/benkeil/warframe-market-operator/api/v1alpha1"
	v1alpha1ac "github.com/benkeil/warframe-market-operator/internal/applyconfiguration/api/v1alpha1"
	"github.com/benkeil/warframe-market-operator/internal/domain/service"
)

const conditionTypePriceSynced = "PriceSynced"

// ItemPriceWatchResult is the output of ItemPriceWatchUseCase.Execute.
// The use case does not mutate the input.
type ItemPriceWatchResult struct {
	Status *v1alpha1ac.ItemPriceWatchStatusApplyConfiguration
}

// ItemPriceWatchUseCase fetches the top sell orders for the item configured in the ItemPriceWatch spec,
// writes the cheapest price into its status, and sends a notification when the price reaches
// or drops below the configured threshold for the first time — or when the cheapest seller changes.
type ItemPriceWatchUseCase struct {
	marketService       service.WarframeMarketService
	notificationService service.NotificationService
}

// NewItemPriceWatchUseCase creates a new ItemPriceWatchUseCase.
func NewItemPriceWatchUseCase(marketService service.WarframeMarketService, notificationService service.NotificationService) *ItemPriceWatchUseCase {
	return &ItemPriceWatchUseCase{
		marketService:       marketService,
		notificationService: notificationService,
	}
}

// Execute fetches the top sell orders for the item in priceWatch.Spec and returns the desired
// status to apply. It does not mutate priceWatch.
func (uc *ItemPriceWatchUseCase) Execute(ctx context.Context, priceWatch *warframemarketv1alpha1.ItemPriceWatch) (*ItemPriceWatchResult, error) {
	log := logf.FromContext(ctx).WithValues("item", priceWatch.Spec.ItemSlug, "threshold", priceWatch.Spec.Threshold)

	status := v1alpha1ac.ItemPriceWatchStatus()
	condition := clientgov1.Condition().
		WithType(conditionTypePriceSynced).
		WithObservedGeneration(priceWatch.Generation).
		WithLastTransitionTime(metav1.Now())

	log.Info("fetching top orders")
	topOrders, err := uc.marketService.GetTopOrdersByItem(ctx, priceWatch.Spec.ItemSlug, service.OrdersFilter{})
	if err != nil {
		log.Error(err, "failed to fetch top orders")
		status.WithConditions(condition.
			WithStatus(metav1.ConditionFalse).
			WithReason("FetchFailed").
			WithMessage(fmt.Sprintf("Failed to fetch top orders: %v", err)))
		return &ItemPriceWatchResult{Status: status}, err
	}

	if len(topOrders.Sell) == 0 {
		log.Info("no sell orders found")
		status.WithNotifiedOrderID("").WithConditions(condition.
			WithStatus(metav1.ConditionFalse).
			WithReason("NoSellOrders").
			WithMessage(fmt.Sprintf("No sell orders found for item %q", priceWatch.Spec.ItemSlug)))
		return &ItemPriceWatchResult{Status: status}, nil
	}

	cheapestOrder := minOrder(topOrders.Sell)
	log.Info("price check", "cheapest", cheapestOrder.Platinum)
	status.WithCheapestPrice(cheapestOrder.Platinum)
	condition.
		WithStatus(metav1.ConditionTrue).
		WithReason("PriceFetched").
		WithMessage(fmt.Sprintf("Cheapest sell price is %d platinum", cheapestOrder.Platinum))

	if cheapestOrder.Platinum > priceWatch.Spec.Threshold {
		status.WithNotifiedOrderID("").WithConditions(condition)
		log.Info("notification skipped", "reason", "price above threshold")
		return &ItemPriceWatchResult{Status: status}, nil
	}

	// Read current notified order ID from existing status — input is not mutated.
	notifiedOrderID := priceWatch.Status.NotifiedOrderID
	if notifiedOrderID != "" {
		stillVisible := false
		for _, o := range topOrders.Sell {
			if o.ID == notifiedOrderID {
				stillVisible = true
				break
			}
		}
		if !stillVisible {
			log.Info("previously notified seller is offline, resetting")
			notifiedOrderID = ""
		}
	}

	if notifiedOrderID == cheapestOrder.ID {
		log.Info("notification skipped", "reason", "no new low")
		status.WithNotifiedOrderID(notifiedOrderID).WithConditions(condition)
		return &ItemPriceWatchResult{Status: status}, nil
	}

	if priceWatch.Spec.NotificationWindow != nil && !isWithinWindow(time.Now(), priceWatch.Spec.NotificationWindow.From, priceWatch.Spec.NotificationWindow.To) {
		log.Info("notification skipped", "reason", "outside notification window")
		status.WithNotifiedOrderID(notifiedOrderID).WithConditions(condition)
		return &ItemPriceWatchResult{Status: status}, nil
	}

	log.Info("sending notification", "cheapest", cheapestOrder.Platinum, "seller", cheapestOrder.User.IngameName)
	title := fmt.Sprintf("Price alert: %s", priceWatch.Spec.ItemSlug)
	message := fmt.Sprintf("%d platinum | seller: %s (%s) | threshold: %d",
		cheapestOrder.Platinum, cheapestOrder.User.IngameName, cheapestOrder.User.Status, priceWatch.Spec.Threshold)
	if err := uc.notificationService.Notify(ctx, title, message); err != nil {
		log.Error(err, "failed to send notification")
		status.WithNotifiedOrderID(notifiedOrderID).WithConditions(condition)
		return &ItemPriceWatchResult{Status: status}, fmt.Errorf("sending notification: %w", err)
	}

	now := metav1.Now()
	status.WithNotifiedOrderID(cheapestOrder.ID).WithLastNotifiedAt(now).WithConditions(condition)
	log.Info("notification sent", "cheapest", cheapestOrder.Platinum)
	return &ItemPriceWatchResult{Status: status}, nil
}

// isWithinWindow reports whether t falls within the [from, to] time-of-day range.
// Both from and to are in "HH:MM" format. The window is treated as same-day only
// (crossing midnight is not supported).
func isWithinWindow(t time.Time, from, to string) bool {
	parseHHMM := func(s string) (int, int) {
		var h, m int
		_, _ = fmt.Sscanf(s, "%d:%d", &h, &m)
		return h, m
	}
	fh, fm := parseHHMM(from)
	th, tm := parseHHMM(to)
	cur := t.Hour()*60 + t.Minute()
	return cur >= fh*60+fm && cur <= th*60+tm
}

func minOrder(orders []service.Order) service.Order {
	return slices.MinFunc(orders, func(a, b service.Order) int {
		return cmp.Compare(a.Platinum, b.Platinum)
	})
}
