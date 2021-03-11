package declcfg

import (
	"sort"

	"github.com/operator-framework/operator-registry/pkg/model"
)

func ConvertFromModel(mpkgs model.Model) DeclarativeConfig {
	cfg := DeclarativeConfig{}
	for _, mpkg := range mpkgs {
		bundles := traverseModelChannels(*mpkg)

		var i *icon
		if mpkg.Icon != nil {
			i = &icon{
				Data:      mpkg.Icon.Data,
				MediaType: mpkg.Icon.MediaType,
			}
		}
		cfg.Packages = append(cfg.Packages, pkg{
			Schema:         schemaPackage,
			Name:           mpkg.Name,
			DefaultChannel: mpkg.DefaultChannel.Name,
			Icon:           i,
			Description:    mpkg.Description,
		})
		cfg.Bundles = append(cfg.Bundles, bundles...)
	}

	sort.Slice(cfg.Packages, func(i, j int) bool {
		return cfg.Packages[i].Name < cfg.Packages[j].Name
	})
	sort.Slice(cfg.Bundles, func(i, j int) bool {
		return cfg.Bundles[i].Name < cfg.Bundles[j].Name
	})

	return cfg
}

func traverseModelChannels(mpkg model.Package) []bundle {
	var bundles []bundle
	bundleNames := map[string]struct{}{}

	for _, ch := range mpkg.Channels {
		for _, chb := range ch.Bundles {
			_, ok := bundleNames[chb.Name]
			if ok {
				continue
			}
			bundleNames[chb.Name] = struct{}{}
			bundles = append(bundles, bundle{
				Schema:     schemaBundle,
				Name:       chb.Name,
				Image:      chb.Image,
				Properties: modelPropertiesToProperties(chb.Properties),

				// TODO(joelanford): Should related images be included in the model?,
				RelatedImages: nil,
			})
		}
	}
	return bundles
}

func modelPropertiesToProperties(props []model.Property) []property {
	var out []property
	for _, p := range props {
		out = append(out, property{
			Type:  p.Type,
			Value: p.Value,
		})
	}
	return out
}
