package declcfg

import (
	"fmt"

	"github.com/operator-framework/operator-registry/pkg/model"
)

func ConvertToModel(cfg DeclarativeConfig) (model.Model, error) {
	mpkgs := model.Model{}
	defaultChannels := map[string]string{}
	for _, p := range cfg.Packages {
		mpkg := &model.Package{
			Name:        p.Name,
			Description: p.Description,
			Channels:    map[string]*model.Channel{},
		}
		if p.Icon != nil {
			mpkg.Icon = &model.Icon{
				Data:      p.Icon.Data,
				MediaType: p.Icon.MediaType,
			}
		}
		defaultChannels[p.Name] = p.DefaultChannel
		mpkgs[p.Name] = mpkg
	}

	for _, b := range cfg.Bundles {
		defaultChannelName := defaultChannels[b.Package]
		if b.Package == "" {
			return nil, fmt.Errorf("package name must be set for bundle %q", b.Name)
		}
		mpkg, ok := mpkgs[b.Package]
		if !ok {
			return nil, fmt.Errorf("unknown package %q for bundle %q", b.Package, b.Name)
		}

		props, err := parseProperties(b.Properties)
		if err != nil {
			return nil, fmt.Errorf("parse properties for bundle %q: %v", b.Name, err)
		}
		for _, bundleChannel := range props.channels {
			pkgChannel, ok := mpkg.Channels[bundleChannel.Name]
			if !ok {
				pkgChannel = &model.Channel{
					Package: mpkg,
					Name:    bundleChannel.Name,
					Bundles: map[string]*model.Bundle{},
				}
				if bundleChannel.Name == defaultChannelName {
					mpkg.DefaultChannel = pkgChannel
				}
				mpkg.Channels[bundleChannel.Name] = pkgChannel
			}

			pkgChannel.Bundles[b.Name] = &model.Bundle{
				Package:       mpkg,
				Channel:       pkgChannel,
				Name:          b.Name,
				Image:         b.Image,
				Replaces:      bundleChannel.Replaces,
				Skips:         props.skips,
				Properties:    propertiesToModelProperties(b.Properties),
				RelatedImages: relatedImagesToModelRelatedImages(b.RelatedImages),
			}
		}
		if mpkg.DefaultChannel == nil {
			dch := &model.Channel{
				Package: mpkg,
				Name:    defaultChannelName,
			}
			mpkg.DefaultChannel = dch
			mpkg.Channels[dch.Name] = dch
		}
	}

	if err := mpkgs.Validate(); err != nil {
		return nil, err
	}
	mpkgs.Normalize()
	return mpkgs, nil
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

func relatedImagesToModelRelatedImages(in []relatedImage) []model.RelatedImage {
	var out []model.RelatedImage
	for _, p := range in {
		out = append(out, model.RelatedImage{
			Name:  p.Name,
			Image: p.Image,
		})
	}
	return out
}
