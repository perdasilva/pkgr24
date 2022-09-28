package solver

// Identifier values uniquely identify particular Entities within
// the input to a single call to Solve.
type Identifier string

func (id Identifier) String() string {
	return string(id)
}

// IdentifierFromString returns an Identifier based on a provided
// string.
func IdentifierFromString(s string) Identifier {
	return Identifier(s)
}

// *DeppyEntity values are the basic unit of problems and solutions
// understood by this package.
type Variable interface {
	// Identifier returns the Identifier that uniquely identifies
	// this *DeppyEntity among all other Entities in a given
	// problem.
	Identifier() Identifier
	// Constraints returns the set of constraints that apply to
	// this *DeppyEntity.
	Constraints() []Constraint
}

// zeroVariable is returned by VariableOf in error cases.
type zeroVariable struct{}

var _ Variable = zeroVariable{}

func (zeroVariable) Identifier() Identifier {
	return ""
}

func (zeroVariable) Constraints() []Constraint {
	return nil
}
