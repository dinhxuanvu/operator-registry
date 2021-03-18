package declcfg

import (
	"fmt"
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
			cfg, err := LoadFile(s.file)
			s.assertion(t, err)
			if err == nil {
				require.NotNil(t, cfg)
				assert.Equal(t, len(cfg.Packages), s.expectNumPackages, "unexpected package count")
				assert.Equal(t, len(cfg.Bundles), s.expectNumBundles, "unexpected bundle count")
				assert.Equal(t, len(cfg.Others), s.expectNumOthers, "unexpected others count")
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
				assert.Equal(t, len(cfg.Others), s.expectNumOthers, "unexpected others count")
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
			cfg:       buildValidDeclarativeConfig(true),
			setupDir:  setupNonExistentDir,
			assertion: require.NoError,
		},
		{
			name:      "Success/EmptyDir",
			cfg:       buildValidDeclarativeConfig(true),
			setupDir:  setupEmptyDir,
			assertion: require.NoError,
		},
		{
			name:      "Error/NotADir",
			cfg:       buildValidDeclarativeConfig(true),
			setupDir:  setupFile,
			assertion: require.Error,
		},
		{
			name:      "Error/NonEmptyDir",
			cfg:       buildValidDeclarativeConfig(true),
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
				filenames := []string{}
				for _, f := range files {
					filenames = append(filenames, f.Name())
				}

				expectedFiles := []string{fmt.Sprintf("%s.json", globalName), "anakin.json", "boba-fett.json"}
				require.ElementsMatch(t, expectedFiles, filenames)

				anakin, err := LoadFile(filepath.Join(testDir, "anakin.json"))
				require.NoError(t, err)
				assert.Len(t, anakin.Packages, 1)
				assert.Len(t, anakin.Bundles, 3)
				assert.Len(t, anakin.Others, 1)

				bobaFett, err := LoadFile(filepath.Join(testDir, "boba-fett.json"))
				require.NoError(t, err)
				assert.Len(t, bobaFett.Packages, 1)
				assert.Len(t, bobaFett.Bundles, 2)
				assert.Len(t, bobaFett.Others, 1)

				globals, err := LoadFile(filepath.Join(testDir, fmt.Sprintf("%s.json", globalName)))
				require.NoError(t, err)
				assert.Len(t, globals.Packages, 0)
				assert.Len(t, globals.Bundles, 0)
				assert.Len(t, globals.Others, 2)

				all, err := LoadDir(testDir)
				require.NoError(t, err)

				assert.Len(t, all.Packages, 2)
				assert.Len(t, all.Bundles, 5)
				assert.Len(t, all.Others, 4)
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
			cfg:       buildValidDeclarativeConfig(true),
			setupFile: getFilename,
			assertion: require.NoError,
		},
		{
			name:      "Error/NotAFile",
			cfg:       buildValidDeclarativeConfig(true),
			setupFile: getDirectory,
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
		toFile := buildValidDeclarativeConfig(true)

		filename := filepath.Join(os.TempDir(), "decl-write-file-"+rand.String(5)+".json")
		defer func() {
			require.NoError(t, os.RemoveAll(filename))
		}()
		require.NoError(t, WriteFile(toFile, filename))

		fromFile, err := LoadFile(filename)
		require.NoError(t, err)

		removeJSONWhitespace(&toFile)
		removeJSONWhitespace(fromFile)

		assert.Equal(t, toFile, *fromFile)
	})

	t.Run("Dir", func(t *testing.T) {
		toDir := buildValidDeclarativeConfig(true)
		dirname := filepath.Join(os.TempDir(), "decl-write-dir-"+rand.String(5))
		defer func() {
			require.NoError(t, os.RemoveAll(dirname))
		}()
		require.NoError(t, WriteDir(toDir, dirname))

		fromDir, err := LoadDir(dirname)
		require.NoError(t, err)

		removeJSONWhitespace(&toDir)
		removeJSONWhitespace(fromDir)

		assert.Equal(t, toDir, *fromDir)
	})
}

func removeJSONWhitespace(cfg *DeclarativeConfig) {
	replacer := strings.NewReplacer(" ", "", "\n", "")
	for ib := range cfg.Bundles {
		for ip := range cfg.Bundles[ib].Properties {
			cfg.Bundles[ib].Properties[ip].Value = []byte(replacer.Replace(string(cfg.Bundles[ib].Properties[ip].Value)))
		}
	}
	for io := range cfg.Others {
		for _, v := range cfg.Others[io].data {
			cfg.Others[io].data = []byte(replacer.Replace(string(v)))
		}
	}
}
