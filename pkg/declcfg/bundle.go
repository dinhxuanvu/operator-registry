package declcfg

import (
	"github.com/operator-framework/operator-registry/pkg/property"
)

type bundle struct {
	Schema        string              `json:"schema"`
	Name          string              `json:"name"`
	Package       string              `json:"package"`
	Image         string              `json:"image"`
	Properties    []property.Property `json:"properties,omitempty"`
	RelatedImages []relatedImage      `json:"relatedImages,omitempty"`

	// These fields are present so that we can continue serving
	// the GRPC API the way packageserver expects us to in a
	// backwards-compatible way. These are populated by a separate
	// "olm.objects" directory in the configs directory/tar.
	//
	// These fields should never be persisted in the bundle blob.
	// Instead they will be written to a separate directory in
	// the configs dir: "objects/<pkgName>/<bundleName>/"
	CsvJSON string   `json:"-"`
	Objects []string `json:"-"`
}

type relatedImage struct {
	Name  string `json:"name"`
	Image string `json:"image"`
}
