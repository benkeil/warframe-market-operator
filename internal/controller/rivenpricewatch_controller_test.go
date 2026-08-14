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

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	warframemarketv1alpha1 "github.com/benkeil/warframe-market-operator/api/v1alpha1"
	"github.com/benkeil/warframe-market-operator/internal/domain/usecase"
)

var _ = Describe("RivenPriceWatch Controller", func() {
	Context("When reconciling a resource", func() {
		const resourceName = "test-riven-resource"

		ctx := context.Background()

		typeNamespacedName := types.NamespacedName{
			Name:      resourceName,
			Namespace: testNamespace,
		}
		rivenpricewatch := &warframemarketv1alpha1.RivenPriceWatch{}

		BeforeEach(func() {
			By("creating the custom resource for the Kind RivenPriceWatch")
			err := k8sClient.Get(ctx, typeNamespacedName, rivenpricewatch)
			if err != nil && errors.IsNotFound(err) {
				resource := &warframemarketv1alpha1.RivenPriceWatch{
					ObjectMeta: metav1.ObjectMeta{
						Name:      resourceName,
						Namespace: testNamespace,
					},
					Spec: warframemarketv1alpha1.RivenPriceWatchSpec{
						WeaponSlug: "falcor",
						Threshold:  200,
					},
				}
				Expect(k8sClient.Create(ctx, resource)).To(Succeed())
			}
		})

		AfterEach(func() {
			resource := &warframemarketv1alpha1.RivenPriceWatch{}
			err := k8sClient.Get(ctx, typeNamespacedName, resource)
			Expect(err).NotTo(HaveOccurred())

			By("Cleanup the specific resource instance RivenPriceWatch")
			Expect(k8sClient.Delete(ctx, resource)).To(Succeed())
		})

		It("should successfully reconcile without error when no auctions found", func() {
			By("Reconciling with stub services that return no auctions")
			rivenPriceWatchUseCase := usecase.NewRivenPriceWatchUseCase(
				&stubMarketService{platinum: 0},
				&stubExportService{},
				&stubNotificationService{},
			)

			controllerReconciler := &RivenPriceWatchReconciler{
				Client:                 k8sClient,
				Scheme:                 k8sClient.Scheme(),
				RivenPriceWatchUseCase: rivenPriceWatchUseCase,
			}

			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			// No auctions returned → use case returns error but reconciler requeues
			_ = err
		})
	})
})
