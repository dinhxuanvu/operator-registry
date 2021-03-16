package declcfg

import (
	"encoding/json"
	"fmt"

	"github.com/operator-framework/operator-registry/pkg/model"
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

	var others []json.RawMessage
	if includeUnrecognized {
		others = []json.RawMessage{
			json.RawMessage(`{ "schema": "custom.1" }`),
			json.RawMessage(`{ "schema": "custom.2" }`),
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
		Others: others,
	}
}

func buildInvalidDeclarativeConfig() DeclarativeConfig {
	return DeclarativeConfig{
		Packages: []pkg{},
		Bundles: []bundle{
			newTestBundle("anakin", "0.1.0", skipProvidedPackage()),
		},
	}
}

type bundleOpt func(*bundle)

func skipProvidedPackage() func(*bundle) {
	return func(b *bundle) {
		i := 0
		for _, p := range b.Properties {
			if p.Type != propertyTypeProvidedPackage {
				b.Properties[i] = p
				i++
			}
		}
		b.Properties = b.Properties[:i]
	}
}

func withChannel(name, replaces string) func(*bundle) {
	return func(b *bundle) {
		b.Properties = append(b.Properties, channelProperty(name, replaces))
	}
}

func channelProperty(name, replaces string) property {
	return property{
		Type:  propertyTypeChannel,
		Value: channelPropertyValue(name, replaces),
	}
}

func channelPropertyValue(name, replaces string) json.RawMessage {
	if replaces == "" {
		return json.RawMessage(fmt.Sprintf(`{"name":%q}`, name))
	}
	return json.RawMessage(fmt.Sprintf(`{"name":%q,"replaces":%q}`, name, replaces))
}

func withSkips(name string) func(*bundle) {
	return func(b *bundle) {
		b.Properties = append(b.Properties, skipsProperty(name))
	}
}

func skipsProperty(skips string) property {
	return property{
		Type:  propertyTypeSkips,
		Value: skipsPropertyValue(skips),
	}
}

func skipsPropertyValue(skips string) json.RawMessage {
	return json.RawMessage(fmt.Sprintf("%q", skips))
}

func providedPackageProperty(packageName, version string) property {
	return property{
		Type:  propertyTypeProvidedPackage,
		Value: providedPackagePropertyValue(packageName, version),
	}
}

func providedPackagePropertyValue(packageName, version string) json.RawMessage {
	return json.RawMessage(fmt.Sprintf(`{"packageName":%q, "version":%q}`, packageName, version))
}

func newTestBundle(packageName, version string, opts ...bundleOpt) bundle {
	b := bundle{
		Schema: schemaBundle,
		Name:   testBundleName(packageName, version),
		Image:  testBundleImage(packageName, version),
		Properties: []property{
			providedPackageProperty(packageName, version),
		},
	}
	for _, opt := range opts {
		opt(&b)
	}
	return b
}

const svgSmallCircle = `<svg viewBox="0 0 100 100"><circle cx="25" cy="25" r="25"/></svg>`
const svgBigCircle = `<svg viewBox="0 0 100 100"><circle cx="50" cy="50" r="50"/></svg>`

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

	versions := map[string][]channel{
		"0.0.1": {{"light", ""}, {"dark", ""}},
		"0.1.0": {
			{"light", testBundleName(pkgName, "0.0.1")},
			{"dark", testBundleName(pkgName, "0.0.1")},
		},
		"0.1.1": {{"dark", testBundleName(pkgName, "0.0.1")}},
	}
	for version, channels := range versions {
		props := []model.Property{
			{
				Type:  propertyTypeProvidedPackage,
				Value: providedPackagePropertyValue(pkgName, version),
			},
		}
		for _, channel := range channels {
			props = append(props, model.Property{
				Type:  propertyTypeChannel,
				Value: channelPropertyValue(channel.Name, channel.Replaces),
			})
			ch := pkg.Channels[channel.Name]
			bName := testBundleName(pkgName, version)
			bImage := testBundleImage(pkgName, version)
			skips := []string{}
			if version == "0.1.1" {
				skips = append(skips, testBundleName(pkgName, "0.1.0"))
				props = append(props, model.Property{
					Type:  propertyTypeSkips,
					Value: skipsPropertyValue(testBundleName(pkgName, "0.1.0")),
				})
			}
			bundle := &model.Bundle{
				Package:    pkg,
				Channel:    ch,
				Name:       bName,
				Image:      bImage,
				Replaces:   channel.Replaces,
				Skips:      skips,
				Properties: props,
			}
			ch.Bundles[bName] = bundle
		}
	}
	if err := pkg.Validate(); err != nil {
		panic(err)
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

	versions := map[string][]channel{
		"1.0.0": {{"mando", ""}},
		"2.0.0": {{"mando", testBundleName(pkgName, "1.0.0")}},
	}
	for version, channels := range versions {
		props := []model.Property{
			{
				Type:  propertyTypeProvidedPackage,
				Value: providedPackagePropertyValue(pkgName, version),
			},
		}
		for _, channel := range channels {
			props = append(props, model.Property{
				Type:  propertyTypeChannel,
				Value: channelPropertyValue(channel.Name, channel.Replaces),
			})
		}
		for _, channel := range channels {
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
			}
			ch.Bundles[bName] = bundle
		}
	}
	if err := pkg.Validate(); err != nil {
		panic(err)
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