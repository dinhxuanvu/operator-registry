package declcfg

import "github.com/operator-framework/operator-registry/pkg/property"

type bundle struct {
	Schema        string              `json:"schema"`
	Name          string              `json:"name"`
	Package       string              `json:"package"`
	Image         string              `json:"image"`
	Properties    []property.Property `json:"properties,omitempty"`
	RelatedImages []relatedImage      `json:"relatedImages,omitempty"`
}

type relatedImage struct {
	Name  string `json:"name"`
	Image string `json:"image"`
}
