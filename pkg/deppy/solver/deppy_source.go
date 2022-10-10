package solver

import (
	"context"
	"fmt"
	"runtime"
	"sort"
	"strconv"
)

type DeppyId string

type ConstraintType string

const (
	ConstraintTypeMandatory  = "mandatory"
	ConstraintTypeConflict   = "conflict"
	ConstraintTypeProhibited = "prohibited"
	ConstraintTypeAtMost     = "atMost"
	ConstraintTypeDependency = "dependency"
)

type DeppyConstraint struct {
	Type       ConstraintType        `json:"type"`
	Mandatory  *MandatoryConstraint  `json:"mandatory,omitempty"`
	Conflict   *ConflictConstraint   `json:"conflicts,omitempty"`
	Prohibited *ProhibitedConstraint `json:"prohibited,omitempty"`
	AtMost     *AtMostConstraint     `json:"atMost,omitempty"`
	Dependency *DependencyConstraint `json:"dependency,omitempty"`
}

type MandatoryConstraint struct {
	Subject DeppyId `json:"subject,omitempty"`
}

func (c *MandatoryConstraint) ToSolverConstraints(_ []*DeppyEntity) ([]Constraint, error) {
	return []Constraint{Mandatory(Identifier(c.Subject))}, nil
}

func (c *MandatoryConstraint) BindSubject(subject DeppyId) {
	c.Subject = subject
}

func (c *MandatoryConstraint) BindIds(_ []DeppyId) {
	// do nothing
}

type ProhibitedConstraint struct {
	Subject DeppyId `json:"subject,omitempty"`
}

func (c *ProhibitedConstraint) ToSolverConstraints(_ []*DeppyEntity) ([]Constraint, error) {
	return []Constraint{Prohibited(Identifier(c.Subject))}, nil
}

func (c *ProhibitedConstraint) BindSubject(subject DeppyId) {
	c.Subject = subject
}

func (c *ProhibitedConstraint) BindIds(_ []DeppyId) {
	// do nothing
}

type ConflictConstraint struct {
	Subject DeppyId   `json:"subject,omitempty"`
	Ids     []DeppyId `json:"ids,omitempty"`
}

func (c *ConflictConstraint) ToSolverConstraints(_ []*DeppyEntity) ([]Constraint, error) {
	constrs := make([]Constraint, len(c.Ids))
	for i, _ := range c.Ids {
		constrs[i] = Conflict(Identifier(c.Subject), Identifier(c.Ids[i]))
	}
	return constrs, nil
}

func (c *ConflictConstraint) BindSubject(subject DeppyId) {
	c.Subject = subject
}

func (c *ConflictConstraint) BindIds(ids []DeppyId) {
	c.Ids = ids
}

type AtMostConstraint struct {
	Subject DeppyId   `json:"subject,omitempty"`
	N       string    `json:"limit"`
	Ids     []DeppyId `json:"ids,omitempty"`
}

func (c *AtMostConstraint) ToSolverConstraints(universe []*DeppyEntity) (constrs []Constraint, err error) {
	defer func() {
		if panicErr := recover(); panicErr != nil {
			constrs = nil
			err = panicErr.(runtime.Error)
		}
	}()

	limit, _ := strconv.Atoi(c.N)

	ids := make([]Identifier, len(c.Ids))
	for i, _ := range c.Ids {
		ids[i] = Identifier(c.Ids[i])
	}
	return []Constraint{AtMost(Identifier(c.Subject), limit, ids...)}, nil
}

func (c *AtMostConstraint) BindSubject(_ DeppyId) {
	// do nothing
}

func (c *AtMostConstraint) BindIds(ids []DeppyId) {
	c.Ids = ids
}

type DependencyConstraint struct {
	Subject DeppyId   `json:"subject,omitempty"`
	Ids     []DeppyId `json:"ids,omitempty"`
}

func (c *DependencyConstraint) ToSolverConstraints(universe []*DeppyEntity) (constrs []Constraint, err error) {
	ids := make([]Identifier, len(c.Ids))
	for i, _ := range c.Ids {
		ids[i] = Identifier(c.Ids[i])
	}
	return []Constraint{Dependency(Identifier(c.Subject), ids...)}, nil
}

