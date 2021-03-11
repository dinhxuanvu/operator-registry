package declcfg

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConvertToModel(t *testing.T) {
	type spec struct {
		name      string
		cfg       DeclarativeConfig
		expectErr bool
	}

	specs := []spec{
		{
			name:      "Error/BundleMissingProvidedPackage",
			expectErr: true,
			cfg: DeclarativeConfig{
				Packages: []pkg{newTestPackage("foo", "alpha", svgSmallCircle)},
				Bundles:  []bundle{newTestBundle("foo", "0.1.0", skipProvidedPackage())},
			},
		},
		{
			name:      "Error/BundleUnknownPackage",
			expectErr: true,
			cfg: DeclarativeConfig{
				Packages: []pkg{newTestPackage("foo", "alpha", svgSmallCircle)},
				Bundles:  []bundle{newTestBundle("bar", "0.1.0")},
			},
		},
		{
			name:      "Error/FailedModelValidation",
			expectErr: true,
			cfg: DeclarativeConfig{
				Packages: []pkg{newTestPackage("foo", "alpha", svgSmallCircle)},
				Bundles:  []bundle{newTestBundle("foo", "0.1.0")},
			},
		},
		{
			name: "Success/ValidModel",
			cfg: DeclarativeConfig{
				Packages: []pkg{newTestPackage("foo", "alpha", svgSmallCircle)},
				Bundles:  []bundle{newTestBundle("foo", "0.1.0", withChannel("alpha", ""))},
			},
		},
	}

	for _, s := range specs {
		t.Run(s.name, func(t *testing.T) {
			_, err := ConvertToModel(s.cfg)
			if s.expectErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestConvertToModelRoundtrip(t *testing.T) {
	in := buildValidDeclarativeConfig()
	expected := buildValidDeclarativeConfig()

	m, err := ConvertToModel(in)
	require.NoError(t, err)
	actual := ConvertFromModel(m)

	assert.Equal(t, expected, actual)
}
