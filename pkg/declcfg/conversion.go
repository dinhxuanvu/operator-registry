package declcfg

import (
	"encoding/json"
	"fmt"

	"github.com/operator-framework/operator-registry/pkg/model"
)

func ConvertToModel(cfg *DeclarativeConfig) (model.Model, error) {
	pkgs := initializeModelPackages(cfg.Packages)
	if err := populateModelChannels(pkgs, cfg.Bundles); err != nil {
		return nil, fmt.Errorf("populate channels: %v", err)
	}
	if err := pkgs.Validate(); err != nil {
		return nil, err
	}
	return pkgs, nil
}

func ConvertFromModel(m model.Model) DeclarativeConfig {
	packages := []pkg{}
	bundleMap := map[string]*bundle{}

	for _, p := range m {
		var i *icon
		if p.Icon != nil {
			i = &icon{
				Base64Data: p.Icon.Data,
				MediaType:  p.Icon.MediaType,
			}
		}

		var channels []string
		for _, ch := range p.Channels {
			channels = append(channels, ch.Name)

			for _, chb := range ch.Bundles {
				b, ok := bundleMap[chb.Name]
				if !ok {
					b = &bundle{
						Schema:     schemaBundle,
						Name:       chb.Name,
						Package:    p.Name,
						Image:      chb.Image,
						Version:    chb.Version,
						Properties: extractGlobalPropertiesFromModelBundle(*chb),
					}
				}
				if chb.Replaces == "" {
					b.Properties = append(b.Properties, property{
						Type:  propertyTypeChannel,
						Value: json.RawMessage(fmt.Sprintf(`{"name":%q}`, ch.Name)),
					})
				} else {
					b.Properties = append(b.Properties, property{
						Type:  propertyTypeChannel,
						Value: json.RawMessage(fmt.Sprintf(`{"name":%q,"replaces":%q}`, ch.Name, chb.Replaces)),
					})
				}
				bundleMap[chb.Name] = b
			}
		}
		packages = append(packages, pkg{
			Schema:         schemaPackage,
			Name:           p.Name,
			DefaultChannel: p.DefaultChannel.Name,
			Icon:           i,
			Channels:       channels,
			Description:    p.Description,
		})
	}

	var bundles []bundle
	for _, bundle := range bundleMap {
		bundles = append(bundles, *bundle)
	}

	return DeclarativeConfig{
		Packages: packages,
		Bundles:  bundles,
	}
}