func (c *DependencyConstraint) BindSubject(subject DeppyId) {
	c.Subject = subject
}

func (c *DependencyConstraint) BindIds(ids []DeppyId) {
	c.Ids = ids
}

type AndConstraint struct {
	Constraints []DeppyConstraint `json:"constraints"`
}

func (c *AndConstraint) ToSolverConstraints(universe []*DeppyEntity) (constrs []Constraint, err error) {
	return nil, nil
}

type OrConstraint struct {
	Constraints []DeppyConstraint `json:"constraints"`
}

func (c *OrConstraint) ToSolverConstraints(universe []*DeppyEntity) (constrs []Constraint, err error) {
	return nil, nil
}

type GroupByConstraintBuilder struct {
	GroupBy        GroupByExpression `json:"groupBy"`
	SubjectFormat  string            `json:"subjectFormat,omitempty"`
	Constraint     DeppyConstraint   `json:"constraint"`
	SortExpression *SortExpression   `json:"sort,omitempty"`
}

func (c *GroupByConstraintBuilder) formatSubject(subject DeppyId) DeppyId {
	if c.SubjectFormat != "" {
		return DeppyId(fmt.Sprintf(c.SubjectFormat, subject))
	}
	return subject
}

func (c *GroupByConstraintBuilder) ToSolverConstraints(universe []*DeppyEntity) (constrs []Constraint, err error) {
	defer func() {
		if panicErr := recover(); panicErr != nil {
			constrs = nil
			err = panicErr.(runtime.Error)
		}
	}()
	idMap := map[string][]*DeppyEntity{}
	for _, entity := range universe {
		ids, err := c.GroupBy.evaluate(entity)
		if err != nil {
			return nil, err
		}
		for _, id := range ids {
			idMap[id] = append(idMap[id], entity)
		}
	}

	return constrs, nil
}

type FilterConstraintBuilder struct {
	FilterExpression SelectorExpression `json:"filter"`
	SortExpression   *SortExpression    `json:"sort,omitempty"`
	Constraint       DeppyConstraint    `json:"constraint"`
}

func (c *FilterConstraintBuilder) ToSolverConstraints(universe []*DeppyEntity) (constrs []Constraint, err error) {
	defer func() {
		if panicErr := recover(); panicErr != nil {
			constrs = nil
			err = panicErr.(runtime.Error)
		}
	}()

	entities := make([]*DeppyEntity, 0)
	for _, entity := range universe {
		eval, err := c.FilterExpression.evaluate(entity)
		if err != nil {
			return nil, err
		}
		if eval == true {
			entities = append(entities, entity)
		}
	}
	if c.SortExpression != nil {
		sort.Slice(entities, func(i, j int) bool {
			eval, err := c.SortExpression.evaluate(entities[i], entities[j])
			if err != nil {
				panic(err)
			}
			return eval < 0
		})
	}
	ids := make([]DeppyId, len(entities))
	for i, _ := range entities {
		ids[i] = entities[i].Identifier
	}
	return nil, err
}

type ForEachConstraintBuilder struct {
	SubjectFormat    string             `json:"subjectFormat,omitempty"`
	FilterExpression SelectorExpression `json:"filter"`
	Constraint       DeppyConstraint    `json:"constraint"`
}

func (c *ForEachConstraintBuilder) formatSubject(subject DeppyId) DeppyId {
	if c.SubjectFormat != "" {
		return DeppyId(fmt.Sprintf(c.SubjectFormat, subject))
	}
	return subject
}

func (c *ForEachConstraintBuilder) ToSolverConstraints(universe []*DeppyEntity) (constrs []Constraint, err error) {
	entities := make([]*DeppyEntity, 0)
	for _, entity := range universe {
		eval, err := c.FilterExpression.evaluate(entity)
		if err != nil {
			return nil, err
		}
		if eval == true {
			entities = append(entities, entity)
		}
	}
	return constrs, nil
}

type DeppySource interface {
	GetEntities(ctx context.Context) ([]*DeppyEntity, error)
}
