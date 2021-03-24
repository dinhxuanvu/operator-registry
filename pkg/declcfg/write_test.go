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

func TestWriteDir(t *testing.T) {
	type spec struct {
		name      string
		cfg       DeclarativeConfig
		setupDir  func() (string, error)
		assertion require.ErrorAssertionFunc
	}
	setupNonExistentDir := func() (string, error) {
		return filepath.Join(os.TempDir(), "decl-write-dir-"+rand.String(5)), nil
	}
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
				entries, err := ioutil.ReadDir(testDir)
				require.NoError(t, err)
				entryNames := []string{}
				for _, f := range entries {
					entryNames = append(entryNames, f.Name())
				}

				expectedEntryNames := []string{
					fmt.Sprintf("%s.json", globalName),
					"anakin.json",
					"boba-fett.json",
					"objects",
				}
				require.ElementsMatch(t, expectedEntryNames, entryNames)

				anakin, err := loadFile(filepath.Join(testDir, "anakin.json"))
				require.NoError(t, err)
				assert.Len(t, anakin.Packages, 1)
				assert.Len(t, anakin.Bundles, 3)
				assert.Len(t, anakin.others, 1)

				bobaFett, err := loadFile(filepath.Join(testDir, "boba-fett.json"))
				require.NoError(t, err)
				assert.Len(t, bobaFett.Packages, 1)
				assert.Len(t, bobaFett.Bundles, 2)
				assert.Len(t, bobaFett.others, 1)

				globals, err := loadFile(filepath.Join(testDir, fmt.Sprintf("%s.json", globalName)))
				require.NoError(t, err)
				assert.Len(t, globals.Packages, 0)
				assert.Len(t, globals.Bundles, 0)
				assert.Len(t, globals.others, 2)

				all, err := LoadDir(testDir)
				require.NoError(t, err)

				assert.Len(t, all.Packages, 2)
				assert.Len(t, all.Bundles, 5)
				assert.Len(t, all.others, 4)
			}
		})
	}
}

func TestWriteLoadRoundtrip(t *testing.T) {
	type spec struct {
		name  string
		write func(DeclarativeConfig, string) error
		load  func(string) (*DeclarativeConfig, error)
	}

	specs := []spec{
		{
			name:  "Dir",
			write: WriteDir,
			load:  LoadDir,
		},
		{
			name:  "Tar",
			write: WriteTar,
			load:  LoadTar,
		},
	}
	for _, s := range specs {
		t.Run(s.name, func(t *testing.T) {
			to := buildValidDeclarativeConfig(true)

			filename := filepath.Join(os.TempDir(), "declcfg-"+rand.String(5))
			defer func() {
				require.NoError(t, os.RemoveAll(filename))
			}()
			require.NoError(t, s.write(to, filename))

			from, err := s.load(filename)
			require.NoError(t, err)

			equalsDeclarativeConfig(t, to, *from)
		})
	}
}

func equalsDeclarativeConfig(t *testing.T, expected, actual DeclarativeConfig) {
	removeJSONWhitespace(&expected)
	removeJSONWhitespace(&actual)

	assert.ElementsMatch(t, expected.Packages, actual.Packages)
	assert.ElementsMatch(t, expected.Bundles, actual.Bundles)
	assert.ElementsMatch(t, expected.others, actual.others)

	// In case new fields are added to the DeclarativeConfig struct in the future,
	// test that the rest is Equal.
	expected.Packages, actual.Packages = nil, nil
	expected.Bundles, actual.Bundles = nil, nil
	expected.others, actual.others = nil, nil
	assert.Equal(t, expected, actual)
}

func removeJSONWhitespace(cfg *DeclarativeConfig) {
	replacer := strings.NewReplacer(" ", "", "\n", "")
	for ib := range cfg.Bundles {
		for ip := range cfg.Bundles[ib].Properties {
			cfg.Bundles[ib].Properties[ip].Value = []byte(replacer.Replace(string(cfg.Bundles[ib].Properties[ip].Value)))
		}
	}
	for io := range cfg.others {
		for _, v := range cfg.others[io].data {
			cfg.others[io].data = []byte(replacer.Replace(string(v)))
		}
	}
}