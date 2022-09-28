package source

import (
	"context"

	"github.com/perdasilva/pkgr24/pkg/deppy/solver"
)

type DomainDeppySource struct {
	domainEntities    []*solver.DeppyEntity
	domainConstraints []solver.DeppyConstraint
}

func (d DomainDeppySource) GetEntities(_ context.Context) ([]*solver.DeppyEntity, error) {
	return d.domainEntities, nil
}

func (d DomainDeppySource) GetConstraints(_ context.Context) ([]solver.DeppyConstraint, error) {
	return d.domainConstraints, nil
}

func NewDomainDeppySource() solver.DeppySource {
	return &DomainDeppySource{
		domainEntities: nil,
		domainConstraints: []solver.DeppyConstraint{
			{
				Type: solver.ConstraintTypeGroupByBuilder,
				GroupBy: &solver.GroupByConstraintBuilder{
					GroupBy: solver.GroupByExpression{
						Expression: `Entity.Properties["olm.gvk"]`,
					},
					Constraint: solver.DeppyConstraint{
						Type: solver.ConstraintTypeAtMost,
						AtMost: &solver.AtMostConstraint{
							N: "1",
						},
					},
				},
			}, {
				Type: solver.ConstraintTypeGroupByBuilder,
				GroupBy: &solver.GroupByConstraintBuilder{
					GroupBy: solver.GroupByExpression{
						Expression: `Entity.Properties["olm.package"]`,
					},
					Constraint: solver.DeppyConstraint{
						Type: solver.ConstraintTypeAtMost,
						AtMost: &solver.AtMostConstraint{
							N: "1",
						},
					},
				},
			},
		},
	}
}
