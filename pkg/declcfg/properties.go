package declcfg

import (
	"encoding/json"
	"fmt"
)

type property struct {
	Type  string          `json:"type"`
	Value json.RawMessage `json:"value"`
}

const (
	// Minimal property set required to build model.
	propertyTypeChannel         = "olm.channel"
	propertyTypeSkips           = "olm.skips"
	propertyTypeProvidedPackage = "olm.package.provided"
)

type channel struct {
	Name     string `json:"name"`
	Replaces string `json:"replaces,omitempty"`
}

type providedPackage struct {
	PackageName string `json:"packageName"`
	Version     string `json:"version"`
}

type properties struct {
	channels        []channel
	skips           []string
	providedPackage providedPackage
}

func parseProperties(props []property) (*properties, error) {
	var (
		ps properties
		pp *providedPackage
	)

	channels := map[string]struct{}{}
	for i, prop := range props {
		switch prop.Type {
		case propertyTypeChannel:
			var p channel
			if err := json.Unmarshal(prop.Value, &p); err != nil {
				return nil, propertyParseError{i: i, t: prop.Type, err: err}
			}
			if _, ok := channels[p.Name]; ok {
				return nil, propertyDuplicateError{i: i, t: prop.Type, key: p.Name}
			}
			channels[p.Name] = struct{}{}
			ps.channels = append(ps.channels, p)
		case propertyTypeSkips:
			var p string
			if err := json.Unmarshal(prop.Value, &p); err != nil {
				return nil, propertyParseError{i: i, t: prop.Type, err: err}
			}
			ps.skips = append(ps.skips, p)
		case propertyTypeProvidedPackage:
			var p providedPackage
			if err := json.Unmarshal(prop.Value, &p); err != nil {
				return nil, propertyParseError{i: i, t: prop.Type, err: err}
			}
			if pp != nil {
				return nil, propertyMultipleNotAllowedError{i: i, t: prop.Type}
			}
			pp = &p
		}
	}
	if pp == nil {
		return nil, propertyNotFoundError{t: propertyTypeProvidedPackage}
	}
	ps.providedPackage = *pp

	return &ps, nil
}

type propertyParseError struct {
	i   int
	t   string
	err error
}

func (e propertyParseError) Error() string {
	return fmt.Sprintf("properties[%d].value parse error for %q: %v", e.i, e.t, e.err)
}

type propertyMultipleNotAllowedError struct {
	i int
	t string
}

func (e propertyMultipleNotAllowedError) Error() string {
	return fmt.Sprintf("properties[%d]: multiple properties of type %q not allowed", e.i, e.t)
}

type propertyNotFoundError struct {
	t string
}

func (e propertyNotFoundError) Error() string {
	return fmt.Sprintf("required property of type %q not found", e.t)
}

type propertyDuplicateError struct {
	i   int
	t   string
	key string
}

func (e propertyDuplicateError) Error() string {
	return fmt.Sprintf("properties[%d]: duplicate property of type %q found with key %q", e.i, e.t, e.key)
}
