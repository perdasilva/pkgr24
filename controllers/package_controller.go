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
	"time"

	"github.com/perdasilva/pkgr24/pkg/resolver"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/reference"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/blang/semver/v4"
	rukpakv1alpha1 "github.com/operator-framework/rukpak/api/v1alpha1"

	pkgr24iov1alpha1 "github.com/perdasilva/pkgr24/api/v1alpha1"
)

type statusOption func(status *pkgr24iov1alpha1.PackageStatus)

func withReason(format string, args ...interface{}) statusOption {
	return func(status *pkgr24iov1alpha1.PackageStatus) {
		status.Reason = fmt.Sprintf(format, args...)
	}
}

func withCurrentVersion(version string) statusOption {
	return func(status *pkgr24iov1alpha1.PackageStatus) {
		status.CurrentVersion = version
	}
}

func withBundleDeploymentReference(bundleDeploymentRef *v1.ObjectReference) statusOption {
	return func(status *pkgr24iov1alpha1.PackageStatus) {
		status.BundleRef = bundleDeploymentRef
	}
}

func withCleanStatus() statusOption {
	return func(status *pkgr24iov1alpha1.PackageStatus) {
		status.Phase = pkgr24iov1alpha1.PhaseNoPhase
		status.Reason = pkgr24iov1alpha1.ReasonNoReason
		status.BundleRef = nil
		status.Dependencies = nil
	}
}

func withoutReason() statusOption {
	return func(status *pkgr24iov1alpha1.PackageStatus) {
		status.Reason = pkgr24iov1alpha1.ReasonNoReason
	}
}

func withDependencies(refs []pkgr24iov1alpha1.DependencyRef) statusOption {
	return func(status *pkgr24iov1alpha1.PackageStatus) {
		status.Dependencies = refs
	}
}

// PackageReconciler reconciles a Package object
type PackageReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=pkgr24.io,resources=packages,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=pkgr24.io,resources=packages/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=pkgr24.io,resources=packages/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Package object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.12.2/pkg/reconcile
func (r *PackageReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = log.FromContext(ctx)

	packageInstance := &pkgr24iov1alpha1.Package{}
	if err := r.Get(ctx, req.NamespacedName, packageInstance); err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	switch packageInstance.Status.State {
	case pkgr24iov1alpha1.StateNoState:
		return r.transitionToState(ctx, packageInstance, pkgr24iov1alpha1.StateNotInstalled, withCleanStatus())
	case pkgr24iov1alpha1.StateNotInstalled:
		return r.handleNotInstalled(ctx, packageInstance)
	case pkgr24iov1alpha1.StateInstalling:
		return r.handleInstalling(ctx, packageInstance)
	case pkgr24iov1alpha1.StateUpgradeable:
	case pkgr24iov1alpha1.StateInstalled:
		return r.handleInstalled(ctx, packageInstance)
	case pkgr24iov1alpha1.StateRemoving:
		return r.handleRemoving(ctx, packageInstance)
	default:
	}

	return ctrl.Result{}, nil
}

func (r *PackageReconciler) handleNotInstalled(ctx context.Context, pkg *pkgr24iov1alpha1.Package) (ctrl.Result, error) {
	if pkg.Spec.Install {
		return r.transitionToState(ctx, pkg, pkgr24iov1alpha1.StateInstalling, withCleanStatus())
	}
	return ctrl.Result{}, nil
}

