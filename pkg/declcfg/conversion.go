package declcfg

import (
	"encoding/json"
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/operator-framework/operator-registry/pkg/model"
)

func ConvertToModel(cfg *DeclarativeConfig) (model.Model, error) {
	pkgs := initializeModelPackages(cfg.Packages)
	for _, dPkg := range cfg.Packages {
		if err := populatePackageChannels(pkgs, dPkg, cfg.Bundles); err != nil {
			return nil, fmt.Errorf("populate channel %q: %v", dPkg.Name, err)
		}
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
				Data:      p.Icon.Data,
				MediaType: p.Icon.MediaType,
			}
		}

		var validChannelNames []string
		for _, ch := range p.Channels {
			validChannelNames = append(validChannelNames, ch.Name)

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
			Schema:            schemaPackage,
			Name:              p.Name,
			DefaultChannel:    p.DefaultChannel.Name,
			Icon:              i,
			ValidChannelNames: validChannelNames,
			Description:       p.Description,
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
				Data:      dPkg.Icon.Data,
				MediaType: dPkg.Icon.MediaType,
			}
		}
		pkg.Channels = map[string]*model.Channel{}
		pkgs[pkg.Name] = &pkg
	}
	return pkgs
}

func populatePackageChannels(pkgs model.Model, dPkg pkg, bundles []bundle) error {
	for _, b := range bundles {
		// TODO(joelanford): The below two checks are a little hacky since we're dealing
		//   with lists instead of maps. It would be good to revisit this (perhaps by
		//   arranging the bundles with their package in a map keyed by the package name)

		// Check if the package name declared by the bundle actually exists in the set
		// of bundles we already initialized in `pkgs`. We do this before comparing
		// b.Package with dPkg.Name; otherwise we would silently ignore bundles with
		// unknown packages.
		mPkg, ok := pkgs[b.Package]
		if !ok {
			return fmt.Errorf("unknown package %q for bundle %q", b.Package, b.Name)
		}

		// Only once we've confirmed that the bundle package is actually in our set of
		// packages (above), do we check to see if we should ignore this bundle because
		// it isn't for the package we're populating.
		if b.Package != dPkg.Name {
			continue
		}

		props, err := parseProperties(b.Properties)
		if err != nil {
			return fmt.Errorf("parse properties: %v", err)
		}

		for _, bundleChannel := range props.channels {
			pkgChannel, ok := mPkg.Channels[bundleChannel.Name]
			if !ok {
				if !dPkg.isValidChannel(bundleChannel.Name) {
					return fmt.Errorf("bundle %q channel %q is not in package's valid channel names %q", b.Name, bundleChannel.Name, dPkg.ValidChannelNames)
				}
				pkgChannel = &model.Channel{
					Package: mPkg,
					Name:    bundleChannel.Name,
					Bundles: map[string]*model.Bundle{},
				}
				if bundleChannel.Name == dPkg.DefaultChannel {
					mPkg.DefaultChannel = pkgChannel
				}
				mPkg.Channels[bundleChannel.Name] = pkgChannel
			}

			pkgChannel.Bundles[b.Name] = &model.Bundle{
				Package:          mPkg,
				Channel:          pkgChannel,
				Name:             b.Name,
				Version:          b.Version,
				Image:            b.Image,
				Replaces:         bundleChannel.Replaces,
				Skips:            props.skips,
				SkipRange:        props.skipRange,
				ProvidedAPIs:     gvksToModelGVKs(props.providedGVKs),
				RequiredAPIs:     gvksToModelGVKs(props.requiredGVKs),
				RequiredPackages: requiredPackagesToModelRequiredPackages(props.requiredPackages),
				Properties:       propertiesToModelProperties(props.all),
			}
		}
	}
	return nil
}

func gvksToModelGVKs(in []gvk) []model.GroupVersionKind {
	var out []model.GroupVersionKind
	for _, i := range in {
		out = append(out, model.GroupVersionKind{
			Group:   i.Group,
			Version: i.Version,
			Kind:    i.Kind,
			Plural:  i.Plural,
		})
	}
	return out
}

