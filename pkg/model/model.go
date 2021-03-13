package model

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/h2non/filetype"
	"github.com/h2non/filetype/matchers"
	"github.com/h2non/filetype/types"
	svg "github.com/h2non/go-is-svg"
)

func init() {
	t := types.NewType("svg", "image/svg+xml")
	filetype.AddMatcher(t, svg.Is)
	matchers.Image[types.NewType("svg", "image/svg+xml")] = svg.Is
}

type Model map[string]*Package

func (m Model) Validate() error {
	for name, pkg := range m {
		if name != pkg.Name {
			return fmt.Errorf("package key %q does not match package name %q", name, pkg.Name)
		}
		if err := pkg.Validate(); err != nil {
			return fmt.Errorf("invalid package %q: %v", pkg.Name, err)
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

	if err := m.Icon.Validate(); err != nil {
		return fmt.Errorf("invalid icon: %v", err)
	}

	if len(m.Channels) == 0 {
		return fmt.Errorf("package must contain at least one channel")
	}

	if m.DefaultChannel == nil {
		return fmt.Errorf("default channel must be set")
	}

	foundDefault := false
	for name, ch := range m.Channels {
		if name != ch.Name {
			return fmt.Errorf("channel key %q does not match channel name %q", name, ch.Name)
		}
		if err := ch.Validate(); err != nil {
			return fmt.Errorf("invalid channel %q: %v", ch.Name, err)
		}
		if ch == m.DefaultChannel {
			foundDefault = true
		}
		if ch.Package != m {
			return fmt.Errorf("channel %q not correctly linked to parent package", ch.Name)
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
	if i == nil {
		return nil
	}
	if len(i.Data) == 0 {
		return errors.New("icon data must be set if icon is defined")
	}
	if len(i.MediaType) == 0 {
		return errors.New("icon mediatype must be set if icon is defined")
	}
	// TODO(joelanford): Should we detect the media type of the data and
	//   compare it to the mediatype listed in the icon field?
	return i.validateData()
}

func (i *Icon) validateData() error {
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
//   incoming edges, based on replaces and skips. It also expects to find exactly one such bundle.
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

	if len(c.Bundles) == 0 {
		return fmt.Errorf("channel must contain at least one bundle")
	}

	if _, err := c.Head(); err != nil {
		return err
	}

	for name, b := range c.Bundles {
		if name != b.Name {
			return fmt.Errorf("bundle key %q does not match bundle name %q", name, b.Name)
		}
		if err := b.Validate(); err != nil {
			return fmt.Errorf("invalid bundle %q: %v", b.Name, err)
		}
		if b.Channel != c {
			return fmt.Errorf("bundle %q not correctly linked to parent channel", b.Name)
		}
	}
	return nil
}

type Bundle struct {
	Package    *Package
	Channel    *Channel
	Name       string
	Image      string
	Replaces   string
	Skips      []string
	Properties []Property
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

	return nil
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

	var v json.RawMessage
	if err := json.Unmarshal(p.Value, &v); err != nil {
		return fmt.Errorf("invalid value: %v", err)
	}
	return nil
}
