package solver

import (
	"bytes"
	"context"
	"fmt"
	"sort"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
)

func entity(id DeppyId) *DeppyEntity {
	return &DeppyEntity{
		Identifier: id,
	}
}

func mandatoryConstraint(subject DeppyId) DeppyConstraint {
	return DeppyConstraint{
		Type: ConstraintTypeMandatory,
		Mandatory: &MandatoryConstraint{
			Subject: subject,
		},
	}
}

func prohibitedConstraint(subject DeppyId) DeppyConstraint {
	return DeppyConstraint{
		Type: ConstraintTypeProhibited,
		Prohibited: &ProhibitedConstraint{
			Subject: subject,
		},
	}
}

func conflictConstraint(subject DeppyId, id DeppyId) DeppyConstraint {
	return DeppyConstraint{
		Type: ConstraintTypeConflict,
		Conflict: &ConflictConstraint{
			Subject: subject,
			Ids:     []DeppyId{id},
		},
	}
}

func atMostConstraint(n int, ids ...DeppyId) DeppyConstraint {
	return DeppyConstraint{
		Type: ConstraintTypeAtMost,
		AtMost: &AtMostConstraint{
			N:   strconv.Itoa(n),
			Ids: ids,
		},
	}
}

func dependencyConstraint(subject DeppyId, ids ...DeppyId) DeppyConstraint {
	return DeppyConstraint{
		Type: ConstraintTypeDependency,
		Dependency: &DependencyConstraint{
			Subject: subject,
			Ids:     ids,
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
		Name        string
		Entities    []*DeppyEntity
		Constraints []DeppyConstraint
		Installed   []Identifier
		Error       error
	}

	for _, tt := range []tc{
		{
			Name: "no variables",
		},
		{
			Name:     "unnecessary entity is not installed",
			Entities: []*DeppyEntity{entity("a")},
		},
		{
			Name:     "single mandatory entity is installed",
			Entities: []*DeppyEntity{entity("a")},
			Constraints: []DeppyConstraint{
				mandatoryConstraint("a"),
			},
			Installed: []Identifier{"a"},
		},
		{
			Name:     "both mandatory and prohibited produce error",
			Entities: []*DeppyEntity{entity("a")},
			Constraints: []DeppyConstraint{
				mandatoryConstraint("a"),
				prohibitedConstraint("a"),
			},
			Error: NotSatisfiable{
				Mandatory("a"),
				Prohibited("a"),
			},
		},
		{
			Name: "dependency is installed",
			Entities: []*DeppyEntity{
				entity("a"),
				entity("b"),
			},
			Constraints: []DeppyConstraint{
				mandatoryConstraint("b"),
				dependencyConstraint("b", "a"),
			},
			Installed: []Identifier{"a", "b"},
		},
		{
			Name: "transitive dependency is installed",
			Entities: []*DeppyEntity{
				entity("a"),
				entity("b"),
				entity("c"),
			},
			Constraints: []DeppyConstraint{
				dependencyConstraint("b", "a"),
				mandatoryConstraint("c"),
				dependencyConstraint("c", "b"),
			},
			Installed: []Identifier{"a", "b", "c"},
		},
		{
			Name: "both dependencies are installed",
			Entities: []*DeppyEntity{
				entity("a"),
				entity("b"),
				entity("c"),
			},
			Constraints: []DeppyConstraint{
				mandatoryConstraint("c"),
				dependencyConstraint("c", "a"),
				dependencyConstraint("c", "b"),
			},
			Installed: []Identifier{"a", "b", "c"},
		},
		{
			Name: "solution with first dependency is selected",
			Entities: []*DeppyEntity{
				entity("a"),
				entity("b"),
				entity("c"),
			},
			Constraints: []DeppyConstraint{
				conflictConstraint("b", "a"),
				mandatoryConstraint("c"),
				dependencyConstraint("c", "a", "b"),
			},
			Installed: []Identifier{"a", "c"},
		},
		{
			Name: "solution with only first dependency is selected",
			Entities: []*DeppyEntity{
				entity("a"),
				entity("b"),
				entity("c"),
			},
			Constraints: []DeppyConstraint{
				mandatoryConstraint("c"),
				dependencyConstraint("c", "a", "b"),
			},
			Installed: []Identifier{"a", "c"},
		},
		{
			Name: "solution with first dependency is selected (reverse)",
			Entities: []*DeppyEntity{
				entity("a"),
				entity("b"),
				entity("c"),
			},
			Constraints: []DeppyConstraint{
				conflictConstraint("b", "a"),
				mandatoryConstraint("c"),
				dependencyConstraint("c", "b", "a"),
			},
			Installed: []Identifier{"b", "c"},
		},
		{
			Name: "two mandatory but conflicting packages",
			Entities: []*DeppyEntity{
				entity("a"),
				entity("b"),
			},
			Constraints: []DeppyConstraint{
				mandatoryConstraint("a"),
				mandatoryConstraint("b"),
				conflictConstraint("b", "a"),
			},
			Error: NotSatisfiable{
				Mandatory("a"),
				Mandatory("b"),
				Conflict("b", "a"),
			},
		},
		{
			Name: "irrelevant dependencies don't influence search order",
			Entities: []*DeppyEntity{
				entity("a"),
				entity("b"),
				entity("x"),
				entity("y"),
			},
			Constraints: []DeppyConstraint{
				dependencyConstraint("a", "x", "y"),
				mandatoryConstraint("b"),
				dependencyConstraint("b", "y", "x"),
			},
			Installed: []Identifier{"b", "y"},
		},
		{
			Name: "cardinality constraint prevents resolution",
			Entities: []*DeppyEntity{
				entity("a"),
				entity("x"),
				entity("y"),
			},
			Constraints: []DeppyConstraint{
				mandatoryConstraint("a"),
				dependencyConstraint("a", "x", "y"),
				atMostConstraint(1, "x", "y"),
				mandatoryConstraint("x"),
				mandatoryConstraint("y"),
			},
			Error: NotSatisfiable{
				AtMost(1, "x", "y"),
				Mandatory("x"),
				Mandatory("y"),
			},
		},
		{
			Name: "cardinality constraint forces alternative",
			Entities: []*DeppyEntity{
				entity("a"),
				entity("b"),
				entity("x"),
				entity("y"),
			},
			Constraints: []DeppyConstraint{
				mandatoryConstraint("a"),
				dependencyConstraint("a", "x", "y"),
				atMostConstraint(1, "x", "y"),
				mandatoryConstraint("b"),
				dependencyConstraint("b", "y"),
			},
			Installed: []Identifier{"a", "b", "y"},
		},
		{
			Name: "two dependencies satisfied by one entity",
			Entities: []*DeppyEntity{
				entity("a"),
				entity("b"),
				entity("x"),
				entity("y"),
			},
			Constraints: []DeppyConstraint{
				mandatoryConstraint("a"),
				dependencyConstraint("a", "y"),
				mandatoryConstraint("b"),
				dependencyConstraint("b", "x", "y"),
			},
			Installed: []Identifier{"a", "b", "y"},
		},
		{
			Name: "foo two dependencies satisfied by one entity",
			Entities: []*DeppyEntity{
				entity("a"),
				entity("b"),
				entity("x"),
				entity("y"),
				entity("z"),
				entity("m"),
			},
			Constraints: []DeppyConstraint{
				mandatoryConstraint("a"),
				dependencyConstraint("a", "y", "z", "m"),
				mandatoryConstraint("b"),
				dependencyConstraint("b", "x", "y"),
			},
			Installed: []Identifier{"a", "b", "y"},
		},
		{
			Name: "result size larger than minimum due to preference",
			Entities: []*DeppyEntity{
				entity("a"),
				entity("b"),
				entity("x"),
				entity("y"),
			},
			Constraints: []DeppyConstraint{
				mandatoryConstraint("a"),
				dependencyConstraint("a", "x", "y"),
				mandatoryConstraint("b"),
				dependencyConstraint("b", "y"),
			},
			Installed: []Identifier{"a", "b", "x", "y"},
		},
		{
			Name: "only the least preferable choice is acceptable",
			Entities: []*DeppyEntity{
				entity("a"),
				entity("a1"),
				entity("a2"),
				entity("b"),
				entity("b1"),
				entity("b2"),
				entity("c"),
				entity("c1"),
				entity("c2"),
			},
			Constraints: []DeppyConstraint{
				mandatoryConstraint("a"),
				dependencyConstraint("a", "a1", "a2"),
				conflictConstraint("a1", "c1"),
				conflictConstraint("a1", "c2"),
				conflictConstraint("a2", "c1"),
				mandatoryConstraint("b"),
				dependencyConstraint("b", "b1", "b2"),
				conflictConstraint("b1", "c1"),
				conflictConstraint("b1", "c2"),
				conflictConstraint("b2", "c1"),
				mandatoryConstraint("c"),
				dependencyConstraint("c", "c1", "c2"),
			},
			Installed: []Identifier{"a", "a2", "b", "b2", "c", "c2"},
		},
		{
			Name: "preferences respected with multiple dependencies per entity",
			Entities: []*DeppyEntity{
				entity("a"),
				entity("x1"),
				entity("x2"),
				entity("y1"),
				entity("y2"),
			},
			Constraints: []DeppyConstraint{
				mandatoryConstraint("a"),
				dependencyConstraint("a", "x1", "x2"),
				dependencyConstraint("a", "y1", "y2"),
			},
			Installed: []Identifier{"a", "x1", "y1"},
		},
		//{
		//	Name: "all constraint works",
		//	Entities: []*DeppyEntity{
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
			s, err := New(WithInput(tt.Entities, tt.Constraints), WithTracer(LoggingTracer{Writer: &traces}))
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
	}, nil))
	assert.Equal(t, DuplicateIdentifier("a"), err)
}
