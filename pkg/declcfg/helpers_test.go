package declcfg

import (
	"encoding/json"
	"fmt"

	"github.com/operator-framework/operator-registry/pkg/model"
	"github.com/operator-framework/operator-registry/pkg/property"
)

func buildValidDeclarativeConfig(includeUnrecognized bool) DeclarativeConfig {
	a001 := newTestBundle("anakin", "0.0.1",
		withChannel("light", ""),
		withChannel("dark", ""),
	)
	a010 := newTestBundle("anakin", "0.1.0",
		withChannel("light", testBundleName("anakin", "0.0.1")),
		withChannel("dark", testBundleName("anakin", "0.0.1")),
	)
	a011 := newTestBundle("anakin", "0.1.1",
		withChannel("dark", testBundleName("anakin", "0.0.1")),
		withSkips(testBundleName("anakin", "0.1.0")),
	)
	b1 := newTestBundle("boba-fett", "1.0.0",
		withChannel("mando", ""),
	)
	b2 := newTestBundle("boba-fett", "2.0.0",
		withChannel("mando", testBundleName("boba-fett", "1.0.0")),
	)

	var others []meta
	if includeUnrecognized {
		others = []meta{
			{schema: "custom.1", data: json.RawMessage(`{"schema": "custom.1"}`)},
			{schema: "custom.2", data: json.RawMessage(`{"schema": "custom.2"}`)},
			{schema: "custom.3", pkgName: "anakin", data: json.RawMessage(`{
				"schema": "custom.3",
				"package": "anakin",
				"myField": "foobar"
			}`)},
			{schema: "custom.3", pkgName: "boba-fett", data: json.RawMessage(`{
				"schema": "custom.3",
				"package": "boba-fett",
				"myField": "foobar"
			}`)},
		}
	}

	return DeclarativeConfig{
		Packages: []pkg{
			newTestPackage("anakin", "dark", svgSmallCircle),
			newTestPackage("boba-fett", "mando", svgBigCircle),
		},
		Bundles: []bundle{
			a001, a010, a011,
			b1, b2,
		},
		others: others,
	}
}

type bundleOpt func(*bundle)

func withChannel(name, replaces string) func(*bundle) {
	return func(b *bundle) {
		b.Properties = append(b.Properties, property.MustBuildChannel(name, replaces))
	}
}

func withSkips(name string) func(*bundle) {
	return func(b *bundle) {
		b.Properties = append(b.Properties, property.MustBuildSkips(name))
	}
}

func newTestBundle(packageName, version string, opts ...bundleOpt) bundle {
	b := bundle{
		Schema:  schemaBundle,
		Name:    testBundleName(packageName, version),
		Package: packageName,
		Image:   testBundleImage(packageName, version),
		Properties: []property.Property{
			property.MustBuildPackage(packageName, version),
			property.MustBuildPackageProvided(packageName, version),
		},
		RelatedImages: []relatedImage{
			{
				Name:  "bundle",
				Image: testBundleImage(packageName, version),
			},
		},
		CsvJSON: `{"kind": "ClusterServiceVersion", "apiVersion": "operators.coreos.com/v1alpha1"}`,
		Objects: []string{
			`{"kind": "ClusterServiceVersion", "apiVersion": "operators.coreos.com/v1alpha1"}`,
			`{"kind": "CustomResourceDefinition", "apiVersion": "apiextensions.k8s.io/v1"}`,
		},
	}
	for _, opt := range opts {
		opt(&b)
	}
	return b
}

const (
	svgSmallCircle = `<svg viewBox="0 0 100 100"><circle cx="25" cy="25" r="25"/></svg>`
	svgBigCircle   = `<svg viewBox="0 0 100 100"><circle cx="50" cy="50" r="50"/></svg>`
)

func newTestPackage(packageName, defaultChannel, svgData string) pkg {
	p := pkg{
		Schema:         schemaPackage,
		Name:           packageName,
		DefaultChannel: defaultChannel,
		Icon:           &icon{Data: []byte(svgData), MediaType: "image/svg+xml"},
		Description:    testPackageDescription(packageName),
	}
	return p
}

func buildTestModel() model.Model {
	return model.Model{
		"anakin":    buildAnakinPkgModel(),
		"boba-fett": buildBobaFettPkgModel(),
	}
}

