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
		assertion require.ErrorAssertionFunc
	}

	specs := []spec{
		{
			name:      "Error/BundleMissingProvidedPackage",
			assertion: require.Error,
			cfg: DeclarativeConfig{
				Packages: []pkg{newTestPackage("foo", "alpha", svgSmallCircle)},
				Bundles:  []bundle{newTestBundle("foo", "0.1.0", skipProvidedPackage())},
			},
		},
		{
			name:      "Error/BundleUnknownPackage",
			assertion: require.Error,
			cfg: DeclarativeConfig{
				Packages: []pkg{newTestPackage("foo", "alpha", svgSmallCircle)},
				Bundles:  []bundle{newTestBundle("bar", "0.1.0")},
			},
		},
		{
			name:      "Error/FailedModelValidation",
			assertion: require.Error,
			cfg: DeclarativeConfig{
				Packages: []pkg{newTestPackage("foo", "alpha", svgSmallCircle)},
				Bundles:  []bundle{newTestBundle("foo", "0.1.0")},
			},
		},
		{
			name:      "Success/ValidModel",
			assertion: require.NoError,
			cfg: DeclarativeConfig{
				Packages: []pkg{newTestPackage("foo", "alpha", svgSmallCircle)},
				Bundles:  []bundle{newTestBundle("foo", "0.1.0", withChannel("alpha", ""))},
			},
		},
	}

	for _, s := range specs {
		t.Run(s.name, func(t *testing.T) {
			_, err := ConvertToModel(s.cfg)
			s.assertion(t, err)
		})
	}
}

func TestConvertToModelRoundtrip(t *testing.T) {
	expected := buildValidDeclarativeConfig()

	m, err := ConvertToModel(expected)
	require.NoError(t, err)
	actual := ConvertFromModel(m)

	assert.Equal(t, expected, actual)
}
