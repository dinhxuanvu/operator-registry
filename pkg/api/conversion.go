package api

import (
	"fmt"

	"github.com/operator-framework/operator-registry/pkg/model"
)

const (
	dependencyTypeGVK     = "olm.gvk"
	dependencyTypePackage = "olm.package"
)

func BundleFromModel(b model.Bundle) *Bundle {
	return &Bundle{
		CsvName:      b.Name,
		PackageName:  b.Package.Name,
		ChannelName:  b.Channel.Name,
		BundlePath:   b.Image,
		ProvidedApis: gvksFromModel(b.ProvidedAPIs),
		RequiredApis: gvksFromModel(b.RequiredAPIs),
		Version:      b.Version,
		SkipRange:    b.SkipRange,
		Dependencies: dependenciesFromModel(b),
		Properties:   propertiesFromModel(b),
		Replaces:     b.Replaces,
		Skips:        b.Skips,
	}
}

func gvksFromModel(gvks []model.GroupVersionKind) []*GroupVersionKind {
	var out []*GroupVersionKind
	for _, gvk := range gvks {
		out = append(out, gvkFromModel(gvk))
	}
	return out
}

func gvkFromModel(gvk model.GroupVersionKind) *GroupVersionKind {
	return &GroupVersionKind{
		Group:   gvk.Group,
		Version: gvk.Version,
		Kind:    gvk.Kind,
		Plural:  gvk.Plural,
	}
}

func dependencyFromModelPkgReq(pkgReq model.PackageRequirement) *Dependency {
	return &Dependency{
		Type:  dependencyTypePackage,
		Value: fmt.Sprintf(`{"packageName":%q,"version":%q}`, pkgReq.PackageName, pkgReq.Version),
	}
}

func dependencyFromModelGVK(gvk model.GroupVersionKind) *Dependency {
	return &Dependency{
		Type:  dependencyTypeGVK,
		Value: fmt.Sprintf(`{"group":%q,"version":%q,"kind":%q}`, gvk.Group, gvk.Version, gvk.Kind),
	}
}

func dependenciesFromModel(b model.Bundle) []*Dependency {
	var deps []*Dependency
	for _, pkgReq := range b.RequiredPackages {
		deps = append(deps, dependencyFromModelPkgReq(pkgReq))
	}
	for _, papi := range b.RequiredAPIs {
		deps = append(deps, dependencyFromModelGVK(papi))
	}
	return deps
}

func propertiesFromModel(b model.Bundle) []*Property {
	var props []*Property
	for _, prop := range b.Properties {
		props = append(props, &Property{
			Type:  prop.Type,
			Value: prop.Value,
		})
	}
	return props
}
