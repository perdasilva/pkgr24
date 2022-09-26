package solver

import (
	"fmt"
	"strings"

	"github.com/go-air/gini/logic"
	"github.com/go-air/gini/z"
)

type Constraint interface {
	Order() []Identifier
	Anchor() bool
	Apply(c *logic.C, lm *LitMapping) z.Lit
	String() string
	Subject() Identifier
}

type mandatory struct {
	subject Identifier
}

func (constraint *mandatory) String() string {
	return fmt.Sprintf("%s is mandatory", constraint.subject)
}

func (constraint *mandatory) Apply(_ *logic.C, lm *LitMapping) z.Lit {
	return lm.LitOf(constraint.subject)
}

func (constraint *mandatory) Order() []Identifier {
	return nil
}

func (constraint *mandatory) Anchor() bool {
	return true
}

func (constraint *mandatory) Subject() Identifier {
	return constraint.subject
}

func Mandatory(subject Identifier) Constraint {
	return &mandatory{
		subject: subject,
	}
}

type prohibited struct {
	subject Identifier
}

func (constraint *prohibited) Order() []Identifier {
	return nil
}

func (constraint *prohibited) Anchor() bool {
	return false
}

func (constraint *prohibited) Apply(c *logic.C, lm *LitMapping) z.Lit {
	return lm.LitOf(constraint.subject).Not()
}

func (constraint *prohibited) String() string {
	return fmt.Sprintf("%s is prohibited", constraint.subject)
}

func (constraint *prohibited) Subject() Identifier {
	return constraint.subject
}

func Prohibited(subject Identifier) Constraint {
	return &prohibited{
		subject: subject,
	}
}

type dependency struct {
	subject      Identifier
	dependencies []Identifier
}

func (constraint *dependency) String() string {
	if len(constraint.dependencies) == 0 {
		return fmt.Sprintf("%s has a dependency without any candidates to satisfy it", constraint.subject)
	}
	s := make([]string, len(constraint.dependencies))
	for i, each := range constraint.dependencies {
		s[i] = string(each)
	}
	return fmt.Sprintf("%s requires at least one of %s", constraint.subject, strings.Join(s, ", "))
}

func (constraint *dependency) Apply(c *logic.C, lm *LitMapping) z.Lit {
	m := lm.LitOf(constraint.subject).Not()
	ms := make([]z.Lit, len(constraint.dependencies))
	for i, each := range constraint.dependencies {
		ms[i] = lm.LitOf(each)
	}
	return c.Ors(append(ms, m)...)
}

func (constraint *dependency) Order() []Identifier {
	return constraint.dependencies
}

func (constraint *dependency) Anchor() bool {
	return false
}

func (constraint *dependency) Subject() Identifier {
	return constraint.subject
}

// Dependency returns a Constraint that will only permit solutions
// containing a given *DeppyEntity on the condition that at least one
// of the Variables identified by the given Identifiers also
// appears in the solution. Identifiers appearing earlier in the
// argument list have higher preference than those appearing later.
func Dependency(subject Identifier, dependencies ...Identifier) Constraint {
	return &dependency{
		subject:      subject,
		dependencies: dependencies,
	}
}

type conflict struct {
	subject  Identifier
	conflict Identifier
}

func (constraint *conflict) String() string {
	return fmt.Sprintf("%s conflicts with %s", constraint.subject, constraint.conflict)
}

func (constraint *conflict) Apply(c *logic.C, lm *LitMapping) z.Lit {
	return c.Or(lm.LitOf(constraint.subject).Not(), lm.LitOf(constraint.conflict).Not())
}

func (constraint *conflict) Order() []Identifier {
	return nil
}

func (constraint *conflict) Anchor() bool {
	return false
}

func (constraint *conflict) Subject() Identifier {
	return constraint.subject
}

// Conflict returns a Constraint that will permit solutions containing
// either the constrained *DeppyEntity, the *DeppyEntity identified by
// the given Identifier, or neither, but not both.
func Conflict(subject Identifier, conlict Identifier) Constraint {
	return &conflict{
		subject:  subject,
		conflict: conlict,
	}
}

type atMost struct {
	ids []Identifier
	n   int
}

