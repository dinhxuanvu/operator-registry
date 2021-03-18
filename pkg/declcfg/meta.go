package declcfg

import (
	"encoding/json"
)

type meta struct {
	schema  string
	pkgName string

	data json.RawMessage
}

func (m meta) Schema() string {
	return m.schema
}

func (m meta) Package() string {
	return m.pkgName
}

func (m meta) MarshalJSON() ([]byte, error) {
	return m.data, nil
}

func (m *meta) UnmarshalJSON(d []byte) error {
	type tmp struct {
		Schema     string     `json:"schema"`
		Package    string     `json:"package,omitempty"`
		Properties []property `json:"properties,omitempty"`
	}
	var t tmp
	if err := json.Unmarshal(d, &t); err != nil {
		return err
	}
	m.schema = t.Schema
	m.pkgName = t.Package
	m.data = d
	return nil
}
