package declcfg

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/operator-framework/operator-registry/pkg/model"
)

func TestConvertFromModel(t *testing.T) {
	type spec struct {
		name      string
		m         model.Model
		expectCfg DeclarativeConfig
	}

	specs := []spec{
		{
			name:      "Success",
			m:         buildTestModel(),
			expectCfg: buildValidDeclarativeConfig(),
		},
	}

	for _, s := range specs {
		t.Run(s.name, func(t *testing.T) {
			actual := ConvertFromModel(s.m)
			assert.Equal(t, s.expectCfg, actual)
		})
	}
}
