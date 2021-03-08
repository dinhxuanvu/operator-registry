package api

import (
	"encoding/json"

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

func convertModelGVKsToAPIGVKs(gvks []model.GroupVersionKind) []*GroupVersionKind {
	var out []*GroupVersionKind
	for _, gvk := range gvks {
		out = append(out, &GroupVersionKind{
			Group:   gvk.Group,
			Version: gvk.Version,
			Kind:    gvk.Kind,
		})
	}
	return out
}

func convertModelPropertiesToAPIProperties(props []model.Property) []*Property {
	var out []*Property
	for _, prop := range props {
		switch prop.Type {
		case propertyTypeGVKProvided:
			out = append(out, &Property{
				Type:  apiPropertyTypeGVK,
				Value: string(prop.Value),
			})
		case propertyTypePackageProvided:
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
				Value: string(prop.Value),
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
