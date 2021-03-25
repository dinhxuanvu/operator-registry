package property

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"reflect"
)

type Property struct {
	Type  string          `json:"type"`
	Value json.RawMessage `json:"value"`
}

func (p Property) Validate() error {
	if len(p.Type) == 0 {
		return errors.New("type must be set")
	}
	if len(p.Value) == 0 {
		return errors.New("value must be set")
	}
	var raw json.RawMessage
	if err := json.Unmarshal(p.Value, &raw); err != nil {
		return fmt.Errorf("value is not valid json: %v", err)
	}
	return nil
}

type Package struct {
	PackageName string `json:"packageName"`
	Version     string `json:"version"`
}

type PackageRequired struct {
	PackageName  string `json:"packageName"`
	VersionRange string `json:"versionRange"`
}

type Channel struct {
	Name     string `json:"name"`
	Replaces string `json:"replaces,omitempty"`
}

type GVK struct {
	Group   string `json:"group"`
	Kind    string `json:"kind"`
	Version string `json:"version"`
}

type GVKRequired struct {
	Group   string `json:"group"`
	Kind    string `json:"kind"`
	Version string `json:"version"`
}

type Skips string
type SkipRange string

type Properties struct {
	Packages         []Package
	PackagesRequired []PackageRequired
	Channels         []Channel
	GVKs             []GVK
	GVKsRequired     []GVKRequired
	Skips            []Skips
	SkipRanges       []SkipRange

	All    []Property
	Others []Property
}

const (
	TypePackage         = "olm.package"
	TypePackageRequired = "olm.package.required"
	TypeChannel         = "olm.channel"
	TypeGVK             = "olm.gvk"
	TypeGVKRequired     = "olm.gvk.required"
	TypeSkips           = "olm.skips"
	TypeSkipRange       = "olm.skipRange"
)

func Parse(in []Property) (*Properties, error) {
	var out Properties
	for i, prop := range in {
		out.All = append(out.All, prop)
		switch prop.Type {
		case TypePackage:
			var p Package
			if err := json.Unmarshal(prop.Value, &p); err != nil {
				return nil, ParseError{Idx: i, Typ: prop.Type, Err: err}
			}
			out.Packages = append(out.Packages, p)
		case TypePackageRequired:
			var p PackageRequired
			if err := json.Unmarshal(prop.Value, &p); err != nil {
				return nil, ParseError{Idx: i, Typ: prop.Type, Err: err}
			}
			out.PackagesRequired = append(out.PackagesRequired, p)
		case TypeChannel:
			var p Channel
			if err := json.Unmarshal(prop.Value, &p); err != nil {
				return nil, ParseError{Idx: i, Typ: prop.Type, Err: err}
			}
			out.Channels = append(out.Channels, p)
		case TypeGVK:
			var p GVK
			if err := json.Unmarshal(prop.Value, &p); err != nil {
				return nil, ParseError{Idx: i, Typ: prop.Type, Err: err}
			}
			out.GVKs = append(out.GVKs, p)
		case TypeGVKRequired:
			var p GVKRequired
			if err := json.Unmarshal(prop.Value, &p); err != nil {
				return nil, ParseError{Idx: i, Typ: prop.Type, Err: err}
			}
			out.GVKsRequired = append(out.GVKsRequired, p)
		case TypeSkips:
			var p Skips
			if err := json.Unmarshal(prop.Value, &p); err != nil {
				return nil, ParseError{Idx: i, Typ: prop.Type, Err: err}
			}
			out.Skips = append(out.Skips, p)
		case TypeSkipRange:
			var p SkipRange
			if err := json.Unmarshal(prop.Value, &p); err != nil {
				return nil, ParseError{Idx: i, Typ: prop.Type, Err: err}
			}
			out.SkipRanges = append(out.SkipRanges, p)
		default:
			var p json.RawMessage
			if err := json.Unmarshal(prop.Value, &p); err != nil {
				return nil, ParseError{Idx: i, Typ: prop.Type, Err: err}
			}
			out.Others = append(out.Others, prop)
		}
	}
	return &out, nil
}

func Deduplicate(in []Property) []Property {
	type key struct {
		typ   string
		value string
	}

	props := map[key]Property{}
	var out []Property
	for _, p := range in {
		k := key{p.Type, string(p.Value)}
		if _, ok := props[k]; ok {
			continue
		}
		props[k] = p
		out = append(out, p)
	}
	return out
}

func ValidateBackCompat(in Properties) error {
	if len(in.Packages) != 1 {
		return fmt.Errorf("property type %q is required", TypePackage)
	}
	return nil
}

func Build(p interface{}) (*Property, error) {
	var (
		typ string
		val interface{}
	)
	if prop, ok := p.(*Property); ok {
		typ = prop.Type
		val = prop.Value
	} else {
		t := reflect.TypeOf(p)
		if t.Kind() != reflect.Ptr {
			panic("All types must be pointers to structs.")
		}
		typ, ok = scheme[t]
		if !ok {
			panic(fmt.Sprintf("%s not a known property type", t))
		}
		val = p
	}
	d, err := jsonMarshal(val)
	if err != nil {
		return nil, err
	}

	return &Property{
		Type:  typ,
		Value: bytes.TrimSpace(d),
	}, nil
}

func MustBuild(p interface{}) Property {
	prop, err := Build(p)
	if err != nil {
		panic(err)
	}
	return *prop
}

func jsonMarshal(p interface{}) ([]byte, error) {
	buf := &bytes.Buffer{}
	enc := newJSONEncoder(buf)
	if err := enc.Encode(p); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func newJSONEncoder(w io.Writer) json.Encoder {
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "")
	return *enc
}

func MustBuildPackage(name, version string) Property {
	return MustBuild(&Package{PackageName: name, Version: version})
}
func MustBuildPackageRequired(name, versionRange string) Property {
	return MustBuild(&PackageRequired{name, versionRange})
}
func MustBuildChannel(name, replaces string) Property {
	return MustBuild(&Channel{name, replaces})
}
func MustBuildGVK(group, version, kind string) Property {
	return MustBuild(&GVK{group, kind, version})
}
func MustBuildGVKRequired(group, version, kind string) Property {
	return MustBuild(&GVKRequired{group, kind, version})
}
func MustBuildSkips(skips string) Property {
	s := Skips(skips)
	return MustBuild(&s)
}
func MustBuildSkipRange(skipRange string) Property {
	s := SkipRange(skipRange)
	return MustBuild(&s)
}
