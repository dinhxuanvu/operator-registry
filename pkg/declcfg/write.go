package declcfg

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/ghodss/yaml"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

const (
	objectsDirName = "objects"
	globalName     = "__global"
)

func WriteDir(cfg DeclarativeConfig, configDir string) error {
	entries, err := ioutil.ReadDir(configDir)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if len(entries) > 0 {
		return fmt.Errorf("config dir %q must be empty", configDir)
	}

	return writeToFS(cfg, &diskWriter{}, configDir)
}

type fsWriter interface {
	MkdirAll(path string, mode os.FileMode) error
	WriteFile(path string, data []byte, mode os.FileMode) error
}

var _ fsWriter = &diskWriter{}

type diskWriter struct{}

func (w diskWriter) MkdirAll(path string, mode os.FileMode) error {
	return os.MkdirAll(path, mode)
}

func (w diskWriter) WriteFile(path string, data []byte, mode os.FileMode) error {
	return ioutil.WriteFile(path, data, mode)
}

func writeToFS(cfg DeclarativeConfig, w fsWriter, rootDir string) error {
	bundlesByPackage := map[string][]Bundle{}
	for _, b := range cfg.Bundles {
		bundlesByPackage[b.Package] = append(bundlesByPackage[b.Package], b)
	}
	othersByPackage := map[string][]Meta{}
	for _, o := range cfg.Others {
		pkgName := o.Package
		if pkgName == "" {
			pkgName = globalName
		}
		othersByPackage[pkgName] = append(othersByPackage[pkgName], o)
	}

	if err := w.MkdirAll(rootDir, 0755); err != nil {
		return fmt.Errorf("mkdir %q: %v", rootDir, err)
	}

	for _, p := range cfg.Packages {
		fcfg := DeclarativeConfig{
			Packages: []Package{p},
			Bundles:  bundlesByPackage[p.Name],
			Others:   othersByPackage[p.Name],
		}
		filename := filepath.Join(rootDir, fmt.Sprintf("%s.json", p.Name))
		if err := writeFile(fcfg, w, filename); err != nil {
			return err
		}
	}

	for pkgName, bundles := range bundlesByPackage {
		pkgDir := filepath.Join(rootDir, objectsDirName, pkgName)
		for _, b := range bundles {
			if len(b.Objects) > 0 {
				bundleDir := filepath.Join(pkgDir, b.Name)
				if err := w.MkdirAll(bundleDir, 0755); err != nil {
					return fmt.Errorf("mkdir %q: %v", rootDir, err)
				}
				for i, obj := range b.Objects {
					objFilename := filepath.Join(bundleDir, objectFilename(obj, i))
					if err := w.WriteFile(objFilename, []byte(obj), 0644); err != nil {
						return fmt.Errorf("write file %q: %v", objFilename, err)
					}
				}
			}
		}
	}

	if globals, ok := othersByPackage[globalName]; ok {
		gcfg := DeclarativeConfig{
			Others: globals,
		}
		filename := filepath.Join(rootDir, fmt.Sprintf("%s.json", globalName))
		if err := writeFile(gcfg, w, filename); err != nil {
			return err
		}
	}
	return nil
}

func objectFilename(obj string, idx int) string {
	name, kind := fmt.Sprintf("obj%04d", idx), ""
	u := unstructured.Unstructured{}
	if err := yaml.Unmarshal([]byte(obj), &u); err == nil {
		if u.GetName() != "" {
			name = u.GetName()
		}
		gvk := u.GroupVersionKind()
		kind = fmt.Sprintf("%s_%s_%s", gvk.Group, gvk.Version, strings.ToLower(gvk.Kind))
	}
	if kind == "" {
		return fmt.Sprintf("%s", name)
	}
	return fmt.Sprintf("%s_%s.yaml", name, kind)
}

func writeFile(cfg DeclarativeConfig, w fsWriter, filename string) error {
	buf := &bytes.Buffer{}
	if err := writeJSON(cfg, buf); err != nil {
		return fmt.Errorf("write to buffer for %q: %v", filename, err)
	}
	if err := w.WriteFile(filename, buf.Bytes(), 0644); err != nil {
		return fmt.Errorf("write file %q: %v", filename, err)
	}
	return nil
}

func writeJSON(cfg DeclarativeConfig, w io.Writer) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "    ")
	enc.SetEscapeHTML(false)

	bundlesByPackage := map[string][]Bundle{}
	for _, b := range cfg.Bundles {
		pkgName := b.Package
		bundlesByPackage[pkgName] = append(bundlesByPackage[pkgName], b)
	}

	for _, p := range cfg.Packages {
		if err := enc.Encode(p); err != nil {
			return err
		}
		for _, b := range bundlesByPackage[p.Name] {
			if err := enc.Encode(b); err != nil {
				return err
			}
		}
	}
	for _, o := range cfg.Others {
		if err := enc.Encode(o); err != nil {
			return err
		}
	}
	return nil
}
