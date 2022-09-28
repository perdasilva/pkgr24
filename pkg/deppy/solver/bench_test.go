package solver

import (
	"context"
	"math/rand"
	"strconv"
	"testing"
)

var BenchmarkInput, BenchmarkConstraints = func() ([]*DeppyEntity, []DeppyConstraint) {
	const (
		length      = 256
		seed        = 9
		pMandatory  = .1
		pDependency = .15
		nDependency = 6
		pConflict   = .05
		nConflict   = 3
	)

	id := func(i int) DeppyId {
		return DeppyId(strconv.Itoa(i))
	}

	input := func(i int) (*DeppyEntity, []DeppyConstraint) {
		var c []DeppyConstraint
		if rand.Float64() < pMandatory {
			constr := DeppyConstraint{
				Type:      ConstraintTypeMandatory,
				Mandatory: &MandatoryConstraint{},
			}
			c = append(c, constr)
		}
		if rand.Float64() < pDependency {
			n := rand.Intn(nDependency-1) + 1
			var d []DeppyId
			for x := 0; x < n; x++ {
				y := i
				for y == i {
					y = rand.Intn(length)
				}
				d = append(d, id(y))
			}
			constr := DeppyConstraint{
				Type: ConstraintTypeDependency,
				Dependency: &DependencyConstraint{
					Ids: d,
				},
			}
			c = append(c, constr)
		}
		if rand.Float64() < pConflict {
			n := rand.Intn(nConflict-1) + 1
			for x := 0; x < n; x++ {
				y := i
				for y == i {
					y = rand.Intn(length)
				}
				constr := DeppyConstraint{
					Type: ConstraintTypeConflict,
					Conflict: &ConflictConstraint{
						Ids: []DeppyId{id(y)},
					},
				}
				c = append(c, constr)
			}
		}
		return &DeppyEntity{
			Identifier: id(i),
		}, c
	}

	rand.Seed(seed)
	result := make([]*DeppyEntity, length)
	constraints := make([]DeppyConstraint, 0)
	for i := range result {
		entity, constrs := input(i)
		result[i] = entity
		constraints = append(constraints, constrs...)
	}
	return result, constraints
}()

func BenchmarkSolve(b *testing.B) {
	for i := 0; i < b.N; i++ {
		s, err := New(WithInput(BenchmarkInput, BenchmarkConstraints))
		if err != nil {
			b.Fatalf("failed to initialize solver: %s", err)
		}
		s.Solve(context.Background())
	}
}

func BenchmarkNewInput(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_, err := New(WithInput(BenchmarkInput, BenchmarkConstraints))
		if err != nil {
			b.Fatalf("failed to initialize solver: %s", err)
		}
	}
}
