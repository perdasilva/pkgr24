package solver

import (
	"context"
	"fmt"
	"runtime"
	"sort"
	"strconv"

	"github.com/antonmedv/expr"
	"github.com/antonmedv/expr/vm"
	"github.com/blang/semver/v4"
	"github.com/tidwall/gjson"
)

type DeppyId string

type EntityProperty interface {
	Type() string
	Value() string
	Equals(other EntityProperty) bool
}

type DeppyEntity struct {
	Identifier DeppyId             `json:"id"`
	Properties map[string][]string `json:"properties,omitempty"`
}

func (e *DeppyEntity) DeepCopyInto(out *DeppyEntity) {
	*out = *e
	out.Identifier = e.Identifier

	props := make(map[string][]string, len(e.Properties))
	for key, values := range e.Properties {
		vals := make([]string, len(values))
		for i, _ := range values {
			vals[i] = values[i]
		}
		props[key] = vals
	}
	out.Properties = props
}

type ConstraintType string

type GroupByExpression struct {
	Expression string      `json:"expression"`
	program    *vm.Program `json:"-"`
}

func (e *GroupByExpression) evaluate(entity *DeppyEntity) ([]string, error) {
	env := map[string]interface{}{
		"Entity": entity,
		"InSemverRange": func(version string, versionRange string) (bool, error) {
			ver, err := semver.Parse(version)
			if err != nil {
				return false, err
			}
			rng, err := semver.ParseRange(versionRange)
			if err != nil {
				return false, err
			}
			return rng(ver), nil
		},
		"JSONPath": func(obj string, path string) (gjson.Result, error) {
			result := gjson.Get(obj, path)
			if result.Exists() {
				return result, nil
			}
			return gjson.Result{}, fmt.Errorf("object path (%s) not found for object: %s", path, obj)
		},
	}

	if e.program == nil {
		var err error
		e.program, err = expr.Compile(e.Expression)
		if err != nil {
			return nil, err
		}
	}

	output, err := expr.Run(e.program, env)
	if err != nil {
		return nil, nil
	}

	return output.([]string), nil
}

type SelectorExpression struct {
	Expression string      `json:"expression"`
	program    *vm.Program `json:"-"`
}

func (e *SelectorExpression) evaluate(entity *DeppyEntity) (bool, error) {
	env := map[string]interface{}{
		"Entity": entity,
		"InSemverRange": func(version string, versionRange string) (bool, error) {
			ver, err := semver.Parse(version)
			if err != nil {
				return false, err
			}
			rng, err := semver.ParseRange(versionRange)
			if err != nil {
				return false, err
			}
			return rng(ver), nil
		},
		"JSONPath": func(obj string, path string) (gjson.Result, error) {
			result := gjson.Get(obj, path)
			if result.Exists() {
				return result, nil
			}
			return gjson.Result{}, fmt.Errorf("object path (%s) not found for object: %s", path, obj)
		},
	}

	if e.program == nil {
		var err error
		e.program, err = expr.Compile(e.Expression)
		if err != nil {
			return false, err
		}
	}

	output, err := expr.Run(e.program, env)
	if err != nil {
		return false, nil
	}

	return output.(bool), nil
}

type SortExpression struct {
	Expression string      `json:"expression"`
	program    *vm.Program `json:"-"`
}

func (e *SortExpression) evaluate(entityOne *DeppyEntity, entityTwo *DeppyEntity) (int, error) {
	env := map[string]interface{}{
		"EntityOne": entityOne,
		"EntityTwo": entityTwo,
		"SemverCompare": func(versionOne string, versionTwo string) (int, error) {
			v1, err := semver.Parse(versionOne)
			if err != nil {
				return 0, err
			}
			v2, err := semver.Parse(versionTwo)
			if err != nil {
				return 0, err
			}
			return v1.Compare(v2), nil
		},
		"JSONPath": func(obj string, path string) (gjson.Result, error) {
			result := gjson.Get(obj, path)
			if result.Exists() {
				return result, nil
			}
			return gjson.Result{}, fmt.Errorf("object path (%s) not found for object: %s", path, obj)
		},
	}

	if e.program == nil {
		var err error
		e.program, err = expr.Compile(e.Expression, expr.AsInt64())
		if err != nil {
			return 0, err
		}
	}

	output, err := expr.Run(e.program, env)
	if err != nil {
		return 0, nil
	}

	return int(output.(int64)), nil
}

