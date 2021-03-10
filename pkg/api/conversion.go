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

func ConvertModelBundleToAPIBundle(b model.Bundle) *Bundle {
	return &Bundle{
		CsvName:      b.Name,
		PackageName:  b.Package.Name,
		ChannelName:  b.Channel.Name,
		BundlePath:   b.Image,
		ProvidedApis: convertModelGVKsToAPIGVKs(b.ProvidedAPIs),
		RequiredApis: convertModelGVKsToAPIGVKs(b.RequiredAPIs),
		Version:      b.Version,
		SkipRange:    b.SkipRange,
		Dependencies: convertModelPropertiesToAPIDependencies(b.Properties),
		Properties:   convertModelPropertiesToAPIProperties(b.Properties),
		Replaces:     b.Replaces,
		Skips:        b.Skips,
	}
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
		// Remove the "plural" field from GVK properties.
		value := prop.Value
		if prop.Type == propertyTypeGVKProvided || prop.Type == propertyTypeGVKRequired {
			value = removePluralFromGVKProperty(value)
		}
		out = append(out, &Property{
			Type:  prop.Type,
			Value: string(value),
		})

		switch prop.Type {
		case propertyTypeGVKProvided:
			// For backwards-compatibility, add duplicate property with
			// type set to "olm.gvk"
			out = append(out, &Property{
				Type:  apiPropertyTypeGVK,
				Value: string(value),
			})
		case propertyTypePackageProvided:
			// For backwards-compatibility, add duplicate property with
			// type set to "olm.package"
			out = append(out, &Property{
				Type:  apiPropertyTypePackage,
				Value: string(value),
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
