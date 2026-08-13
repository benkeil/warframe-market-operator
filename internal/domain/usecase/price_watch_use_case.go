package usecase

import (
	"cmp"
	"context"
	"fmt"
	"slices"

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

	if uc.shouldNotify(cheapest, priceWatch.Spec.Threshold, priceWatch.Status.LastNotifiedPrice) {
		title := fmt.Sprintf("Price alert: %s", priceWatch.Spec.ItemSlug)
		message := fmt.Sprintf("%d platinum (threshold: %d)", cheapest, priceWatch.Spec.Threshold)
		if err := uc.notificationService.Notify(ctx, title, message); err != nil {
			return fmt.Errorf("sending notification: %w", err)
		}
		priceWatch.Status.LastNotifiedPrice = &cheapest
	}

	return nil
}

// shouldNotify returns true when:
//   - cheapest is at or below the threshold, AND
//   - no notification has been sent before (lastNotifiedPrice == nil), OR
//   - cheapest is strictly below the last notified price (new low).
func (uc *PriceWatchUseCase) shouldNotify(cheapest, threshold int, lastNotifiedPrice *int) bool {
	if cheapest > threshold {
		return false
	}
	return lastNotifiedPrice == nil || cheapest < *lastNotifiedPrice
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
