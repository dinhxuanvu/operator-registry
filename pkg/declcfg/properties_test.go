package declcfg

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseProperties(t *testing.T) {
	type spec struct {
		name          string
		properties    []property
		expectErrType error
		expectProps   *properties
	}

	specs := []spec{
		{
			name: "Error/InvalidChannel",
			properties: []property{
				{Type: propertyTypeChannel, Value: json.RawMessage(`""`)},
			},
			expectErrType: propertyParseError{},
		},
		{
			name: "Error/InvalidSkips",
			properties: []property{
				{Type: propertyTypeSkips, Value: json.RawMessage(`{}`)},
			},
			expectErrType: propertyParseError{},
		},
		{
			name: "Error/DuplicateChannels",
			properties: []property{
				channelProperty("alpha", "foo.v0.0.3"),
				channelProperty("beta", "foo.v0.0.3"),
				channelProperty("alpha", "foo.v0.0.4"),
			},
			expectErrType: propertyDuplicateError{},
		},
		{
			name: "Success/Valid",
			properties: []property{
				channelProperty("alpha", "foo.v0.0.3"),
				channelProperty("beta", "foo.v0.0.4"),
				skipsProperty("foo.v0.0.1"),
				skipsProperty("foo.v0.0.2"),
			},
			expectProps: &properties{
				channels: []channel{
					{Name: "alpha", Replaces: "foo.v0.0.3"},
					{Name: "beta", Replaces: "foo.v0.0.4"},
				},
				skips: []string{"foo.v0.0.1", "foo.v0.0.2"},
			},
		},
	}

	for _, s := range specs {
		t.Run(s.name, func(t *testing.T) {
			props, err := parseProperties(s.properties)
			if s.expectErrType != nil {
				assert.IsType(t, s.expectErrType, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, s.expectProps, props)
			}
		})
	}
}