func buildAnakinPkgModel() *model.Package {
	pkgName := "anakin"
	pkg := &model.Package{
		Name:        pkgName,
		Description: testPackageDescription(pkgName),
		Icon: &model.Icon{
			Data:      []byte(svgSmallCircle),
			MediaType: "image/svg+xml",
		},
		Channels: map[string]*model.Channel{},
	}

	for _, chName := range []string{"light", "dark"} {
		ch := &model.Channel{
			Package: pkg,
			Name:    chName,
			Bundles: map[string]*model.Bundle{},
		}
		pkg.Channels[ch.Name] = ch
	}
	pkg.DefaultChannel = pkg.Channels["dark"]

	versions := map[string][]property.Channel{
		"0.0.1": {{Name: "light"}, {Name: "dark"}},
		"0.1.0": {
			{Name: "light", Replaces: testBundleName(pkgName, "0.0.1")},
			{Name: "dark", Replaces: testBundleName(pkgName, "0.0.1")},
		},
		"0.1.1": {{Name: "dark", Replaces: testBundleName(pkgName, "0.0.1")}},
	}
	for version, channels := range versions {
		props := []property.Property{
			property.MustBuildPackage(pkgName, version),
			property.MustBuildPackageProvided(pkgName, version),
		}
		for _, channel := range channels {
			props = append(props, property.MustBuild(&channel))
			ch := pkg.Channels[channel.Name]
			bName := testBundleName(pkgName, version)
			bImage := testBundleImage(pkgName, version)
			skips := []string{}
			if version == "0.1.1" {
				skip := testBundleName(pkgName, "0.1.0")
				skips = append(skips, skip)
				props = append(props, property.MustBuildSkips(skip))
			}
			bundle := &model.Bundle{
				Package:    pkg,
				Channel:    ch,
				Name:       bName,
				Image:      bImage,
				Replaces:   channel.Replaces,
				Skips:      skips,
				Properties: props,
				RelatedImages: []model.RelatedImage{{
					Name:  "bundle",
					Image: testBundleImage(pkgName, version),
				}},
				CsvJSON: `{"kind": "ClusterServiceVersion", "apiVersion": "operators.coreos.com/v1alpha1"}`,
				Objects: []string{
					`{"kind": "ClusterServiceVersion", "apiVersion": "operators.coreos.com/v1alpha1"}`,
					`{"kind": "CustomResourceDefinition", "apiVersion": "apiextensions.k8s.io/v1"}`,
				},
			}
			ch.Bundles[bName] = bundle
		}
	}
	return pkg
}

func buildBobaFettPkgModel() *model.Package {
	pkgName := "boba-fett"
	pkg := &model.Package{
		Name:        pkgName,
		Description: testPackageDescription(pkgName),
		Icon: &model.Icon{
			Data:      []byte(svgBigCircle),
			MediaType: "image/svg+xml",
		},
		Channels: map[string]*model.Channel{},
	}
	ch := &model.Channel{
		Package: pkg,
		Name:    "mando",
		Bundles: map[string]*model.Bundle{},
	}
	pkg.Channels[ch.Name] = ch
	pkg.DefaultChannel = ch

	versions := map[string][]property.Channel{
		"1.0.0": {{Name: "mando"}},
		"2.0.0": {{Name: "mando", Replaces: testBundleName(pkgName, "1.0.0")}},
	}
	for version, channels := range versions {
		props := []property.Property{
			property.MustBuildPackage(pkgName, version),
			property.MustBuildPackageProvided(pkgName, version),
		}
		for _, channel := range channels {
			props = append(props, property.MustBuild(&channel))
			ch := pkg.Channels[channel.Name]
			bName := testBundleName(pkgName, version)
			bImage := testBundleImage(pkgName, version)
			bundle := &model.Bundle{
				Package:    pkg,
				Channel:    ch,
				Name:       bName,
				Image:      bImage,
				Replaces:   channel.Replaces,
				Properties: props,
				RelatedImages: []model.RelatedImage{{
					Name:  "bundle",
					Image: testBundleImage(pkgName, version),
				}},
				CsvJSON: `{"kind": "ClusterServiceVersion", "apiVersion": "operators.coreos.com/v1alpha1"}`,
				Objects: []string{
					`{"kind": "ClusterServiceVersion", "apiVersion": "operators.coreos.com/v1alpha1"}`,
					`{"kind": "CustomResourceDefinition", "apiVersion": "apiextensions.k8s.io/v1"}`,
				},
			}
			ch.Bundles[bName] = bundle
		}
	}
	return pkg
}

func testPackageDescription(pkg string) string {
	return fmt.Sprintf("%s operator", pkg)
}

func testBundleName(pkg, version string) string {
	return fmt.Sprintf("%s.v%s", pkg, version)
}

func testBundleImage(pkg, version string) string {
	return fmt.Sprintf("%s-bundle:v%s", pkg, version)
}
