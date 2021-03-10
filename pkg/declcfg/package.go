package declcfg

type pkg struct {
	Schema            string   `json:"schema"`
	Name              string   `json:"name"`
	DefaultChannel    string   `json:"defaultChannel"`
	Icon              *icon    `json:"icon,omitempty"`
	ValidChannelNames []string `json:"validChannelNames,omitempty"`
	Description       string   `json:"description,omitempty"`
}

type icon struct {
	Data      []byte `json:"base64data"`
	MediaType string `json:"mediatype"`
}

func (p pkg) isValidChannel(name string) bool {
	if len(p.ValidChannelNames) == 0 {
		return true
	}
	for _, validName := range p.ValidChannelNames {
		if validName == name {
			return true
		}
	}
	return false
}
