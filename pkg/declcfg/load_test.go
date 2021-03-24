package declcfg

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadFile(t *testing.T) {
	type spec struct {
		name              string
		file              string
		assertion         require.ErrorAssertionFunc
		expectNumPackages int
		expectNumBundles  int
		expectNumOthers   int
	}
	specs := []spec{
		{
			name:      "Error/NonExistentFile",
			file:      "testdata/invalid/non-existent.json",
			assertion: require.Error,
		},
		{
			name:      "Error/NotJSON",
			file:      "testdata/invalid/not-json.txt",
			assertion: require.Error,
		},
		{
			name:      "Error/NotJSONObject",
			file:      "testdata/invalid/not-json-object.json",
			assertion: require.Error,
		},
		{
			name:      "Error/InvalidPackageJSON",
			file:      "testdata/invalid/invalid-package-json.json",
			assertion: require.Error,
		},
		{
			name:      "Error/InvalidBundleJSON",
			file:      "testdata/invalid/invalid-bundle-json.json",
			assertion: require.Error,
		},
		{
			name:              "Success/UnrecognizedSchema",
			file:              "testdata/valid/unrecognized-schema.json",
			assertion:         require.NoError,
			expectNumPackages: 1,
			expectNumBundles:  1,
			expectNumOthers:   1,
		},
		{
			name:              "Success/ValidFile",
			file:              "testdata/valid/etcd.json",
			assertion:         require.NoError,
			expectNumPackages: 1,
			expectNumBundles:  6,
			expectNumOthers:   0,
		},
	}

	for _, s := range specs {
		t.Run(s.name, func(t *testing.T) {
			cfg, err := loadFile(s.file)
			s.assertion(t, err)
			if err == nil {
				require.NotNil(t, cfg)
				assert.Equal(t, len(cfg.Packages), s.expectNumPackages, "unexpected package count")
				assert.Equal(t, len(cfg.Bundles), s.expectNumBundles, "unexpected bundle count")
				assert.Equal(t, len(cfg.others), s.expectNumOthers, "unexpected others count")
			}
		})
	}
}

func TestLoadDir(t *testing.T) {
	type spec struct {
		name              string
		dir               string
		assertion         require.ErrorAssertionFunc
		expectNumPackages int
		expectNumBundles  int
		expectNumOthers   int
	}
	specs := []spec{
		{
			name:      "Error/NonExistentDir",
			dir:       "testdata/nonexistent",
			assertion: require.Error,
		},
		{
			name:      "Error/Invalid",
			dir:       "testdata/invalid",
			assertion: require.Error,
		},
		{
			name:              "Success/ValidDir",
			dir:               "testdata/valid",
			assertion:         require.NoError,
			expectNumPackages: 3,
			expectNumBundles:  12,
			expectNumOthers:   1,
		},
	}

	for _, s := range specs {
		t.Run(s.name, func(t *testing.T) {
			cfg, err := LoadDir(s.dir)
			s.assertion(t, err)
			if err == nil {
				require.NotNil(t, cfg)
				assert.Equal(t, len(cfg.Packages), s.expectNumPackages, "unexpected package count")
				assert.Equal(t, len(cfg.Bundles), s.expectNumBundles, "unexpected bundle count")
				assert.Equal(t, len(cfg.others), s.expectNumOthers, "unexpected others count")
			}
		})
	}
}