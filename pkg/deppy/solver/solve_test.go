package solver

import (
	"bytes"
	"context"
	"fmt"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
)

func entity(id DeppyId, constraints ...DeppyConstraint) *DeppyEntity {
	return &DeppyEntity{
		Identifier:  id,
		Constraints: constraints,
	}
}

func mandatoryConstraint() DeppyConstraint {
	return DeppyConstraint{
		Type:      ConstraintTypeMandatory,
		Mandatory: &MandatoryConstraint{},
	}
}

func prohibitedConstraint() DeppyConstraint {
	return DeppyConstraint{
		Type:       ConstraintTypeProhibited,
		Prohibited: &ProhibitedConstraint{},
	}
}

func conflictConstraint(id DeppyId) DeppyConstraint {
	return DeppyConstraint{
		Type: ConstraintTypeConflict,
		Conflicts: &ConflictConstraint{
			Id: id,
		},
	}
}

func atMostConstraint(n int, ids ...Identifier) DeppyConstraint {
	return DeppyConstraint{
		Type: ConstraintTypeAtMost,
		AtMost: &AtMostConstraint{
			N:   n,
			Ids: ids,
		},
	}
}

func dependencyConstraint(ids ...Identifier) DeppyConstraint {
	return DeppyConstraint{
		Type: ConstraintTypeDependency,
		Dependency: &DependencyConstraint{
			Ids: ids,
		},
	}
}

func TestNotSatisfiableError(t *testing.T) {
	type tc struct {
		Name   string
		Error  NotSatisfiable
		String string
	}

	for _, tt := range []tc{
		{
			Name:   "nil",
			String: "constraints not satisfiable",
		},
		{
			Name:   "empty",
			String: "constraints not satisfiable",
			Error:  NotSatisfiable{},
		},
		{
			Name: "single failure",
			Error: NotSatisfiable{
				Mandatory("a"),
			},
			String: fmt.Sprintf("constraints not satisfiable: %s",
				Mandatory("a")),
		},
		{
			Name: "multiple failures",
			Error: NotSatisfiable{
				Mandatory("a"),
				Prohibited("b"),
			},
			String: fmt.Sprintf("constraints not satisfiable: %s, %s",
				Mandatory("a"), Prohibited("b")),
		},
	} {
		t.Run(tt.Name, func(t *testing.T) {
			assert.Equal(t, tt.String, tt.Error.Error())
		})
	}
}

