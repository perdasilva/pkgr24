package solver

import (
	"fmt"
	"strings"

	"github.com/go-air/gini/inter"
	"github.com/go-air/gini/logic"
	"github.com/go-air/gini/z"
)

type DuplicateIdentifier Identifier

func (e DuplicateIdentifier) Error() string {
	return fmt.Sprintf("duplicate identifier %q in input", Identifier(e))
}

type inconsistentLitMapping []error

func (inconsistentLitMapping) Error() string {
	return "internal solver failure"
}

// LitMapping performs translation between the input and output types of
// Solve (Constraints, Variables, etc.) and the variables that
// appear in the SAT formula.
type LitMapping struct {
	inorder                []*DeppyEntity
	variables              map[z.Lit]*DeppyEntity
	constraintsForVariable map[z.Lit][]Constraint
	lits                   map[Identifier]z.Lit
	constraints            map[z.Lit]Constraint
	c                      *logic.C
	errs                   inconsistentLitMapping
}

// NewLitMapping returns a new LitMapping with its state initialized based on
// the provided slice of Entities. This includes construction of
// the translation tables between Entities/Constraints and the
// inputs to the underlying solver.
func NewLitMapping(universe []*DeppyEntity, constraints []DeppyConstraint) (*LitMapping, error) {
	d := LitMapping{
		inorder:                universe,
		variables:              make(map[z.Lit]*DeppyEntity, len(universe)),
		constraintsForVariable: make(map[z.Lit][]Constraint, len(universe)),
		lits:                   make(map[Identifier]z.Lit, len(universe)),
		constraints:            make(map[z.Lit]Constraint),
		c:                      logic.NewCCap(len(universe)),
	}

	// First pass to assign lits:
	for _, entity := range universe {
		im := d.c.Lit()
		if _, ok := d.lits[Identifier(entity.Identifier)]; ok {
			return nil, DuplicateIdentifier(entity.Identifier)
		}
		d.lits[Identifier(entity.Identifier)] = im
		d.variables[im] = entity
	}

	for _, constraint := range constraints {
		solverConstraints, err := constraint.ToSolverConstraints(universe)
		if err != nil {
			return nil, err
		}

		for _, solverConstraint := range solverConstraints {
			m := solverConstraint.Apply(d.c, &d)
			if m == z.LitNull {
				// This constraint doesn't have a
				// useful representation in the SAT
				// inputs.
				continue
			}

			d.constraints[m] = solverConstraint
			litForVar := d.lits[solverConstraint.Subject()]
			d.constraintsForVariable[litForVar] = append(d.constraintsForVariable[litForVar], solverConstraint)
		}
	}

	return &d, nil
}

// LitOf returns the positive literal corresponding to the *DeppyEntity
// with the given Identifier.
func (d *LitMapping) LitOf(id Identifier) z.Lit {
	m, ok := d.lits[id]
	if ok {
		return m
	}
	d.errs = append(d.errs, fmt.Errorf("entity %q referenced but not provided", id))
	return z.LitNull
}

// VariableOf returns the *DeppyEntity corresponding to the provided
// literal, or a zeroVariable if no such *DeppyEntity exists.
func (d *LitMapping) VariableOf(m z.Lit) *DeppyEntity {
	i, ok := d.variables[m]
	if ok {
		return i
	}
	d.errs = append(d.errs, fmt.Errorf("no entity corresponding to %s", m))
	return &DeppyEntity{
		Identifier: "",
		Properties: nil,
	}
}

func (d *LitMapping) ConstraintsFor(m z.Lit) []Constraint {
	constrs, ok := d.constraintsForVariable[m]
	if ok {
		return constrs
	}
	return nil
}

// ConstraintOf returns the constraint application corresponding to
// the provided literal, or a zeroConstraint if no such constraint
// exists.
func (d *LitMapping) ConstraintOf(m z.Lit) Constraint {
	if a, ok := d.constraints[m]; ok {
		return a
	}
	d.errs = append(d.errs, fmt.Errorf("no constraint corresponding to %s", m))
	return ZeroConstraint
}

// Error returns a single error value that is an aggregation of all
// errors encountered during a LitMapping's lifetime, or nil if there have
// been no errors. A non-nil return value likely indicates a problem
// with the solver or constraint implementations.
func (d *LitMapping) Error() error {
	if len(d.errs) == 0 {
		return nil
	}
	s := make([]string, len(d.errs))
	for i, err := range d.errs {
		s[i] = err.Error()
	}
	return fmt.Errorf("%d errors encountered: %s", len(s), strings.Join(s, ", "))
}

// AddConstraints adds the current constraints encoded in the embedded circuit to the
// solver g
func (d *LitMapping) AddConstraints(g inter.S) {
	d.c.ToCnf(g)
}

func (d *LitMapping) AssumeConstraints(s inter.S) {
	for m := range d.constraints {
		s.Assume(m)
	}
}

// CardinalityConstrainer constructs a sorting network to provide
// cardinality constraints over the provided slice of literals. Any
// new clauses and variables are translated to CNF and taught to the
// given inter.Adder, so this function will panic if it is in a test
// context.
func (d *LitMapping) CardinalityConstrainer(g inter.Adder, ms []z.Lit) *logic.CardSort {
	clen := d.c.Len()
	cs := d.c.CardSort(ms)
	marks := make([]int8, clen, d.c.Len())
	for i := range marks {
		marks[i] = 1
	}
	for w := 0; w <= cs.N(); w++ {
		marks, _ = d.c.CnfSince(g, marks, cs.Leq(w))
	}
	return cs
}

// AnchorIdentifiers returns a slice containing the Identifiers of
// every variable with at least one "anchor" constraint, in the
// order they appear in the input.
func (d *LitMapping) AnchorIdentifiers() []Identifier {
	var ids []Identifier
	for _, entity := range d.inorder {
		id := d.lits[Identifier(entity.Identifier)]
		for _, constr := range d.constraintsForVariable[id] {
			if constr.Anchor() {
				ids = append(ids, Identifier(entity.Identifier))
				break
			}
		}
	}
	return ids
}

func (d *LitMapping) Variables(g inter.S) []*DeppyEntity {
	var result []*DeppyEntity
	for _, i := range d.inorder {
		if g.Value(d.LitOf(Identifier(i.Identifier))) {
			result = append(result, i)
		}
	}
	return result
}

func (d *LitMapping) Lits(dst []z.Lit) []z.Lit {
	if cap(dst) < len(d.inorder) {
		dst = make([]z.Lit, 0, len(d.inorder))
	}
	dst = dst[:0]
	for _, i := range d.inorder {
		m := d.LitOf(Identifier(i.Identifier))
		dst = append(dst, m)
	}
	return dst
}

func (d *LitMapping) Conflicts(g inter.Assumable) []Constraint {
	whys := g.Why(nil)
	as := make([]Constraint, 0, len(whys))
	for _, why := range whys {
		if a, ok := d.constraints[why]; ok {
			as = append(as, a)
		}
	}
	return as
}
