package model

import (
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/blang/semver"
	"github.com/h2non/filetype"
	"k8s.io/apimachinery/pkg/util/validation"
)

type Model map[string]*Package

func (m Model) Validate() error {
	for name, pkg := range m {
		if err := pkg.Validate(); err != nil {
			return fmt.Errorf("invalid package %q: %v", pkg.Name, err)
		}
		if name != pkg.Name {
			return fmt.Errorf("package key %q does not match package name %q", name, pkg.Name)
		}
	}
	return nil
}

type Package struct {
	Name           string
	Description    string
	Icon           *Icon
	DefaultChannel *Channel
	Channels       map[string]*Channel
}

func (m *Package) Validate() error {
	if m.Name == "" {
		return errors.New("package name must not be empty")
	}

	if m.Icon != nil {
		if err := m.Icon.Validate(); err != nil {
			return fmt.Errorf("invalid icon: %v", err)
		}
	}

	if m.DefaultChannel == nil {
		return fmt.Errorf("default channel must be set")
	}

	foundDefault := false
	for name, ch := range m.Channels {
		if err := ch.Validate(); err != nil {
			return fmt.Errorf("invalid channel %q: %v", ch.Name, err)
		}
		if ch == m.DefaultChannel {
			foundDefault = true
		}
		if ch.Package != m {
			return fmt.Errorf("channel %q not correctly linked to parent package", ch.Name)
		}
		if name != ch.Name {
			return fmt.Errorf("channel key %q does not match channel name %q", name, ch.Name)
		}
	}

	if !foundDefault {
		return fmt.Errorf("default channel %q not found in channels list", m.DefaultChannel.Name)
	}
	return nil
}

type Icon struct {
	Data      []byte
	MediaType string
}

func (i *Icon) Validate() error {
	if len(i.Data) == 0 {
		return errors.New("icon data must be set if icon is defined")
	}
	if !filetype.IsImage(i.Data) {
		return errors.New("icon data is not an image")
	}
	t, err := filetype.Match(i.Data)
	if err != nil {
		return err
	}
	if t.MIME.Type != i.MediaType {
		return fmt.Errorf("icon media type %q does not match detected media type %q", i.MediaType, t.MIME)
	}
	return nil
}

type Channel struct {
	Package *Package
	Name    string
	Head    *Bundle
	Bundles map[string]*Bundle
}

func (c *Channel) Validate() error {
	if c.Name == "" {
		return errors.New("channel name must not be empty")
	}

	if c.Package == nil {
		return errors.New("package must be set")
	}

	if c.Head == nil {
		return fmt.Errorf("channel head must be set")
	}

	foundHead := false
	for name, b := range c.Bundles {
		if err := b.Validate(); err != nil {
			return fmt.Errorf("invalid bundle %q: %v", b.Name, err)
		}
		if b == c.Head {
			foundHead = true
		}
		if b.Channel != c {
			return fmt.Errorf("bundle %q not correctly linked to parent channel", b.Name)
		}
		if name != b.Name {
			return fmt.Errorf("bundle key %q does not match bundle name %q", name, b.Name)
		}
	}

	if !foundHead {
		return fmt.Errorf("channel head %q not found in bundles list", c.Head.Name)
	}
	return nil
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

func (b *Bundle) Validate() error {
	if b.Name == "" {
		return errors.New("name must be set")
	}
	if b.Channel == nil {
		return errors.New("package must be set")
	}
	if b.Package == nil {
		return errors.New("package must be set")
	}
	if b.Package != b.Channel.Package {
		return errors.New("package does not match channel's package")
	}
	if b.Replaces != "" {
		if _, ok := b.Channel.Bundles[b.Replaces]; !ok {
			return fmt.Errorf("replaces %q not found in channel", b.Replaces)
		}
	}
	for i, prop := range b.Properties {
		if err := prop.Validate(); err != nil {
			return fmt.Errorf("invalid property[%d]: %v", i, err)
		}
	}
	for i, rapi := range b.RequiredAPIs {
		if err := rapi.Validate(); err != nil {
			return fmt.Errorf("invalid required api[%d]: %v", i, err)
		}
	}
	for i, papi := range b.ProvidedAPIs {
		if err := papi.Validate(); err != nil {
			return fmt.Errorf("invalid provided api[%d]: %v", i, err)
		}
	}
	if b.SkipRange != "" {
		if _, err := semver.ParseRange(b.SkipRange); err != nil {
			return fmt.Errorf("invalid skipRange %q: %v", b.SkipRange, err)
		}
	}
	// TODO(joelanford): What is the expected presence of skipped CSVs?
	for _, skip := range b.Skips {
		_ = skip
	}
	// TODO(joelanford): Validate image string as container reference?
	_ = b.Image
	if _, err := semver.Parse(b.Version); err != nil {
		return fmt.Errorf("invalid version %q: %v", b.Version, err)
	}
	for i, reqPkg := range b.RequiredPackages {
		if err := reqPkg.Validate(); err != nil {
			return fmt.Errorf("invalid required package[%d]: %v", i, err)
		}
	}
	return nil
}

func (b Bundle) Provides(group, version, kind string) bool {
	for _, gvk := range b.ProvidedAPIs {
		if group == gvk.Group && version == gvk.Version && kind == gvk.Kind {
			return true
		}
	}
	return false
}

type Property struct {
	Type  string
	Value string
}

func (p Property) Validate() error {
	if p.Type == "" {
		return errors.New("type must be set")
	}
	if p.Value == "" {
		return errors.New("value must be set")
	}
	return nil
}

type GroupVersionKind struct {
	Group   string
	Version string
	Kind    string
	Plural  string
}

const (
	versionPattern = "^v\\d+(?:alpha\\d+|beta\\d+)?$"
)

var (
	versionRegex = regexp.MustCompile(versionPattern)
)

func (gvk GroupVersionKind) Validate() error {
	if errs := validation.IsDNS1123Subdomain(gvk.Group); len(errs) != 0 {
		return fmt.Errorf("invalid group %q: %s", gvk.Group, strings.Join(errs, ", "))
	}
	if !versionRegex.MatchString(gvk.Version) {
		return fmt.Errorf("invalid version %q: must match %s", gvk.Version, versionPattern)
	}
	if errs := validation.IsDNS1035Label(strings.ToLower(gvk.Kind)); len(errs) != 0 {
		return fmt.Errorf("invalid kind %q: %s", gvk.Kind, strings.Join(errs, ", "))
	}
	if string(gvk.Kind[0]) == strings.ToLower(string(gvk.Kind[0])) {
		return fmt.Errorf("invalid kind %q: must start with an uppercase character", gvk.Kind)
	}
	return nil
}

type PackageRequirement struct {
	PackageName string
	Version     string
}

func (r PackageRequirement) Validate() error {
	if r.PackageName == "" {
		return errors.New("package name must be set")
	}
	if _, err := semver.ParseRange(r.Version); err != nil {
		return fmt.Errorf("invalid version %q: %v", r.Version, err)
	}
	return nil
}