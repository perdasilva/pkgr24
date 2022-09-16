package resolver

import (
	"context"
	"fmt"
	"strings"

	"github.com/perdasilva/pkgr24/api/v1alpha1"
	"github.com/perdasilva/pkgr24/pkg/solver"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const bundleSeparator = "-"

type ClientPackageResolutionAdapter struct {
	client client.Client
}

type PackageBundleVariable struct {
	pkg         *v1alpha1.Package
	bundle      *v1alpha1.Bundle
	constraints []solver.Constraint
}

func fromPackage(pkg *v1alpha1.Package, bundle *v1alpha1.Bundle) *PackageBundleVariable {
	return &PackageBundleVariable{
		pkg:    pkg,
		bundle: bundle,
	}
}

func (p *PackageBundleVariable) Identifier() solver.Identifier {
	return solver.Identifier(strings.Join([]string{p.pkg.GetName(), p.bundle.Version}, bundleSeparator))
}

func (p *PackageBundleVariable) Constraints() []solver.Constraint {
	return p.constraints
}

func (p *PackageBundleVariable) AddConstraint(constraint solver.Constraint) {
	p.constraints = append(p.constraints, constraint)
}

func NewClientPackageResolutionAdapter(client client.Client) *ClientPackageResolutionAdapter {
	return &ClientPackageResolutionAdapter{
		client: client,
	}
}

func (p *ClientPackageResolutionAdapter) CollectVariables(ctx context.Context) ([]solver.Variable, error) {
	packageList := &v1alpha1.PackageList{}
	if err := p.client.List(ctx, packageList); err != nil {
		return nil, err
	}

	var variables []solver.Variable
	for i, _ := range packageList.Items {
		pkg := &packageList.Items[i]
		for j, _ := range pkg.Spec.Bundles {
			bundle := &pkg.Spec.Bundles[j]
			pkgBundleVar := fromPackage(pkg, bundle)
			if pkg.Status.CurrentVersion == bundle.Version {
				pkgBundleVar.AddConstraint(solver.Mandatory())
			}
			for _, dep := range bundle.Dependencies {
				depId := solver.Identifier(strings.Join([]string{dep.Package, dep.Version}, bundleSeparator))
				pkgBundleVar.AddConstraint(solver.Dependency(depId))
			}
			variables = append(variables, pkgBundleVar)
		}
	}
	return variables, nil
}

func (p *ClientPackageResolutionAdapter) UnpackVariables(variables []solver.Variable) ([]*PackageBundleReference, error) {
	var packages []*PackageBundleReference
	for _, variable := range variables {
		pkgVar, ok := variable.(*PackageBundleVariable)
		if !ok {
			return nil, fmt.Errorf("could not convert variable to PackageBundleVariable")
		}
		packages = append(packages, NewPackageBundleReference(pkgVar.pkg.GetName(), pkgVar.bundle.Version))
	}
	return packages, nil
}
