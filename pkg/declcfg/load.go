package declcfg

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/ghodss/yaml"
	"github.com/operator-framework/api/pkg/operators"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func LoadDir(configDir string) (*DeclarativeConfig, error) {
	w := &dirWalker{}
	return loadFS(configDir, w)
}

func loadFile(configFile string) (*DeclarativeConfig, error) {
	f, err := os.Open(configFile)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %v", err)
	}
	defer f.Close()
	return readJSON(f)
}

func loadFS(root string, w fsWalker) (*DeclarativeConfig, error) {
	cfg := &DeclarativeConfig{}
	objects := map[string]map[string][]string{}
	if err := w.WalkFiles(root, func(path string, r io.Reader) error {
		relPath := strings.TrimPrefix(path, root+"/")
		relPathSegments := strings.Split(relPath, "/")

		// Looking for "objects/<pkgName>/<bundleName>/*
		if len(relPathSegments) == 4 && relPathSegments[0] == objectsDirName {
			pkgName := relPathSegments[1]
			bundleName := relPathSegments[2]
			obj, err := ioutil.ReadAll(r)
			if err != nil {
				return fmt.Errorf("read object from path %q: %v", path, err)
			}
			if _, pkgExists := objects[pkgName]; !pkgExists {
				objects[pkgName] = map[string][]string{}
			}
			objects[pkgName][bundleName] = append(objects[pkgName][bundleName], string(obj))
		} else {
			fileCfg, err := readJSON(r)
			if err != nil {
				return fmt.Errorf("could not load config file %q: %v", path, err)
			}
			cfg.Packages = append(cfg.Packages, fileCfg.Packages...)
			cfg.Bundles = append(cfg.Bundles, fileCfg.Bundles...)
			cfg.Others = append(cfg.Others, fileCfg.Others...)
		}
		return nil
	}); err != nil {
		return nil, fmt.Errorf("failed to read declarative configs dir: %v", err)
	}

	for i, b := range cfg.Bundles {
		pkg, ok := objects[b.Package]
		if !ok {
			continue
		}
		objs := pkg[b.Name]
		cfg.Bundles[i].Objects = objs
		cfg.Bundles[i].CsvJSON = extractCSV(objs)
	}
	return cfg, nil
}

func extractCSV(objs []string) string {
	for _, obj := range objs {
		u := unstructured.Unstructured{}
		if err := yaml.Unmarshal([]byte(obj), &u); err != nil {
			continue
		}
		if u.GetKind() == operators.ClusterServiceVersionKind {
			return obj
		}
	}
	return ""
}

func readJSON(r io.Reader) (*DeclarativeConfig, error) {
	cfg := &DeclarativeConfig{}
	dec := json.NewDecoder(r)
	for dec.More() {
		doc := &json.RawMessage{}
		if err := dec.Decode(doc); err != nil {
			return nil, fmt.Errorf("parse error at offset %d: %v", dec.InputOffset(), err)
		}

		var in Meta
		if err := json.Unmarshal(*doc, &in); err != nil {
			return nil, fmt.Errorf("parse meta object at offset %d: %v", dec.InputOffset(), err)
		}

		switch in.Schema {
		case schemaPackage:
			var p Package
			if err := json.Unmarshal(*doc, &p); err != nil {
				return nil, fmt.Errorf("parse package at offset %d: %v", dec.InputOffset(), err)
			}
			cfg.Packages = append(cfg.Packages, p)
		case schemaBundle:
			var b Bundle
			if err := json.Unmarshal(*doc, &b); err != nil {
				return nil, fmt.Errorf("parse bundle at offset %d: %v", dec.InputOffset(), err)
			}
			cfg.Bundles = append(cfg.Bundles, b)
		default:
			cfg.Others = append(cfg.Others, in)
		}
	}
	return cfg, nil
}

type fsWalker interface {
	WalkFiles(root string, f func(path string, r io.Reader) error) error
}

type dirWalker struct{}

func (w dirWalker) WalkFiles(root string, f func(string, io.Reader) error) error {
	return filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		file, err := os.Open(path)
		if err != nil {
			return err
		}
		return f(path, file)
	})
}
