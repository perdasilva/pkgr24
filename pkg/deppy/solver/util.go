package solver

func ToIdentifierList(list []*DeppyEntity) []Identifier {
	ids := make([]Identifier, len(list))
	for idx, _ := range list {
		ids[idx] = Identifier(list[idx].Identifier)
	}
	return ids
}
