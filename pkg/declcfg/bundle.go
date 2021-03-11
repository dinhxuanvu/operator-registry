package declcfg

type bundle struct {
	Schema        string         `json:"schema"`
	Name          string         `json:"name"`
	Image         string         `json:"image"`
	Properties    []property     `json:"properties,omitempty"`
	RelatedImages []relatedImage `json:"relatedImages,omitempty"`
}

type relatedImage struct {
	Name  string `json:"name"`
	Image string `json:"image"`
}
