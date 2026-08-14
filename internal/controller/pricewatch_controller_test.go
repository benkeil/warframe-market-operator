/*
Copyright 2026.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"context"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	warframemarketv1alpha1 "github.com/benkeil/warframe-market-operator/api/v1alpha1"
	"github.com/benkeil/warframe-market-operator/internal/domain/service"
	"github.com/benkeil/warframe-market-operator/internal/domain/usecase"
)

// stubMarketService is a test double that returns a fixed cheapest price.
type stubMarketService struct {
	platinum int
}

func (s *stubMarketService) GetItemBySlug(_ context.Context, _ string) (*service.ItemDetail, error) {
	return nil, nil
}

func (s *stubMarketService) GetItems(_ context.Context) ([]service.Item, error) {
	return nil, nil
}

func (s *stubMarketService) GetOrdersByItem(_ context.Context, _ string, _ service.OrdersFilter) ([]service.Order, error) {
	return nil, nil
}

func (s *stubMarketService) GetTopOrdersByItem(_ context.Context, _ string, _ service.OrdersFilter) (*service.TopOrders, error) {
	return &service.TopOrders{
		Sell: []service.Order{{Platinum: s.platinum}},
	}, nil
}

func (s *stubMarketService) SearchAuctions(_ context.Context, _ service.AuctionFilter) ([]service.Auction, error) {
	return nil, nil
}

// stubNotificationService is a test double that records sent notifications.
type stubNotificationService struct {
	sent []string
}

func (s *stubNotificationService) Notify(_ context.Context, title, _ string) error {
	s.sent = append(s.sent, fmt.Sprintf("notified: %s", title))
	return nil
}

var _ = Describe("PriceWatch Controller", func() {
	Context("When reconciling a resource", func() {
		const resourceName = "test-resource"

		ctx := context.Background()

		typeNamespacedName := types.NamespacedName{
			Name:      resourceName,
			Namespace: "default",
		}
		pricewatch := &warframemarketv1alpha1.PriceWatch{}

		BeforeEach(func() {
			By("creating the custom resource for the Kind PriceWatch")
			err := k8sClient.Get(ctx, typeNamespacedName, pricewatch)
			if err != nil && errors.IsNotFound(err) {
				resource := &warframemarketv1alpha1.PriceWatch{
					ObjectMeta: metav1.ObjectMeta{
						Name:      resourceName,
						Namespace: "default",
					},
					Spec: warframemarketv1alpha1.PriceWatchSpec{
						ItemSlug:  "primed_firestorm",
						Threshold: 50,
					},
				}
				Expect(k8sClient.Create(ctx, resource)).To(Succeed())
			}
		})

		AfterEach(func() {
			resource := &warframemarketv1alpha1.PriceWatch{}
			err := k8sClient.Get(ctx, typeNamespacedName, resource)
			Expect(err).NotTo(HaveOccurred())

			By("Cleanup the specific resource instance PriceWatch")
			Expect(k8sClient.Delete(ctx, resource)).To(Succeed())
		})

		It("should successfully reconcile and update cheapest price", func() {
			By("Reconciling the created resource")
			notifSvc := &stubNotificationService{}
			priceWatchUseCase := usecase.NewPriceWatchUseCase(&stubMarketService{platinum: 42}, notifSvc)

			controllerReconciler := &PriceWatchReconciler{
				Client:            k8sClient,
				Scheme:            k8sClient.Scheme(),
				PriceWatchUseCase: priceWatchUseCase,
			}

			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())

			By("Checking that the cheapest price was written to status")
			updated := &warframemarketv1alpha1.PriceWatch{}
			Expect(k8sClient.Get(ctx, typeNamespacedName, updated)).To(Succeed())
			Expect(updated.Status.CheapestPrice).To(Equal(42))

			By("Checking that a notification was sent (price 42 <= threshold 50)")
			Expect(notifSvc.sent).To(HaveLen(1))
		})
	})
})