func initializeModelPackages(dPkgs []pkg) model.Model {
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

func populateModelChannels(pkgs model.Model, bundles []bundle) error {
	for _, b := range bundles {
		pkg, ok := pkgs[b.Package]
		if !ok {
			return fmt.Errorf("unknown package %q for bundle %q", b.Package, b.Name)
		}

		channels, err := extractChannelProperties(b.Properties)
		if err != nil {
			return fmt.Errorf("get channels for bundle %q", b.Name)
		}

		for _, bundleChannel := range channels {
			pkgChannel, ok := pkg.Channels[bundleChannel.Name]
			if !ok {
				return fmt.Errorf("unknown channel %q for bundle %q", bundleChannel.Name, b.Name)
			}

			// Parse "olm.skips" properties. Combine all found into a flattened list.
			skips, err := convertSkipsToModelSkips(b.Properties)
			if err != nil {
				return fmt.Errorf("extract skips properties for bundle %q: %v", b.Name, err)
			}

			// Parse "olm.skipRange" properties. Allow max of 1.
			skipRange, err := convertSkipRangeToModelSkipRange(b.Properties)
			if err != nil {
				return fmt.Errorf("extract skipRange properties for bundle %q: %v", b.Name, err)
			}

			// Parse "olm.gvk.provided" properties.
			providedAPIs, err := convertProvidedGVKsToModelGVKs(b.Properties)
			if err != nil {
				return fmt.Errorf("extract provided GVK properties for bundle %q: %v", b.Name, err)
			}

			// Parse "olm.gvk.required" properties.
			requiredAPIs, err := convertRequiredGVKsToModelGVKs(b.Properties)
			if err != nil {
				return fmt.Errorf("extract required GVK properties for bundle %q: %v", b.Name, err)
			}

			requiredPackages, err := convertRequiredPackagedToModelRequiredPackages(b.Properties)
			if err != nil {
				return fmt.Errorf("extract required package properties for bundle %q: %v", b.Name, err)
			}

			// Convert all properties as a catch all. This handles forwarding along properties we don't know about.
			props := convertPropertiesToModelProperties(b.Properties)

			pkgChannel.Bundles[b.Name] = &model.Bundle{
				Package:          pkg,
				Channel:          pkgChannel,
				Name:             b.Name,
				Version:          b.Version,
				Image:            b.Image,
				Replaces:         bundleChannel.Replaces,
				Skips:            skips,
				SkipRange:        skipRange,
				ProvidedAPIs:     providedAPIs,
				RequiredAPIs:     requiredAPIs,
				RequiredPackages: requiredPackages,
				Properties:       props,
			}
		}
	}
	return nil
}

type propertyParseError struct {
	i   int
	err error
}

func (e propertyParseError) Error() string {
	return fmt.Sprintf("properties[%d].value parse error for: %v", e.i, e.err)
}

const (
	propertyTypeChannel   = "olm.channel"
	propertyTypeSkips     = "olm.skips"
	propertyTypeSkipRange = "olm.skipRange"

	propertyTypePackageProvided = "olm.package.provided"
	propertyTypePackageRequired = "olm.package.required"
	propertyTypeGVKProvided     = "olm.gvk.provided"
	propertyTypeGVKRequired     = "olm.gvk.required"
)

type channelProperty struct {
	Name     string `json:"name"`
	Replaces string `json:"replaces"`
}

func extractChannelProperties(props []property) ([]channelProperty, error) {
	var out []channelProperty
	for i, prop := range props {
		if prop.Type != propertyTypeChannel {
			continue
		}
		var obj channelProperty
		if err := json.Unmarshal(prop.Value, &obj); err != nil {
			return nil, propertyParseError{i, err}
		}
		out = append(out, obj)
	}
	return out, nil
}

func convertSkipsToModelSkips(props []property) ([]string, error) {
	var out []string
	for i, prop := range props {
		if prop.Type != propertyTypeSkips {
			continue
		}
		var obj []string
		if err := json.Unmarshal(prop.Value, &obj); err != nil {
			return nil, propertyParseError{i, err}
		}
		out = append(out, obj...)
	}
	return out, nil
}

func convertSkipRangeToModelSkipRange(props []property) (string, error) {
	var skipRanges []string
	for i, prop := range props {
		if prop.Type != propertyTypeSkipRange {
			continue
		}
		var skipRange string
		if err := json.Unmarshal(prop.Value, &skipRange); err != nil {
			return "", propertyParseError{i, err}
		}
		skipRanges = append(skipRanges, skipRange)
	}
	switch len(skipRanges) {
	case 0:
		return "", nil
	case 1:
		return skipRanges[0], nil
	}
	return "", fmt.Errorf("found multiple olm.skipRange properties")
}

func convertProvidedGVKsToModelGVKs(props []property) ([]model.GroupVersionKind, error) {
	return convertPropertiesToModelGVKs(props, propertyTypeGVKProvided)
}

func convertRequiredGVKsToModelGVKs(props []property) ([]model.GroupVersionKind, error) {
	return convertPropertiesToModelGVKs(props, propertyTypeGVKRequired)
}

func convertPropertiesToModelGVKs(props []property, propType string) ([]model.GroupVersionKind, error) {
	var gvks []model.GroupVersionKind
	for i, p := range props {
		if p.Type == propType {
			var gvk model.GroupVersionKind
			if err := json.Unmarshal(p.Value, &gvk); err != nil {
				return nil, propertyParseError{i, err}
			}
			gvks = append(gvks, gvk)
		}
	}
	return gvks, nil
}

func convertRequiredPackagedToModelRequiredPackages(props []property) ([]model.RequiredPackage, error) {
	var rps []model.RequiredPackage
	for i, p := range props {
		if p.Type == propertyTypePackageRequired {
			var rp model.RequiredPackage
			if err := json.Unmarshal(p.Value, &rp); err != nil {
				return nil, propertyParseError{i, err}
			}
			rps = append(rps, rp)
		}
	}
	return rps, nil
}

func convertPropertiesToModelProperties(props []property) []model.Property {
	var out []model.Property
	for _, p := range props {
		out = append(out, model.Property(p))
	}
	return out
}

func extractGlobalPropertiesFromModelBundle(b model.Bundle) []property {
	var out []property

	out = append(out, property{
		Type:  propertyTypePackageProvided,
		Value: json.RawMessage(fmt.Sprintf(`{"packageName":%q,"version":%q}`, b.Package.Name, b.Version)),
	})

	for _, rp := range b.RequiredPackages {
		out = append(out, property{
			Type:  propertyTypePackageRequired,
			Value: json.RawMessage(fmt.Sprintf(`{"packageName":%q,"versionRange":%q}`, rp.PackageName, rp.VersionRange)),
		})
	}

	for _, papi := range b.ProvidedAPIs {
		out = append(out, property{
			Type:  propertyTypeGVKProvided,
			Value: json.RawMessage(fmt.Sprintf(`{"group":%q,"version":%q,"kind":%q}`, papi.Group, papi.Version, papi.Kind)),
		})
	}

	for _, rapi := range b.RequiredAPIs {
		out = append(out, property{
			Type:  propertyTypeGVKRequired,
			Value: json.RawMessage(fmt.Sprintf(`{"group":%q,"version":%q,"kind":%q}`, rapi.Group, rapi.Version, rapi.Kind)),
		})
	}

	if len(b.Skips) > 0 {
		skips, _ := json.Marshal(b.Skips)
		out = append(out, property{
			Type:  propertyTypeSkips,
			Value: skips,
		})
	}

	if b.SkipRange != "" {
		out = append(out, property{
			Type:  propertyTypeSkipRange,
			Value: json.RawMessage(fmt.Sprintf("%q", b.SkipRange)),
		})
	}
	return out
}