func requiredPackagesToModelRequiredPackages(in []requiredPackage) []model.RequiredPackage {
	var out []model.RequiredPackage
	for _, rp := range in {
		out = append(out, model.RequiredPackage{
			PackageName:  rp.PackageName,
			VersionRange: rp.VersionRange,
		})
	}
	return out
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

type propertyParseError struct {
	i   int
	t   string
	err error
}

func (e propertyParseError) Error() string {
	return fmt.Sprintf("properties[%d].value parse error for %q: %v", e.i, e.t, e.err)
}

type propertyMultipleNotAllowedError struct {
	i int
	t string
}

func (e propertyMultipleNotAllowedError) Error() string {
	return fmt.Sprintf("properties[%d]: multiple properties of type %q not allowed", e.i, e.t)
}

func extractGlobalPropertiesFromModelBundle(b model.Bundle) []property {
	var out []property

	out = append(out, property{
		Type: propertyTypePackageProvided,
		Value: mustJSONMarshal(providedPackage{
			PackageName: b.Package.Name,
			Version:     b.Version,
		}),
	})

	for _, rp := range b.RequiredPackages {
		out = append(out, property{
			Type:  propertyTypePackageRequired,
			Value: mustJSONMarshal(rp),
		})
	}

	for _, papi := range b.ProvidedAPIs {
		out = append(out, property{
			Type:  propertyTypeGVKProvided,
			Value: mustJSONMarshal(papi),
		})
	}

	for _, rapi := range b.RequiredAPIs {
		out = append(out, property{
			Type:  propertyTypeGVKRequired,
			Value: mustJSONMarshal(rapi),
		})
	}

	for _, skip := range b.Skips {
		out = append(out, property{
			Type:  propertyTypeSkips,
			Value: mustJSONMarshal(skip),
		})
	}

	if b.SkipRange != "" {
		out = append(out, property{
			Type:  propertyTypeSkipRange,
			Value: mustJSONMarshal(b.SkipRange),
		})
	}

	return out
}

func mustJSONMarshal(v interface{}) []byte {
	out, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return out
}

const (
	// Required to build model.
	propertyTypeChannel = "olm.channel"
	propertyTypeSkips   = "olm.skips"

	// TODO(joelanford): Not required, but maybe nice to validate?
	propertyTypeSkipRange       = "olm.skipRange"
	propertyTypePackageProvided = "olm.package.provided"
	propertyTypePackageRequired = "olm.package.required"
	propertyTypeGVKRequired     = "olm.gvk.required"

	// The following properties are required to maintain backwards-compatibility
	// with the GRPC Bundle API.
	propertyTypeGVKProvided = "olm.gvk.provided"
	propertyTypeObject      = "olm.object"
)

type channel struct {
	Name     string `json:"name"`
	Replaces string `json:"replaces"`
}

type providedPackage struct {
	PackageName string `json:"packageName"`
	Version     string `json:"version"`
}

type requiredPackage struct {
	PackageName  string `json:"packageName"`
	VersionRange string `json:"versionRange"`
}

type gvk struct {
	Group   string `json:"group"`
	Version string `json:"version"`
	Kind    string `json:"kind"`
	Plural  string `json:"plural,omitempty"`
}

type properties struct {
	channels         []channel
	skips            []string
	skipRange        string
	providedPackage  *providedPackage
	requiredPackages []requiredPackage
	providedGVKs     []gvk
	requiredGVKs     []gvk
	objects          []unstructured.Unstructured
	others           []property
	all              []property
}

func parseProperties(props []property) (*properties, error) {
	ps := properties{}

	for i, prop := range props {
		ps.all = append(ps.all, prop)
		switch prop.Type {
		case propertyTypeChannel:
			var p channel
			if err := json.Unmarshal(prop.Value, &p); err != nil {
				return nil, propertyParseError{i: i, t: prop.Type, err: err}
			}
			ps.channels = append(ps.channels, p)
		case propertyTypeSkips:
			var p string
			if err := json.Unmarshal(prop.Value, &p); err != nil {
				return nil, propertyParseError{i: i, t: prop.Type, err: err}
			}
			ps.skips = append(ps.skips, p)
		case propertyTypeSkipRange:
			var p string
			if err := json.Unmarshal(prop.Value, &p); err != nil {
				return nil, propertyParseError{i: i, t: prop.Type, err: err}
			}
			if ps.skipRange != "" {
				return nil, propertyMultipleNotAllowedError{i: i, t: prop.Type}
			}
			ps.skipRange = p
		case propertyTypePackageProvided:
			var p providedPackage
			if err := json.Unmarshal(prop.Value, &p); err != nil {
				return nil, propertyParseError{i: i, t: prop.Type, err: err}
			}
			if ps.providedPackage != nil {
				return nil, propertyMultipleNotAllowedError{i: i, t: prop.Type}
			}
			ps.providedPackage = &p
		case propertyTypePackageRequired:
			var p requiredPackage
			if err := json.Unmarshal(prop.Value, &p); err != nil {
				return nil, propertyParseError{i: i, t: prop.Type, err: err}
			}
			ps.requiredPackages = append(ps.requiredPackages, p)
		case propertyTypeGVKProvided:
			var p gvk
			if err := json.Unmarshal(prop.Value, &p); err != nil {
				return nil, propertyParseError{i: i, t: prop.Type, err: err}
			}
			ps.providedGVKs = append(ps.providedGVKs, p)
		case propertyTypeGVKRequired:
			var p gvk
			if err := json.Unmarshal(prop.Value, &p); err != nil {
				return nil, propertyParseError{i: i, t: prop.Type, err: err}
			}
			ps.requiredGVKs = append(ps.requiredGVKs, p)
		default:
			ps.others = append(ps.others, prop)
		}
	}

	return &ps, nil
}
