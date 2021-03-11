package api

import (
	"encoding/json"
	"fmt"

	"github.com/operator-framework/operator-registry/pkg/model"
)

const (
	// Minimal property set required to translate between GRPC API and model.
	apiTypePackage              = "olm.package"
	propertyTypeProvidedPackage = "olm.package.provided"
	propertyTypeRequiredPackage = "olm.package.required"
	apiTypeGVK                  = "olm.gvk"
	propertyTypeProvidedGVK     = "olm.gvk.provided"
	propertyTypeRequiredGVK     = "olm.gvk.required"
	propertyTypeChannel         = "olm.channel"
	propertyTypeSkips           = "olm.skips"
	propertyTypeSkipRange       = "olm.skipRange"
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
	providedPackage providedPackage
	providedGVKs    []*GroupVersionKind
	requiredGVKs    []*GroupVersionKind
	skipRange       string
}

func parseProperties(props []model.Property) (*properties, error) {
	var (
		ps properties
		pp *providedPackage
	)
	for i, prop := range props {
		switch prop.Type {
		case propertyTypeProvidedPackage:
			var p providedPackage
			if err := json.Unmarshal(prop.Value, &p); err != nil {
				return nil, propertyParseError{i: i, t: prop.Type, err: err}
			}
			if pp != nil {
				return nil, propertyMultipleNotAllowedError{i: i, t: prop.Type}
			}
			pp = &p
		case propertyTypeProvidedGVK:
			var p GroupVersionKind
			if err := json.Unmarshal(prop.Value, &p); err != nil {
				return nil, propertyParseError{i: i, t: prop.Type, err: err}
			}
			ps.providedGVKs = append(ps.providedGVKs, &p)
		case propertyTypeRequiredGVK:
			var p GroupVersionKind
			if err := json.Unmarshal(prop.Value, &p); err != nil {
				return nil, propertyParseError{i: i, t: prop.Type, err: err}
			}
			ps.requiredGVKs = append(ps.requiredGVKs, &p)
		case propertyTypeSkipRange:
			var p string
			if err := json.Unmarshal(prop.Value, &p); err != nil {
				return nil, propertyParseError{i: i, t: prop.Type, err: err}
			}
			if pp != nil {
				return nil, propertyMultipleNotAllowedError{i: i, t: prop.Type}
			}
			ps.skipRange = p
		}
	}
	if pp == nil {
		return nil, fmt.Errorf("required property %q not found", propertyTypeProvidedPackage)
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