const (
	ConstraintTypeMandatory      = "mandatory"
	ConstraintTypeConflict       = "conflict"
	ConstraintTypeProhibited     = "prohibited"
	ConstraintTypeAtMost         = "atMost"
	ConstraintTypeDependency     = "dependency"
	ConstraintTypeGroupByBuilder = "groupByBuilder"
	ConstraintTypeFilterBuilder  = "filterBuilder"
	ConstraintTypeForEachBuilder = "forEachBuilder"
)

type DeppyConstraint struct {
	Type       ConstraintType            `json:"type"`
	Mandatory  *MandatoryConstraint      `json:"mandatory,omitempty"`
	Conflict   *ConflictConstraint       `json:"conflicts,omitempty"`
	Prohibited *ProhibitedConstraint     `json:"prohibited,omitempty"`
	AtMost     *AtMostConstraint         `json:"atMost,omitempty"`
	Dependency *DependencyConstraint     `json:"dependency,omitempty"`
	GroupBy    *GroupByConstraintBuilder `json:"groupByBuilder,omitempty"`
	Filter     *FilterConstraintBuilder  `json:"filterBuilder,omitempty"`
	ForEach    *ForEachConstraintBuilder `json:"forEachBuilder,omitempty"`
}

type ConstraintBinder interface {
	BindSubject(subject DeppyId)
	BindIds(ids []DeppyId)
}

func (c *DeppyConstraint) DeepCopyInto(out *DeppyConstraint) {
	*out = *c
	out.Type = c.Type
	switch c.Type {
	case ConstraintTypeMandatory:
		*out.Mandatory = *c.Mandatory
	case ConstraintTypeConflict:
		*out.Conflict = *c.Conflict
	case ConstraintTypeProhibited:
		*out.Prohibited = *c.Prohibited
	case ConstraintTypeAtMost:
		*out.AtMost = *c.AtMost
	case ConstraintTypeDependency:
		*out.Dependency = *c.Dependency
	case ConstraintTypeGroupByBuilder:
		*out.GroupBy = *c.GroupBy
	case ConstraintTypeFilterBuilder:
		*out.Filter = *c.Filter
	case ConstraintTypeForEachBuilder:
		*out.ForEach = *c.ForEach
	}
}

func (c *DeppyConstraint) ToSolverConstraints(universe []*DeppyEntity) ([]Constraint, error) {
	constr, err := func() (SolverConstraintConverter, error) {
		switch c.Type {
		case ConstraintTypeMandatory:
			return c.Mandatory, nil
		case ConstraintTypeConflict:
			return c.Conflict, nil
		case ConstraintTypeProhibited:
			return c.Prohibited, nil
		case ConstraintTypeAtMost:
			return c.AtMost, nil
		case ConstraintTypeDependency:
			return c.Dependency, nil
		case ConstraintTypeGroupByBuilder:
			return c.GroupBy, nil
		case ConstraintTypeFilterBuilder:
			return c.Filter, nil
		case ConstraintTypeForEachBuilder:
			return c.ForEach, nil
		}
		return nil, fmt.Errorf("unrecognized constraint type (%s)", c.Type)
	}()
	if err != nil {
		return nil, err
	}
	if constr == nil {
		return nil, fmt.Errorf("constraint of type (%s) is undefined", c.Type)
	}
	solverConstraints, err := constr.ToSolverConstraints(universe)
	if err != nil {
		return nil, err
	}
	return solverConstraints, nil
}

