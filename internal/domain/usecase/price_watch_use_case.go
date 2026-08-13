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

// NotifyReason describes why a notification was or was not sent.
type NotifyReason string

const (
	NotifyReasonPriceAboveThreshold    NotifyReason = "price above threshold"
	NotifyReasonOutsideWindow          NotifyReason = "outside notification window"
	NotifyReasonNewDay                 NotifyReason = "new day reset"
	NotifyReasonFirstNotificationToday NotifyReason = "first notification today"
	NotifyReasonNewLow                 NotifyReason = "new low today"
	NotifyReasonNoNewLow               NotifyReason = "no new low today"
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
		condition.Status = metav1.ConditionFalse
		condition.Reason = "NoSellOrders"
		condition.Message = fmt.Sprintf("No sell orders found for item %q", priceWatch.Spec.ItemSlug)
		setCondition(&priceWatch.Status.Conditions, condition)
		return fmt.Errorf("no sell orders found for item %q", priceWatch.Spec.ItemSlug)
	}

	cheapest := cheapestPlatinum(topOrders.Sell)
	priceWatch.Status.CheapestPrice = cheapest
	log.Info("price check", "cheapest", cheapest)

	condition.Status = metav1.ConditionTrue
	condition.Reason = "PriceFetched"
	condition.Message = fmt.Sprintf("Cheapest sell price is %d platinum", cheapest)
	setCondition(&priceWatch.Status.Conditions, condition)

	notify, reason := uc.shouldNotify(cheapest, priceWatch.Spec.Threshold, &priceWatch.Status, priceWatch.Spec.NotificationWindow)
	if notify {
		log.Info("sending notification", "cheapest", cheapest)
		title := fmt.Sprintf("Price alert: %s", priceWatch.Spec.ItemSlug)
		message := fmt.Sprintf("%d platinum (threshold: %d)", cheapest, priceWatch.Spec.Threshold)
		if err := uc.notificationService.Notify(ctx, title, message); err != nil {
			log.Error(err, "failed to send notification")
			return fmt.Errorf("sending notification: %w", err)
		}
		now := metav1.Now()
		priceWatch.Status.LastNotifiedPrice = &cheapest
		priceWatch.Status.LastNotifiedAt = &now
		log.Info("notification sent", "cheapest", cheapest)
	} else {
		log.Info("notification skipped", "cheapest", cheapest, "reason", reason)
	}

	return nil
}

// shouldNotify returns whether a notification should be sent and the reason for the decision.
func (uc *PriceWatchUseCase) shouldNotify(cheapest, threshold int, status *warframemarketv1alpha1.PriceWatchStatus, window *warframemarketv1alpha1.NotificationWindow) (bool, NotifyReason) {
	if cheapest > threshold {
		return false, NotifyReasonPriceAboveThreshold
	}

	now := time.Now()

	if window != nil && !isWithinWindow(now, window.From, window.To) {
		return false, NotifyReasonOutsideWindow
	}

	if status.LastNotifiedAt != nil {
		ly, lm, ld := status.LastNotifiedAt.Date()
		ty, tm, td := now.Date()
		if ly != ty || lm != tm || ld != td {
			return true, NotifyReasonNewDay
		}
	}

	if status.LastNotifiedPrice == nil {
		return true, NotifyReasonFirstNotificationToday
	}
	if cheapest < *status.LastNotifiedPrice {
		return true, NotifyReasonNewLow
	}
	return false, NotifyReasonNoNewLow
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
