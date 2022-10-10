package search

import "github.com/perdasilva/pkgr24/pkg/deppy/solver"

type Predicate func(entity *solver.DeppyEntity) bool

func And(predicates ...Predicate) Predicate {
	return func(entity *solver.DeppyEntity) bool {
		eval := true
		for _, predicate := range predicates {
			eval = eval && predicate(entity)
			if !eval {
				return false
			}
		}
		return eval
	}
}

func Or(predicates ...Predicate) Predicate {
	return func(entity *solver.DeppyEntity) bool {
		eval := false
		for _, predicate := range predicates {
			eval = eval || predicate(entity)
			if eval {
				return true
			}
		}
		return eval
	}
}

func Not(predicate Predicate) Predicate {
	return func(entity *solver.DeppyEntity) bool {
		return !predicate(entity)
	}
}
