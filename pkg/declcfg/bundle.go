package declcfg

import (
	"fmt"
	"strings"

	"github.com/operator-framework/operator-registry/pkg/model"
	"github.com/operator-framework/operator-registry/pkg/registry"
)

const (
	propertyNamePackage   = "olm.package"
	propertyNameGVK       = "olm.gvk"
	propertyNameChannel   = "olm.channel"
	propertyTypeRequired  = "required"
	propertyTypeProvided  = "provided"
	propertyNameSkips     = "skips"
	propertyNameSkipRange = "skipRange"
)

type Bundle struct {
	Schema        string           `json:"schema"`
	Name          string           `json:"name"`
	Package       string           `json:"package"`
	Image         string           `json:"image"`
	Version       string           `json:"version"`
	Properties    []BundleProperty `json:"properties"`
	RelatedImages []RelatedImage   `json:"relatedImages"`
}

type BundleProperty struct {
	Name     string            `json:"name"`
	Type     string            `json:"type,omitempty"`
	Values   map[string]string `json:"values,omitempty"`
	Value    string            `json:"value,omitempty"`
	Replaces string            `json:"replaces,omitempty"`
}

type RelatedImage struct {
	Name  string `json:"name"`
	Image string `json:"image"`
}

func (b Bundle) RequiredAPIs() []model.GroupVersionKind {
	return b.getAPIs(propertyTypeRequired)
}

func (b Bundle) ProvidedAPIs() []model.GroupVersionKind {
	return b.getAPIs(propertyTypeProvided)
}

func (b Bundle) getAPIs(propType string) []model.GroupVersionKind {
	var gvks []model.GroupVersionKind
	for _, p := range b.Properties {
		if p.Name == propertyNameGVK && p.Type == propType {
			gvks = append(gvks, model.GroupVersionKind{
				Group:   p.Values["group"],
				Version: p.Values["version"],
				Kind:    p.Values["kind"],
				Plural:  p.Values["plural"],
			})
		}
	}
	return gvks
}

func (b Bundle) ChannelEntries() []registry.ChannelEntry {
	var channels []registry.ChannelEntry
	for _, p := range b.Properties {
		if p.Name == propertyNameChannel {
			channels = append(channels, registry.ChannelEntry{
				PackageName: b.Package,
				ChannelName: p.Value,
				BundleName:  b.Name,
				Replaces:    p.Replaces,
			})
		}
	}
	return channels
}

func (b Bundle) Skips() []string {
	var skipList []string
	for _, p := range b.Properties {
		if p.Name == propertyNameSkips {
			for _, skip := range strings.Split(p.Value, ",") {
				skipList = append(skipList, strings.TrimSpace(skip))
			}
		}
	}
	return skipList
}

func (b Bundle) SkipRange() string {
	for _, p := range b.Properties {
		if p.Name == propertyNameSkipRange {
			return p.Value
		}
	}
	return ""
}

func (b Bundle) GetProperties() ([]model.Property, error) {
	var props []model.Property
	pkg, err := b.ProvidedPackage()
	if err != nil {
		return nil, err
	}
	props = append(props, model.Property{
		Type:  propertyNamePackage,
		Value: fmt.Sprintf(`{"packageName":%q,"version":%q}`, pkg.PackageName, pkg.Version),
	})
	for _, papi := range b.ProvidedAPIs() {
		props = append(props, model.Property{
			Type:  propertyNameGVK,
			Value: fmt.Sprintf(`{"group":%q,"version":%q,"kind":%q}`, papi.Group, papi.Version, papi.Kind),
		})
	}
	return props, nil
}

func (b Bundle) ProvidedPackage() (*model.PackageRequirement, error) {
	pkgs := b.getPackages(propertyTypeProvided)
	if len(pkgs) == 0 {
		return nil, fmt.Errorf("provided package property not found")
	}
	if len(pkgs) > 1 {
		return nil, fmt.Errorf("multiple provided package properties found, only one permitted")
	}
	return &pkgs[0], nil
}

func (b Bundle) RequiredPackages() []model.PackageRequirement {
	return b.getPackages(propertyTypeRequired)
}

// TODO(joelanford): The Declarative Config EP does not mention handling for dependencies.yaml
//   Currently going to go on the assumption that properties with name "olm.package" will include
//   a type field with either "provided" or "required", similar to the "olm.gvk" properties.
func (b Bundle) getPackages(propType string) []model.PackageRequirement {
	var pkgs []model.PackageRequirement
	for _, p := range b.Properties {
		if p.Name == propertyNamePackage && p.Type == propType {
			pkgs = append(pkgs, model.PackageRequirement{
				PackageName: p.Values["packageName"],
				Version:     p.Values["version"],
			})
		}
	}
	return pkgs
}