func TestSolve(t *testing.T) {
	type tc struct {
		Name      string
		Variables []*DeppyEntity
		Installed []Identifier
		Error     error
	}

	for _, tt := range []tc{
		{
			Name: "no variables",
		},
		{
			Name:      "unnecessary entity is not installed",
			Variables: []*DeppyEntity{entity("a")},
		},
		{
			Name:      "single mandatory entity is installed",
			Variables: []*DeppyEntity{entity("a", mandatoryConstraint())},
			Installed: []Identifier{"a"},
		},
		{
			Name:      "both mandatory and prohibited produce error",
			Variables: []*DeppyEntity{entity("a", mandatoryConstraint(), prohibitedConstraint())},
			Error: NotSatisfiable{
				Mandatory("a"),
				Prohibited("a"),
			},
		},
		{
			Name: "dependency is installed",
			Variables: []*DeppyEntity{
				entity("a"),
				entity("b", mandatoryConstraint(), dependencyConstraint("a")),
			},
			Installed: []Identifier{"a", "b"},
		},
		{
			Name: "transitive dependency is installed",
			Variables: []*DeppyEntity{
				entity("a"),
				entity("b", dependencyConstraint("a")),
				entity("c", mandatoryConstraint(), dependencyConstraint("b")),
			},
			Installed: []Identifier{"a", "b", "c"},
		},
		{
			Name: "both dependencies are installed",
			Variables: []*DeppyEntity{
				entity("a"),
				entity("b"),
				entity("c", mandatoryConstraint(), dependencyConstraint("a"), dependencyConstraint("b")),
			},
			Installed: []Identifier{"a", "b", "c"},
		},
		{
			Name: "solution with first dependency is selected",
			Variables: []*DeppyEntity{
				entity("a"),
				entity("b", conflictConstraint("a")),
				entity("c", mandatoryConstraint(), dependencyConstraint("a", "b")),
			},
			Installed: []Identifier{"a", "c"},
		},
		{
			Name: "solution with only first dependency is selected",
			Variables: []*DeppyEntity{
				entity("a"),
				entity("b"),
				entity("c", mandatoryConstraint(), dependencyConstraint("a", "b")),
			},
			Installed: []Identifier{"a", "c"},
		},
		{
			Name: "solution with first dependency is selected (reverse)",
			Variables: []*DeppyEntity{
				entity("a"),
				entity("b", conflictConstraint("a")),
				entity("c", mandatoryConstraint(), dependencyConstraint("b", "a")),
			},
			Installed: []Identifier{"b", "c"},
		},
		{
			Name: "two mandatory but conflicting packages",
			Variables: []*DeppyEntity{
				entity("a", mandatoryConstraint()),
				entity("b", mandatoryConstraint(), conflictConstraint("a")),
			},
			Error: NotSatisfiable{
				Mandatory("a"),
				Mandatory("b"),
				Conflict("b", "a"),
			},
		},
		{
			Name: "irrelevant dependencies don't influence search order",
			Variables: []*DeppyEntity{
				entity("a", dependencyConstraint("x", "y")),
				entity("b", mandatoryConstraint(), dependencyConstraint("y", "x")),
				entity("x"),
				entity("y"),
			},
			Installed: []Identifier{"b", "y"},
		},
		{
			Name: "cardinality constraint prevents resolution",
			Variables: []*DeppyEntity{
				entity("a", mandatoryConstraint(), dependencyConstraint("x", "y"), atMostConstraint(1, "x", "y")),
				entity("x", mandatoryConstraint()),
				entity("y", mandatoryConstraint()),
			},
			Error: NotSatisfiable{
				AtMost(1, "x", "y"),
				Mandatory("x"),
				Mandatory("y"),
			},
		},
		{
			Name: "cardinality constraint forces alternative",
			Variables: []*DeppyEntity{
				entity("a", mandatoryConstraint(), dependencyConstraint("x", "y"), atMostConstraint(1, "x", "y")),
				entity("b", mandatoryConstraint(), dependencyConstraint("y")),
				entity("x"),
				entity("y"),
			},
			Installed: []Identifier{"a", "b", "y"},
		},
		{
			Name: "two dependencies satisfied by one entity",
			Variables: []*DeppyEntity{
				entity("a", mandatoryConstraint(), dependencyConstraint("y")),
				entity("b", mandatoryConstraint(), dependencyConstraint("x", "y")),
				entity("x"),
				entity("y"),
			},
			Installed: []Identifier{"a", "b", "y"},
		},
		{
			Name: "foo two dependencies satisfied by one entity",
			Variables: []*DeppyEntity{
				entity("a", mandatoryConstraint(), dependencyConstraint("y", "z", "m")),
				entity("b", mandatoryConstraint(), dependencyConstraint("x", "y")),
				entity("x"),
				entity("y"),
				entity("z"),
				entity("m"),
			},
			Installed: []Identifier{"a", "b", "y"},
		},
		{
			Name: "result size larger than minimum due to preference",
			Variables: []*DeppyEntity{
				entity("a", mandatoryConstraint(), dependencyConstraint("x", "y")),
				entity("b", mandatoryConstraint(), dependencyConstraint("y")),
				entity("x"),
				entity("y"),
			},
			Installed: []Identifier{"a", "b", "x", "y"},
		},
		{
			Name: "only the least preferable choice is acceptable",
			Variables: []*DeppyEntity{
				entity("a", mandatoryConstraint(), dependencyConstraint("a1", "a2")),
				entity("a1", conflictConstraint("c1"), conflictConstraint("c2")),
				entity("a2", conflictConstraint("c1")),
				entity("b", mandatoryConstraint(), dependencyConstraint("b1", "b2")),
				entity("b1", conflictConstraint("c1"), conflictConstraint("c2")),
				entity("b2", conflictConstraint("c1")),
				entity("c", mandatoryConstraint(), dependencyConstraint("c1", "c2")),
				entity("c1"),
				entity("c2"),
			},
			Installed: []Identifier{"a", "a2", "b", "b2", "c", "c2"},
		},
		{
			Name: "preferences respected with multiple dependencies per entity",
			Variables: []*DeppyEntity{
				entity("a", mandatoryConstraint(), dependencyConstraint("x1", "x2"), dependencyConstraint("y1", "y2")),
				entity("x1"),
				entity("x2"),
				entity("y1"),
				entity("y2"),
			},
			Installed: []Identifier{"a", "x1", "y1"},
		},
		//{
		//	Name: "all constraint works",
		//	Variables: []*DeppyEntity{
		//		entity("a", Mandatory(), All(Dependency("x1", "x2", "x3"), Dependency("x5", "x4", "x3"))),
		//		entity("x1"),
		//		entity("x2"),
		//		entity("x3"),
		//		entity("x4"),
		//		entity("x5"),
		//	},
		//	Installed: []Identifier{"a", "x3"},
		//},
	} {
		t.Run(tt.Name, func(t *testing.T) {
			assert := assert.New(t)

			var traces bytes.Buffer
			s, err := New(WithInput(tt.Variables), WithTracer(LoggingTracer{Writer: &traces}))
			if err != nil {
				t.Fatalf("failed to initialize solver: %s", err)
			}

			installed, err := s.Solve(context.TODO())

			if installed != nil {
				sort.SliceStable(installed, func(i, j int) bool {
					return installed[i].Identifier < installed[j].Identifier
				})
			}

			var ids []Identifier
			for _, entity := range installed {
				ids = append(ids, Identifier(entity.Identifier))
			}
			assert.Equal(tt.Installed, ids)

			if tt.Error != nil {
				assert.ElementsMatch(tt.Error, err)
			}

			if t.Failed() {
				t.Logf("\n%s", traces.String())
			}
		})
	}
}

func TestDuplicateIdentifier(t *testing.T) {
	_, err := New(WithInput([]*DeppyEntity{
		entity("a"),
		entity("a"),
	}))
	assert.Equal(t, DuplicateIdentifier("a"), err)
}
