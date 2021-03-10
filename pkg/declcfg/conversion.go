package declcfg

import (
	"encoding/base64"
	"encoding/json"
	"fmt"

	"github.com/blang/semver/v4"
	"github.com/operator-framework/api/pkg/lib/version"
	"github.com/operator-framework/api/pkg/operators/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

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
				Data:      p.Icon.Data,
				MediaType: p.Icon.MediaType,
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
				Data:      dPkg.Icon.Data,
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

		props, err := parseProperties(b.Properties)
		if err != nil {
			return fmt.Errorf("parse properties: %v", err)
		}

		bundleVersion, err := semver.ParseTolerant(b.Version)
		if err != nil {
			return fmt.Errorf("parse version for bundle %q: %v", b.Name, err)
		}

		var icons []v1alpha1.Icon
		if pkg.Icon != nil {
			icons = []v1alpha1.Icon{modelIconToCSVIcon(*pkg.Icon)}
		}

		var csvProvider v1alpha1.AppLink
		if props.csvProvider != nil {
			csvProvider = *props.csvProvider
		}

		for _, bundleChannel := range props.channels {
			pkgChannel, ok := pkg.Channels[bundleChannel.Name]
			if !ok {
				return fmt.Errorf("unknown channel %q for bundle %q", bundleChannel.Name, b.Name)
			}

			csv := &v1alpha1.ClusterServiceVersion{
				TypeMeta: metav1.TypeMeta{
					APIVersion: v1alpha1.SchemeGroupVersion.String(),
					Kind:       v1alpha1.ClusterServiceVersionKind,
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:        b.Name,
					Annotations: props.csvAnnotations,
				},
				Spec: v1alpha1.ClusterServiceVersionSpec{
					DisplayName:  props.csvDisplayName,
					Icon:         icons,
					Version:      version.OperatorVersion{Version: bundleVersion},
					Provider:     csvProvider,
					Annotations:  props.csvAnnotations,
					Keywords:     props.csvKeywords,
					Links:        props.csvLinks,
					Maintainers:  props.csvMaintainers,
					Maturity:     props.csvMaturity,
					Description:  props.csvDescription,
					InstallModes: props.csvInstallModes,

					// TODO(joelanford): Fill these in?
					CustomResourceDefinitions: v1alpha1.CustomResourceDefinitions{},
					APIServiceDefinitions:     v1alpha1.APIServiceDefinitions{},
					NativeAPIs:                nil,

					MinKubeVersion: props.csvMinKubeVersion,
				},
			}

			if props.skipRange != "" {
				csv.ObjectMeta.Annotations["olm.skipRange"] = props.skipRange
			}

			pkgChannel.Bundles[b.Name] = &model.Bundle{
				Package:          pkg,
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
				CSV:              csv,
				Objects:          props.objects,
			}
		}
	}
	return nil
}

