package usecase

import (
	"context"
	"fmt"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	warframemarketv1alpha1 "github.com/benkeil/warframe-market-operator/api/v1alpha1"
	"github.com/benkeil/warframe-market-operator/internal/domain/service"
)

// RivenPriceWatchUseCase searches riven auctions for the weapon configured in the
// RivenPriceWatch spec, computes roll quality, and sends a notification when a
// qualifying auction's price is at or below the threshold.
type RivenPriceWatchUseCase struct {
	marketService       service.WarframeMarketService
	exportService       service.WarframeExportService
	notificationService service.NotificationService
	calculator          *RivenCalculator
}

// NewRivenPriceWatchUseCase creates a new RivenPriceWatchUseCase.
func NewRivenPriceWatchUseCase(
	marketService service.WarframeMarketService,
	exportService service.WarframeExportService,
	notificationService service.NotificationService,
) *RivenPriceWatchUseCase {
	return &RivenPriceWatchUseCase{
		marketService:       marketService,
		exportService:       exportService,
		notificationService: notificationService,
		calculator:          &RivenCalculator{},
	}
}

// Execute searches for riven auctions, scores them, and notifies when a qualifying
// auction is found. It mutates rivenPriceWatch.Status with the results.
func (uc *RivenPriceWatchUseCase) Execute(ctx context.Context, rpw *warframemarketv1alpha1.RivenPriceWatch) error {
	log := logf.FromContext(ctx).WithValues("weapon", rpw.Spec.WeaponSlug, "threshold", rpw.Spec.Threshold)

	statusFilter := toUserStatuses(rpw.Spec.PlayerStatus)
	filter := service.AuctionFilter{
		WeaponURLName: rpw.Spec.WeaponSlug,
		PositiveStats: rpw.Spec.PositiveStats,
		SortBy:        "price_asc",
		BuyoutOnly:    rpw.Spec.BuyoutOnly,
		Status:        statusFilter,
	}
	if rpw.Spec.NegativeStats != "" {
		filter.NegativeStats = rpw.Spec.NegativeStats
	}
	if rpw.Spec.MaxReRolls > 0 {
		filter.ReRollsMax = rpw.Spec.MaxReRolls
	}
	filter.BuyoutPriceMax = rpw.Spec.Threshold

	condition := metav1.Condition{
		Type:               conditionTypePriceSynced,
		ObservedGeneration: rpw.Generation,
		LastTransitionTime: metav1.Now(),
	}

	log.Info("searching riven auctions")
	auctions, err := uc.marketService.SearchAuctions(ctx, filter)
	if err != nil {
		log.Error(err, "failed to search auctions")
		condition.Status = metav1.ConditionFalse
		condition.Reason = "FetchFailed"
		condition.Message = fmt.Sprintf("Failed to search auctions: %v", err)
		setCondition(&rpw.Status.Conditions, condition)
		return err
	}

	if len(auctions) == 0 {
		log.Info("no auctions found")
		condition.Status = metav1.ConditionTrue
		condition.Reason = "NoAuctions"
		condition.Message = fmt.Sprintf("No auctions found for weapon %q", rpw.Spec.WeaponSlug)
		setCondition(&rpw.Status.Conditions, condition)
		return nil
	}

	weapon, err := uc.exportService.GetWeaponByName(ctx, rpw.Spec.WeaponSlug)
	if err != nil {
		log.Error(err, "failed to get weapon info for scoring")
		// Not fatal — we continue without scoring
	}

	// Find cheapest auction that meets MinRollQuality.
	var best *service.Auction
	var bestQuality int
	for i := range auctions {
		a := &auctions[i]
		avgQuality := avgRollQuality(uc.calculator.ScoreAuction(*a, weapon))
		if avgQuality < rpw.Spec.MinRollQuality {
			continue
		}
		if best == nil || a.BuyoutPrice < best.BuyoutPrice {
			best = a
			bestQuality = avgQuality
		}
	}

	if best == nil {
		log.Info("no auction meets minimum roll quality", "minRollQuality", rpw.Spec.MinRollQuality)
		condition.Status = metav1.ConditionTrue
		condition.Reason = "QualityThresholdNotMet"
		condition.Message = fmt.Sprintf("No auction meets minimum roll quality of %d%%", rpw.Spec.MinRollQuality)
		setCondition(&rpw.Status.Conditions, condition)
		return nil
	}

	cheapest := best.BuyoutPrice
	rpw.Status.CheapestPrice = cheapest
	rpw.Status.BestRollQuality = bestQuality
	log.Info("best auction", "price", cheapest, "rollQuality", bestQuality)

	condition.Status = metav1.ConditionTrue
	condition.Reason = "AuctionFound"
	condition.Message = fmt.Sprintf("Cheapest qualifying auction: %d platinum (roll quality: %d%%)", cheapest, bestQuality)
	setCondition(&rpw.Status.Conditions, condition)

	notify, reason := uc.shouldNotifyRiven(cheapest, rpw.Spec.Threshold, &rpw.Status, rpw.Spec.NotificationWindow)
	if notify {
		log.Info("sending notification", "price", cheapest)
		title := fmt.Sprintf("Riven alert: %s", rpw.Spec.WeaponSlug)
		message := fmt.Sprintf("%d platinum | roll quality: %d%% | threshold: %d", cheapest, bestQuality, rpw.Spec.Threshold)
		if err := uc.notificationService.Notify(ctx, title, message); err != nil {
			log.Error(err, "failed to send notification")
			return fmt.Errorf("sending notification: %w", err)
		}
		now := metav1.Now()
		rpw.Status.LastNotifiedPrice = &cheapest
		rpw.Status.LastNotifiedAt = &now
		log.Info("notification sent", "price", cheapest)
	} else {
		log.Info("notification skipped", "price", cheapest, "reason", reason)
	}

	return nil
}

func (uc *RivenPriceWatchUseCase) shouldNotifyRiven(cheapest, threshold int, status *warframemarketv1alpha1.RivenPriceWatchStatus, window *warframemarketv1alpha1.NotificationWindow) (bool, NotifyReason) {
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

// avgRollQuality returns the average roll quality (0–100) across positive stats.
// Negative stats are excluded (they are already scored inverted in the CLI display).
func avgRollQuality(scores []RivenStatScore) int {
	if len(scores) == 0 {
		return 0
	}
	var sum float64
	var count int
	for _, s := range scores {
		if s.Positive && s.Known {
			sum += s.RollQuality
			count++
		}
	}
	if count == 0 {
		return 0
	}
	return int(sum / float64(count))
}

// toUserStatuses converts a slice of status strings from the spec to domain types.
// Defaults to [ingame, online] when empty.
func toUserStatuses(statuses []string) []service.UserStatus {
	if len(statuses) == 0 {
		return []service.UserStatus{service.UserStatusIngame, service.UserStatusOnline}
	}
	result := make([]service.UserStatus, 0, len(statuses))
	for _, s := range statuses {
		result = append(result, service.UserStatus(s))
	}
	return result
}
