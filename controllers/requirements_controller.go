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
	"encoding/json"
	"fmt"
	"strings"

	"github.com/blang/semver/v4"
	"github.com/operator-framework/operator-registry/pkg/api"
	"github.com/operator-framework/operator-registry/pkg/registry"
	"github.com/perdasilva/pkgr24/pkg/querier"
	"github.com/perdasilva/pkgr24/pkg/solver"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

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

	reqs, err := reqsToreqs(pkgQuerier, requirementsInstance.Spec.Requirements)
	if err != nil {
		logger.Info(fmt.Sprintf("error getting requirements: %s", err))
		return ctrl.Result{}, nil
	}

	bundles, err := pkgQuerier.ListBundles(ctx)
	if err != nil {
		logger.Info(fmt.Sprintf("error listing bundles: %s", err))
		return ctrl.Result{}, err
	}

	requirementsCopy := requirementsInstance.DeepCopy()
	solution, err := solveWithSolver(reqs, bundles)
	if err != nil {
		logger.Info(fmt.Sprintf("could not find solution: %s", err))
		requirementsCopy.Status.Solution = nil
		requirementsCopy.Status.Message = fmt.Sprintf("resolution failed: %s", err)
	} else {
		selectedBundles := make([]string, 0)
		for _, b := range solution {
			selectedBundles = append(selectedBundles, fmt.Sprintf("%s==%s (%s)", b.PackageName, b.Version, b.ChannelName))
		}
		requirementsCopy.Status.Solution = selectedBundles
		requirementsCopy.Status.Message = "resolution successful"
	}

	return ctrl.Result{}, r.Client.Status().Update(ctx, requirementsCopy)
}

// SetupWithManager sets up the controller with the Manager.
func (r *RequirementsReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&pkgr24iov1alpha1.Requirements{}).
		Complete(r)
}

func solveWithSolver(wantedPackages []PackageRequirement, bundles []*api.Bundle) ([]*api.Bundle, error) {
	pkgToBundles := map[string][]*api.Bundle{}
	gvkToBundles := map[*api.GroupVersionKind][]*api.Bundle{}
	bundleToVariable := map[*api.Bundle]*BundleVariable{}
	metas := make([]*MetaVariable, 0)

	// add a lit for each bundle
	for _, bundle := range bundles {
		pkgToBundles[bundle.PackageName] = append(pkgToBundles[bundle.PackageName], bundle)
		for _, gvk := range bundle.ProvidedApis {
			gvkToBundles[gvk] = append(gvkToBundles[gvk], bundle)
		}
		bundleToVariable[bundle] = NewBundleVariable(bundle)
	}

	// add selected packages
	for _, wantedPkg := range wantedPackages {
		wantedBundles, ok := pkgToBundles[wantedPkg.Name()]
		if !ok {
			return nil, fmt.Errorf("package: %s not found", wantedPkg.packageName)
		}
		metadeps := make([]solver.Identifier, len(wantedBundles))
		for i, _ := range wantedBundles {
			bundleVar := bundleToVariable[wantedBundles[i]]
			inRange, err := wantedPkg.InRange(wantedBundles[i].Version)
			if err != nil {
				return nil, err
			}

			if !inRange || !wantedPkg.InChannel(wantedBundles[i].ChannelName) {
				bundleVar.AddConstraint(solver.Prohibited())
			}

			metadeps[i] = bundleToVariable[wantedBundles[i]].Identifier()
		}
		meta := &MetaVariable{
			id: solver.Identifier(fmt.Sprintf("wantedPackage:%s", wantedPkg.packageName)),
		}
		meta.AddConstraint(solver.Mandatory())
		meta.AddConstraint(solver.Dependency(metadeps...))
		metas = append(metas, meta)
	}

	// add package dependencies
	for _, bundle := range bundles {
		var deps []solver.Identifier
		for _, dependency := range bundle.Dependencies {
			switch dependency.Type {
			case "olm.gvk":
				fallthrough
			case gvkRequired:
				gvk := &api.GroupVersionKind{}
				if err := json.Unmarshal([]byte(dependency.Value), gvk); err != nil {
					return nil, err
				}
				for _, provider := range gvkToBundles[gvk] {
					deps = append(deps, bundleToVariable[provider].Identifier())
				}
			case "olm.package":
				fallthrough
			case packageRequired:
				var pkg struct {
					PackageName  string `json:"packageName"`
					VersionRange string `json:"version"`
				}
				if err := json.Unmarshal([]byte(dependency.Value), &pkg); err != nil {
					return nil, err
				}
				versionRange, err := semver.ParseRange(pkg.VersionRange)
				if err != nil {
					return nil, err
				}
				for _, pkgBundle := range pkgToBundles[pkg.PackageName] {
					bundleVersion, err := semver.Parse(pkgBundle.Version)
					if err != nil {
						return nil, err
					}
					if versionRange(bundleVersion) {
						deps = append(deps, bundleToVariable[pkgBundle].Identifier())
					}
				}
			default:
				// fmt.Println("Unknown dependency: ", dependency.Type)
			}
		}
		if deps != nil {
			bundleToVariable[bundle].AddConstraint(solver.Dependency(deps...))
		}
	}

	variables := make([]solver.Variable, 0, len(bundleToVariable))
	for _, bundleVariable := range bundleToVariable {
		variables = append(variables, bundleVariable)
	}
	for i, _ := range metas {
		variables = append(variables, metas[i])
	}

	pkgSolver, err := solver.New(solver.WithInput(variables))
	if err != nil {
		return nil, err
	}

	solution, err := pkgSolver.Solve(context.Background())
	if err != nil {
		return nil, err
	}

	selectedBundles := make([]*api.Bundle, 0)
	for i, _ := range solution {
		variable := solution[i]
		// ignore metas
		if _, ok := variable.(*BundleVariable); ok {
			selectedBundles = append(selectedBundles, solution[i].(*BundleVariable).apiBundle)
		}
	}

	return selectedBundles, nil
}

