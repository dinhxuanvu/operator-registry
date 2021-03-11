package declcfg

import (
	"fmt"

	"github.com/operator-framework/operator-registry/pkg/model"
)

type packageBundles struct {
	p       pkg
	bundles []bundle
	props   map[string]*properties
}

func ConvertToModel(cfg DeclarativeConfig) (model.Model, error) {
	pbs, err := buildPackageBundles(cfg)
	if err != nil {
		return nil, err
	}
	mpkgs := initPackages(pbs)
	if err := mpkgs.Validate(); err != nil {
		return nil, err
	}
	return mpkgs, nil
}

func buildPackageBundles(cfg DeclarativeConfig) (map[string]*packageBundles, error) {
	pbs := map[string]*packageBundles{}

	for _, p := range cfg.Packages {
		pbs[p.Name] = &packageBundles{
			p:     p,
			props: map[string]*properties{},
		}
	}

	for _, b := range cfg.Bundles {
		props, err := parseProperties(b.Properties)
		if err != nil {
			return nil, fmt.Errorf("parse properties for bundle %q: %v", b.Name, err)
		}
		pkgName := props.providedPackage.PackageName
		pb, ok := pbs[pkgName]
		if !ok {
			return nil, fmt.Errorf("unknown package %q for bundle %q", pkgName, b.Name)
		}
		pb.bundles = append(pb.bundles, b)
		pb.props[b.Name] = props
	}
	return pbs, nil
}

func initPackages(pbs map[string]*packageBundles) model.Model {
	mpkgs := model.Model{}
	for _, pb := range pbs {
		mpkgs[pb.p.Name] = initPackage(pb)
	}
	return mpkgs
}

func initPackage(pb *packageBundles) *model.Package {
	p, bundles, props := pb.p, pb.bundles, pb.props
	mpkg := &model.Package{
		Name:        p.Name,
		Description: p.Description,
	}
	if pb.p.Icon != nil {
		mpkg.Icon = &model.Icon{
			Data:      p.Icon.Data,
			MediaType: p.Icon.MediaType,
		}
	}
	mpkg.Channels = map[string]*model.Channel{}

	for _, b := range bundles {
		bundleProps := props[b.Name]
		for _, bundleChannel := range bundleProps.channels {
			pkgChannel, ok := mpkg.Channels[bundleChannel.Name]
			if !ok {
				pkgChannel = &model.Channel{
					Package: mpkg,
					Name:    bundleChannel.Name,
					Bundles: map[string]*model.Bundle{},
				}
				if bundleChannel.Name == p.DefaultChannel {
					mpkg.DefaultChannel = pkgChannel
				}
				mpkg.Channels[bundleChannel.Name] = pkgChannel
			}

			pkgChannel.Bundles[b.Name] = &model.Bundle{
				Package:    mpkg,
				Channel:    pkgChannel,
				Name:       b.Name,
				Image:      b.Image,
				Replaces:   bundleChannel.Replaces,
				Skips:      bundleProps.skips,
				Properties: propertiesToModelProperties(b.Properties),
			}
		}
	}
	if mpkg.DefaultChannel == nil {
		dch := &model.Channel{
			Package: mpkg,
			Name:    p.DefaultChannel,
			Bundles: nil,
		}
		mpkg.DefaultChannel = dch
		mpkg.Channels[dch.Name] = dch
	}
	return mpkg
}

func propertiesToModelProperties(in []property) []model.Property {
	var out []model.Property
	for _, p := range in {
		out = append(out, model.Property{
			Type:  p.Type,
			Value: p.Value,
		})
	}
	return out
}
