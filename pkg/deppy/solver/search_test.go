//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 -o zz_search_test.go ../../../../../vendor/github.com/go-air/gini/inter S

package solver

import (
	"context"
	"testing"

	"github.com/go-air/gini/inter"
	"github.com/go-air/gini/z"
	"github.com/stretchr/testify/assert"
)

type TestScopeCounter struct {
	depth *int
	inter.S
}

func (c *TestScopeCounter) Test(dst []z.Lit) (result int, out []z.Lit) {
	result, out = c.S.Test(dst)
	*c.depth++
	return
}

func (c *TestScopeCounter) Untest() (result int) {
	result = c.S.Untest()
	*c.depth--
	return
}

func TestSearch(t *testing.T) {
	type tc struct {
		Name          string
		Entities      []*DeppyEntity
		Constraints   []DeppyConstraint
		TestReturns   []int
		UntestReturns []int
		Result        int
		Assumptions   []Identifier
	}

	for _, tt := range []tc{
		{
			Name: "children popped from back of deque when guess popped",
			Entities: []*DeppyEntity{
				entity("a"),
				entity("b"),
				entity("c"),
			},
			Constraints: []DeppyConstraint{
				mandatoryConstraint("a"),
				dependencyConstraint("a", "c"),
				mandatoryConstraint("b"),
			},
			TestReturns:   []int{0, -1},
			UntestReturns: []int{-1, -1},
			Result:        -1,
			Assumptions:   nil,
		},
		{
			Name: "candidates exhausted",
			Entities: []*DeppyEntity{
				entity("a"),
				entity("b"),
				entity("x"),
				entity("y"),
			},
			Constraints: []DeppyConstraint{
				mandatoryConstraint("a"),
				dependencyConstraint("a", "x"),
				mandatoryConstraint("b"),
				dependencyConstraint("b", "y"),
			},
			TestReturns:   []int{0, 0, -1, 1},
			UntestReturns: []int{0},
			Result:        1,
			Assumptions:   []Identifier{"a", "b", "y"},
		},
	} {
		t.Run(tt.Name, func(t *testing.T) {
			assert := assert.New(t)

			var s FakeS
			for i, result := range tt.TestReturns {
				s.TestReturnsOnCall(i, result, nil)
			}
			for i, result := range tt.UntestReturns {
				s.UntestReturnsOnCall(i, result)
			}

			var depth int
			counter := &TestScopeCounter{depth: &depth, S: &s}

			lits, err := NewLitMapping(tt.Entities, tt.Constraints)
			assert.NoError(err)
			h := search{
				s:      counter,
				lits:   lits,
				tracer: DefaultTracer{},
			}

			var anchors []z.Lit
			for _, id := range h.lits.AnchorIdentifiers() {
				anchors = append(anchors, h.lits.LitOf(id))
			}

			result, ms, _ := h.Do(context.Background(), anchors)

			assert.Equal(tt.Result, result)
			var ids []Identifier
			for _, m := range ms {
				ids = append(ids, Identifier(lits.VariableOf(m).Identifier))
			}

			assert.Equal(tt.Assumptions, ids)
			assert.Equal(0, depth)
		})
	}
}
