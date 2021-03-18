package declcfg

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
)

type DeclarativeConfig struct {
	Packages []pkg
	Bundles  []bundle
	Others   []meta
}

func LoadDir(configDir string) (*DeclarativeConfig, error) {
	cfg := &DeclarativeConfig{}

	if err := filepath.Walk(configDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		fileCfg, err := LoadFile(path)
		if err != nil {
			return fmt.Errorf("could not load config file %q: %v", path, err)
		}
		cfg.Packages = append(cfg.Packages, fileCfg.Packages...)
		cfg.Bundles = append(cfg.Bundles, fileCfg.Bundles...)
		cfg.Others = append(cfg.Others, fileCfg.Others...)
		return nil
	}); err != nil {
		return nil, fmt.Errorf("failed to read declarative configs dir: %v", err)
	}
	return cfg, nil
}

const (
	schemaPackage = "olm.package"
	schemaBundle  = "olm.bundle"
)

func LoadReader(r io.Reader) (*DeclarativeConfig, error) {
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
			cfg.Others = append(cfg.Others, in)
		}
	}
	return cfg, nil
}

func LoadFile(configFile string) (*DeclarativeConfig, error) {
	f, err := os.Open(configFile)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %v", err)
	}
	defer f.Close()
	return LoadReader(f)
}

const globalName = "__global"

func WriteDir(cfg DeclarativeConfig, configDir string) error {
	entries, err := ioutil.ReadDir(configDir)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if len(entries) > 0 {
		return fmt.Errorf("config dir %q must be empty", configDir)
	}
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return err
	}

	bundlesByPackage := map[string][]bundle{}
	for _, b := range cfg.Bundles {
		bundlesByPackage[b.Package] = append(bundlesByPackage[b.Package], b)
	}
	othersByPackage := map[string][]meta{}
	for _, o := range cfg.Others {
		pkgName := o.Package()
		if pkgName == "" {
			pkgName = globalName
		}
		othersByPackage[pkgName] = append(othersByPackage[pkgName], o)
	}

	for _, p := range cfg.Packages {
		fcfg := DeclarativeConfig{
			Packages: []pkg{p},
			Bundles:  bundlesByPackage[p.Name],
			Others:   othersByPackage[p.Name],
		}
		if err := WriteFile(fcfg, filepath.Join(configDir, fmt.Sprintf("%s.json", p.Name))); err != nil {
			return err
		}
	}

	globals, ok := othersByPackage[globalName]
	if ok {
		ocfg := DeclarativeConfig{
			Others: globals,
		}
		if err := WriteFile(ocfg, filepath.Join(configDir, fmt.Sprintf("%s.json", globalName))); err != nil {
			return err
		}
	}
	return nil
}

func WriteFile(cfg DeclarativeConfig, configFile string) error {
	f, err := os.OpenFile(configFile, os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	enc.SetIndent("", "    ")
	enc.SetEscapeHTML(false)

	bundlesByPackage := map[string][]bundle{}
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
