package model

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/blang/semver"
	"github.com/h2non/filetype"
	svg "github.com/h2non/go-is-svg"
	"github.com/operator-framework/api/pkg/operators/v1alpha1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
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
	return nil
	// TODO(joelanford): Should we detect the media type of the data and
	//   compare it to the mediatype listed in the icon field?
	// return i.validateData()
}

func (i *Icon) validateData() error {
	// TODO(joelanford): Are SVG images valid?
	if i.MediaType == "image/svg+xml" {
		if !svg.IsSVG(i.Data) {
			return fmt.Errorf("icon media type %q does not match data", i.MediaType)
		}
		return nil
	}
	if !filetype.IsImage(i.Data) {
		return errors.New("icon data is not an image")
	}
	t, err := filetype.Match(i.Data)
	if err != nil {
		return err
	}
	if t.MIME.Value != i.MediaType {
		return fmt.Errorf("icon media type %q does not match detected media type %q", i.MediaType, t.MIME.Value)
	}
	return nil
}

type Channel struct {
	Package *Package
	Name    string
	Bundles map[string]*Bundle
}

// TODO(joelanford): This function determines the channel head by finding the bundle that has 0
//   incoming edges, based on replaces, skips, and skipRange. It also expects to find exactly one such bundle.
//   Is this the correct algorithm?
func (c Channel) Head() (*Bundle, error) {
	incoming := map[string]int{}
	for _, b := range c.Bundles {
		if b.Replaces != "" {
			incoming[b.Replaces] += 1
		}
		for _, skip := range b.Skips {
			incoming[skip] += 1
		}
		if b.SkipRange != "" {
			skipRange, err := semver.ParseRange(b.SkipRange)
			if err != nil {
				return nil, fmt.Errorf("invalid skip range %q for bundle %q: %v", b.SkipRange, b.Name, err)
			}
			for _, skipCandidate := range c.Bundles {
				if skipCandidate == b {
					continue
				}
				version, err := semver.Parse(skipCandidate.Version)
				if err != nil {
					return nil, fmt.Errorf("invalid version %q for bundle %q: %v", skipCandidate.SkipRange, skipCandidate.Name, err)
				}
				if skipRange(version) {
					incoming[skipCandidate.Name] += 1
				}
			}
		}
	}
	var heads []*Bundle
	for _, b := range c.Bundles {
		if _, ok := incoming[b.Name]; !ok {
			heads = append(heads, b)
		}
	}
	if len(heads) == 0 {
		return nil, fmt.Errorf("no channel head found in graph")
	}
	if len(heads) > 1 {
		var headNames []string
		for _, head := range heads {
			headNames = append(headNames, head.Name)
		}
		return nil, fmt.Errorf("multiple channel heads found in graph: %s", strings.Join(headNames, ", "))
	}
	return heads[0], nil
}

func (c *Channel) Validate() error {
	if c.Name == "" {
		return errors.New("channel name must not be empty")
	}

	if c.Package == nil {
		return errors.New("package must be set")
	}

	if _, err := c.Head(); err != nil {
		return err
	}

	for name, b := range c.Bundles {
		if err := b.Validate(); err != nil {
			return fmt.Errorf("invalid bundle %q: %v", b.Name, err)
		}
		if b.Channel != c {
			return fmt.Errorf("bundle %q not correctly linked to parent channel", b.Name)
		}
		if name != b.Name {
			return fmt.Errorf("bundle key %q does not match bundle name %q", name, b.Name)
		}
	}
	return nil
}

type Bundle struct {
	Package    *Package
	Channel    *Channel
	Name       string
	Version    string
	Image      string
	Replaces   string
	Skips      []string
	SkipRange  string
	Properties []Property

	// For backwards-compat reasons, include a CSV, ProvidedAPIs,
	// and Objects in the model.
	CSV          *v1alpha1.ClusterServiceVersion
	ProvidedAPIs []GroupVersionKind
	Objects      []unstructured.Unstructured

	// TODO(joelanford): we may be able to remove these from the model.
	//   Need to check to see if their presence here would simplify GRPC
	//   serving for backwards-compatibility convenience.
	RequiredAPIs     []GroupVersionKind
	RequiredPackages []RequiredPackage
}

