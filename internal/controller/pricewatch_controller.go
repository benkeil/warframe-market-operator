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
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	warframemarketv1alpha1 "github.com/benkeil/warframe-market-operator/api/v1alpha1"
	"github.com/benkeil/warframe-market-operator/internal/domain/usecase"
)

// PriceWatchReconciler reconciles a PriceWatch object
type PriceWatchReconciler struct {
	client.Client
	Scheme            *runtime.Scheme
	PriceWatchUseCase *usecase.PriceWatchUseCase
}

// +kubebuilder:rbac:groups=warframe.market,resources=pricewatches,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=warframe.market,resources=pricewatches/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=warframe.market,resources=pricewatches/finalizers,verbs=update

func (r *PriceWatchReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	priceWatch := &warframemarketv1alpha1.PriceWatch{}
	if err := r.Get(ctx, req.NamespacedName, priceWatch); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	useCaseErr := r.PriceWatchUseCase.Execute(ctx, priceWatch)

	if err := r.Status().Update(ctx, priceWatch); err != nil {
		log.Error(err, "failed to update PriceWatch status")
		return ctrl.Result{}, err
	}

	return ctrl.Result{RequeueAfter: 5 * time.Minute}, useCaseErr
}

// SetupWithManager sets up the controller with the Manager.
func (r *PriceWatchReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&warframemarketv1alpha1.PriceWatch{}).
		Named("pricewatch").
		Complete(r)
}
