package declcfg

type pkg struct {
	Schema         string `json:"schema"`
	Name           string `json:"name"`
	DefaultChannel string `json:"defaultChannel"`
	Icon           *icon  `json:"icon,omitempty"`
	Description    string `json:"description,omitempty"`
}

type icon struct {
	Data      []byte `json:"base64data"`
	MediaType string `json:"mediatype"`
}
