package solver

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_DeppyConstraints(t *testing.T) {

	universe := []*DeppyEntity{
		{
			Identifier: DeppyId("one"),
			Properties: map[string]string{
				"olm.gvk":     `{"group": "g1", "version": "v1", "kind": "k1"}`,
				"olm.version": "1.0.0",
			},
		}, {
			Identifier: DeppyId("two"),
			Properties: map[string]string{
				"olm.gvk":     `{"group": "g1", "version": "v1", "kind": "k2"}`,
				"olm.version": "1.0.1",
			},
		}, {
			Identifier: DeppyId("three"),
			Properties: map[string]string{
				"olm.gvk":     `{"group": "g1", "version": "v2", "kind": "k1"}`,
				"olm.version": "1.0.8",
			},
		}, {
			Identifier: DeppyId("four"),
			Properties: map[string]string{
				"olm.gvk":     `{"group": "g2", "version": "v1", "kind": "k1"}`,
				"olm.version": "2.0.0",
			},
		}, {
			Identifier: DeppyId("five"),
			Properties: map[string]string{
				"olm.gvk":     `{"group": "g1", "version": "v1", "kind": "k1"}`,
				"olm.version": "1.0.9",
			},
		},
	}

	testCases := []struct {
		name                string
		expression          string
		universe            []*DeppyEntity
		parameters          map[string]string
		expectedConstraints []Constraint
		expectedError       error
	}{
		//{
		//	name:                "mandatory",
		//	expression:          `map(filter(Universe, {.Properties["olm.version"] == "1.0.9"}), {#.Mandatory()})`,
		//	expectedConstraints: []Constraint{Mandatory("five")},
		//	universe:            universe,
		//},
		{
			name:       "atMost",
			expression: `map(unique(map(Universe, {#.Properties["olm.gvk"]})), {filter(Universe, {.Properties["olm.gvk"] == #})})`,
			universe:   universe,
		},
	}

	for _, tt := range testCases {
		defn := DeppyConstraintDefinition{
			Name:       tt.name,
			Expression: tt.expression,
		}
		constraints, err := defn.Execute(tt.universe, tt.parameters)
		assert.NotNil(t, err)
		assert.Equal(t, constraints, tt.expectedConstraints)
	}

}
