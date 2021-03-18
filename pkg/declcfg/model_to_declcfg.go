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
	bundles := map[string]*bundle{}

	for _, ch := range mpkg.Channels {
		for _, chb := range ch.Bundles {
			b, ok := bundles[chb.Name]
			if !ok {
				b = &bundle{
					Schema:        schemaBundle,
					Name:          chb.Name,
					Package:       chb.Package.Name,
					Image:         chb.Image,
					RelatedImages: modelRelatedImagesToRelatedImages(chb.RelatedImages),
				}
				bundles[b.Name] = b
			}
			b.Properties = append(b.Properties, modelPropertiesToProperties(chb.Properties)...)
		}
	}

	var out []bundle
	for _, b := range bundles {
		b.Properties = deduplicateProperties(b.Properties)
		out = append(out, *b)
	}
	return out
}

func modelRelatedImagesToRelatedImages(relatedImages []model.RelatedImage) []relatedImage {
	var out []relatedImage
	for _, ri := range relatedImages {
		out = append(out, relatedImage{
			Name:  ri.Name,
			Image: ri.Image,
		})
	}
	return out
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

func deduplicateProperties(in []property) []property {
	type key struct {
		typ   string
		value string
	}

	props := map[key]property{}
	var out []property
	for _, p := range in {
		k := key{p.Type, string(p.Value)}
		if _, ok := props[k]; ok {
			continue
		}
		props[k] = p
		out = append(out, p)
	}
	return out
}
