package constraints

import (
	"fmt"
	"reflect"
	"regexp"
	"strings"

	"github.com/antonmedv/expr"
	"github.com/blang/semver/v4"
	pkgr24iov1alpha1 "github.com/perdasilva/pkgr24/api/v1alpha1"
	"github.com/perdasilva/pkgr24/pkg/deppy"
	"github.com/perdasilva/pkgr24/pkg/deppy/search"
	"github.com/perdasilva/pkgr24/pkg/deppy/solver"
	"github.com/tidwall/gjson"
)

type ConstraintBuilder struct {
	universe *deppy.EntityUniverse
}

func NewConstraintBuilder(universe *deppy.EntityUniverse) *ConstraintBuilder {
	return &ConstraintBuilder{
		universe: universe,
	}
}

func (b *ConstraintBuilder) GatherConstraints(requirements *pkgr24iov1alpha1.Requirements) ([]solver.Constraint, error) {
	allConstraints := make([]solver.Constraint, 0)
	for _, constraintExpression := range requirements.Spec.Constraints {
		constraints, err := b.runConstraint(constraintExpression)
		if err != nil {
			return nil, err
		}
		allConstraints = append(allConstraints, constraints...)
	}

	visited := map[solver.Identifier]struct{}{}
	queue := make([]solver.Identifier, 0)
	for _, constraint := range allConstraints {
		queue = append(queue, constraint.Order()...)
	}

	for len(queue) > 0 {
		var head solver.Identifier
		head, queue = queue[0], queue[1:]

		// skipped visited ids
		if _, ok := visited[head]; ok {
			continue
		}

		// mark id as visited
		visited[head] = struct{}{}

		// get related entity if any
		entity := b.universe.Get(head)

		// ignore non-entity ids (they are just constraint subjects)
		if entity == nil {
			continue
		}

		// get package dependencies
		prop, ok := entity.Properties["olm.package.required"]
		if !ok {
			continue
		}

		for _, req := range gjson.Parse(prop).Array() {
			packageName := gjson.Get(req.String(), "packageName").String()
			versionRange := gjson.Get(req.String(), "versionRange").String()
			dependencyConstraint := b.packageDependency(solver.Identifier(entity.Identifier), packageName, versionRange)
			queue = append(queue, dependencyConstraint[0].Order()...)
			allConstraints = append(allConstraints, dependencyConstraint...)
		}

		// get gvk dependencies
		prop, ok = entity.Properties["olm.gvk.required"]
		if !ok {
			continue
		}

		for _, req := range gjson.Parse(prop).Array() {
			group := gjson.Get(req.String(), "group").String()
			version := gjson.Get(req.String(), "version").String()
			kind := gjson.Get(req.String(), "kind").String()
			dependencyConstraint := b.gvkDependency(solver.Identifier(entity.Identifier), group, version, kind)
			queue = append(queue, dependencyConstraint[0].Order()...)
		}
	}

	// allConstraints = append(allConstraints, b.gvkUniqueness()...)
	// allConstraints = append(allConstraints, b.packageUniqueness()...)

	return allConstraints, nil
}

func (b *ConstraintBuilder) runConstraint(constraint string) ([]solver.Constraint, error) {
	env := map[string]interface{}{
		"requirePackage":      b.requirePackage,
		"maxOpenShiftVersion": b.maxOpenShiftVersion,
		"installedPackage":    b.installedPackage,
		"gvkUniqueness":       b.gvkUniqueness,
		"pkgUniqueness":       b.packageUniqueness,
	}

	program, err := expr.Compile(constraint)
	if err != nil {
		return nil, err
	}

	output, err := expr.Run(program, env)
	if err != nil {
		return nil, err
	}

	if _, ok := output.([]solver.Constraint); ok {
		return output.([]solver.Constraint), nil
	}

	return nil, fmt.Errorf("unrecognized return type (%s) in constraints expression", reflect.TypeOf(output))
}