func (constraint *atMost) String() string {
	s := make([]string, len(constraint.ids))
	for i, each := range constraint.ids {
		s[i] = string(each)
	}
	return fmt.Sprintf("at most %d of %s are permitted", constraint.n, strings.Join(s, ", "))
}

func (constraint *atMost) Apply(c *logic.C, lm *LitMapping) z.Lit {
	ms := make([]z.Lit, len(constraint.ids))
	for i, each := range constraint.ids {
		ms[i] = lm.LitOf(each)
	}
	return c.CardSort(ms).Leq(constraint.n)
}

func (constraint *atMost) Order() []Identifier {
	return nil
}

func (constraint *atMost) Anchor() bool {
	return false
}

func (constraint *atMost) Subject() Identifier {
	return ""
}

// AtMost returns a Constraint that forbids solutions that contain
// more than n of the Variables identified by the given
// Identifiers.
func AtMost(n int, ids ...Identifier) Constraint {
	return &atMost{
		ids: ids,
		n:   n,
	}
}

type and struct {
	clauses []Constraint
}

func And(clauses ...Constraint) Constraint {
	return &and{
		clauses: clauses,
	}
}

func (constraint *and) String() string {
	collectIds := func() []string {
		ids := make([]string, len(constraint.clauses))
		for i, clause := range constraint.clauses {
			ids[i] = clause.String()
		}
		return ids
	}
	return fmt.Sprintf("%s are required", strings.Join(collectIds(), " and "))
}

func (constraint *and) Apply(c *logic.C, lm *LitMapping) z.Lit {
	collectTerms := func() []z.Lit {
		terms := make([]z.Lit, len(constraint.clauses))
		for i, clause := range constraint.clauses {
			terms[i] = clause.Apply(c, lm)
		}
		return terms
	}
	return c.Ands(collectTerms()...)
}

func (constraint *and) Order() []Identifier {
	return nil
}

func (constraint *and) Anchor() bool {
	return false
}

func (constraint *and) Subject() Identifier {
	return ""
}

type or struct {
	clauses []Constraint
}

func Or(clauses ...Constraint) Constraint {
	return &or{
		clauses: clauses,
	}
}

func (constraint *or) String() string {
	collectIds := func() []string {
		ids := make([]string, len(constraint.clauses))
		for i, clause := range constraint.clauses {
			ids[i] = clause.String()
		}
		return ids
	}
	return fmt.Sprintf("%s are required", strings.Join(collectIds(), " or "))
}

func (constraint *or) Apply(c *logic.C, lm *LitMapping) z.Lit {
	collectTerms := func() []z.Lit {
		terms := make([]z.Lit, len(constraint.clauses))
		for i, clause := range constraint.clauses {
			terms[i] = clause.Apply(c, lm)
		}
		return terms
	}
	return c.Ors(collectTerms()...)
}

func (constraint *or) Order() []Identifier {
	return nil
}

func (constraint *or) Anchor() bool {
	return false
}

func (constraint *or) Subject() Identifier {
	return ""
}

type not struct {
	clause Constraint
}

func Not(clause Constraint) Constraint {
	return &not{
		clause: clause,
	}
}

func (constraint *not) String() string {
	return fmt.Sprintf("not %s", constraint.clause.String())
}

func (constraint *not) Apply(c *logic.C, lm *LitMapping) z.Lit {
	return constraint.clause.Apply(c, lm).Not()
}

func (constraint *not) Order() []Identifier {
	return nil
}

func (constraint *not) Anchor() bool {
	return false
}

func (constraint *not) Subject() Identifier {
	return ""
}

// zeroConstraint is returned by ConstraintOf in error cases.
type zeroConstraint struct {
}

var _ Constraint = &zeroConstraint{}

var ZeroConstraint = &zeroConstraint{}

func (*zeroConstraint) String() string {
	return ""
}

func (*zeroConstraint) Apply(_ *logic.C, _ *LitMapping) z.Lit {
	return z.LitNull
}

func (*zeroConstraint) Order() []Identifier {
	return nil
}

func (*zeroConstraint) Anchor() bool {
	return false
}

func (*zeroConstraint) Subject() Identifier {
	return ""
}
