package source

import (
	"context"

	pkgr24iov1alpha1 "github.com/perdasilva/pkgr24/api/v1alpha1"
	"github.com/perdasilva/pkgr24/pkg/deppy/solver"
)

type RequirementsDeppySource struct {
	constraints []solver.DeppyConstraint
}

func NewRequirementsDeppySource(requirements *pkgr24iov1alpha1.Requirements) solver.DeppySource {
	constraints := make([]solver.DeppyConstraint, len(requirements.Spec.Constraints))
	for i, _ := range requirements.Spec.Constraints {
		constraints[i] = requirements.Spec.Constraints[i]
	}
	return &RequirementsDeppySource{
		constraints: constraints,
	}
}

func (r RequirementsDeppySource) GetConstraints(ctx context.Context) ([]solver.DeppyConstraint, error) {
	return r.constraints, nil
}

func (r RequirementsDeppySource) GetEntities(_ context.Context) ([]*solver.DeppyEntity, error) {
	return nil, nil
}
