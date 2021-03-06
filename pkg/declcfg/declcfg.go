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
			return nil, fmt.Errorf("decode error at offset %d: %v", dec.InputOffset(), err)
		}

		var in struct{ Schema string }
		if err := json.Unmarshal(*doc, &in); err != nil {
			return nil, fmt.Errorf("unmarshal for schema at offset %d: %v", dec.InputOffset(), err)
		}

		switch in.Schema {
		case schemaPackage:
			var pkg pkg
			if err := json.Unmarshal(*doc, &pkg); err != nil {
				return nil, fmt.Errorf("unmarshal as package at offset %d: %v", dec.InputOffset(), err)
			}
			cfg.Packages = append(cfg.Packages, pkg)
		case schemaBundle:
			var bundle bundle
			if err := json.Unmarshal(*doc, &bundle); err != nil {
				return nil, fmt.Errorf("unmarshal as bundle at offset %d: %v", dec.InputOffset(), err)
			}
			cfg.Bundles = append(cfg.Bundles, bundle)
		default:
			return nil, fmt.Errorf("unrecognized schema at offset %d", dec.InputOffset())
		}
	}
	return cfg, nil
}