func (b *ConstraintBuilder) requirePackage(packageName string, versionRange string, channel string) []solver.Constraint {
	ids := b.universe.Search(search.And(
		withPackageName(packageName),
		withinVersion(versionRange),
		withChannel(channel))).Sort(byChannelAndVersion).CollectIds()
	subject := subject("require", packageName, versionRange, channel)
	return []solver.Constraint{solver.Mandatory(subject), solver.Dependency(subject, ids...)}
}

func (b *ConstraintBuilder) installedPackage(packageName string, version string, channel string) []solver.Constraint {
	channelBundles := b.universe.Search(search.And(withPackageName(packageName), withChannel(channel), withinVersion(fmt.Sprintf(">= %s", version)))).Sort(byVersionIncreasing)
	installedBundleId := fmt.Sprintf("%s.v%s", packageName, version)
	reacheable := map[string]struct{}{}
	ids := make([]solver.Identifier, 0)
	for _, bundle := range channelBundles {
		bundleId := fmt.Sprintf("%s.v%s", bundle.Properties["olm.packageName"], bundle.Properties["olm.version"])
		if bundleId == installedBundleId || bundle.Properties["olm.replaces"] == installedBundleId {
			reacheable[bundleId] = struct{}{}
			ids = append([]solver.Identifier{solver.Identifier(bundle.Identifier)}, ids...)
			continue
		}

		if bundle.Properties["olm.replaces"] != "" {
			if _, ok := reacheable[bundle.Properties["olm.replaces"]]; ok {
				reacheable[bundleId] = struct{}{}
				ids = append([]solver.Identifier{solver.Identifier(bundle.Identifier)}, ids...)
			}
		}
	}
	subj := subject("installed", packageName, version, channel)
	return []solver.Constraint{solver.Mandatory(subj), solver.Dependency(subj, ids...)}
}

func (b *ConstraintBuilder) packageDependency(subject solver.Identifier, packageName string, versionRange string) []solver.Constraint {
	ids := b.universe.Search(search.And(withPackageName(packageName), withinVersion(versionRange))).Sort(byChannelAndVersion).CollectIds()
	return []solver.Constraint{solver.Dependency(subject, ids...)}
}

func (b *ConstraintBuilder) gvkDependency(subject solver.Identifier, group string, version string, kind string) []solver.Constraint {
	ids := b.universe.Search(withExportsGVK(group, version, kind)).Sort(byChannelAndVersion).CollectIds()
	return []solver.Constraint{solver.Dependency(subject, ids...)}
}

func (b *ConstraintBuilder) gvkUniqueness() []solver.Constraint {
	gvkToIdMap := map[string][]solver.Identifier{}
	for _, entity := range b.universe.AllEntities() {
		if gvks, ok := entity.Properties["olm.gvk"]; ok {
			gvkArray := gjson.Parse(gvks).Array()
			for _, val := range gvkArray {
				gvk := fmt.Sprintf("%s/%s/%s", val.Get("group"), val.Get("version"), val.Get("kind"))
				gvkToIdMap[gvk] = append(gvkToIdMap[gvk], solver.Identifier(entity.Identifier))
			}
		}
	}
	constrs := make([]solver.Constraint, 0, len(gvkToIdMap))
	for gvk, ids := range gvkToIdMap {
		constrs = append(constrs, solver.AtMost(subject(gvk, "uniqueness"), 1, ids...))
	}
	return constrs
}

func (b *ConstraintBuilder) maxOpenShiftVersion(version string) []solver.Constraint {
	ocpVersion := semver.MustParse(version)
	subject := subject(fmt.Sprintf("maxOpenShiftVersion>=%s", version))
	constrs := make([]solver.Constraint, 0)
	constrs = append(constrs, solver.Mandatory(subject))
	for _, entity := range b.universe.AllEntities() {
		if entity.Properties["olm.maxOpenShiftVersion"] != "" {
			maxSupportedVersion := semver.MustParse(fmt.Sprintf("%s.0", strings.Trim(entity.Properties["olm.maxOpenShiftVersion"], "\"")))
			if ocpVersion.GT(maxSupportedVersion) {
				constrs = append(constrs, solver.Conflict(subject, solver.Identifier(entity.Identifier)))
			}
		}
	}
	return constrs
}

