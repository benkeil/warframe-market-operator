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
	v1alpha1ac "github.com/benkeil/warframe-market-operator/internal/applyconfiguration/api/v1alpha1"
	"github.com/benkeil/warframe-market-operator/internal/domain/usecase"
)

// RivenPriceWatchReconciler reconciles a RivenPriceWatch object.
type RivenPriceWatchReconciler struct {
	client.Client
	Scheme                 *runtime.Scheme
	RivenPriceWatchUseCase *usecase.RivenPriceWatchUseCase
}

// +kubebuilder:rbac:groups=warframe.market,resources=rivenpricewatches,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=warframe.market,resources=rivenpricewatches/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=warframe.market,resources=rivenpricewatches/finalizers,verbs=update

func (r *RivenPriceWatchReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	original := &warframemarketv1alpha1.RivenPriceWatch{}
	if err := r.Get(ctx, req.NamespacedName, original); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	result, useCaseErr := r.RivenPriceWatchUseCase.Execute(ctx, original)

	if result != nil {
		statusPatch := v1alpha1ac.RivenPriceWatch(original.Name, original.Namespace).
			WithStatus(result.Status)
		if err := r.SubResource("status").Apply(ctx, statusPatch, client.FieldOwner("rivenpricewatch-controller"), client.ForceOwnership); err != nil {
			log.Error(err, "failed to apply RivenPriceWatch status")
			return ctrl.Result{}, err
		}
	}

	if useCaseErr != nil {
		log.Error(useCaseErr, "reconcile failed")
		return ctrl.Result{}, useCaseErr
	}

	log.Info("reconcile successful")
	return ctrl.Result{RequeueAfter: 5 * time.Minute}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *RivenPriceWatchReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&warframemarketv1alpha1.RivenPriceWatch{}, builder.WithPredicates(predicate.GenerationChangedPredicate{})).
		Named("rivenpricewatch").
		Complete(r)
}
