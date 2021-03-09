package declcfg

import (
	"encoding/base64"
	"encoding/json"
	"fmt"

	"github.com/blang/semver/v4"
	"github.com/operator-framework/api/pkg/lib/version"
	"github.com/operator-framework/api/pkg/operators/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

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

	if len(b.Skips) > 0 {
		out = append(out, property{
			Type:  propertyTypeSkips,
			Value: mustJSONMarshal(b.Skips),
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
		if len(b.CSV.Spec.InstallModes) > 0 {
			out = append(out, property{
				Type:  propertyTypeCSVInstallModes,
				Value: mustJSONMarshal(b.CSV.Spec.InstallModes),
			})
		}
		if len(b.CSV.Spec.Keywords) > 0 {
			out = append(out, property{
				Type:  propertyTypeCSVKeywords,
				Value: mustJSONMarshal(b.CSV.Spec.Keywords),
			})
		}
		if len(b.CSV.Spec.Links) > 0 {
			out = append(out, property{
				Type:  propertyTypeCSVLinks,
				Value: mustJSONMarshal(b.CSV.Spec.Links),
			})
		}
		if len(b.CSV.Spec.Maintainers) > 0 {
			out = append(out, property{
				Type:  propertyTypeCSVMaintainers,
				Value: mustJSONMarshal(b.CSV.Spec.Maintainers),
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

	// Required to populate model's provided APIs for backwards-compatibility
	// with GRPC API to answer queries for bundles/channels that provided
	// requested GVKs.
	propertyTypeGVKProvided = "olm.gvk.provided"

	// Required to populate model's CSV for backwards-compatibility
	propertyTypeCSVAnnotations    = "olm.csv.annotations"
	propertyTypeCSVDescription    = "olm.csv.description"
	propertyTypeCSVDisplayName    = "olm.csv.displayName"
	propertyTypeCSVInstallModes   = "olm.csv.installModes"
	propertyTypeCSVKeywords       = "olm.csv.keywords"
	propertyTypeCSVLinks          = "olm.csv.links"
	propertyTypeCSVMaintainers    = "olm.csv.maintainers"
	propertyTypeCSVMaturity       = "olm.csv.maturity"
	propertyTypeCSVMinKubeVersion = "olm.csv.minKubeVersion"
	propertyTypeCSVProvider       = "olm.csv.provider"
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
			var p []string
			if err := json.Unmarshal(prop.Value, &p); err != nil {
				return nil, propertyParseError{i: i, t: prop.Type, err: err}
			}
			ps.skips = append(ps.skips, p...)
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
		case propertyTypeCSVInstallModes:
			var p []v1alpha1.InstallMode
			if err := json.Unmarshal(prop.Value, &p); err != nil {
				return nil, propertyParseError{i: i, t: prop.Type, err: err}
			}
			ps.csvInstallModes = append(ps.csvInstallModes, p...)
		case propertyTypeCSVKeywords:
			var p []string
			if err := json.Unmarshal(prop.Value, &p); err != nil {
				return nil, propertyParseError{i: i, t: prop.Type, err: err}
			}
			ps.csvKeywords = append(ps.csvKeywords, p...)
		case propertyTypeCSVLinks:
			var p []v1alpha1.AppLink
			if err := json.Unmarshal(prop.Value, &p); err != nil {
				return nil, propertyParseError{i: i, t: prop.Type, err: err}
			}
			ps.csvLinks = append(ps.csvLinks, p...)
		case propertyTypeCSVMaintainers:
			var p []v1alpha1.Maintainer
			if err := json.Unmarshal(prop.Value, &p); err != nil {
				return nil, propertyParseError{i: i, t: prop.Type, err: err}
			}
			ps.csvMaintainers = append(ps.csvMaintainers, p...)
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
		default:
			ps.others = append(ps.others, prop)
		}
	}

	return &ps, nil
}

// {
//  "annotations": {
//    "alm-examples": "[{ \"apiVersion\": \"charts.helm.k8s.io/v1alpha1\", \"kind\": \"Cockroachdb\", \"metadata\": { \"name\": \"example\" }, \"spec\": { \"Name\": \"cdb\", \"Image\": \"cockroachdb/cockroach\", \"ImageTag\": \"v19.1.3\", \"ImagePullPolicy\": \"Always\", \"Replicas\": 3, \"MaxUnavailable\": 1, \"Component\": \"cockroachdb\", \"InternalGrpcPort\": 26257, \"ExternalGrpcPort\": 26257, \"InternalGrpcName\": \"grpc\", \"ExternalGrpcName\": \"grpc\", \"InternalHttpPort\": 8080, \"ExternalHttpPort\": 8080, \"HttpName\": \"http\", \"Resources\": { \"requests\": { \"cpu\": \"500m\", \"memory\": \"512Mi\" } }, \"InitPodResources\": { }, \"Storage\": \"10Gi\", \"StorageClass\": null, \"CacheSize\": \"25%\", \"MaxSQLMemory\": \"25%\", \"ClusterDomain\": \"cluster.local\", \"NetworkPolicy\": { \"Enabled\": false, \"AllowExternal\": true }, \"Service\": { \"type\": \"ClusterIP\", \"annotations\": { } }, \"PodManagementPolicy\": \"Parallel\", \"UpdateStrategy\": { \"type\": \"RollingUpdate\" }, \"NodeSelector\": { }, \"Tolerations\": { }, \"Secure\": { \"Enabled\": false, \"RequestCertsImage\": \"cockroachdb/cockroach-k8s-request-cert\", \"RequestCertsImageTag\": \"0.4\", \"ServiceAccount\": { \"Create\": true } } } }]",
//    "capabilities": "Basic Install",
//    "categories": "Database",
//    "certified": "false",
//    "containerImage": "quay.io/helmoperators/cockroachdb:2.1.1",
//    "createdAt": "2019-01-24T15-33-43Z",
//    "description": "CockroachDB Operator based on the CockroachDB helm chart",
//    "repository": "https://github.com/dmesser/cockroachdb-operator",
//    "support": "a-robinson"
//  },
//  "apiservicedefinitions": {},
//  "customresourcedefinitions": {
//    "owned": [
//      {
//        "description": "Represents a CockroachDB cluster",
//        "displayName": "CockroachDB",
//        "kind": "Cockroachdb",
//        "name": "cockroachdbs.charts.helm.k8s.io",
//        "version": "v1alpha1"
//      }
//    ]
//  },
//  "description": "CockroachDB is a scalable, survivable, strongly-consistent SQL database.\n\n## About this Operator\n\nThis Operator is based on a Helm chart for CockroachDB. It supports reconfiguration for some parameters, but notably does not handle scale down of the replica count in a seamless manner. Scale up works great.\n\n## Core capabilities\n* **StatefulSet** - Sets up a dynamically scalable CockroachDB cluster using a Kubernetes StatefulSet\n* **Expand Replicas** - Supports expanding the set of replicas by simply editing your object\n* **Dashboard** - Installs the CockroachDB user interface to administer your cluster. Easily expose it via an Ingress rule.\n\nReview all of the [confiuguration options](https://github.com/helm/charts/tree/master/stable/cockroachdb#configuration) to best run your database instance. The example configuration is derived from the chart's [`values.yaml`](https://github.com/helm/charts/blob/master/stable/cockroachdb/values.yaml).\n\n## Using the cluster\n\nThe resulting cluster endpoint can be consumed from a `Service` that follows the pattern: `<StatefulSet-name>-public`. For example to connect using the command line client, use something like the following to obtain the name of the service:\n\n```\nkubectl get service -l chart=cockroachdb-2.0.11\nNAME                                           TYPE        CLUSTER-IP       EXTERNAL-IP   PORT(S)              AGE\nexample-9f8ngwzrxbxrulxqmdqfhn51h-cdb          ClusterIP   None             <none>        26257/TCP,8080/TCP   24m\nexample-9f8ngwzrxbxrulxqmdqfhn51h-cdb-public   ClusterIP   10.106.249.134   <none>        26257/TCP,8080/TCP   24m\n```\n\nThen you can use the CockroachDB command line client to connect to the database cluster:\n\n```\nkubectl run -it --rm cockroach-client --image=cockroachdb/cockroach --restart=Never --command -- ./cockroach sql --insecure --host example-9f8ngwzrxbxrulxqmdqfhn51h-cdb-public\n```\n\n## Before you start\n\nThis Operator requires a cluster with PV support in order to run correctly.\n",
//  "displayName": "CockroachDB",
//  "installModes": [
//    {
//      "supported": true,
//      "type": "OwnNamespace"
//    },
//    {
//      "supported": true,
//      "type": "SingleNamespace"
//    },
//    {
//      "supported": false,
//      "type": "MultiNamespace"
//    },
//    {
//      "supported": true,
//      "type": "AllNamespaces"
//    }
//  ],
//  "keywords": [
//    "cockroach",
//    "cockroachdb",
//    "postgres"
//  ],
//  "links": [
//    {
//      "name": "Helm Chart Source",
//      "url": "https://github.com/helm/charts/tree/master/stable/cockroachdb"
//    },
//    {
//      "name": "Configuration Options",
//      "url": "https://github.com/helm/charts/tree/master/stable/cockroachdb#configuration"
//    },
//    {
//      "name": "CockroachDB Source",
//      "url": "https://github.com/cockroachdb/cockroach"
//    }
//  ],
//  "maintainers": [
//    {
//      "email": "alex@cockroachlabs.com",
//      "name": "a-robinson"
//    },
//    {
//      "email": "dmesser@redhat.com",
//      "name": "Daniel Messer"
//    }
//  ],
//  "maturity": "stable",
//  "minKubeVersion": "1.8.0",
//  "provider": {
//    "name": "Helm Community"
//  },
//  "relatedImages": [
//    "quay.io/helmoperators/cockroachdb:v2.1.11"
//  ],
//  "version": "2.1.11"
//}
