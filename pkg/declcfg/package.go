package declcfg

type pkg struct {
	Schema         string   `json:"schema"`
	Name           string   `json:"name"`
	DefaultChannel string   `json:"defaultChannel"`
	Icon           *icon    `json:"icon,omitempty"`
	Channels       []string `json:"channels"`
	Description    string   `json:"description,omitempty"`
}

type icon struct {
	Base64Data []byte `json:"base64data"`
	MediaType  string `json:"mediatype"`
}
