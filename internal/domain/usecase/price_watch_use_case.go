package usecase

import (
	"cmp"
	"context"
	"fmt"
	"slices"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	warframemarketv1alpha1 "github.com/benkeil/warframe-market-operator/api/v1alpha1"
	"github.com/benkeil/warframe-market-operator/internal/domain/service"
)

const conditionTypePriceSynced = "PriceSynced"

// PriceWatchUseCase fetches the top sell orders for the item configured in the PriceWatch spec,
// writes the cheapest price into its status, and sends a notification when the price reaches
// or drops below the configured threshold for the first time — or drops below the price
// at which the last notification was sent.
type PriceWatchUseCase struct {
	marketService       service.WarframeMarketService
	notificationService service.NotificationService
}

// NewPriceWatchUseCase creates a new PriceWatchUseCase.
func NewPriceWatchUseCase(marketService service.WarframeMarketService, notificationService service.NotificationService) *PriceWatchUseCase {
	return &PriceWatchUseCase{
		marketService:       marketService,
		notificationService: notificationService,
	}
}

// Execute fetches the top sell orders for the item in priceWatch.Spec and mutates
// priceWatch.Status with the cheapest price, a PriceSynced condition, and the last
// notified price when a notification is sent.
func (uc *PriceWatchUseCase) Execute(ctx context.Context, priceWatch *warframemarketv1alpha1.PriceWatch) error {
	topOrders, err := uc.marketService.GetTopOrdersByItem(ctx, priceWatch.Spec.ItemSlug, service.OrdersFilter{})

	condition := metav1.Condition{
		Type:               conditionTypePriceSynced,
		ObservedGeneration: priceWatch.Generation,
		LastTransitionTime: metav1.Now(),
	}

	if err != nil {
		condition.Status = metav1.ConditionFalse
		condition.Reason = "FetchFailed"
		condition.Message = fmt.Sprintf("Failed to fetch top orders: %v", err)
		setCondition(&priceWatch.Status.Conditions, condition)
		return err
	}

	if len(topOrders.Sell) == 0 {
		condition.Status = metav1.ConditionFalse
		condition.Reason = "NoSellOrders"
		condition.Message = fmt.Sprintf("No sell orders found for item %q", priceWatch.Spec.ItemSlug)
		setCondition(&priceWatch.Status.Conditions, condition)
		return fmt.Errorf("no sell orders found for item %q", priceWatch.Spec.ItemSlug)
	}

	cheapest := cheapestPlatinum(topOrders.Sell)
	priceWatch.Status.CheapestPrice = cheapest

	condition.Status = metav1.ConditionTrue
	condition.Reason = "PriceFetched"
	condition.Message = fmt.Sprintf("Cheapest sell price is %d platinum", cheapest)
	setCondition(&priceWatch.Status.Conditions, condition)

	if uc.shouldNotify(cheapest, priceWatch.Spec.Threshold, &priceWatch.Status, priceWatch.Spec.NotificationWindow) {
		title := fmt.Sprintf("Price alert: %s", priceWatch.Spec.ItemSlug)
		message := fmt.Sprintf("%d platinum (threshold: %d)", cheapest, priceWatch.Spec.Threshold)
		if err := uc.notificationService.Notify(ctx, title, message); err != nil {
			return fmt.Errorf("sending notification: %w", err)
		}
		now := metav1.Now()
		priceWatch.Status.LastNotifiedPrice = &cheapest
		priceWatch.Status.LastNotifiedAt = &now
	}

	return nil
}

// shouldNotify returns true when all of the following hold:
//   - cheapest is at or below the threshold
//   - the current time is within the notification window (if configured)
//   - either no notification has been sent today, or cheapest is strictly below the last notified price
//
// When a new calendar day begins, LastNotifiedPrice is treated as nil so a fresh
// notification is sent even if today's price is higher than yesterday's.
func (uc *PriceWatchUseCase) shouldNotify(cheapest, threshold int, status *warframemarketv1alpha1.PriceWatchStatus, window *warframemarketv1alpha1.NotificationWindow) bool {
	if cheapest > threshold {
		return false
	}

	now := time.Now()

	if window != nil && !isWithinWindow(now, window.From, window.To) {
		return false
	}

	// New calendar day → reset: allow notifying regardless of previous price.
	if status.LastNotifiedAt != nil {
		ly, lm, ld := status.LastNotifiedAt.Time.Date()
		ty, tm, td := now.Date()
		if ly != ty || lm != tm || ld != td {
			return true
		}
	}

	return status.LastNotifiedPrice == nil || cheapest < *status.LastNotifiedPrice
}

// isWithinWindow reports whether t falls within the [from, to] time-of-day range.
// Both from and to are in "HH:MM" format. The window is treated as same-day only
// (crossing midnight is not supported).
func isWithinWindow(t time.Time, from, to string) bool {
	parseHHMM := func(s string) (int, int) {
		var h, m int
		fmt.Sscanf(s, "%d:%d", &h, &m)
		return h, m
	}
	fh, fm := parseHHMM(from)
	th, tm := parseHHMM(to)
	cur := t.Hour()*60 + t.Minute()
	return cur >= fh*60+fm && cur <= th*60+tm
}

func cheapestPlatinum(orders []service.Order) int {
	return slices.MinFunc(orders, func(a, b service.Order) int {
		return cmp.Compare(a.Platinum, b.Platinum)
	}).Platinum
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
