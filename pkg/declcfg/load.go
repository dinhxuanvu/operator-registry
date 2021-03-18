package declcfg

import (
	"archive/tar"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func LoadDir(configDir string) (*DeclarativeConfig, error) {
	w := &dirWalker{}
	return loadFS(configDir, w)
}

func LoadTar(tarFile string) (*DeclarativeConfig, error) {
	f, err := os.Open(tarFile)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %v", err)
	}
	defer f.Close()

	tr := tar.NewReader(f)
	tw := tarWalker{tr}
	return loadFS("", tw)
}

func LoadFile(configFile string) (*DeclarativeConfig, error) {
	f, err := os.Open(configFile)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %v", err)
	}
	defer f.Close()
	return readJSON(f)
}

func loadFS(root string, w fsWalker) (*DeclarativeConfig, error) {
	cfg := &DeclarativeConfig{}
	if err := w.WalkFiles(root, func(path string, r io.Reader) error {
		fileCfg, err := readJSON(r)
		if err != nil {
			return fmt.Errorf("could not load config file %q: %v", path, err)
		}
		cfg.Packages = append(cfg.Packages, fileCfg.Packages...)
		cfg.Bundles = append(cfg.Bundles, fileCfg.Bundles...)
		cfg.others = append(cfg.others, fileCfg.others...)
		return nil
	}); err != nil {
		return nil, fmt.Errorf("failed to read declarative configs dir: %v", err)
	}
	return cfg, nil
}

func readJSON(r io.Reader) (*DeclarativeConfig, error) {
	cfg := &DeclarativeConfig{}
	dec := json.NewDecoder(r)
	for dec.More() {
		doc := &json.RawMessage{}
		if err := dec.Decode(doc); err != nil {
			return nil, fmt.Errorf("parse error at offset %d: %v", dec.InputOffset(), err)
		}

		var in meta
		if err := json.Unmarshal(*doc, &in); err != nil {
			return nil, fmt.Errorf("parse meta object at offset %d: %v", dec.InputOffset(), err)
		}

		switch in.Schema() {
		case schemaPackage:
			var p pkg
			if err := json.Unmarshal(*doc, &p); err != nil {
				return nil, fmt.Errorf("parse package at offset %d: %v", dec.InputOffset(), err)
			}
			cfg.Packages = append(cfg.Packages, p)
		case schemaBundle:
			var b bundle
			if err := json.Unmarshal(*doc, &b); err != nil {
				return nil, fmt.Errorf("parse bundle at offset %d: %v", dec.InputOffset(), err)
			}
			cfg.Bundles = append(cfg.Bundles, b)
		default:
			cfg.others = append(cfg.others, in)
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

type tarWalker struct {
	tr *tar.Reader
}

func (w tarWalker) WalkFiles(root string, f func(string, io.Reader) error) error {
	root = strings.TrimPrefix(root, "/")
	for {
		info, err := w.tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return err
		}
		if info.Typeflag != tar.TypeReg {
			continue
		}
		if !strings.HasPrefix(info.Name, root) {
			continue
		}
		if err := f(info.Name, w.tr); err != nil {
			return err
		}
	}
	return nil
}