func modelIconToCSVIcon(in model.Icon) v1alpha1.Icon {
	return v1alpha1.Icon{
		Data:      base64.StdEncoding.EncodeToString(in.Data),
		MediaType: in.MediaType,
	}
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

	if b.CSV != nil {
		if len(b.CSV.Annotations) > 0 {
			out = append(out, property{
				Type:  propertyTypeCSVAnnotations,
				Value: mustJSONMarshal(b.CSV.Annotations),
			})
		}
		if len(b.CSV.Spec.Description) > 0 {
			out = append(out, property{
				Type:  propertyTypeCSVDescription,
				Value: mustJSONMarshal(b.CSV.Spec.Description),
			})
		}
		if len(b.CSV.Spec.DisplayName) > 0 {
			out = append(out, property{
				Type:  propertyTypeCSVDisplayName,
				Value: mustJSONMarshal(b.CSV.Spec.DisplayName),
			})
		}

		for _, im := range b.CSV.Spec.InstallModes {
			out = append(out, property{
				Type:  propertyTypeCSVInstallMode,
				Value: mustJSONMarshal(im),
			})
		}
		for _, k := range b.CSV.Spec.Keywords {
			out = append(out, property{
				Type:  propertyTypeCSVKeyword,
				Value: mustJSONMarshal(k),
			})
		}
		for _, l := range b.CSV.Spec.Links {
			out = append(out, property{
				Type:  propertyTypeCSVLink,
				Value: mustJSONMarshal(l),
			})
		}
		for _, m := range b.CSV.Spec.Maintainers {
			out = append(out, property{
				Type:  propertyTypeCSVMaintainer,
				Value: mustJSONMarshal(m),
			})
		}
		if len(b.CSV.Spec.Maturity) > 0 {
			out = append(out, property{
				Type:  propertyTypeCSVMaturity,
				Value: mustJSONMarshal(b.CSV.Spec.Maturity),
			})
		}
		if len(b.CSV.Spec.MinKubeVersion) > 0 {
			out = append(out, property{
				Type:  propertyTypeCSVMinKubeVersion,
				Value: mustJSONMarshal(b.CSV.Spec.MinKubeVersion),
			})
		}
		if len(b.CSV.Spec.Provider.Name) > 0 || len(b.CSV.Spec.Provider.URL) > 0 {
			out = append(out, property{
				Type:  propertyTypeCSVProvider,
				Value: mustJSONMarshal(b.CSV.Spec.Provider),
			})
		}
	}

	for _, obj := range b.Objects {
		out = append(out, property{
			Type:  propertyTypeObject,
			Value: mustJSONMarshal(obj.Object),
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
	propertyTypeGVKProvided       = "olm.gvk.provided"
	propertyTypeCSVAnnotations    = "olm.csv.annotations"
	propertyTypeCSVDescription    = "olm.csv.description"
	propertyTypeCSVDisplayName    = "olm.csv.displayName"
	propertyTypeCSVInstallMode    = "olm.csv.installMode"
	propertyTypeCSVKeyword        = "olm.csv.keyword"
	propertyTypeCSVLink           = "olm.csv.link"
	propertyTypeCSVMaintainer     = "olm.csv.maintainer"
	propertyTypeCSVMaturity       = "olm.csv.maturity"
	propertyTypeCSVMinKubeVersion = "olm.csv.minKubeVersion"
	propertyTypeCSVProvider       = "olm.csv.provider"
	propertyTypeObject            = "olm.object"
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
	channels          []channel
	skips             []string
	skipRange         string
	providedPackage   *providedPackage
	requiredPackages  []requiredPackage
	providedGVKs      []gvk
	requiredGVKs      []gvk
	csvAnnotations    map[string]string
	csvDescription    string
	csvDisplayName    string
	csvInstallModes   []v1alpha1.InstallMode
	csvKeywords       []string
	csvLinks          []v1alpha1.AppLink
	csvMaintainers    []v1alpha1.Maintainer
	csvMaturity       string
	csvMinKubeVersion string
	csvProvider       *v1alpha1.AppLink
	objects           []unstructured.Unstructured
	others            []property
	all               []property
}

func parseProperties(props []property) (*properties, error) {
	ps := properties{
		csvAnnotations: map[string]string{},
	}

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
		case propertyTypeCSVAnnotations:
			p := map[string]string{}
			if err := json.Unmarshal(prop.Value, &p); err != nil {
				return nil, propertyParseError{i: i, t: prop.Type, err: err}
			}
			for k, v := range p {
				ps.csvAnnotations[k] = v
			}
		case propertyTypeCSVDescription:
			var p string
			if err := json.Unmarshal(prop.Value, &p); err != nil {
				return nil, propertyParseError{i: i, t: prop.Type, err: err}
			}
			if ps.csvDescription != "" {
				return nil, propertyMultipleNotAllowedError{i: i, t: prop.Type}
			}
			ps.csvDescription = p
		case propertyTypeCSVDisplayName:
			var p string
			if err := json.Unmarshal(prop.Value, &p); err != nil {
				return nil, propertyParseError{i: i, t: prop.Type, err: err}
			}
			if ps.csvDisplayName != "" {
				return nil, propertyMultipleNotAllowedError{i: i, t: prop.Type}
			}
			ps.csvDisplayName = p
		case propertyTypeCSVInstallMode:
			var p v1alpha1.InstallMode
			if err := json.Unmarshal(prop.Value, &p); err != nil {
				return nil, propertyParseError{i: i, t: prop.Type, err: err}
			}
			ps.csvInstallModes = append(ps.csvInstallModes, p)
		case propertyTypeCSVKeyword:
			var p string
			if err := json.Unmarshal(prop.Value, p); err != nil {
				return nil, propertyParseError{i: i, t: prop.Type, err: err}
			}
			ps.csvKeywords = append(ps.csvKeywords, p)
		case propertyTypeCSVLink:
			var p v1alpha1.AppLink
			if err := json.Unmarshal(prop.Value, &p); err != nil {
				return nil, propertyParseError{i: i, t: prop.Type, err: err}
			}
			ps.csvLinks = append(ps.csvLinks, p)
		case propertyTypeCSVMaintainer:
			var p v1alpha1.Maintainer
			if err := json.Unmarshal(prop.Value, &p); err != nil {
				return nil, propertyParseError{i: i, t: prop.Type, err: err}
			}
			ps.csvMaintainers = append(ps.csvMaintainers, p)
		case propertyTypeCSVMaturity:
			var p string
			if err := json.Unmarshal(prop.Value, &p); err != nil {
				return nil, propertyParseError{i: i, t: prop.Type, err: err}
			}
			if ps.csvMaturity != "" {
				return nil, propertyMultipleNotAllowedError{i: i, t: prop.Type}
			}
			ps.csvMaturity = p
		case propertyTypeCSVMinKubeVersion:
			var p string
			if err := json.Unmarshal(prop.Value, &p); err != nil {
				return nil, propertyParseError{i: i, t: prop.Type, err: err}
			}
			if ps.csvMinKubeVersion != "" {
				return nil, propertyMultipleNotAllowedError{i: i, t: prop.Type}
			}
			ps.csvMinKubeVersion = p
		case propertyTypeCSVProvider:
			var p v1alpha1.AppLink
			if err := json.Unmarshal(prop.Value, &p); err != nil {
				return nil, propertyParseError{i: i, t: prop.Type, err: err}
			}
			if ps.csvProvider != nil {
				return nil, propertyMultipleNotAllowedError{i: i, t: prop.Type}
			}
			ps.csvProvider = &p
		case propertyTypeObject:
			var p unstructured.Unstructured
			if err := json.Unmarshal(prop.Value, &p); err != nil {
				return nil, propertyParseError{i: i, t: prop.Type, err: err}
			}
			ps.objects = append(ps.objects, p)
		default:
			ps.others = append(ps.others, prop)
		}
	}

	return &ps, nil
}
