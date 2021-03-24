package api

import (
	"encoding/json"
	"fmt"

	"github.com/operator-framework/operator-registry/pkg/model"
	"github.com/operator-framework/operator-registry/pkg/property"
)

func ConvertModelBundleToAPIBundle(b model.Bundle) (*Bundle, error) {
	props, err := parseProperties(b.Properties)
	if err != nil {
		return nil, fmt.Errorf("parse properties: %v", err)
	}
	skipRange := ""
	if len(props.SkipRanges) > 0 {
		skipRange = string(props.SkipRanges[0])
	}
	return &Bundle{
		CsvName:      b.Name,
		PackageName:  b.Package.Name,
		ChannelName:  b.Channel.Name,
		BundlePath:   b.Image,
		ProvidedApis: gvksProvidedtoAPIGVKs(props.GVKsProvided),
		RequiredApis: gvksRequirestoAPIGVKs(props.GVKsRequired),
		Version:      props.Packages[0].Version,
		SkipRange:    skipRange,
		Dependencies: convertModelPropertiesToAPIDependencies(b.Properties),
		Properties:   convertModelPropertiesToAPIProperties(b.Properties),
		Replaces:     b.Replaces,
		Skips:        b.Skips,
		CsvJson:      b.CsvJSON,
		Object:       b.Objects,
	}, nil
}

func parseProperties(in []property.Property) (*property.Properties, error) {
	props, err := property.Parse(in)
	if err != nil {
		return nil, err
	}

	if len(props.Packages) != 1 {
		return nil, fmt.Errorf("expected exactly 1 property of type %q, found %d", property.TypePackage, len(props.Packages))
	}

	if len(props.SkipRanges) > 1 {
		return nil, fmt.Errorf("multiple properties of type %q not allowed", property.TypeSkipRange)
	}

	return props, nil
}

func gvksProvidedtoAPIGVKs(in []property.GVKProvided) []*GroupVersionKind {
	var out []*GroupVersionKind
	for _, gvk := range in {
		out = append(out, &GroupVersionKind{
			Group:   gvk.Group,
			Version: gvk.Version,
			Kind:    gvk.Kind,
			Plural:  gvk.Plural,
		})
	}
	return out
}
func gvksRequirestoAPIGVKs(in []property.GVKRequired) []*GroupVersionKind {
	var out []*GroupVersionKind
	for _, gvk := range in {
		out = append(out, &GroupVersionKind{
			Group:   gvk.Group,
			Version: gvk.Version,
			Kind:    gvk.Kind,
			Plural:  gvk.Plural,
		})
	}
	return out
}

func convertModelPropertiesToAPIProperties(props []property.Property) []*Property {
	var out []*Property
	for _, prop := range props {
		// Remove the "plural" field from GVK properties.
		value := prop.Value
		if prop.Type == property.TypeGVKProvided || prop.Type == property.TypeGVKRequired {
			value = marshalAsGVKProperty(value)
		}

		// Copy property to API Properties list
		out = append(out, &Property{
			Type:  prop.Type,
			Value: string(value),
		})
	}
	return out
}

func convertModelPropertiesToAPIDependencies(props []property.Property) []*Dependency {
	var out []*Dependency
	for _, prop := range props {
		switch prop.Type {
		case property.TypeGVKRequired:
			out = append(out, &Dependency{
				Type:  property.TypeGVK,
				Value: string(marshalAsGVKProperty(prop.Value)),
			})
		case property.TypePackageRequired:
			out = append(out, &Dependency{
				Type:  property.TypePackage,
				Value: string(prop.Value),
			})
		}
	}
	return out
}

func marshalAsGVKProperty(in json.RawMessage) json.RawMessage {
	var v GroupVersionKind
	if err := json.Unmarshal(in, &v); err != nil {
		return in
	}
	p := property.MustBuildGVK(v.Group, v.Version, v.Kind, "")
	return p.Value
}
