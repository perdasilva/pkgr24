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
	Identifier  DeppyId             `json:"id"`
	Properties  map[string][]string `json:"properties,omitempty"`
	Constraints []DeppyConstraint   `json:"constraints,omitempty"`
	Meta        bool                `json:"meta,omitempty"`
}

func (e *DeppyEntity) ExportConstraints(universe []*DeppyEntity) ([]Constraint, error) {
	entityConstraints := make([]Constraint, 0)
	for _, deppyConstraint := range e.Constraints {
		converter, err := e.toSolverConstraintConverter(deppyConstraint)
		if err != nil {
			return nil, err
		}
		constr, err := converter.toSolverConstraint(e.Identifier, universe)
		if err != nil {
			return nil, err
		}
		entityConstraints = append(entityConstraints, constr)
	}
	return entityConstraints, nil
}

func (e *DeppyEntity) toSolverConstraintConverter(deppyConstraint DeppyConstraint) (SolverConstraintConverter, error) {
	constr, err := func() (SolverConstraintConverter, error) {
		switch deppyConstraint.Type {
		case ConstraintTypeMandatory:
			return deppyConstraint.Mandatory, nil
		case ConstraintTypeConflict:
			return deppyConstraint.Conflicts, nil
		case ConstraintTypeProhibited:
			return deppyConstraint.Prohibited, nil
		case ConstraintTypeAtMost:
			return deppyConstraint.AtMost, nil
		case ConstraintTypeDependency:
			return deppyConstraint.Dependency, nil
		}
		return nil, fmt.Errorf("unrecognized constraint type (%s)", deppyConstraint.Type)
	}()
	if err != nil {
		return nil, err
	}
	if constr == nil {
		return nil, fmt.Errorf("constraint of type (%s) is undefined", deppyConstraint.Type)
	}
	return constr, nil
}

func (e *DeppyEntity) DeepCopyInto(out *DeppyEntity) {
	*out = *e
	out.Meta = e.Meta
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

	constrs := make([]DeppyConstraint, len(e.Constraints))
	for i, _ := range e.Constraints {
		constr := DeppyConstraint{
			Type: e.Constraints[i].Type,
		}
		switch constr.Type {
		case ConstraintTypeMandatory:
			constr.Mandatory = &MandatoryConstraint{}
		case ConstraintTypeProhibited:
			constr.Prohibited = &ProhibitedConstraint{}
		case ConstraintTypeConflict:
			constr.Conflicts = &ConflictConstraint{
				Id: e.Constraints[i].Conflicts.Id,
			}
		case ConstraintTypeAtMost:
			constr.AtMost = &AtMostConstraint{
				N:          e.Constraints[i].AtMost.N,
				Ids:        e.Constraints[i].AtMost.Ids,
				GroupBy:    e.Constraints[i].AtMost.GroupBy,
				Selector:   e.Constraints[i].AtMost.Selector,
				Comparator: e.Constraints[i].AtMost.Comparator,
			}
		case ConstraintTypeDependency:
			constr.Dependency = &DependencyConstraint{
				Ids:        e.Constraints[i].Dependency.Ids,
				Selector:   e.Constraints[i].Dependency.Selector,
				Comparator: e.Constraints[i].Dependency.Comparator,
			}
		default:
			panic(fmt.Sprintf("unknown constraint type %s", constr.Type))
		}
		constrs[i] = constr
	}
	out.Constraints = constrs
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
		e.program, err = expr.Compile(e.Expression, expr.AsBool())
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
	ConstraintTypeMandatory  = "mandatory"
	ConstraintTypeConflict   = "conflict"
	ConstraintTypeProhibited = "prohibited"
	ConstraintTypeAtMost     = "atMost"
	ConstraintTypeDependency = "dependency"
)

type DeppyConstraint struct {
	Type       ConstraintType        `json:"type"`
	Mandatory  *MandatoryConstraint  `json:"mandatory,omitempty"`
	Conflicts  *ConflictConstraint   `json:"conflicts,omitempty"`
	Prohibited *ProhibitedConstraint `json:"prohibited,omitempty"`
	AtMost     *AtMostConstraint     `json:"atMost,omitempty"`
	Dependency *DependencyConstraint `json:"dependency,omitempty"`
}

