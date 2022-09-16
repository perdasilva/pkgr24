package resolver

import (
	"context"

	"github.com/perdasilva/pkgr24/pkg/solver"
)

type PackageBundleReference struct {
	packageName   string
	bundleVersion string
}

func (p *PackageBundleReference) PackageName() string {
	return p.packageName
}

func (p *PackageBundleReference) Version() string {
	return p.bundleVersion
}

func NewPackageBundleReference(packageName string, bundleVersion string) *PackageBundleReference {
	return &PackageBundleReference{
		packageName:   packageName,
		bundleVersion: bundleVersion,
	}
}

type DependencyResolverOption func(resolver *PackageResolver)

type PackageResolutionAdapter interface {
	CollectVariables(ctx context.Context) ([]solver.Variable, error)
	UnpackVariables([]solver.Variable) ([]*PackageBundleReference, error)
}

type PackageResolver struct {
	pkgResolutionAdapter PackageResolutionAdapter
}

func NewPackageResolver(pkgResolutionAdapter PackageResolutionAdapter) *PackageResolver {
	return &PackageResolver{
		pkgResolutionAdapter: pkgResolutionAdapter,
	}
}

func (d *PackageResolver) Resolve(ctx context.Context) ([]*PackageBundleReference, error) {
	// get solver input
	solverInput, err := d.pkgResolutionAdapter.CollectVariables(ctx)
	if err != nil {
		return nil, err
	}

	// create sat solver
	satSolver, err := solver.New(solver.WithInput(solverInput))
	if err != nil {
		return nil, err
	}

	// resolve packages
	variables, err := satSolver.Solve(ctx)
	if err != nil {
		return nil, err
	}

	// unpack packages from variables
	return d.pkgResolutionAdapter.UnpackVariables(variables)
}
