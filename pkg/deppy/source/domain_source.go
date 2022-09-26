package source

import (
	"context"

	"github.com/perdasilva/pkgr24/pkg/deppy/solver"
)

type DomainDeppySource struct {
	domainEntities []*solver.DeppyEntity
}

func NewDomainDeppySource() solver.DeppySource {
	return &DomainDeppySource{
		domainEntities: []*solver.DeppyEntity{
			{
				Identifier: "at most one for bundle per gvk",
				Constraints: []solver.DeppyConstraint{
					{
						Type: solver.ConstraintTypeAtMost,
						AtMost: &solver.AtMostConstraint{
							N: "1",
							GroupBy: &solver.GroupByExpression{
								Expression: `Entity.Properties["olm.gvk"]`,
							},
						},
					},
				},
			}, {
				Identifier: "at most one for bundle per package",
				Constraints: []solver.DeppyConstraint{
					{
						Type: solver.ConstraintTypeAtMost,
						AtMost: &solver.AtMostConstraint{
							N: "1",
							GroupBy: &solver.GroupByExpression{
								Expression: `Entity.Properties["olm.package"]`,
							},
						},
					},
				},
			},
		},
	}
}

func (s *DomainDeppySource) GetEntities(_ context.Context) ([]*solver.DeppyEntity, error) {
	return s.domainEntities, nil
}
