package declcfg

import (
	"encoding/json"
)

type bundle struct {
	Schema        string         `json:"schema"`
	Name          string         `json:"name"`
	Package       string         `json:"package"`
	Image         string         `json:"image"`
	Version       string         `json:"version"`
	Properties    []property     `json:"properties,omitempty"`
	RelatedImages []relatedImage `json:"relatedImages,omitempty"`
}

type property struct {
	Type  string          `json:"type"`
	Value json.RawMessage `json:"value"`
}

type relatedImage struct {
	Name  string `json:"name"`
	Image string `json:"image"`
}