func (r *PackageReconciler) handleInstalling(ctx context.Context, pkg *pkgr24iov1alpha1.Package) (ctrl.Result, error) {
	switch pkg.Status.Phase {
	case pkgr24iov1alpha1.PhaseNoPhase:
		// get bundle to install
		installationBundle, err := r.getBundleWithVersion(pkg.Spec.Version, pkg)
		if err != nil {
			return r.transitionToPhase(ctx, pkg, pkgr24iov1alpha1.PhaseFailure, withReason("error determining installation bundle version: %s", err))
		}
		return r.transitionToPhase(ctx, pkg, pkgr24iov1alpha1.PhaseResolvingDependencies, withoutReason(), withCurrentVersion(installationBundle.Version))
	case pkgr24iov1alpha1.PhaseResolvingDependencies:
		resolver := resolver.NewPackageResolver(resolver.NewClientPackageResolutionAdapter(r.Client))
		pkgRefs, err := resolver.Resolve(ctx)
		if err != nil {
			return r.transitionToPhase(ctx, pkg, pkgr24iov1alpha1.PhaseFailure, withReason("could not resolve packages: %s", err))
		}
		for _, pkgRef := range pkgRefs {
			if pkgRef.PackageName() == pkg.GetName() {
				continue
			}

			resolvedPkg := &pkgr24iov1alpha1.Package{
				ObjectMeta: metav1.ObjectMeta{
					Name: pkgRef.PackageName(),
				},
			}
			if err := r.Client.Get(ctx, client.ObjectKeyFromObject(resolvedPkg), resolvedPkg); err != nil {
				return ctrl.Result{}, err
			}

			if resolvedPkg.Spec.Install == false {
				pkgCopy := resolvedPkg.DeepCopy()
				pkgCopy.Spec.Install = true
				if err := r.Client.Update(ctx, pkgCopy); err != nil {
					return ctrl.Result{}, err
				}
			}
			//if resolvedPkg.Status.State == pkgr24iov1alpha1.StateNotInstalled {
			//	pkgCopy := resolvedPkg.DeepCopy()
			//	pkgCopy.Status.State = pkgr24iov1alpha1.StateInstalling
			//	pkgCopy.Status.CurrentVersion = pkgRef.Version()
			//	pkgCopy.Status.Phase = pkgr24iov1alpha1.PhaseDeployingBundle
			//	pkgCopy.Status.Reason = pkgr24iov1alpha1.ReasonNoReason
			//	if err := r.Client.Status().Update(ctx, pkgCopy); err != nil {
			//		return ctrl.Result{}, err
			//	}
			//}

		}
		return r.transitionToPhase(ctx, pkg, pkgr24iov1alpha1.PhaseDeployingBundle, withoutReason())
	case pkgr24iov1alpha1.PhaseDeployingBundle:
		bundle, err := r.getBundleWithVersion(pkg.Status.CurrentVersion, pkg)
		if err != nil {
			return r.transitionToPhase(ctx, pkg, pkgr24iov1alpha1.PhaseFailure, withReason("error getting bundle (version: %s): %s", pkg.Status.CurrentVersion, err))
		}

		var readyCount = 0
		var dependencyRefs []pkgr24iov1alpha1.DependencyRef
		if len(bundle.Dependencies) > 0 {
			for _, dep := range bundle.Dependencies {
				depPkg := &pkgr24iov1alpha1.Package{
					ObjectMeta: metav1.ObjectMeta{
						Name: dep.Package,
					},
				}
				if err := r.Client.Get(ctx, client.ObjectKeyFromObject(depPkg), depPkg); err != nil {
					return ctrl.Result{}, err
				}
				dependencyRefs = append(dependencyRefs, pkgr24iov1alpha1.DependencyRef{
					Package:   dep.Package,
					Version:   dep.Version,
					State:     depPkg.Status.State,
					Reason:    depPkg.Status.Reason,
					BundleRef: depPkg.Status.BundleRef,
				})
				if depPkg.Status.State == pkgr24iov1alpha1.StateInstalled {
					readyCount++
				}
			}
		}

		if readyCount < len(bundle.Dependencies) {
			_, err := r.transitionToPhase(ctx, pkg, pkgr24iov1alpha1.PhaseDeployingBundle, withReason("waiting for dependencies to install"), withDependencies(dependencyRefs))
			if err != nil {
				return ctrl.Result{}, err
			}
			return ctrl.Result{Requeue: true, RequeueAfter: time.Second}, nil
		}

		deployment := r.createRukPakBundleDeployment(pkg.Name, bundle)
		if err = controllerutil.SetControllerReference(pkg, deployment, r.Scheme); err != nil {
			return ctrl.Result{}, err
		}
		err = r.Client.Create(ctx, deployment)
		if err != nil && errors.IsAlreadyExists(err) {
			err = r.Client.Get(ctx, client.ObjectKeyFromObject(deployment), deployment)
			if err != nil {
				return ctrl.Result{}, err
			}

			isOwnedByPkg, err := r.isOwnedBy(pkg, deployment, r.Scheme)
			if err != nil {
				return ctrl.Result{}, err
			}

			if !isOwnedByPkg {
				return r.transitionToPhase(ctx, pkg, pkgr24iov1alpha1.PhaseFailure, withReason("bundle deployment for package %s already exists", pkg.Name))
			}
		}
		deploymentRef, err := reference.GetReference(r.Scheme, deployment)
		if err != nil {
			return ctrl.Result{}, err
		}
		return r.transitionToPhase(ctx, pkg, pkgr24iov1alpha1.PhaseInstallingBundle, withoutReason(), withBundleDeploymentReference(deploymentRef), withCurrentVersion(bundle.Version), withDependencies(dependencyRefs))
	case pkgr24iov1alpha1.PhaseInstallingBundle:
		if pkg.Status.BundleRef == nil {
			return r.transitionToPhase(ctx, pkg, pkgr24iov1alpha1.PhaseFailure, withReason("no bundle is referenced by this package"))
		}
		bundleDeployment := &rukpakv1alpha1.BundleDeployment{
			ObjectMeta: metav1.ObjectMeta{
				Name: pkg.Status.BundleRef.Name,
			},
		}
		if err := r.Client.Get(ctx, client.ObjectKeyFromObject(bundleDeployment), bundleDeployment); err != nil {
			return ctrl.Result{}, nil
		}
		if len(bundleDeployment.Status.Conditions) > 0 {
			latestCondition := bundleDeployment.Status.Conditions[len(bundleDeployment.Status.Conditions)-1]
			if latestCondition.Type == rukpakv1alpha1.TypeInstalled {
				return r.transitionToState(ctx, pkg, pkgr24iov1alpha1.StateInstalled)
			} else {
				return r.transitionToPhase(ctx, pkg, pkgr24iov1alpha1.PhaseInstallingBundle, withReason(latestCondition.Message))
			}
		}
		return ctrl.Result{Requeue: true, RequeueAfter: time.Second}, nil
	case pkgr24iov1alpha1.PhaseFailure:
		if pkg.Spec.Install == false {
			return r.transitionToState(ctx, pkg, pkgr24iov1alpha1.StateRemoving)
		}
	default:
		break
	}
	return ctrl.Result{}, nil
}

