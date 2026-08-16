package usecase

import (
	"context"
	"fmt"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientgov1 "k8s.io/client-go/applyconfigurations/meta/v1"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	warframemarketv1alpha1 "github.com/benkeil/warframe-market-operator/api/v1alpha1"
	v1alpha1ac "github.com/benkeil/warframe-market-operator/internal/applyconfiguration/api/v1alpha1"
	"github.com/benkeil/warframe-market-operator/internal/domain/service"
)

// RivenPriceWatchResult is the output of RivenPriceWatchUseCase.Execute.
// The use case does not mutate the input.
type RivenPriceWatchResult struct {
	Status *v1alpha1ac.RivenPriceWatchStatusApplyConfiguration
}

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

// Execute searches for riven auctions, scores them, and notifies for each new qualifying
// auction found. It returns the desired status to apply and does not mutate rpw.
func (uc *RivenPriceWatchUseCase) Execute(ctx context.Context, rpw *warframemarketv1alpha1.RivenPriceWatch) (*RivenPriceWatchResult, error) {
	log := logf.FromContext(ctx).WithValues("weapon", rpw.Spec.WeaponSlug, "threshold", rpw.Spec.Threshold)

	status := v1alpha1ac.RivenPriceWatchStatus()
	condition := clientgov1.Condition().
		WithType(conditionTypePriceSynced).
		WithObservedGeneration(rpw.Generation).
		WithLastTransitionTime(metav1.Now())

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

	log.Info("searching riven auctions")
	auctions, err := uc.marketService.SearchAuctions(ctx, filter)
	if err != nil {
		log.Error(err, "failed to search auctions")
		status.WithConditions(condition.
			WithStatus(metav1.ConditionFalse).
			WithReason("FetchFailed").
			WithMessage(fmt.Sprintf("Failed to search auctions: %v", err)))
		return &RivenPriceWatchResult{Status: status}, err
	}

	if len(auctions) == 0 {
		log.Info("no auctions found")
		status.WithConditions(condition.
			WithStatus(metav1.ConditionTrue).
			WithReason("NoAuctions").
			WithMessage(fmt.Sprintf("No auctions found for weapon %q", rpw.Spec.WeaponSlug)))
		return &RivenPriceWatchResult{Status: status}, nil
	}

	weapon, err := uc.exportService.GetWeaponByName(ctx, rpw.Spec.WeaponSlug)
	if err != nil {
		log.Error(err, "failed to get weapon info for scoring")
		// Not fatal — continue without scoring
	}

	// Reset notified IDs at the start of a new day so auctions are re-notified daily.
	notifiedIDs := rpw.Status.NotifiedAuctionIDs
	if rpw.Status.LastNotifiedAt != nil {
		ly, lm, ld := rpw.Status.LastNotifiedAt.Date()
		ty, tm, td := time.Now().Date()
		if ly != ty || lm != tm || ld != td {
			log.Info("new day — resetting notified auction IDs")
			notifiedIDs = nil
		}
	}
	notifiedSet := make(map[string]bool, len(notifiedIDs))
	for _, id := range notifiedIDs {
		notifiedSet[id] = true
	}

	var cheapest int
	var bestQuality int
	var notifyCount int
	newNotifiedIDs := append([]string(nil), notifiedIDs...)

	for i := range auctions {
		a := &auctions[i]
		scores := uc.calculator.ScoreAuction(*a, weapon)
		quality := avgRollQuality(scores)

		if quality < rpw.Spec.MinRollQuality {
			continue
		}

		if cheapest == 0 || a.BuyoutPrice < cheapest {
			cheapest = a.BuyoutPrice
			bestQuality = quality
		}

		if notifiedSet[a.ID] {
			continue
		}

		window := rpw.Spec.NotificationWindow
		if window != nil && !isWithinWindow(time.Now(), window.From, window.To) {
			log.Info("notification skipped (outside window)", "auctionID", a.ID)
			continue
		}

		log.Info("sending notification", "auctionID", a.ID, "price", a.BuyoutPrice, "rollQuality", quality)
		title := fmt.Sprintf("Riven alert: %s", weapon.Name)
		message := fmt.Sprintf("%dp | roll quality: %d%% | seller: %s (%s)", a.BuyoutPrice, quality, a.Owner.IngameName, a.Owner.Status)
		notification := service.Notification{
			Title:   title,
			Message: message,
			Tags:    []string{"loudspeaker"},
			Actions: []service.Action{
				{
					Type:  service.ActionTypeView,
					Label: "Open on Warframe Market",
					Url:   fmt.Sprintf("https://warframe.market/auction/%s", a.ID),
				},
			},
		}
		if err := uc.notificationService.Send(ctx, notification); err != nil {
			log.Error(err, "failed to send notification", "auctionID", a.ID)
			status.WithNotifiedAuctionIDs(newNotifiedIDs...).WithConditions(condition.
				WithStatus(metav1.ConditionTrue).
				WithReason("AuctionFound").
				WithMessage(fmt.Sprintf("Cheapest qualifying auction: %d platinum (roll quality: %d%%)", cheapest, bestQuality)))
			return &RivenPriceWatchResult{Status: status}, fmt.Errorf("sending notification: %w", err)
		}

		newNotifiedIDs = append(newNotifiedIDs, a.ID)
		notifiedSet[a.ID] = true
		notifyCount++
	}

	if cheapest == 0 {
		log.Info("no auction meets minimum roll quality", "minRollQuality", rpw.Spec.MinRollQuality)
		status.WithConditions(condition.
			WithStatus(metav1.ConditionTrue).
			WithReason("QualityThresholdNotMet").
			WithMessage(fmt.Sprintf("No auction meets minimum roll quality of %d%%", rpw.Spec.MinRollQuality)))
		return &RivenPriceWatchResult{Status: status}, nil
	}

	status.WithCheapestPrice(cheapest).WithBestRollQuality(bestQuality).WithNotifiedAuctionIDs(newNotifiedIDs...)
	if notifyCount > 0 {
		now := metav1.Now()
		status.WithLastNotifiedAt(now)
		log.Info("notifications sent", "count", notifyCount)
	}
	status.WithConditions(condition.
		WithStatus(metav1.ConditionTrue).
		WithReason("AuctionFound").
		WithMessage(fmt.Sprintf("Cheapest qualifying auction: %d platinum (roll quality: %d%%)", cheapest, bestQuality)))

	return &RivenPriceWatchResult{Status: status}, nil
}

// avgRollQuality returns the average roll quality (0–100) across positive stats.
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
