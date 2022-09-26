/*
Copyright 2022.

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

package controllers

import (
	"context"
	"fmt"

	"github.com/perdasilva/pkgr24/pkg/deppy/solver"
	"github.com/perdasilva/pkgr24/pkg/deppy/source"
	"github.com/perdasilva/pkgr24/pkg/querier"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	pkgr24iov1alpha1 "github.com/perdasilva/pkgr24/api/v1alpha1"
)

const (
	gvkRequired     = "olm.gvk.required"
	packageRequired = "olm.package.required"
	// labelRequired   = "olm.label.required"
)

// RequirementsReconciler reconciles a Requirements object
type RequirementsReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=pkgr24.io,resources=requirements,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=pkgr24.io,resources=requirements/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=pkgr24.io,resources=requirements/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Requirements object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.12.2/pkg/reconcile
func (r *RequirementsReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	requirementsInstance := &pkgr24iov1alpha1.Requirements{}
	if err := r.Get(ctx, req.NamespacedName, requirementsInstance); err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	pkgQuerier, err := querier.NewPackageQuerier(ctx, r.Client)
	if err != nil {
		logger.Info(fmt.Sprintf("error creating querier: %s", err))
		return ctrl.Result{}, err
	}

	deppySource, err := source.NewRegistryQuerierDeppySource(ctx, pkgQuerier)
	if err != nil {
		return ctrl.Result{}, err
	}

	domainSource := source.NewDomainDeppySource()
	requirementsSource := source.NewRequirementsDeppySource(requirementsInstance)

	requirementsCopy := requirementsInstance.DeepCopy()
	solution, err := solveWithDeppy(ctx, []solver.DeppySource{deppySource, domainSource, requirementsSource})
	if err != nil {
		logger.Info(fmt.Sprintf("could not find solution: %s", err))
		requirementsCopy.Status.Solution = []string{}
		requirementsCopy.Status.Message = fmt.Sprintf("resolution failed: %s", err)
	} else {
		selectedBundles := make([]string, 0)
		for _, b := range solution {
			if !b.Meta {
				selectedBundles = append(selectedBundles, fmt.Sprintf("%s", b.Identifier))
			}
		}
		requirementsCopy.Status.Solution = selectedBundles
		requirementsCopy.Status.Message = "resolution successful"
		logger.Info(fmt.Sprintf("resolution successful"))
	}

	err = r.Client.Status().Update(ctx, requirementsCopy)
	if err != nil {
		logger.Info(fmt.Sprintf("error updating status: %s", err))
	}

	return ctrl.Result{}, err
}

// SetupWithManager sets up the controller with the Manager.
func (r *RequirementsReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&pkgr24iov1alpha1.Requirements{}).
		WithEventFilter(predicate.GenerationChangedPredicate{}).
		Complete(r)
}

func solveWithDeppy(ctx context.Context, srcs []solver.DeppySource) ([]*solver.DeppyEntity, error) {
	// collect universe and great high level constraints
	universe := make([]*solver.DeppyEntity, 0)

	for _, src := range srcs {
		entities, err := src.GetEntities(ctx)
		if err != nil {
			return nil, err
		}
		for _, entity := range entities {
			universe = append(universe, entity)
		}
	}

	depSolver, err := solver.New(solver.WithInput(universe))
	if err != nil {
		return nil, err
	}

	return depSolver.Solve(ctx)
}
