package model

import (
	"fmt"

	"github.com/operator-framework/operator-registry/pkg/api"
)

const (
	dependencyTypeGVK     = "olm.gvk"
	dependencyTypePackage = "olm.package"
)

type Model map[string]*Package

type Package struct {
	Name           string
	Description    string
	Icon           *Icon
	DefaultChannel *Channel
	Channels       map[string]*Channel
}

type Icon struct {
	Data      []byte
	MediaType string
}

type Channel struct {
	Package *Package
	Name    string
	Head    *Bundle
	Bundles map[string]*Bundle
}

type GroupVersionKind struct {
	Group   string
	Version string
	Kind    string
	Plural  string
}

func (in GroupVersionKind) AsAPIDependency() api.Dependency {
	return api.Dependency{
		Type:  dependencyTypeGVK,
		Value: fmt.Sprintf(`{"group":%q,"version":%q,"kind":%q}`, in.Group, in.Version, in.Kind),
	}
}

func (in GroupVersionKind) AsAPIGVK() api.GroupVersionKind {
	return api.GroupVersionKind{
		Group:   in.Group,
		Version: in.Version,
		Kind:    in.Kind,
		Plural:  in.Plural,
	}
}

func apiGVKs(gvks []GroupVersionKind) []*api.GroupVersionKind {
	var out []*api.GroupVersionKind
	for _, gvk := range gvks {
		apiGVK := gvk.AsAPIGVK()
		out = append(out, &apiGVK)
	}
	return out
}

type Property struct {
	Type  string
	Value string
}

type PackageRequirement struct {
	PackageName string
	Version     string
}

func (in PackageRequirement) AsAPIDependency() api.Dependency {
	return api.Dependency{
		Type:  dependencyTypePackage,
		Value: fmt.Sprintf(`{"packageName":%q,"version":%q}`, in.PackageName, in.Version),
	}
}

type Bundle struct {
	Package          *Package
	Channel          *Channel
	Name             string
	Version          string
	Image            string
	Replaces         string
	Skips            []string
	SkipRange        string
	ProvidedAPIs     []GroupVersionKind
	RequiredAPIs     []GroupVersionKind
	Properties       []Property
	RequiredPackages []PackageRequirement
}

func (b Bundle) apiDependencies() []*api.Dependency {
	var deps []*api.Dependency
	for _, pkgReq := range b.RequiredPackages {
		dep := pkgReq.AsAPIDependency()
		deps = append(deps, &dep)
	}
	for _, papi := range b.RequiredAPIs {
		dep := papi.AsAPIDependency()
		deps = append(deps, &dep)
	}
	return deps
}

func (b Bundle) apiProperties() []*api.Property {
	var props []*api.Property
	for _, prop := range b.Properties {
		props = append(props, &api.Property{
			Type:  prop.Type,
			Value: prop.Value,
		})
	}
	return props
}

func (b Bundle) Provides(group, version, kind string) bool {
	for _, gvk := range b.ProvidedAPIs {
		if group == gvk.Group && version == gvk.Version && kind == gvk.Kind {
			return true
		}
	}
	return false
}

func (b Bundle) ConvertToAPI() *api.Bundle {
	return &api.Bundle{
		CsvName:      b.Name,
		PackageName:  b.Package.Name,
		ChannelName:  b.Channel.Name,
		BundlePath:   b.Image,
		ProvidedApis: apiGVKs(b.ProvidedAPIs),
		RequiredApis: apiGVKs(b.RequiredAPIs),
		Version:      b.Version,
		SkipRange:    b.SkipRange,
		Dependencies: b.apiDependencies(),
		Properties:   b.apiProperties(),
		Replaces:     b.Replaces,
		Skips:        b.Skips,
	}
}
