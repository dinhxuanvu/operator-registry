package api

import (
	"encoding/json"
	"fmt"

	"github.com/operator-framework/operator-registry/pkg/model"
	"github.com/operator-framework/operator-registry/pkg/property"
)

func ConvertAPIBundleToModelBundle(b *Bundle) (*model.Bundle, error) {
	bundleProps, err := convertAPIBundleToModelProperties(b)
	if err != nil {
		return nil, fmt.Errorf("convert properties: %v", err)
	}

	relatedImages, err := getRelatedImages(b.CsvJson)
	if err != nil {
		return nil, fmt.Errorf("get related iamges: %v", err)
	}

	return &model.Bundle{
		Name:          b.CsvName,
		Image:         b.BundlePath,
		Replaces:      b.Replaces,
		Skips:         b.Skips,
		CsvJSON:       b.CsvJson,
		Objects:       b.Object,
		Properties:    bundleProps,
		RelatedImages: relatedImages,
	}, nil
}

func convertAPIBundleToModelProperties(b *Bundle) ([]property.Property, error) {
	var out []property.Property

	for _, skip := range b.Skips {
		out = append(out, property.MustBuildSkips(skip))
	}

	if b.SkipRange != "" {
		out = append(out, property.MustBuildSkipRange(b.SkipRange))
	}

	out = append(out, property.MustBuildChannel(b.ChannelName, b.Replaces))

	providedGVKs := map[property.GVKProvided]*property.GVKProvided{}
	requiredGVKs := map[property.GVKRequired]*property.GVKRequired{}

	foundPackageProperty := false
	for i, p := range b.Properties {
		switch p.Type {
		case property.TypeGVK:
			var v GroupVersionKind
			if err := json.Unmarshal(json.RawMessage(p.Value), &v); err != nil {
				return nil, property.ParseError{Idx: i, Typ: p.Type, Err: err}
			}
			k := property.GVKProvided{Group: v.Group, Kind: v.Kind, Version: v.Version}
			providedGVKs[k] = &property.GVKProvided{Group: v.Group, Kind: v.Kind, Version: v.Version, Plural: v.Plural}
		case property.TypePackage:
			foundPackageProperty = true
			out = append(out, property.Property{
				Type:  property.TypePackage,
				Value: json.RawMessage(p.Value),
			})
		default:
			out = append(out, property.Property{
				Type:  p.Type,
				Value: json.RawMessage(p.Value),
			})
		}
	}

	for i, p := range b.Dependencies {
		switch p.Type {
		case property.TypeGVK:
			var v GroupVersionKind
			if err := json.Unmarshal(json.RawMessage(p.Value), &v); err != nil {
				return nil, property.ParseError{Idx: i, Typ: p.Type, Err: err}
			}
			k := property.GVKRequired{Group: v.Group, Kind: v.Kind, Version: v.Version}
			requiredGVKs[k] = &property.GVKRequired{Group: v.Group, Kind: v.Kind, Version: v.Version, Plural: v.Plural}
		case property.TypePackage:
			out = append(out, property.Property{
				Type:  property.TypePackageRequired,
				Value: json.RawMessage(p.Value),
			})
		}
	}

	if !foundPackageProperty {
		out = append(out, property.MustBuildPackage(b.PackageName, b.Version))
	}

	for _, p := range b.ProvidedApis {
		k := property.GVKProvided{Group: p.Group, Kind: p.Kind, Version: p.Version}
		if v, ok := providedGVKs[k]; !ok {
			providedGVKs[k] = &property.GVKProvided{Group: p.Group, Kind: p.Kind, Version: p.Version, Plural: p.Plural}
		} else {
			v.Plural = p.Plural
		}
	}
	for _, p := range b.RequiredApis {
		k := property.GVKRequired{Group: p.Group, Kind: p.Kind, Version: p.Version}
		if v, ok := requiredGVKs[k]; !ok {
			requiredGVKs[k] = &property.GVKRequired{Group: p.Group, Kind: p.Kind, Version: p.Version, Plural: p.Plural}
		} else {
			v.Plural = p.Plural
		}
	}

	for _, p := range providedGVKs {
		out = append(out, property.MustBuildGVKProvided(p.Group, p.Version, p.Kind, p.Plural))
		out = append(out, property.MustBuildGVK(p.Group, p.Version, p.Kind, ""))
	}

	for _, p := range requiredGVKs {
		out = append(out, property.MustBuildGVKRequired(p.Group, p.Version, p.Kind, p.Plural))
	}

	return out, nil
}

func getRelatedImages(csvJSON string) ([]model.RelatedImage, error) {
	if len(csvJSON) == 0 {
		return nil, nil
	}
	type csv struct {
		Spec struct {
			RelatedImages []struct {
				Name  string `json:"name"`
				Image string `json:"image"`
			} `json:"relatedImages"`
		} `json:"spec"`
	}
	c := csv{}
	if err := json.Unmarshal([]byte(csvJSON), &c); err != nil {
		return nil, fmt.Errorf("unmarshal csv: %v", err)
	}
	relatedImages := []model.RelatedImage{}
	for _, ri := range c.Spec.RelatedImages {
		relatedImages = append(relatedImages, model.RelatedImage(ri))
	}
	return relatedImages, nil
}
