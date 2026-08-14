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
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	warframemarketv1alpha1 "github.com/benkeil/warframe-market-operator/api/v1alpha1"
	"github.com/benkeil/warframe-market-operator/internal/domain/usecase"
)

// ItemPriceWatchReconciler reconciles an ItemPriceWatch object.
type ItemPriceWatchReconciler struct {
	client.Client
	Scheme                *runtime.Scheme
	ItemPriceWatchUseCase *usecase.ItemPriceWatchUseCase
}

// +kubebuilder:rbac:groups=warframe.market,resources=itempricewatches,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=warframe.market,resources=itempricewatches/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=warframe.market,resources=itempricewatches/finalizers,verbs=update

func (r *ItemPriceWatchReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	itemPriceWatch := &warframemarketv1alpha1.ItemPriceWatch{}
	if err := r.Get(ctx, req.NamespacedName, itemPriceWatch); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	useCaseErr := r.ItemPriceWatchUseCase.Execute(ctx, itemPriceWatch)

	if err := r.Status().Update(ctx, itemPriceWatch); err != nil {
		log.Error(err, "failed to update ItemPriceWatch status")
		return ctrl.Result{}, err
	}

	if useCaseErr != nil {
		log.Error(useCaseErr, "reconcile failed")
		return ctrl.Result{}, useCaseErr
	}

	log.Info("reconcile successful")
	return ctrl.Result{RequeueAfter: 5 * time.Minute}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ItemPriceWatchReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&warframemarketv1alpha1.ItemPriceWatch{}, builder.WithPredicates(predicate.GenerationChangedPredicate{})).
		Named("itempricewatch").
		Complete(r)
}
