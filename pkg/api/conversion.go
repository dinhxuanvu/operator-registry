package api

import (
	"encoding/json"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/operator-framework/operator-registry/pkg/model"
)

const (
	apiPropertyTypePackage = "olm.package"
	apiPropertyTypeGVK     = "olm.gvk"

	apiDependencyTypePackage = "olm.package"
	apiDependencyTypeGVK     = "olm.gvk"

	propertyTypePackageProvided = "olm.package.provided"
	propertyTypePackageRequired = "olm.package.required"
	propertyTypeGVKProvided     = "olm.gvk.provided"
	propertyTypeGVKRequired     = "olm.gvk.required"
)

func ConvertModelBundleToAPIBundle(b model.Bundle) (*Bundle, error) {
	csvJson := ""
	if b.CSV != nil {
		d, err := json.Marshal(b.CSV)
		if err != nil {
			return nil, err
		}
		csvJson = string(d)
	}

	return &Bundle{
		CsvName:      b.Name,
		PackageName:  b.Package.Name,
		ChannelName:  b.Channel.Name,
		CsvJson:      csvJson,
		Object:       unstructuredToStrings(b.Objects),
		BundlePath:   b.Image,
		ProvidedApis: convertModelGVKsToAPIGVKs(b.ProvidedAPIs),
		RequiredApis: convertModelGVKsToAPIGVKs(b.RequiredAPIs),
		Version:      b.Version,
		SkipRange:    b.SkipRange,
		Dependencies: convertModelPropertiesToAPIDependencies(b.Properties),
		Properties:   convertModelPropertiesToAPIProperties(b.Properties),
		Replaces:     b.Replaces,
		Skips:        b.Skips,
	}, nil
}

func unstructuredToStrings(in []unstructured.Unstructured) []string {
	var out []string
	for _, obj := range in {
		d, err := json.Marshal(obj)
		if err != nil {
			panic(err)
		}
		out = append(out, string(d))
	}
	return out
}

func convertModelGVKsToAPIGVKs(gvks []model.GroupVersionKind) []*GroupVersionKind {
	var out []*GroupVersionKind
	for _, gvk := range gvks {
		out = append(out, &GroupVersionKind{
			Group:   gvk.Group,
			Version: gvk.Version,
			Kind:    gvk.Kind,
			Plural:  gvk.Plural,
		})
	}
	return out
}

func convertModelPropertiesToAPIProperties(props []model.Property) []*Property {
	var out []*Property
	for _, prop := range props {
		switch prop.Type {
		case propertyTypeGVKProvided:
			// For backwards-compatibility, rename property type to
			// "olm.gvk" and remove the "plural" field from the value.
			out = append(out, &Property{
				Type:  apiPropertyTypeGVK,
				Value: string(removePluralFromGVKProperty(prop.Value)),
			})
		case propertyTypePackageProvided:
			// For backwards-compatibility, rename property type to
			// "olm.package".
			out = append(out, &Property{
				Type:  apiPropertyTypePackage,
				Value: string(prop.Value),
			})
		default:
			out = append(out, &Property{
				Type:  prop.Type,
				Value: string(prop.Value),
			})
		}

	}
	return out
}

func convertModelPropertiesToAPIDependencies(props []model.Property) []*Dependency {
	var out []*Dependency
	for _, prop := range props {
		switch prop.Type {
		case propertyTypeGVKRequired:
			out = append(out, &Dependency{
				Type:  apiDependencyTypeGVK,
				Value: string(removePluralFromGVKProperty(prop.Value)),
			})
		case propertyTypePackageRequired:
			out = append(out, &Dependency{
				Type:  apiDependencyTypePackage,
				Value: string(prop.Value),
			})
		}
	}
	return out
}

func removePluralFromGVKProperty(in json.RawMessage) json.RawMessage {
	var gvk GroupVersionKind
	if err := json.Unmarshal(in, &gvk); err != nil {
		return in
	}
	gvk.Plural = ""
	out, err := json.Marshal(gvk)
	if err != nil {
		return in
	}
	return out
}
