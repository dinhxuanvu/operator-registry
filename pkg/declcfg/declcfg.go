package declcfg

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
)

type DeclarativeConfig struct {
	Packages []pkg
	Bundles  []bundle
}

func LoadDir(configDir string) (*DeclarativeConfig, error) {
	cfg := &DeclarativeConfig{}
	files, err := ioutil.ReadDir(configDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read declarative configs dir: %v", err)
	}
	for _, finfo := range files {
		filename := filepath.Join(configDir, finfo.Name())
		fileCfg, err := LoadFile(filename)
		if err != nil {
			return nil, fmt.Errorf("could not load config file %q: %v", filename, err)
		}
		cfg.Packages = append(cfg.Packages, fileCfg.Packages...)
		cfg.Bundles = append(cfg.Bundles, fileCfg.Bundles...)
	}
	return cfg, nil
}

const (
	schemaPackage = "olm.package"
	schemaBundle  = "olm.bundle"
)

func LoadFile(configFile string) (*DeclarativeConfig, error) {
	f, err := os.Open(configFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %v", err)
	}
	defer f.Close()

	cfg := &DeclarativeConfig{}
	dec := json.NewDecoder(f)
	for dec.More() {
		doc := &json.RawMessage{}
		if err := dec.Decode(doc); err != nil {
			return nil, fmt.Errorf("parse error at offset %d: %v", dec.InputOffset(), err)
		}

		var in struct{ Schema string }
		if err := json.Unmarshal(*doc, &in); err != nil {
			return nil, fmt.Errorf("parse object for schema at offset %d: %v", dec.InputOffset(), err)
		}

		switch in.Schema {
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
			return nil, fmt.Errorf("unrecognized schema %q at offset %d", in.Schema, dec.InputOffset())
		}
	}
	return cfg, nil
}

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
		props, err := parseProperties(b.Properties)
		if err != nil {
			return fmt.Errorf("parse properties for bundle %q: %v", b.Name, err)
		}
		pkgName := props.providedPackage.PackageName
		bundlesByPackage[pkgName] = append(bundlesByPackage[pkgName], b)
	}

	for _, p := range cfg.Packages {
		fcfg := DeclarativeConfig{
			Packages: []pkg{p},
			Bundles:  bundlesByPackage[p.Name],
		}
		if err := WriteFile(fcfg, filepath.Join(configDir, fmt.Sprintf("%s.json", p.Name))); err != nil {
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
		props, err := parseProperties(b.Properties)
		if err != nil {
			return fmt.Errorf("parse properties for bundle %q: %v", b.Name, err)
		}
		pkgName := props.providedPackage.PackageName
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
	return nil
}
