package mode

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"

	"github.com/h2non/filetype"
	"github.com/h2non/filetype/matchers"
	"github.com/h2non/filetype/types"
)

type Mode string

const (
	ModeSqlite            Mode = "sqlite"
	ModeDeclarativeConfig Mode = "declcfg"
)

var typeJSON = types.NewType("json", "application/json")

func init() {
	filetype.AddMatcher(typeJSON, matchJson)
}

func matchJson(data []byte) bool {
	dec := json.NewDecoder(bytes.NewBuffer(data))
	var j json.RawMessage
	if err := dec.Decode(&j); err != nil {
		return false
	}
	return true
}

func DetectSourceMode(path string) (Mode, error) {
	info, err := os.Stat(path)
	if err != nil {
		return "", err
	}

	if info.IsDir() {
		return ModeDeclarativeConfig, nil
	}

	t, err := filetype.MatchFile(path)
	if err != nil {
		return "", err
	}
	switch t {
	case matchers.TypeSqlite:
		return ModeSqlite, nil
	case typeJSON:
		return ModeDeclarativeConfig, nil
	}
	return "", fmt.Errorf("cannot use filetype %q as registry source", t)
}
