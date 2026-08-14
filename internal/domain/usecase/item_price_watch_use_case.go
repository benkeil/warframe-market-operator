package usecase

import (
	"cmp"
	"context"
	"fmt"
	"slices"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	warframemarketv1alpha1 "github.com/benkeil/warframe-market-operator/api/v1alpha1"
	"github.com/benkeil/warframe-market-operator/internal/domain/service"
)

const conditionTypePriceSynced = "PriceSynced"

// ItemPriceWatchUseCase fetches the top sell orders for the item configured in the ItemPriceWatch spec,
// writes the cheapest price into its status, and sends a notification when the price reaches
// or drops below the configured threshold for the first time — or drops below the price
// at which the last notification was sent.
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

// Execute fetches the top sell orders for the item in priceWatch.Spec and mutates
// priceWatch.Status with the cheapest price and a PriceSynced condition.
// A notification is sent when the cheapest seller changes (new seller or previously
// offline seller returning online), provided price is at or below the threshold.
func (uc *ItemPriceWatchUseCase) Execute(ctx context.Context, priceWatch *warframemarketv1alpha1.ItemPriceWatch) error {
	log := logf.FromContext(ctx).WithValues("item", priceWatch.Spec.ItemSlug, "threshold", priceWatch.Spec.Threshold)

	log.Info("fetching top orders")
	topOrders, err := uc.marketService.GetTopOrdersByItem(ctx, priceWatch.Spec.ItemSlug, service.OrdersFilter{})

	condition := metav1.Condition{
		Type:               conditionTypePriceSynced,
		ObservedGeneration: priceWatch.Generation,
		LastTransitionTime: metav1.Now(),
	}

	if err != nil {
		log.Error(err, "failed to fetch top orders")
		condition.Status = metav1.ConditionFalse
		condition.Reason = "FetchFailed"
		condition.Message = fmt.Sprintf("Failed to fetch top orders: %v", err)
		setCondition(&priceWatch.Status.Conditions, condition)
		return err
	}

	if len(topOrders.Sell) == 0 {
		log.Info("no sell orders found")
		priceWatch.Status.NotifiedOrderID = ""
		condition.Status = metav1.ConditionFalse
		condition.Reason = "NoSellOrders"
		condition.Message = fmt.Sprintf("No sell orders found for item %q", priceWatch.Spec.ItemSlug)
		setCondition(&priceWatch.Status.Conditions, condition)
		return nil
	}

	cheapestOrder := minOrder(topOrders.Sell)
	priceWatch.Status.CheapestPrice = cheapestOrder.Platinum
	log.Info("price check", "cheapest", cheapestOrder.Platinum)

	condition.Status = metav1.ConditionTrue
	condition.Reason = "PriceFetched"
	condition.Message = fmt.Sprintf("Cheapest sell price is %d platinum", cheapestOrder.Platinum)
	setCondition(&priceWatch.Status.Conditions, condition)

	// If price is above threshold, reset so we re-notify when it drops again.
	if cheapestOrder.Platinum > priceWatch.Spec.Threshold {
		priceWatch.Status.NotifiedOrderID = ""
		log.Info("notification skipped", "reason", "price above threshold")
		return nil
	}

	// Check whether the previously notified order is still visible (seller still online).
	// If not, the seller went offline — clear the ID so we re-notify when they return.
	if priceWatch.Status.NotifiedOrderID != "" {
		stillVisible := false
		for _, o := range topOrders.Sell {
			if o.ID == priceWatch.Status.NotifiedOrderID {
				stillVisible = true
				break
			}
		}
		if !stillVisible {
			log.Info("previously notified seller is offline, resetting")
			priceWatch.Status.NotifiedOrderID = ""
		}
	}

	// Already notified for this exact order (seller still online and cheapest).
	if priceWatch.Status.NotifiedOrderID == cheapestOrder.ID {
		log.Info("notification skipped", "reason", "no new low")
		return nil
	}

	if priceWatch.Spec.NotificationWindow != nil && !isWithinWindow(time.Now(), priceWatch.Spec.NotificationWindow.From, priceWatch.Spec.NotificationWindow.To) {
		log.Info("notification skipped", "reason", "outside notification window")
		return nil
	}

	log.Info("sending notification", "cheapest", cheapestOrder.Platinum, "seller", cheapestOrder.User.IngameName)
	title := fmt.Sprintf("Price alert: %s", priceWatch.Spec.ItemSlug)
	message := fmt.Sprintf("%d platinum | seller: %s (%s) | threshold: %d",
		cheapestOrder.Platinum, cheapestOrder.User.IngameName, cheapestOrder.User.Status, priceWatch.Spec.Threshold)
	if err := uc.notificationService.Notify(ctx, title, message); err != nil {
		log.Error(err, "failed to send notification")
		return fmt.Errorf("sending notification: %w", err)
	}
	now := metav1.Now()
	priceWatch.Status.NotifiedOrderID = cheapestOrder.ID
	priceWatch.Status.LastNotifiedAt = &now
	log.Info("notification sent", "cheapest", cheapestOrder.Platinum)

	return nil
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

// setCondition upserts a condition into the slice (matched by Type).
func setCondition(conditions *[]metav1.Condition, newCondition metav1.Condition) {
	for i, existing := range *conditions {
		if existing.Type == newCondition.Type {
			(*conditions)[i] = newCondition
			return
		}
	}
	*conditions = append(*conditions, newCondition)
}
