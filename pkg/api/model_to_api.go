package api

import (
	"encoding/json"
	"fmt"

	"github.com/operator-framework/operator-registry/pkg/model"
)

func ConvertModelBundleToAPIBundle(b model.Bundle) (*Bundle, error) {
	props, err := parseProperties(b.Properties)
	if err != nil {
		return nil, fmt.Errorf("parse properties: %v", err)
	}
	return &Bundle{
		CsvName:      b.Name,
		PackageName:  b.Package.Name,
		ChannelName:  b.Channel.Name,
		BundlePath:   b.Image,
		ProvidedApis: props.providedGVKs,
		RequiredApis: props.requiredGVKs,
		Version:      props.providedPackage.Version,
		SkipRange:    props.skipRange,
		Dependencies: convertModelPropertiesToAPIDependencies(b.Properties),
		Properties:   convertModelPropertiesToAPIProperties(b.Properties),
		Replaces:     b.Replaces,
		Skips:        b.Skips,
	}, nil
}

func convertModelPropertiesToAPIProperties(props []model.Property) []*Property {
	var out []*Property
	for _, prop := range props {
		// TODO(joelanford): It's a little hard to tell where the plural field is expected
		//   and where it isn't. What clients expect the `plural` field in GVKs and in which
		//   fields of the API?
		// Remove the "plural" field from GVK properties.
		value := prop.Value
		if prop.Type == propertyTypeProvidedGVK || prop.Type == propertyTypeRequiredGVK {
			value = removePluralFromGVKProperty(value)
		}

		// Copy property to API Properties list
		out = append(out, &Property{
			Type:  prop.Type,
			Value: string(value),
		})
	}
	return out
}

func convertModelPropertiesToAPIDependencies(props []model.Property) []*Dependency {
	var out []*Dependency
	for _, prop := range props {
		switch prop.Type {
		case propertyTypeRequiredGVK:
			out = append(out, &Dependency{
				Type:  apiTypeGVK,
				Value: string(removePluralFromGVKProperty(prop.Value)),
			})
		case propertyTypeRequiredPackage:
			out = append(out, &Dependency{
				Type:  apiTypePackage,
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
