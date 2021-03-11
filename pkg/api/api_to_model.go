package api

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/operator-framework/operator-registry/pkg/model"
)

func ConvertAPIBundleToModelBundle(b *Bundle) (*model.Bundle, error) {
	bundleProps, err := convertAPIBundleToModelProperties(b)
	if err != nil {
		return nil, fmt.Errorf("convert properties: %v", err)
	}
	return &model.Bundle{
		Name:       b.CsvName,
		Image:      b.BundlePath,
		Replaces:   b.Replaces,
		Skips:      b.Skips,
		Properties: bundleProps,
	}, nil
}

func convertAPIBundleToModelProperties(b *Bundle) ([]model.Property, error) {
	var out []model.Property

	for _, skip := range b.Skips {
		skipJson, err := json.Marshal(skip)
		if err != nil {
			return nil, fmt.Errorf("marshal %q property %q: %v", propertyTypeSkips, skip, err)
		}
		out = append(out, model.Property{
			Type:  propertyTypeSkips,
			Value: skipJson,
		})
	}

	if b.SkipRange != "" {
		// Use a JSON encoder so we can disable HTML escaping.
		buf := &bytes.Buffer{}
		enc := json.NewEncoder(buf)
		enc.SetEscapeHTML(false)
		err := enc.Encode(b.SkipRange)
		if err != nil {
			return nil, fmt.Errorf("marshal %q property %q: %v", propertyTypeSkipRange, b.SkipRange, err)
		}
		out = append(out, model.Property{
			Type:  propertyTypeSkipRange,
			Value: buf.Bytes(),
		})
	}

	ch := channel{Name: b.ChannelName, Replaces: b.Replaces}
	channelJson, err := json.Marshal(ch)
	if err != nil {
		return nil, fmt.Errorf("marshal %q property %+v: %v", propertyTypeChannel, ch, err)
	}
	out = append(out, model.Property{
		Type:  propertyTypeChannel,
		Value: channelJson,
	})

	foundGVKProperty, foundPackageProperty := false, false
	for _, p := range b.Properties {
		switch p.Type {
		case apiTypeGVK:
			foundGVKProperty = true
			out = append(out, model.Property{
				Type:  propertyTypeProvidedGVK,
				Value: json.RawMessage(p.Value),
			})
		case apiTypePackage:
			foundPackageProperty = true
			out = append(out, model.Property{
				Type:  propertyTypeProvidedPackage,
				Value: json.RawMessage(p.Value),
			})
		default:
			out = append(out, model.Property{
				Type:  p.Type,
				Value: json.RawMessage(p.Value),
			})
		}
	}

	foundGVKDependency := false
	for _, p := range b.Dependencies {
		switch p.Type {
		case apiTypeGVK:
			foundGVKDependency = true
			out = append(out, model.Property{
				Type:  propertyTypeRequiredGVK,
				Value: json.RawMessage(p.Value),
			})
		case apiTypePackage:
			out = append(out, model.Property{
				Type:  propertyTypeRequiredPackage,
				Value: json.RawMessage(p.Value),
			})
		}
	}

	if !foundGVKProperty {
		for _, p := range b.ProvidedApis {
			p.Plural = ""
			gvkJson, err := json.Marshal(p)
			if err != nil {
				return nil, fmt.Errorf("marshal %q property %+v: %v", propertyTypeProvidedGVK, p, err)
			}
			out = append(out, model.Property{
				Type:  propertyTypeProvidedGVK,
				Value: gvkJson,
			})
		}
	}
	if !foundPackageProperty {
		provPkg := providedPackage{
			PackageName: b.PackageName,
			Version:     b.Version,
		}
		provPkgJson, err := json.Marshal(provPkg)
		if err != nil {
			return nil, fmt.Errorf("marshal %q property %+v: %v", propertyTypeProvidedPackage, provPkg, err)
		}
		out = append(out, model.Property{
			Type:  propertyTypeProvidedPackage,
			Value: provPkgJson,
		})
	}
	if !foundGVKDependency {
		for _, p := range b.RequiredApis {
			p.Plural = ""
			gvkJson, err := json.Marshal(p)
			if err != nil {
				return nil, fmt.Errorf("marshal %q property %+v: %v", propertyTypeRequiredGVK, p, err)
			}
			out = append(out, model.Property{
				Type:  propertyTypeRequiredGVK,
				Value: gvkJson,
			})
		}
	}

	return out, nil
}