func (b *Bundle) Validate() error {
	if b.Name == "" {
		return errors.New("name must be set")
	}
	if b.Channel == nil {
		return errors.New("channel must be set")
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
			return fmt.Errorf("invalid required api [%d]: %v", i, err)
		}
	}
	for i, papi := range b.ProvidedAPIs {
		if err := papi.Validate(); err != nil {
			return fmt.Errorf("invalid provided api [%d]: %v", i, err)
		}
	}
	version, err := semver.Parse(b.Version)
	if err != nil {
		return fmt.Errorf("invalid version %q: %v", b.Version, err)
	}

	if b.SkipRange != "" {
		skipRange, err := semver.ParseRange(b.SkipRange)
		if err != nil {
			return fmt.Errorf("invalid skipRange %q: %v", b.SkipRange, err)
		}
		if skipRange(version) {
			return fmt.Errorf("skipRange %q includes bundle's own version %q", b.SkipRange, b.Version)
		}
	}
	// TODO(joelanford): What is the expected presence of skipped CSVs?
	for i, skip := range b.Skips {
		if skip == "" {
			return fmt.Errorf("skip[%d] is empty", i)
		}
	}

	// TODO(joelanford): Validate image string as container reference?
	if b.Image == "" {
		return fmt.Errorf("image is unset")
	}

	for i, reqPkg := range b.RequiredPackages {
		if err := reqPkg.Validate(); err != nil {
			return fmt.Errorf("invalid required package [%d]: %v", i, err)
		}
	}
	return nil
}

func (b Bundle) Provides(gvk GroupVersionKind) bool {
	for _, provided := range b.ProvidedAPIs {
		if provided.Group == gvk.Group && provided.Version == gvk.Version && provided.Kind == gvk.Kind {
			return true
		}
	}
	return false
}

type Property struct {
	Type  string
	Value json.RawMessage
}

func (p Property) Validate() error {
	if p.Type == "" {
		return errors.New("type must be set")
	}
	if len(p.Value) == 0 {
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

	// TODO(joelanford): I found an example where this fails validation in an existing index,
	//   so commenting the regex check out for now, and just checking that it is set. Is the
	//   regex-based test too strict?
	//   See: https://github.com/operator-framework/community-operators/blob/ae7a82969500555bab91fc7282ebd3de2e16c8ef/community-operators/percona-server-mongodb-operator/1.4.0/percona-server-mongodb-operator.v1.4.0.clusterserviceversion.yaml#L167
	//if !versionRegex.MatchString(gvk.Version) {
	//	return fmt.Errorf("invalid version %q: must match %s", gvk.Version, versionPattern)
	//}
	if gvk.Version == "" {
		return fmt.Errorf("invalid version %q: must not be empty", gvk.Version)
	}

	if errs := validation.IsDNS1035Label(strings.ToLower(gvk.Kind)); len(errs) != 0 {
		return fmt.Errorf("invalid kind %q: %s", gvk.Kind, strings.Join(errs, ", "))
	}
	// TODO(joelanford): I found an example where this fails validation in an existing index,
	//   so commenting the uppercase letter check out for now. Seems like something that we
	//   should catch and prevent though.
	//if string(gvk.Kind[0]) == strings.ToLower(string(gvk.Kind[0])) {
	//	return fmt.Errorf("invalid kind %q: must start with an uppercase character", gvk.Kind)
	//}

	if gvk.Plural != "" {
		if errs := validation.IsDNS1035Label(gvk.Plural); len(errs) != 0 {
			return fmt.Errorf("invalid plural %q: %s", gvk.Plural, strings.Join(errs, ", "))
		}
	}
	return nil
}

type RequiredPackage struct {
	PackageName  string
	VersionRange string
}

func (r RequiredPackage) Validate() error {
	if r.PackageName == "" {
		return errors.New("package name must be set")
	}
	if _, err := semver.ParseRange(r.VersionRange); err != nil {
		return fmt.Errorf("invalid version range %q: %v", r.VersionRange, err)
	}
	return nil
}
