package model

import (
	"encoding/base64"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type validator interface {
	Validate() error
}

const svgData = `PHN2ZyB2aWV3Qm94PTAgMCAxMDAgMTAwPjxjaXJjbGUgY3g9MjUgY3k9MjUgcj0yNS8+PC9zdmc+`
const pngData = `iVBORw0KGgoAAAANSUhEUgAAAAEAAAABAQMAAAAl21bKAAAAA1BMVEUAAACnej3aAAAAAXRSTlMAQObYZgAAAApJREFUCNdjYAAAAAIAAeIhvDMAAAAASUVORK5CYII=`
const jpegData = `/9j/4AAQSkZJRgABAQEAYABgAAD/2wBDAAgGBgcGBQgHBwcJCQgKDBQNDAsLDBkSEw8UHRofHh0aHBwgJC4nICIsIxwcKDcpLDAxNDQ0Hyc5PTgyPC4zNDL/2wBDAQkJCQwLDBgNDRgyIRwhMjIyMjIyMjIyMjIyMjIyMjIyMjIyMjIyMjIyMjIyMjIyMjIyMjIyMjIyMjIyMjIyMjL/wAARCAABAAEDASIAAhEBAxEB/8QAHwAAAQUBAQEBAQEAAAAAAAAAAAECAwQFBgcICQoL/8QAtRAAAgEDAwIEAwUFBAQAAAF9AQIDAAQRBRIhMUEGE1FhByJxFDKBkaEII0KxwRVS0fAkM2JyggkKFhcYGRolJicoKSo0NTY3ODk6Q0RFRkdISUpTVFVWV1hZWmNkZWZnaGlqc3R1dnd4eXqDhIWGh4iJipKTlJWWl5iZmqKjpKWmp6ipqrKztLW2t7i5usLDxMXGx8jJytLT1NXW19jZ2uHi4+Tl5ufo6erx8vP09fb3+Pn6/8QAHwEAAwEBAQEBAQEBAQAAAAAAAAECAwQFBgcICQoL/8QAtREAAgECBAQDBAcFBAQAAQJ3AAECAxEEBSExBhJBUQdhcRMiMoEIFEKRobHBCSMzUvAVYnLRChYkNOEl8RcYGRomJygpKjU2Nzg5OkNERUZHSElKU1RVVldYWVpjZGVmZ2hpanN0dXZ3eHl6goOEhYaHiImKkpOUlZaXmJmaoqOkpaanqKmqsrO0tba3uLm6wsPExcbHyMnK0tPU1dbX2Nna4uPk5ebn6Onq8vP09fb3+Pn6/9oADAMBAAIRAxEAPwD3+iiigD//2Q==`

func mustBase64Decode(in string) []byte {
	out, err := base64.StdEncoding.DecodeString(in)
	if err != nil {
		panic(err)
	}
	return out
}

func TestChannelHead(t *testing.T) {
	type spec struct {
		name      string
		ch        Channel
		head      *Bundle
		assertion require.ErrorAssertionFunc
	}

	head := &Bundle{
		Name:     "anakin.v0.0.3",
		Replaces: "anakin.v0.0.1",
		Skips:    []string{"anakin.v0.0.2"},
	}

	specs := []spec{
		{
			name: "Success/Valid",
			ch: Channel{Bundles: map[string]*Bundle{
				"anakin.v0.0.1": {Name: "anakin.v0.0.1"},
				"anakin.v0.0.2": {Name: "anakin.v0.0.2"},
				"anakin.v0.0.3": head,
			}},
			head:      head,
			assertion: require.NoError,
		},
		{
			name: "Error/NoChannelHead",
			ch: Channel{Bundles: map[string]*Bundle{
				"anakin.v0.0.1": {Name: "anakin.v0.0.1", Replaces: "anakin.v0.0.3"},
				"anakin.v0.0.3": head,
			}},
			assertion: require.Error,
		},
		{
			name: "Error/MultipleChannelHeads",
			ch: Channel{Bundles: map[string]*Bundle{
				"anakin.v0.0.1": {Name: "anakin.v0.0.1"},
				"anakin.v0.0.3": head,
				"anakin.v0.0.4": {Name: "anakin.v0.0.4", Replaces: "anakin.v0.0.1"},
			}},
			assertion: require.Error,
		},
	}
	for _, s := range specs {
		t.Run(s.name, func(t *testing.T) {
			h, err := s.ch.Head()
			assert.Equal(t, s.head, h)
			s.assertion(t, err)
		})
	}
}

func TestValidators(t *testing.T) {
	type spec struct {
		name      string
		v         validator
		assertion require.ErrorAssertionFunc
	}

	pkg, ch, _ := makePackageChannelBundle()
	pkgIncorrectDefaultChannel, _, _ := makePackageChannelBundle()
	pkgIncorrectDefaultChannel.DefaultChannel = &Channel{Name: "not-found"}

	var nilIcon *Icon = nil

	specs := []spec{
		{
			name: "Model/Success/Valid",
			v: Model{
				pkg.Name: pkg,
			},
			assertion: require.NoError,
		},
		{
			name: "Model/Error/PackageKeyNameMismatch",
			v: Model{
				"foo": pkg,
			},
			assertion: require.Error,
		},
		{
			name: "Model/Error/InvalidPackage",
			v: Model{
				pkgIncorrectDefaultChannel.Name: pkgIncorrectDefaultChannel,
			},
			assertion: require.Error,
		},
		{
			name:      "Package/Success/Valid",
			v:         pkg,
			assertion: require.NoError,
		},
		{
			name:      "Package/Error/NoName",
			v:         &Package{},
			assertion: require.Error,
		},
		{
			name: "Package/Error/InvalidIcon",
			v: &Package{
				Name: "anakin",
				Icon: &Icon{Data: mustBase64Decode(svgData)},
			},
			assertion: require.Error,
		},
		{
			name: "Package/Error/NoChannels",
			v: &Package{
				Name: "anakin",
				Icon: &Icon{Data: mustBase64Decode(svgData), MediaType: "image/svg+xml"},
			},
			assertion: require.Error,
		},
		{
			name: "Package/Error/NoDefaultChannel",
			v: &Package{
				Name:     "anakin",
				Icon:     &Icon{Data: mustBase64Decode(svgData), MediaType: "image/svg+xml"},
				Channels: map[string]*Channel{"light": ch},
			},
			assertion: require.Error,
		},
		{
			name: "Package/Error/ChannelKeyNameMismatch",
			v: &Package{
				Name:           "anakin",
				Icon:           &Icon{Data: mustBase64Decode(svgData), MediaType: "image/svg+xml"},
				DefaultChannel: ch,
				Channels:       map[string]*Channel{"dark": ch},
			},
			assertion: require.Error,
		},
		{
			name: "Package/Error/InvalidChannel",
			v: &Package{
				Name:           "anakin",
				Icon:           &Icon{Data: mustBase64Decode(svgData), MediaType: "image/svg+xml"},
				DefaultChannel: ch,
				Channels:       map[string]*Channel{"light": {Name: "light"}},
			},
			assertion: require.Error,
		},
		{
			name: "Package/Error/InvalidChannelPackageLink",
			v: &Package{
				Name:           "anakin",
				Icon:           &Icon{Data: mustBase64Decode(svgData), MediaType: "image/svg+xml"},
				DefaultChannel: ch,
				Channels:       map[string]*Channel{"light": ch},
			},
			assertion: require.Error,
		},
		{
			name:      "Package/Error/DefaultChannelNotInChannelMap",
			v:         pkgIncorrectDefaultChannel,
			assertion: require.Error,
		},
		{
			name: "Icon/Success/ValidSVG",
			v: &Icon{
				Data:      mustBase64Decode(svgData),
				MediaType: "image/svg+xml",
			},
			assertion: require.NoError,
		},
		{
			name: "Icon/Success/ValidPNG",
			v: &Icon{
				Data:      mustBase64Decode(pngData),
				MediaType: "image/png",
			},
			assertion: require.NoError,
		},
		{
			name: "Icon/Success/ValidJPEG",
			v: &Icon{
				Data:      mustBase64Decode(jpegData),
				MediaType: "image/jpeg",
			},
			assertion: require.NoError,
		},
		{
			name:      "Icon/Success/Nil",
			v:         nilIcon,
			assertion: require.NoError,
		},
		{
			name: "Icon/Error/NoData",
			v: &Icon{
				Data:      nil,
				MediaType: "image/svg+xml",
			},
			assertion: require.Error,
		},
		{
			name: "Icon/Error/NoMediaType",
			v: &Icon{
				Data:      mustBase64Decode(svgData),
				MediaType: "",
			},
			assertion: require.Error,
		},
		{
			name: "Icon/Error/DataIsNotImage",
			v: &Icon{
				Data:      []byte("{}"),
				MediaType: "application/json",
			},
			assertion: require.Error,
		},
		{
			name: "Icon/Error/DataDoesNotMatchMediaType",
			v: &Icon{
				Data:      mustBase64Decode(svgData),
				MediaType: "image/jpeg",
			},
			assertion: require.Error,
		},
		{
			name:      "Channel/Success/Valid",
			v:         ch,
			assertion: require.NoError,
		},
		{
			name:      "Channel/Error/NoName",
			v:         &Channel{},
			assertion: require.Error,
		},
		{
			name: "Channel/Error/NoPackage",
			v: &Channel{
				Name: "light",
			},
			assertion: require.Error,
		},
		{
			name: "Channel/Error/NoBundles",
			v: &Channel{
				Package: pkg,
				Name:    "light",
			},
			assertion: require.Error,
		},
		{
			name: "Channel/Error/InvalidHead",
			v: &Channel{
				Package: pkg,
				Name:    "light",
				Bundles: map[string]*Bundle{
					"anakin.v0.0.0": &Bundle{Name: "anakin.v0.0.0"},
					"anakin.v0.0.1": &Bundle{Name: "anakin.v0.0.1"},
				},
			},
			assertion: require.Error,
		},
		{
			name: "Channel/Error/BundleKeyNameMismatch",
			v: &Channel{
				Package: pkg,
				Name:    "light",
				Bundles: map[string]*Bundle{
					"foo": &Bundle{Name: "bar"},
				},
			},
			assertion: require.Error,
		},
		{
			name: "Channel/Error/InvalidBundle",
			v: &Channel{
				Package: pkg,
				Name:    "light",
				Bundles: map[string]*Bundle{
					"anakin.v0.0.0": {Name: "anakin.v0.0.0"},
				},
			},
			assertion: require.Error,
		},
		{
			name: "Channel/Error/InvalidBundleChannelLink",
			v: &Channel{
				Package: pkg,
				Name:    "light",
				Bundles: map[string]*Bundle{
					"anakin.v0.0.0": &Bundle{
						Package: pkg,
						Channel: ch,
						Name:    "anakin.v0.0.0",
						Image:   "anakin-operator:v0.0.0",
					},
				},
			},
			assertion: require.Error,
		},
		{
			name: "Bundle/Success/Valid",
			v: &Bundle{
				Package:  pkg,
				Channel:  ch,
				Name:     "anakin.v0.1.0",
				Image:    "",
				Replaces: "anakin.v0.0.1",
				Skips:    []string{"anakin.v0.0.2"},
				Properties: []Property{
					providedPackageProp("anakin", "0.1.0"),
					channelProp("light", "anakin.v0.0.1"),
					channelProp("dark", "anakin.v0.0.1"),
					skipsProp("anakin.v0.0.2"),
				},
			},
			assertion: require.NoError,
		},
		{
			name:      "Bundle/Error/NoName",
			v:         &Bundle{},
			assertion: require.Error,
		},
		{
			name: "Bundle/Error/NoChannel",
			v: &Bundle{
				Name: "anakin.v0.1.0",
			},
			assertion: require.Error,
		},
		{
			name: "Bundle/Error/NoPackage",
			v: &Bundle{
				Channel: ch,
				Name:    "anakin.v0.1.0",
			},
			assertion: require.Error,
		},
		{
			name: "Bundle/Error/WrongPackage",
			v: &Bundle{
				Package: &Package{},
				Channel: ch,
				Name:    "anakin.v0.1.0",
			},
			assertion: require.Error,
		},
		{
			name: "Bundle/Error/ReplacesNotInChannel",
			v: &Bundle{
				Package:  pkg,
				Channel:  ch,
				Name:     "anakin.v0.1.0",
				Replaces: "anakin.v0.0.0",
			},
			assertion: require.Error,
		},
		{
			name: "Bundle/Error/InvalidProperty",
			v: &Bundle{
				Package:    pkg,
				Channel:    ch,
				Name:       "anakin.v0.1.0",
				Replaces:   "anakin.v0.0.1",
				Properties: []Property{{Value: json.RawMessage("")}},
			},
			assertion: require.Error,
		},
		{
			name: "Bundle/Error/EmptySkipsValue",
			v: &Bundle{
				Package:    pkg,
				Channel:    ch,
				Name:       "anakin.v0.1.0",
				Replaces:   "anakin.v0.0.1",
				Properties: []Property{{Type: "custom", Value: json.RawMessage("{}")}},
				Skips:      []string{""},
			},
			assertion: require.Error,
		},
		{
			name: "Property/Success/Valid",
			v: Property{
				Type:  "custom.type",
				Value: json.RawMessage("{}"),
			},
			assertion: require.NoError,
		},
		{
			name: "Property/Error/NoType",
			v: Property{
				Value: json.RawMessage(""),
			},
			assertion: require.Error,
		},
		{
			name: "Property/Error/NoValue",
			v: Property{
				Type:  "custom.type",
				Value: nil,
			},
			assertion: require.Error,
		},
		{
			name: "Property/Error/EmptyValue",
			v: Property{
				Type:  "custom.type",
				Value: json.RawMessage{},
			},
			assertion: require.Error,
		},
		{
			name: "Property/Error/ValueNotJSON",
			v: Property{
				Type:  "custom.type",
				Value: json.RawMessage("{"),
			},
			assertion: require.Error,
		},
	}
	for _, s := range specs {
		t.Run(s.name, func(t *testing.T) {
			s.assertion(t, s.v.Validate())
		})
	}
}

func makePackageChannelBundle() (*Package, *Channel, *Bundle) {
	bundle := &Bundle{
		Name:  "anakin.v0.0.1",
		Image: "anakin-operator:v0.0.1",
		Properties: []Property{
			providedPackageProp("anakin", "0.0.1"),
		},
	}
	ch := &Channel{
		Name: "light",
		Bundles: map[string]*Bundle{
			"anakin.v0.0.1": bundle,
		},
	}
	pkg := &Package{
		Name:           "anakin",
		DefaultChannel: ch,
		Channels: map[string]*Channel{
			ch.Name: ch,
		},
	}

	bundle.Channel = ch
	bundle.Package = pkg
	ch.Package = pkg

	return pkg, ch, bundle
}

func makeProperty(typ string, value interface{}) Property {
	v, err := json.Marshal(value)
	if err != nil {
		panic(err)
	}
	return Property{
		Type:  typ,
		Value: v,
	}
}

func providedPackageProp(pkg, version string) Property {
	return makeProperty("olm.package.provided", map[string]string{
		"packageName": pkg,
		"version":     version,
	})
}

func channelProp(name, replaces string) Property {
	return makeProperty("olm.channel", map[string]string{
		"name":     name,
		"replaces": replaces,
	})
}

func skipsProp(skips string) Property {
	return makeProperty("olm.skips", skips)
}
