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
			name:      "Error/BundleMissingPackageName",
			assertion: require.Error,
			cfg: DeclarativeConfig{
				Packages: []pkg{newTestPackage("foo", "alpha", svgSmallCircle)},
				Bundles:  []bundle{{}},
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
			name:      "Error/InvalidProperties",
			assertion: require.Error,
			cfg: DeclarativeConfig{
				Packages: []pkg{newTestPackage("foo", "alpha", svgSmallCircle)},
				Bundles:  []bundle{newTestBundle("foo", "0.1.0", withChannel("alpha", "1"), withChannel("alpha", "2"))},
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
	expected := buildValidDeclarativeConfig(true)

	m, err := ConvertToModel(expected)
	require.NoError(t, err)
	actual := ConvertFromModel(m)

	removeJSONWhitespace(&expected)
	removeJSONWhitespace(&actual)

	assert.Equal(t, expected.Packages, actual.Packages)
	assert.Equal(t, expected.Bundles, actual.Bundles)
	assert.Len(t, actual.others, 0, "expected unrecognized schemas not to make the roundtrip")
}
