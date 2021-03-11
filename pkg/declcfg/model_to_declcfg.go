package declcfg

import (
	"github.com/operator-framework/operator-registry/pkg/model"
)

func ConvertFromModel(mpkgs model.Model) DeclarativeConfig {
	cfg := DeclarativeConfig{}
	for _, mpkg := range mpkgs {
		channels, bundles := traverseModelChannels(*mpkg)

		var i *icon
		if mpkg.Icon != nil {
			i = &icon{
				Data:      mpkg.Icon.Data,
				MediaType: mpkg.Icon.MediaType,
			}
		}
		cfg.Packages = append(cfg.Packages, pkg{
			Schema:            schemaPackage,
			Name:              mpkg.Name,
			DefaultChannel:    mpkg.DefaultChannel.Name,
			Icon:              i,
			ValidChannelNames: channels,
			Description:       mpkg.Description,
		})
		cfg.Bundles = append(cfg.Bundles, bundles...)
	}
	return cfg
}

func traverseModelChannels(mpkg model.Package) ([]string, []bundle) {
	var (
		channels []string
		bundles  []bundle
	)
	bundleNames := map[string]struct{}{}

	for _, ch := range mpkg.Channels {
		channels = append(channels, ch.Name)
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
	return channels, bundles
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
