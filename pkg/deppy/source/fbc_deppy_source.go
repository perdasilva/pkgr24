package source

import (
	"context"
	"fmt"
	"strings"

	"github.com/operator-framework/operator-registry/alpha/property"
	"github.com/operator-framework/operator-registry/pkg/api"
	"github.com/operator-framework/operator-registry/pkg/registry"
	"github.com/perdasilva/pkgr24/pkg/deppy/solver"
	"github.com/tidwall/gjson"
)

type RegistryQuerierDeppySource struct {
	entities []*solver.DeppyEntity
}

func NewRegistryQuerierDeppySource(ctx context.Context, registryQuerier *registry.Querier) (solver.DeppySource, error) {
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
	entityId := solver.DeppyId(fmt.Sprintf("%s:%s:%s", apiBundle.ChannelName, apiBundle.PackageName, apiBundle.Version))
	properties := map[string]string{}
	for _, prop := range apiBundle.Properties {
		switch prop.Type {
		// TODO: modify selectors to use JSONPath
		case property.TypePackage:
			properties["olm.packageName"] = gjson.Get(prop.Value, "packageName").String()
			properties["olm.version"] = gjson.Get(prop.Value, "version").String()
		default:
			if curValue, ok := properties[prop.Type]; ok {
				if curValue[0] != '[' {
					curValue = "[" + curValue + "]"
				}
				properties[prop.Type] = curValue[0:len(curValue)-1] + "," + prop.Value + "]"
			} else {
				properties[prop.Type] = prop.Value
			}
		}
	}
	properties["olm.channel"] = apiBundle.ChannelName
	properties["olm.defaultChannel"] = defaultChannel

	if apiBundle.Replaces != "" {
		properties["olm.replaces"] = apiBundle.Replaces
	}

	if apiBundle.SkipRange != "" {
		properties["olm.skipRange"] = apiBundle.SkipRange
	}

	if len(apiBundle.Skips) > 0 {
		properties["olm.skips"] = fmt.Sprintf("[%s]", strings.Join(apiBundle.Skips, ","))
	}

	return &solver.DeppyEntity{
		Identifier: entityId,
		Properties: properties,
	}, nil
}
