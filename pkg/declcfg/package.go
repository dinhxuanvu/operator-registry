package declcfg

type Package struct {
	Schema         string   `json:"schema"`
	Name           string   `json:"name"`
	DefaultChannel string   `json:"defaultChannel"`
	Icon           *Icon    `json:"icon,omitempty"`
	Channels       []string `json:"channels"`
	Description    string   `json:"description,omitempty"`
}

type Icon struct {
	Base64Data []byte `json:"base64data"`
	MediaType  string `json:"mediatype"`
}
