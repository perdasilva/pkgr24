package deppy

import (
	"sort"

	"github.com/perdasilva/pkgr24/pkg/deppy/search"
	"github.com/perdasilva/pkgr24/pkg/deppy/solver"
)

type EntityUniverse struct {
	universe map[solver.Identifier]solver.DeppyEntity
}

func NewEntityUniverse(entities []*solver.DeppyEntity) *EntityUniverse {
	m := make(map[solver.Identifier]solver.DeppyEntity)
	for _, entity := range entities {
		m[solver.Identifier(entity.Identifier)] = *entity
	}
	return &EntityUniverse{
		universe: m,
	}
}

func (u *EntityUniverse) Get(id solver.Identifier) *solver.DeppyEntity {
	if entity, ok := u.universe[id]; ok {
		return &entity
	}
	return nil
}

func (u *EntityUniverse) Search(predicate search.Predicate) SearchResult {
	out := make([]solver.DeppyEntity, 0)
	for _, entity := range u.universe {
		if predicate(&entity) {
			out = append(out, entity)
		}
	}
	return out
}

func (u *EntityUniverse) AllEntities() SearchResult {
	out := make([]solver.DeppyEntity, 0, len(u.universe))
	for _, entity := range u.universe {
		out = append(out, entity)
	}
	return out
}

type SearchResult []solver.DeppyEntity
type SortFunction func(e1 *solver.DeppyEntity, e2 *solver.DeppyEntity) bool

func (r SearchResult) Sort(fn SortFunction) SearchResult {
	sort.SliceStable(r, func(i, j int) bool {
		return fn(&r[i], &r[j])
	})
	return r
}

func (r SearchResult) CollectIds() []solver.Identifier {
	ids := make([]solver.Identifier, len(r))
	for i, _ := range r {
		ids[i] = solver.Identifier(r[i].Identifier)
	}
	return ids
}
