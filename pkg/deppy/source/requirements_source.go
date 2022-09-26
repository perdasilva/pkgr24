package source

import (
	"context"

	pkgr24iov1alpha1 "github.com/perdasilva/pkgr24/api/v1alpha1"
	"github.com/perdasilva/pkgr24/pkg/deppy/solver"
)

type RequirementsDeppySource struct {
	entities []*solver.DeppyEntity
}

func (r RequirementsDeppySource) GetEntities(_ context.Context) ([]*solver.DeppyEntity, error) {
	return r.entities, nil
}

func NewRequirementsDeppySource(requirements *pkgr24iov1alpha1.Requirements) solver.DeppySource {
	entities := make([]*solver.DeppyEntity, len(requirements.Spec.Requirements))
	for i, _ := range requirements.Spec.Requirements {
		entities[i] = &requirements.Spec.Requirements[i]
	}
	return &RequirementsDeppySource{
		entities: entities,
	}
}