func (r *PackageReconciler) handleInstalled(ctx context.Context, pkg *pkgr24iov1alpha1.Package) (ctrl.Result, error) {
	if pkg.Spec.Install == false {
		return r.transitionToState(ctx, pkg, pkgr24iov1alpha1.StateRemoving)
	}
	return ctrl.Result{}, nil
}

func (r *PackageReconciler) handleRemoving(ctx context.Context, pkg *pkgr24iov1alpha1.Package) (ctrl.Result, error) {
	switch pkg.Status.Phase {
	case pkgr24iov1alpha1.PhaseNoPhase:
		if pkg.Status.CurrentVersion != "" {
			bundle, err := r.getBundleWithVersion(pkg.Status.CurrentVersion, pkg)
			if err != nil {
				return ctrl.Result{}, err
			}
			if len(bundle.Dependencies) == 0 {
				return r.transitionToPhase(ctx, pkg, pkgr24iov1alpha1.PhaseRemovingBundle, withReason(pkgr24iov1alpha1.ReasonNoReason))
			} else {
				return r.transitionToPhase(ctx, pkg, pkgr24iov1alpha1.PhaseRemovingDependencies, withReason(pkgr24iov1alpha1.ReasonNoReason))
			}
		} else {
			return r.transitionToPhase(ctx, pkg, pkgr24iov1alpha1.PhaseRemovingBundle, withReason(pkgr24iov1alpha1.ReasonNoReason))
		}
	case pkgr24iov1alpha1.PhaseRemovingBundle:
		if pkg.Status.BundleRef != nil {
			deployment := &rukpakv1alpha1.BundleDeployment{
				ObjectMeta: metav1.ObjectMeta{
					Name: pkg.Status.BundleRef.Name,
				},
			}
			if err := r.Client.Delete(ctx, deployment); err != nil {
				return ctrl.Result{}, err
			}
		}
		return r.transitionToState(ctx, pkg, pkgr24iov1alpha1.StateNotInstalled, withCleanStatus())
	case pkgr24iov1alpha1.PhaseRemovingDependencies:
	default:
	}

	return ctrl.Result{}, nil
}