func (b *ConstraintBuilder) packageUniqueness() []solver.Constraint {
	pkgToIdMap := map[string][]solver.Identifier{}
	for _, entity := range b.universe.AllEntities() {
		if packageName, ok := entity.Properties["olm.packageName"]; ok {
			pkgToIdMap[packageName] = append(pkgToIdMap[packageName], solver.Identifier(entity.Identifier))
		}
	}
	constrs := make([]solver.Constraint, 0, len(pkgToIdMap))
	for pkg, ids := range pkgToIdMap {
		constrs = append(constrs, solver.AtMost(subject(pkg, "uniqueness"), 1, ids...))
	}
	return constrs
}

func withPackageName(packageName string) search.Predicate {
	return func(entity *solver.DeppyEntity) bool {
		if pkgName, ok := entity.Properties["olm.packageName"]; ok {
			return pkgName == packageName
		}
		return false
	}
}

func withinVersion(semverRange string) search.Predicate {
	return func(entity *solver.DeppyEntity) bool {
		if v, ok := entity.Properties["olm.version"]; ok {
			vrange := semver.MustParseRange(semverRange)
			version := semver.MustParse(v)
			return vrange(version)
		}
		return false
	}
}

func withChannel(channel string) search.Predicate {
	return func(entity *solver.DeppyEntity) bool {
		if channel == "" {
			return true
		}
		if c, ok := entity.Properties["olm.channel"]; ok {
			return c == channel
		}
		return false
	}
}

func withExportsGVK(group string, version string, kind string) search.Predicate {
	return func(entity *solver.DeppyEntity) bool {
		if g, ok := entity.Properties["olm.gvk"]; ok {
			for _, gvk := range gjson.Parse(g).Array() {
				if gjson.Get(gvk.String(), "group").String() == group && gjson.Get(gvk.String(), "version").String() == version && gjson.Get(gvk.String(), "kind").String() == kind {
					return true
				}
			}
		}
		return false
	}
}

func byChannelAndVersion(e1 *solver.DeppyEntity, e2 *solver.DeppyEntity) bool {
	if e1.Properties["olm.packageName"] != e2.Properties["olm.packageName"] {
		return e1.Properties["olm.packageName"] < e2.Properties["olm.packageName"]
	}
	if e1.Properties["olm.channel"] != e2.Properties["olm.channel"] {
		if e1.Properties["olm.channel"] == e1.Properties["olm.defaultChannel"] {
			return true
		}
		if e2.Properties["olm.channel"] == e2.Properties["olm.defaultChannel"] {
			return false
		}
		return e1.Properties["olm.channel"] < e2.Properties["olm.channel"]
	}
	if e1.Properties["olm.version"] == "" {
		return true
	}
	if e1.Properties["olm.version"] != "" && e2.Properties["olm.version"] == "" {
		return false
	}
	v1 := semver.MustParse(e1.Properties["olm.version"])
	v2 := semver.MustParse(e2.Properties["olm.version"])
	return v1.GT(v2)
}

func byVersionIncreasing(e1 *solver.DeppyEntity, e2 *solver.DeppyEntity) bool {
	if e1.Properties["olm.version"] == "" {
		return true
	}
	if e1.Properties["olm.version"] != "" && e2.Properties["olm.version"] == "" {
		return false
	}
	v1 := semver.MustParse(e1.Properties["olm.version"])
	v2 := semver.MustParse(e2.Properties["olm.version"])
	return v1.LT(v2)
}

func subject(str ...string) solver.Identifier {
	return solver.Identifier(regexp.MustCompile("\\s").ReplaceAllString(strings.Join(str, "-"), ""))
}
