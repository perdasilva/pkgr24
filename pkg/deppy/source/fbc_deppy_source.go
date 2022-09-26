package source

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/operator-framework/operator-registry/alpha/property"
	"github.com/operator-framework/operator-registry/pkg/api"
	"github.com/operator-framework/operator-registry/pkg/registry"
	"github.com/perdasilva/pkgr24/pkg/deppy/solver"
)

const (
	gvkSelectorFormat     string = `any(Entity.Properties["olm.gvk"], {JSONPath(#, "group").String() == "%s" && JSONPath(#, "version").String() == "%s" && JSONPath(#, "kind").String() == "%s"})`
	packageSelectorFormat string = `Entity.Properties["olm.package"][0] == "%s" && InSemverRange(Entity.Properties["olm.version"][0], "%s")`
)

var SemverComparator = &solver.SortExpression{
	// Expression: `EntityOne.Properties["olm.package"] == EntityTwo.Properties["olm.package"] ? `,
	Expression: `EntityOne.Properties["olm.package"][0] != EntityTwo.Properties["olm.package"][0] ? EntityOne.Properties["olm.package"][0] < EntityTwo.Properties["olm.package"][0] ? -1 : 1 : EntityOne.Properties["olm.channel"][0] == EntityTwo.Properties["olm.channel"][0] ? -1*SemverCompare(EntityOne.Properties["olm.version"][0], EntityTwo.Properties["olm.version"][0]) : EntityOne.Properties["olm.channel"][0] == EntityOne.Properties["olm.defaultChannel"][0] ? -1 : EntityTwo.Properties["olm.channel"][0] == EntityTwo.Properties["olm.defaultChannel"][0] ? 1 : EntityOne.Properties["olm.channel"][0] < EntityTwo.Properties["olm.channel"][0] ? -1 : 1`,
}

type RegistryQuerierDeppySource struct {
	entities []*solver.DeppyEntity
}

func NewRegistryQuerierDeppySource(ctx context.Context, registryQuerier *registry.Querier) (*RegistryQuerierDeppySource, error) {
	bundles, err := registryQuerier.ListBundles(ctx)
	if err != nil {
		return nil, err
	}

	pkgDefaultChannel := map[string]string{}
	var entities = make([]*solver.DeppyEntity, len(bundles))
	for i, bundle := range bundles {
		if _, ok := pkgDefaultChannel[bundle.PackageName]; !ok {
			pkg, err := registryQuerier.GetPackage(ctx, bundle.PackageName)
			if err != nil {
				return nil, err
			}
			pkgDefaultChannel[bundle.PackageName] = pkg.DefaultChannelName
		}
		deppyEntity, err := bundleToDeppyEntity(bundle, pkgDefaultChannel[bundle.PackageName])
		if err != nil {
			return nil, err
		}
		entities[i] = deppyEntity
	}

	return &RegistryQuerierDeppySource{
		entities: entities,
	}, nil
}

func (s *RegistryQuerierDeppySource) GetEntities(_ context.Context) ([]*solver.DeppyEntity, error) {
	return s.entities, nil
}

func bundleToDeppyEntity(apiBundle *api.Bundle, defaultChannel string) (*solver.DeppyEntity, error) {
	properties := map[string][]string{}
	for _, prop := range apiBundle.Properties {
		switch prop.Type {
		// TODO: modify selectors to use JSONPath
		case property.TypePackage:
			pkg := &struct {
				PackageName string `json:"packageName"`
				Version     string `json:"version"`
			}{}
			if err := json.Unmarshal([]byte(prop.Value), pkg); err != nil {
				return nil, err
			}
			properties[prop.Type] = []string{pkg.PackageName}
			properties["olm.version"] = []string{pkg.Version}
		default:
			properties[prop.Type] = append(properties[prop.Type], prop.Value)
		}
	}
	properties["olm.channel"] = []string{apiBundle.ChannelName}
	properties["olm.defaultChannel"] = []string{defaultChannel}

	deppyConstraints := make([]solver.DeppyConstraint, 0)
	for _, dependency := range apiBundle.Dependencies {
		switch dependency.Type {
		case property.TypeGVK:
			gvk := &struct {
				Group   string `json:"group"`
				Version string `json:"version"`
				Kind    string `json:"kind"`
			}{}
			if err := json.Unmarshal([]byte(dependency.Value), gvk); err != nil {
				return nil, err
			}
			deppyConstraints = append(deppyConstraints, solver.DeppyConstraint{
				Type: solver.ConstraintTypeDependency,
				Dependency: &solver.DependencyConstraint{
					Selector:   GVKSelector(gvk.Group, gvk.Version, gvk.Kind),
					Comparator: SemverComparator,
				},
			})
		case property.TypePackage:
			pkgDep := &struct {
				PackageName  string `json:"packageName"`
				VersionRange string `json:"version"`
			}{}
			if err := json.Unmarshal([]byte(dependency.Value), pkgDep); err != nil {
				return nil, err
			}
			deppyConstraints = append(deppyConstraints, solver.DeppyConstraint{
				Type: solver.ConstraintTypeDependency,
				Dependency: &solver.DependencyConstraint{
					Selector:   PkgSelector(pkgDep.PackageName, pkgDep.VersionRange),
					Comparator: SemverComparator,
				},
			})
		default:
			return nil, fmt.Errorf("unknown dependency type (%s)", dependency.Type)
		}
	}

	return &solver.DeppyEntity{
		Identifier:  solver.DeppyId(fmt.Sprintf("%s:%s:%s", apiBundle.ChannelName, apiBundle.PackageName, apiBundle.Version)),
		Properties:  properties,
		Constraints: deppyConstraints,
	}, nil
}

func GVKSelector(group string, version string, kind string) *solver.SelectorExpression {
	return &solver.SelectorExpression{
		Expression: fmt.Sprintf(gvkSelectorFormat, group, version, kind),
	}
}

func PkgSelector(packageName string, versionRange string) *solver.SelectorExpression {
	return &solver.SelectorExpression{
		Expression: fmt.Sprintf(packageSelectorFormat, packageName, versionRange),
	}
}
