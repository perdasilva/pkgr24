package solver

import (
	"context"
	"math/rand"
	"strconv"
	"testing"
)

var BenchmarkInput = func() []*DeppyEntity {
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

	entity := func(i int) *DeppyEntity {
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
			var d []Identifier
			for x := 0; x < n; x++ {
				y := i
				for y == i {
					y = rand.Intn(length)
				}
				d = append(d, Identifier(id(y)))
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
					Conflicts: &ConflictConstraint{
						Id: id(y),
					},
				}
				c = append(c, constr)
			}
		}
		return &DeppyEntity{
			Identifier:  id(i),
			Constraints: c,
		}
	}

	rand.Seed(seed)
	result := make([]*DeppyEntity, length)
	for i := range result {
		result[i] = entity(i)
	}
	return result
}()

func BenchmarkSolve(b *testing.B) {
	for i := 0; i < b.N; i++ {
		s, err := New(WithInput(BenchmarkInput))
		if err != nil {
			b.Fatalf("failed to initialize solver: %s", err)
		}
		s.Solve(context.Background())
	}
}

func BenchmarkNewInput(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_, err := New(WithInput(BenchmarkInput))
		if err != nil {
			b.Fatalf("failed to initialize solver: %s", err)
		}
	}
}