func (r *PackageReconciler) transitionToState(ctx context.Context, pkg *pkgr24iov1alpha1.Package, state pkgr24iov1alpha1.State, options ...statusOption) (ctrl.Result, error) {
	pkgCopy := pkg.DeepCopy()
	pkgCopy.Status.State = state
	pkgCopy.Status.Phase = pkgr24iov1alpha1.PhaseNoPhase
	pkgCopy.Status.Reason = pkgr24iov1alpha1.ReasonNoReason

	for _, applyOption := range options {
		applyOption(&pkgCopy.Status)
	}

	err := r.Client.Status().Update(ctx, pkgCopy)
	return ctrl.Result{}, err
}

func (r *PackageReconciler) transitionToPhase(ctx context.Context, pkg *pkgr24iov1alpha1.Package, phase pkgr24iov1alpha1.Phase, options ...statusOption) (ctrl.Result, error) {
	pkgCopy := pkg.DeepCopy()
	pkgCopy.Status.Phase = phase
	for _, applyOption := range options {
		applyOption(&pkgCopy.Status)
	}
	err := r.Client.Status().Update(ctx, pkgCopy)
	return ctrl.Result{}, err
}

func (r *PackageReconciler) getBundleWithVersion(version string, pkg *pkgr24iov1alpha1.Package) (*pkgr24iov1alpha1.Bundle, error) {
	var targetVersion *semver.Version
	var err error
	if version == "" {
		targetVersion, err = r.getMaxBundleVersion(pkg)
		if err != nil {
			return nil, err
		}
	} else {
		targetVersion, err = semver.New(version)
		if err != nil {
			return nil, err
		}
	}

	for _, bundle := range pkg.Spec.Bundles {
		bundleVersion, err := semver.New(bundle.Version)
		if err != nil {
			return nil, err
		}
		if bundleVersion.Equals(*targetVersion) {
			return &bundle, nil
		}
	}
	return nil, fmt.Errorf("no bundle found with version %s", version)
}

func (r *PackageReconciler) getMaxBundleVersion(pkg *pkgr24iov1alpha1.Package) (*semver.Version, error) {
	var maxVersion *semver.Version
	for _, bundle := range pkg.Spec.Bundles {
		bundleVersion, err := semver.New(bundle.Version)
		if err != nil {
			return nil, err
		}
		if maxVersion == nil || bundleVersion.GT(*maxVersion) {
			maxVersion = bundleVersion
		}
	}
	return maxVersion, nil
}

func (r *PackageReconciler) createRukPakBundleDeployment(name string, bundle *pkgr24iov1alpha1.Bundle) *rukpakv1alpha1.BundleDeployment {
	return &rukpakv1alpha1.BundleDeployment{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: rukpakv1alpha1.BundleDeploymentSpec{
			ProvisionerClassName: "core-rukpak-io-plain",
			Template: &rukpakv1alpha1.BundleTemplate{
				ObjectMeta: metav1.ObjectMeta{
					Name: name,
				},
				Spec: rukpakv1alpha1.BundleSpec{
					ProvisionerClassName: "core-rukpak-io-plain",
					Source: rukpakv1alpha1.BundleSource{
						Type: rukpakv1alpha1.SourceTypeGit,
						Git: &rukpakv1alpha1.GitSource{
							Repository: bundle.Repo,
							Ref: rukpakv1alpha1.GitRef{
								Branch: bundle.Version,
							},
						},
					},
				},
			},
		},
	}
}

func (r *PackageReconciler) isOwnedBy(owner metav1.Object, object metav1.Object, scheme *runtime.Scheme) (bool, error) {
	ro, ok := owner.(runtime.Object)
	if !ok {
		return false, fmt.Errorf("%T is not a runtime.Object, cannot call isOwnedBy", owner)
	}

	// Create a new owner ref.
	gvk, err := apiutil.GVKForObject(ro, scheme)
	if err != nil {
		return false, err
	}

	for _, ownerReference := range object.GetOwnerReferences() {
		if ownerReference.Name == owner.GetName() &&
			ownerReference.Kind == gvk.Kind &&
			ownerReference.APIVersion == gvk.GroupVersion().String() &&
			ownerReference.UID == owner.GetUID() {
			return true, nil
		}
	}

	return false, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *PackageReconciler) SetupWithManager(mgr ctrl.Manager) error {
	if err := rukpakv1alpha1.AddToScheme(r.Scheme); err != nil {
		return err
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&pkgr24iov1alpha1.Package{}).
		Owns(&rukpakv1alpha1.BundleDeployment{}).
		Complete(r)
}