type MetaVariable struct {
	id          solver.Identifier
	constraints []solver.Constraint
}

func (v *MetaVariable) Identifier() solver.Identifier {
	return v.id
}

func (v *MetaVariable) Constraints() []solver.Constraint {
	return v.constraints
}

func (v *MetaVariable) AddConstraint(constraint solver.Constraint) {
	v.constraints = append(v.constraints, constraint)
}

type BundleVariable struct {
	apiBundle   *api.Bundle
	constraints []solver.Constraint
}

func NewBundleVariable(bundle *api.Bundle) *BundleVariable {
	return &BundleVariable{
		apiBundle:   bundle,
		constraints: nil,
	}
}

func (v *BundleVariable) Identifier() solver.Identifier {
	return solver.Identifier(strings.Join([]string{v.apiBundle.ChannelName, v.apiBundle.PackageName, v.apiBundle.Version}, "/"))
}

func (v *BundleVariable) Constraints() []solver.Constraint {
	return v.constraints
}

func (v *BundleVariable) AddConstraint(constraint solver.Constraint) {
	v.constraints = append(v.constraints, constraint)
}

func (v *BundleVariable) GetBundle() *api.Bundle {
	return v.apiBundle
}

func reqsToreqs(querier *registry.Querier, pkgs []string) ([]PackageRequirement, error) {
	pkgReqs := make([]PackageRequirement, len(pkgs))
	for i, pkg := range pkgs {
		comps := strings.Split(pkg, " ")
		fbcPkg, err := querier.GetPackage(context.Background(), comps[0])
		if err != nil {
			return nil, err
		}
		pkgReq := PackageRequirement{
			packageName:  comps[0],
			versionRange: semver.MustParseRange(">= 0.0.0"),
			channel:      fbcPkg.DefaultChannelName,
		}
		if len(comps) > 1 {
			rng, err := semver.ParseRange(strings.Join(comps[1:], " "))
			if err != nil {
				return nil, err
			}
			pkgReq.versionRange = rng
		}
		pkgReqs[i] = pkgReq
	}

	return pkgReqs, nil
}

type PackageRequirement struct {
	packageName  string
	versionRange semver.Range
	channel      string
}

func (p *PackageRequirement) Name() string {
	return p.packageName
}

func (p *PackageRequirement) InRange(version string) (bool, error) {
	vrn, err := semver.Parse(version)
	if err != nil {
		return false, err
	}
	return p.versionRange(vrn), nil
}

func (p *PackageRequirement) InChannel(channel string) bool {
	return channel == p.channel
}
