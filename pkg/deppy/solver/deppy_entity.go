package solver

type DeppyEntity struct {
	Identifier DeppyId           `json:"id"`
	Properties map[string]string `json:"properties,omitempty"`
}

func (e *DeppyEntity) DeepCopyInto(out *DeppyEntity) {
	*out = *e
	out.Identifier = e.Identifier

	props := make(map[string]string, len(e.Properties))
	for key, value := range e.Properties {
		props[key] = value
	}
	out.Properties = props
}

func (e *DeppyEntity) Mandatory() Constraint {
	return Mandatory(Identifier(e.Identifier))
}

func (e *DeppyEntity) Prohibited() Constraint {
	return Prohibited(Identifier(e.Identifier))
}

func (e *DeppyEntity) Conflict(conflict *DeppyEntity) Constraint {
	return Conflict(Identifier(e.Identifier), Identifier(conflict.Identifier))
}

func (e *DeppyEntity) AtMost(n int, dependencies ...*DeppyEntity) Constraint {
	ids := make([]Identifier, len(dependencies))
	for i, _ := range dependencies {
		ids[i] = Identifier(dependencies[i].Identifier)
	}
	return AtMost(Identifier(e.Identifier), n, ids...)
}

func (e *DeppyEntity) Dependency(dependencies ...*DeppyEntity) Constraint {
	ids := make([]Identifier, len(dependencies))
	for i, _ := range dependencies {
		ids[i] = Identifier(dependencies[i].Identifier)
	}
	return Dependency(Identifier(e.Identifier), ids...)
}
