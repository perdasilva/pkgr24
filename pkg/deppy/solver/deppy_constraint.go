package solver

import (
	"fmt"
	"reflect"

	"github.com/antonmedv/expr"
	"github.com/antonmedv/expr/vm"
	"github.com/blang/semver/v4"
	"github.com/tidwall/gjson"
)

type ConstraintParameters []ConstraintParameter

type ConstraintParameter struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

type DeppyConstraintDefinition struct {
	Name        string               `json:"id"`
	Description string               `json:"description,omitempty"`
	Expression  string               `json:"expression"`
	Parameters  ConstraintParameters `json:"parameters,omitempty"`
	program     *vm.Program          `json:"-"`
}

func (c *DeppyConstraintDefinition) Execute(universe []*DeppyEntity, params map[string]string) ([]Constraint, error) {
	if c.program == nil {
		var err error
		c.program, err = expr.Compile(c.Expression)
		if err != nil {
			return nil, err
		}
	}

	env := newExprEnv()
	for key, value := range params {
		env[key] = value
	}
	env["Universe"] = universe

	output, err := expr.Run(c.program, env)
	if err != nil {
		return nil, err
	}

	if _, ok := output.(Constraint); ok {
		return []Constraint{output.(Constraint)}, nil
	}

	if _, ok := output.([]interface{}); ok {
		out := make([]Constraint, len(output.([]interface{})))
		for idx, _ := range output.([]interface{}) {
			if _, ok := output.([]interface{})[idx].(Constraint); ok {
				out[idx] = output.([]interface{})[idx].(Constraint)
			} else {
				return nil, fmt.Errorf("unrecognized return type (%s) in constraints expression", reflect.TypeOf(output.([]interface{})[idx]))
			}
		}
		return out, nil
	}

	return nil, fmt.Errorf("unrecognized return type (%s) in constraints expression", reflect.TypeOf(output))
}

func newExprEnv() map[string]interface{} {
	return map[string]interface{}{
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
		"unique": func(items []interface{}) []interface{} {
			seen := map[interface{}]bool{}
			out := make([]interface{}, 0)
			for _, item := range items {
				if _, ok := seen[item]; !ok {
					out = append(out, item)
					seen[item] = true
				}
			}
			return out
		},
		"Mandatory":  DeppyMandatory,
		"Prohibited": DeppyProhibited,
		"Conflict":   DeppyConflict,
		"Dependency": DeppyDependency,
		"AtMost":     DeppyAtMost,
		"And":        DeppyAnd,
		"Or":         DeppyOr,
		"Not":        DeppyNot,
	}
}

func DeppyMandatory(subject *DeppyEntity) (Constraint, error) {
	if subject == nil {
		return nil, fmt.Errorf("error creating mandatory constraint: subject is nil")
	}
	return subject.Mandatory(), nil
}

func DeppyProhibited(subject *DeppyEntity) (Constraint, error) {
	if subject == nil {
		return nil, fmt.Errorf("error creating mandatory constraint: subject is nil")
	}
	return subject.Prohibited(), nil
}

func DeppyConflict(subject *DeppyEntity, conflict *DeppyEntity) (Constraint, error) {
	if subject == nil {
		return nil, fmt.Errorf("error creating mandatory constraint: subject is nil")
	}
	return subject.Conflict(conflict), nil
}

func DeppyDependency(subject *DeppyEntity, dependencies ...*DeppyEntity) (Constraint, error) {
	if subject == nil {
		return nil, fmt.Errorf("error creating mandatory constraint: subject is nil")
	}
	return subject.Dependency(dependencies...), nil
}

func DeppyAtMost(subject *DeppyEntity, n int, entities ...*DeppyEntity) Constraint {
	return subject.AtMost(n, entities...)
}

func DeppyAnd(c1 Constraint, c2 Constraint) Constraint {
	return And(c1, c2)
}

func DeppyOr(c1 Constraint, c2 Constraint) Constraint {
	return Or(c1, c2)
}

func DeppyNot(c Constraint) Constraint {
	return Not(c)
}

type DeppyUniverse struct {
	universe []*DeppyEntity
}

type FilterQueryResult []*DeppyEntity

func (u *DeppyUniverse) Filter(expr string) FilterQueryResult {
	return nil
}

func (u *DeppyUniverse) Id(id DeppyId) *DeppyEntity {
	return nil
}