func (c *DeppyConstraint) ToConstraintBinder() ConstraintBinder {
	constr, err := func() (ConstraintBinder, error) {
		switch c.Type {
		case ConstraintTypeMandatory:
			return c.Mandatory, nil
		case ConstraintTypeConflict:
			return c.Conflict, nil
		case ConstraintTypeProhibited:
			return c.Prohibited, nil
		case ConstraintTypeAtMost:
			return c.AtMost, nil
		case ConstraintTypeDependency:
			return c.Dependency, nil
			//case ConstraintTypeGroupByBuilder:
			//	return c.GroupBy, nil
			//case ConstraintTypeFilterBuilder:
			//	return c.Filter, nil
			//case ConstraintTypeForEachBuilder:
			//	return c.ForEach, nil
		}
		return nil, fmt.Errorf("unrecognized constraint type (%s)", c.Type)
	}()
	if err != nil {
		panic(err)
	}
	return constr
}

type SolverConstraintConverter interface {
	ToSolverConstraints(universe []*DeppyEntity) ([]Constraint, error)
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
	N   string    `json:"limit"`
	Ids []DeppyId `json:"ids,omitempty"`
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
	return []Constraint{AtMost(limit, ids...)}, nil
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
	allConstraints := make([]Constraint, 0)
	for _, constr := range c.Constraints {
		solverConstraints, err := constr.ToSolverConstraints(universe)
		if err != nil {
			return nil, err
		}
		allConstraints = append(allConstraints, solverConstraints...)
	}
	return []Constraint{And(allConstraints...)}, nil
}

func (c *AndConstraint) BindSubject(subject DeppyId) {
	for _, constr := range c.Constraints {
		constr.ToConstraintBinder().BindSubject(subject)
	}
}

func (c *AndConstraint) BindIds(ids []DeppyId) {
	for _, constr := range c.Constraints {
		constr.ToConstraintBinder().BindIds(ids)
	}
}

type OrConstraint struct {
	Constraints []DeppyConstraint `json:"constraints"`
}

func (c *OrConstraint) ToSolverConstraints(universe []*DeppyEntity) (constrs []Constraint, err error) {
	allConstraints := make([]Constraint, 0)
	for _, constr := range c.Constraints {
		solverConstraints, err := constr.ToSolverConstraints(universe)
		if err != nil {
			return nil, err
		}
		allConstraints = append(allConstraints, solverConstraints...)
	}
	return []Constraint{Or(allConstraints...)}, nil
}

func (c *OrConstraint) BindSubject(subject DeppyId) {
	for _, constr := range c.Constraints {
		constr.ToConstraintBinder().BindSubject(subject)
	}
}

func (c *OrConstraint) BindIds(ids []DeppyId) {
	for _, constr := range c.Constraints {
		constr.ToConstraintBinder().BindIds(ids)
	}
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
	constrs = make([]Constraint, 0, len(idMap))
	for subject, entities := range idMap {
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

		constr := &DeppyConstraint{}
		c.Constraint.DeepCopyInto(constr)
		constr.ToConstraintBinder().BindSubject(c.formatSubject(DeppyId(subject)))
		constr.ToConstraintBinder().BindIds(ids)

		solverConstraints, err := constr.ToSolverConstraints(universe)
		if err != nil {
			return nil, err
		}
		constrs = append(constrs, solverConstraints...)
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
	constr := &DeppyConstraint{}
	c.Constraint.DeepCopyInto(constr)
	constr.ToConstraintBinder().BindIds(ids)

	return constr.ToSolverConstraints(universe)
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

	constrs = make([]Constraint, 0)
	for i, _ := range entities {
		constr := &DeppyConstraint{}
		c.Constraint.DeepCopyInto(constr)
		constr.ToConstraintBinder().BindSubject(entities[i].Identifier)
		solverConstraints, err := constr.ToSolverConstraints(universe)
		if err != nil {
			return nil, err
		}
		constrs = append(constrs, solverConstraints...)
	}

	return constrs, nil
}

type DeppySource interface {
	GetEntities(ctx context.Context) ([]*DeppyEntity, error)
	GetConstraints(ctx context.Context) ([]DeppyConstraint, error)
}
