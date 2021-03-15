package declcfg

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/util/rand"
)

func TestLoadFile(t *testing.T) {
	type spec struct {
		name              string
		file              string
		assertion         require.ErrorAssertionFunc
		expectNumPackages int
		expectNumBundles  int
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
			name:      "Error/UnrecognizedSchema",
			file:      "testdata/invalid/unrecognized-schema.json",
			assertion: require.Error,
		},
		{
			name:      "Error/InvalidPackageJSON",
			file:      "testdata/invalid/invalid-package-json.json",
			assertion: require.Error,
		},
		{
			name:      "Error/InvalidPackageJSON",
			file:      "testdata/invalid/invalid-bundle-json.json",
			assertion: require.Error,
		},
		{
			name:              "Success/ValidFile",
			file:              "testdata/valid/etcd.json",
			assertion:         require.NoError,
			expectNumPackages: 1,
			expectNumBundles:  6,
		},
	}

	for _, s := range specs {
		t.Run(s.name, func(t *testing.T) {
			cfg, err := LoadFile(s.file)
			s.assertion(t, err)
			if err == nil {
				require.NotNil(t, cfg)
				assert.Equal(t, len(cfg.Packages), s.expectNumPackages, "unexpected package count")
				assert.Equal(t, len(cfg.Bundles), s.expectNumBundles, "unexpected bundle count")
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
			expectNumPackages: 2,
			expectNumBundles:  11,
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
			}
		})
	}
}

func TestWriteDir(t *testing.T) {
	type spec struct {
		name      string
		cfg       DeclarativeConfig
		setupDir  func() (string, error)
		assertion require.ErrorAssertionFunc
	}
	setupNonExistentDir := func() (string, error) { return filepath.Join(os.TempDir(), "decl-write-dir-"+rand.String(5)), nil }
	setupEmptyDir := func() (string, error) { return ioutil.TempDir("", "decl-write-dir-") }
	setupNonEmptyDir := func() (string, error) {
		dir, err := ioutil.TempDir("", "decl-write-dir-")
		if err != nil {
			return "", err
		}
		if _, err := ioutil.TempFile(dir, "decl-write-dir-file-"); err != nil {
			return "", err
		}
		return dir, nil
	}
	setupFile := func() (string, error) {
		f, err := ioutil.TempFile("", "decl-write-dir-file-")
		if err != nil {
			return "", err
		}
		return f.Name(), nil
	}

	specs := []spec{
		{
			name:      "Success/NonExistentDir",
			cfg:       buildValidDeclarativeConfig(),
			setupDir:  setupNonExistentDir,
			assertion: require.NoError,
		},
		{
			name:      "Success/EmptyDir",
			cfg:       buildValidDeclarativeConfig(),
			setupDir:  setupEmptyDir,
			assertion: require.NoError,
		},
		{
			name:      "Error/MissingProvidedPackage",
			cfg:       buildInvalidDeclarativeConfig(),
			setupDir:  setupEmptyDir,
			assertion: require.Error,
		},
		{
			name:      "Error/NotADir",
			cfg:       buildValidDeclarativeConfig(),
			setupDir:  setupFile,
			assertion: require.Error,
		},
		{
			name:      "Error/NonEmptyDir",
			cfg:       buildValidDeclarativeConfig(),
			setupDir:  setupNonEmptyDir,
			assertion: require.Error,
		},
	}

	for _, s := range specs {
		t.Run(s.name, func(t *testing.T) {
			testDir, err := s.setupDir()
			require.NoError(t, err)
			defer func() {
				require.NoError(t, os.RemoveAll(testDir))
			}()

			err = WriteDir(s.cfg, testDir)
			s.assertion(t, err)
			if err == nil {
				files, err := ioutil.ReadDir(testDir)
				require.NoError(t, err)
				require.Equal(t, len(files), 2, "expect two package files")
				assert.Equal(t, files[0].Name(), "anakin.json")
				assert.Equal(t, files[1].Name(), "boba-fett.json")
				_, err = LoadDir(testDir)
				assert.NoError(t, err)
			}
		})
	}
}

func TestWriteFile(t *testing.T) {
	type spec struct {
		name      string
		cfg       DeclarativeConfig
		setupFile func() (string, error)
		assertion require.ErrorAssertionFunc
	}

	getFilename := func() (string, error) {
		return filepath.Join(os.TempDir(), "decl-write-file-"+rand.String(5)+".json"), nil
	}
	getDirectory := func() (string, error) {
		return ioutil.TempDir("", "decl-write-file-")
	}

	specs := []spec{
		{
			name:      "Success/NonExistentFile",
			cfg:       buildValidDeclarativeConfig(),
			setupFile: getFilename,
			assertion: require.NoError,
		},
		{
			name:      "Error/NotAFile",
			cfg:       buildValidDeclarativeConfig(),
			setupFile: getDirectory,
			assertion: require.Error,
		},
		{
			name:      "Error/MissingProvidedPackage",
			cfg:       buildInvalidDeclarativeConfig(),
			setupFile: getFilename,
			assertion: require.Error,
		},
	}
	for _, s := range specs {
		t.Run(s.name, func(t *testing.T) {
			filename, err := s.setupFile()
			require.NoError(t, err)
			defer func() {
				require.NoError(t, os.RemoveAll(filename))
			}()
			err = WriteFile(s.cfg, filename)
			s.assertion(t, err)
			if err == nil {
				_, err = LoadFile(filename)
				assert.NoError(t, err)
			}
		})
	}
}

func TestWriteLoadRoundtrip(t *testing.T) {
	t.Run("File", func(t *testing.T) {
		toFile := buildValidDeclarativeConfig()

		filename := filepath.Join(os.TempDir(), "decl-write-file-"+rand.String(5)+".json")
		defer func() {
			require.NoError(t, os.RemoveAll(filename))
		}()
		require.NoError(t, WriteFile(toFile, filename))

		fromFile, err := LoadFile(filename)
		require.NoError(t, err)

		removeWhitespaceFromProperties(toFile.Bundles)
		removeWhitespaceFromProperties(fromFile.Bundles)

		assert.Equal(t, toFile, *fromFile)
	})

	t.Run("Dir", func(t *testing.T) {
		toDir := buildValidDeclarativeConfig()
		dirname := filepath.Join(os.TempDir(), "decl-write-dir-"+rand.String(5))
		defer func() {
			require.NoError(t, os.RemoveAll(dirname))
		}()
		require.NoError(t, WriteDir(toDir, dirname))

		fromDir, err := LoadDir(dirname)
		require.NoError(t, err)

		removeWhitespaceFromProperties(toDir.Bundles)
		removeWhitespaceFromProperties(fromDir.Bundles)

		assert.Equal(t, toDir, *fromDir)
	})
}

func removeWhitespaceFromProperties(bundles []bundle) {
	for ib := range bundles {
		for ip := range bundles[ib].Properties {
			replacer := strings.NewReplacer(" ", "", "\n", "")
			bundles[ib].Properties[ip].Value = []byte(replacer.Replace(string(bundles[ib].Properties[ip].Value)))
		}
	}
}
