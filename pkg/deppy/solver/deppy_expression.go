package solver

import (
	"fmt"

	"github.com/antonmedv/expr"
	"github.com/antonmedv/expr/vm"
	"github.com/blang/semver/v4"
	"github.com/tidwall/gjson"
)

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
