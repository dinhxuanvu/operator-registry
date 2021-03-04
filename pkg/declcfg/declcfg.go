package declcfg

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/operator-framework/operator-registry/pkg/model"
)

type DeclarativeConfig struct {
	Packages []Package
	Bundles  []Bundle
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
			var pkg Package
			if err := json.Unmarshal(*doc, &pkg); err != nil {
				return nil, fmt.Errorf("unmarshal as package at offset %d: %v", dec.InputOffset(), err)
			}
			cfg.Packages = append(cfg.Packages, pkg)
		case schemaBundle:
			var bundle Bundle
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

func (cfg DeclarativeConfig) ToModel() (model.Model, error) {
	pkgs := initializeModelPackages(cfg.Packages)
	if err := populateModelChannels(pkgs, cfg.Bundles); err != nil {
		return nil, fmt.Errorf("populate channels: %v", err)
	}
	if err := pkgs.Validate(); err != nil {
		return nil, err
	}
	return pkgs, nil
}

func initializeModelPackages(dPkgs []Package) model.Model {
	pkgs := model.Model{}
	for _, dPkg := range dPkgs {
		pkg := model.Package{
			Name:        dPkg.Name,
			Description: dPkg.Description,
		}
		if dPkg.Icon != nil {
			pkg.Icon = &model.Icon{
				Data:      dPkg.Icon.Base64Data,
				MediaType: dPkg.Icon.MediaType,
			}
		}

		pkg.Channels = map[string]*model.Channel{}
		for _, ch := range dPkg.Channels {
			channel := &model.Channel{
				Package: &pkg,
				Name:    ch,
				Bundles: map[string]*model.Bundle{},
			}
			if ch == dPkg.DefaultChannel {
				pkg.DefaultChannel = channel
			}
			pkg.Channels[ch] = channel
		}
		pkgs[pkg.Name] = &pkg
	}
	return pkgs
}

func populateModelChannels(pkgs model.Model, dBundles []Bundle) error {
	for _, dBundle := range dBundles {
		pkg, ok := pkgs[dBundle.Package]
		if !ok {
			return fmt.Errorf("unknown package %q for bundle %q", dBundle.Package, dBundle.Name)
		}
		for _, bundleChannel := range dBundle.ChannelEntries() {
			pkgChannel, ok := pkg.Channels[bundleChannel.ChannelName]
			if !ok {
				return fmt.Errorf("unknown channel %q for bundle %q", bundleChannel.ChannelName, dBundle.Name)
			}
			props, err := dBundle.GetProperties()
			if err != nil {
				return fmt.Errorf("get properties for bundle %q: %v", dBundle.Name, err)
			}
			pkgChannel.Bundles[dBundle.Name] = &model.Bundle{
				Package:          pkg,
				Channel:          pkgChannel,
				Name:             dBundle.Name,
				Version:          dBundle.Version,
				Image:            dBundle.Image,
				Replaces:         bundleChannel.Replaces,
				Skips:            dBundle.Skips(),
				SkipRange:        dBundle.SkipRange(),
				ProvidedAPIs:     dBundle.ProvidedAPIs(),
				RequiredAPIs:     dBundle.RequiredAPIs(),
				Properties:       props,
				RequiredPackages: dBundle.RequiredPackages(),
			}
		}
	}
	return nil
}
