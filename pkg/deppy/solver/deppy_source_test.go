package solver

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_SelectorExpression(t *testing.T) {
	exp := SelectorExpression{
		Expression: "InSemverRange(Entity.Properties[\"@olm.version\"][0], \">1.0.0\")",
	}
	entity := &DeppyEntity{
		Identifier: DeppyId("one"),
		Properties: map[string][]string{
			"@olm.version": {"2.0.0"},
		},
	}
	eval, err := exp.evaluate(entity)
	assert.True(t, eval)
	assert.Nil(t, err)
}

func Test_SortExpression(t *testing.T) {
	exp := SortExpression{
		Expression: "SemverCompare(EntityOne.Properties[\"@olm.version\"][0], EntityTwo.Properties[\"@olm.version\"][0])",
	}
	entityOne := &DeppyEntity{
		Identifier: DeppyId("one"),
		Properties: map[string][]string{
			"@olm.version": {"2.0.0"},
		},
	}
	entityTwo := &DeppyEntity{
		Identifier: DeppyId("two"),
		Properties: map[string][]string{
			"@olm.version": {"1.0.0"},
		},
	}
	eval, err := exp.evaluate(entityOne, entityTwo)
	assert.Nil(t, err)
	assert.Greater(t, eval, 0)
}

func Test_InSemverRange(t *testing.T) {
	subject := DeppyEntity{
		Identifier: DeppyId("main"),
		Constraints: []DeppyConstraint{
			{
				Type: ConstraintTypeDependency,
				Dependency: &DependencyConstraint{
					// take bundles from the pkg package with semver in range >1.0.0 <2.0.0
					Selector: &SelectorExpression{
						Expression: "Entity.Properties[\"@olm.package\"][0] == \"pkg\" && InSemverRange(Entity.Properties[\"@olm.version\"][0], \">1.0.0 <2.0.0\")",
					},

					// order in decreasing semver version (prioritize higher versions)
					Comparator: &SortExpression{
						Expression: "-1*SemverCompare(EntityOne.Properties[\"@olm.version\"][0], EntityTwo.Properties[\"@olm.version\"][0])",
					},
				},
			},
		},
	}

	universe := []*DeppyEntity{
		{
			Identifier: DeppyId("one"),
			Properties: map[string][]string{
				"@olm.package": {"pkg"},
				"@olm.version": {"1.0.0"},
			},
		}, {
			Identifier: DeppyId("two"),
			Properties: map[string][]string{
				"@olm.package": {"pkg"},
				"@olm.version": {"1.0.1"},
			},
		}, {
			Identifier: DeppyId("three"),
			Properties: map[string][]string{
				"@olm.package": {"pkg"},
				"@olm.version": {"1.0.8"},
			},
		}, {
			Identifier: DeppyId("four"),
			Properties: map[string][]string{
				"@olm.package": {"pkg"},
				"@olm.version": {"2.0.0"},
			},
		}, {
			Identifier: DeppyId("five"),
			Properties: map[string][]string{
				"@olm.package": {"pkg5"},
				"@olm.version": {"1.0.9"},
			},
		},
	}

	constrs, err := subject.ExportConstraints(universe)
	assert.Nil(t, err)
	assert.Len(t, constrs, 1)
	assert.Equal(t, constrs[0], Dependency("main", []Identifier{"three", "two"}...))
}

func Test_JSONPath(t *testing.T) {
	subject := DeppyEntity{
		Identifier: DeppyId("main"),
		Constraints: []DeppyConstraint{
			{
				Type: ConstraintTypeDependency,
				Dependency: &DependencyConstraint{
					// take bundles from the pkg package with semver in range >1.0.0 <2.0.0
					Selector: &SelectorExpression{
						Expression: `any(Entity.Properties["@olm.gvk"], {JSONPath(#, "group").String() == "g1" && JSONPath(#, "version").String() == "v1" && JSONPath(#, "kind").String() == "k1"})`,
					},

					// order in decreasing semver version (prioritize higher versions)
					Comparator: &SortExpression{
						Expression: `-1*SemverCompare(EntityOne.Properties["@olm.version"][0], EntityTwo.Properties["@olm.version"][0])`,
					},
				},
			},
		},
	}

	universe := []*DeppyEntity{
		{
			Identifier: DeppyId("one"),
			Properties: map[string][]string{
				"@olm.gvk":     {`{"group": "g1", "version": "v1", "kind": "k1"}`},
				"@olm.version": {"1.0.0"},
			},
		}, {
			Identifier: DeppyId("two"),
			Properties: map[string][]string{
				"@olm.gvk":     {`{"group": "g1", "version": "v1", "kind": "k2"}`},
				"@olm.version": {"1.0.1"},
			},
		}, {
			Identifier: DeppyId("three"),
			Properties: map[string][]string{
				"@olm.gvk":     {`{"group": "g1", "version": "v2", "kind": "k1"}`},
				"@olm.version": {"1.0.8"},
			},
		}, {
			Identifier: DeppyId("four"),
			Properties: map[string][]string{
				"@olm.gvk":     {`{"group": "g2", "version": "v1", "kind": "k1"}`},
				"@olm.version": {"2.0.0"},
			},
		}, {
			Identifier: DeppyId("five"),
			Properties: map[string][]string{
				"@olm.gvk":     {`{"group": "g1", "version": "v1", "kind": "k1"}`},
				"@olm.version": {"1.0.9"},
			},
		},
	}

	constrs, err := subject.ExportConstraints(universe)
	assert.Nil(t, err)
	assert.Len(t, constrs, 1)
	assert.Equal(t, constrs[0], Dependency("main", []Identifier{"five", "one"}...))
}
