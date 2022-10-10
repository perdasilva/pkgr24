package solver

type ConstraintBuilderEntity struct {
	id          DeppyId
	constraints map[DeppyId]map[string]Constraint
}

func (e *ConstraintBuilderEntity) Mandatory() {

}

func (e *ConstraintBuilderEntity) Prohibited() {

}

func (e *ConstraintBuilderEntity) Conflict(id DeppyId) {

}

func (e *ConstraintBuilderEntity) Dependency(constraintId string) {
}

func (e *ConstraintBuilderEntity) Cardinal(constraintId string) {

}