type SolverConstraintConverter interface {
	toSolverConstraint(subject DeppyId, universe []*DeppyEntity) (Constraint, error)
}

type MandatoryConstraint struct{}

func (c *MandatoryConstraint) toSolverConstraint(subject DeppyId, _ []*DeppyEntity) (Constraint, error) {
	return Mandatory(Identifier(subject)), nil
}

type ProhibitedConstraint struct {
}

func (c *ProhibitedConstraint) toSolverConstraint(subject DeppyId, _ []*DeppyEntity) (Constraint, error) {
	return Prohibited(Identifier(subject)), nil
}

type ConflictConstraint struct {
	Id DeppyId `json:"id"`
}

func (c *ConflictConstraint) toSolverConstraint(subject DeppyId, _ []*DeppyEntity) (Constraint, error) {
	return Conflict(Identifier(subject), Identifier(c.Id)), nil
}

type AtMostConstraint struct {
	N          string              `json:"limit"`
	Selector   *SelectorExpression `json:"selector"`
	GroupBy    *GroupByExpression  `json:"groupBy,omitempty"`
	Comparator *SortExpression     `json:"comparator,omitempty"`

	// HACK: need to generalize this later
	Ids []Identifier `json:"ids,omitempty"`
}

func (c *AtMostConstraint) toSolverConstraint(_ DeppyId, universe []*DeppyEntity) (constr Constraint, err error) {
	defer func() {
		if panicErr := recover(); panicErr != nil {
			constr = nil
			err = panicErr.(runtime.Error)
		}
	}()

	limit, _ := strconv.Atoi(c.N)

	if len(c.Ids) > 0 {
		return AtMost(limit, c.Ids...), nil
	}

	if c.GroupBy != nil {
		idMap := map[string][]Identifier{}
		for _, entity := range universe {
			ids, err := c.GroupBy.evaluate(entity)
			if err != nil {
				return nil, err
			}
			for _, id := range ids {
				idMap[id] = append(idMap[id], Identifier(entity.Identifier))
			}
		}
		constrs := make([]Constraint, 0, len(idMap))
		for _, ids := range idMap {
			constrs = append(constrs, AtMost(limit, ids...))
		}
		return And(constrs...), nil
	} else {
		entities := make([]*DeppyEntity, 0)
		for _, entity := range universe {
			eval, err := c.Selector.evaluate(entity)
			if err != nil {
				return nil, err
			}
			if eval == true {
				entities = append(entities, entity)
			}
		}
		if c.Comparator != nil {
			sort.Slice(entities, func(i, j int) bool {
				eval, err := c.Comparator.evaluate(entities[i], entities[j])
				if err != nil {
					panic(err)
				}
				return eval < 0
			})
		}
		ids := make([]Identifier, len(entities))
		for i, _ := range entities {
			ids[i] = Identifier(entities[i].Identifier)
		}
		return AtMost(limit, ids...), nil
	}
}

type DependencyConstraint struct {
	Selector   *SelectorExpression `json:"selector"`
	Comparator *SortExpression     `json:"comparator,omitempty"`
	// HACK: need to generalize this later
	Ids []Identifier `json:"ids,omitempty"`
}

func (c *DependencyConstraint) toSolverConstraint(subject DeppyId, universe []*DeppyEntity) (constr Constraint, err error) {
	defer func() {
		if panicErr := recover(); panicErr != nil {
			constr = nil
			err = panicErr.(runtime.Error)
		}
	}()

	if len(c.Ids) > 0 {
		return Dependency(Identifier(subject), c.Ids...), nil
	}

	entities := make([]*DeppyEntity, 0)
	for _, entity := range universe {
		eval, err := c.Selector.evaluate(entity)
		if err != nil {
			return nil, err
		}
		if eval == true {
			entities = append(entities, entity)
		}
	}
	if c.Comparator != nil {
		sort.Slice(entities, func(i, j int) bool {
			eval, err := c.Comparator.evaluate(entities[i], entities[j])
			if err != nil {
				panic(err)
			}
			return eval < 0
		})
	}
	ids := make([]Identifier, len(entities))
	for i, _ := range entities {
		ids[i] = Identifier(entities[i].Identifier)
	}
	return Dependency(Identifier(subject), ids...), nil
}

type DeppySource interface {
	GetEntities(ctx context.Context) ([]*DeppyEntity, error)
}
