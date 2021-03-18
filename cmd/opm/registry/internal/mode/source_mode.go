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
	ModeSqlite      Mode = "sqlite"
	ModeDeclCfgTar  Mode = "declcfgTar"
	ModeDeclCfgDir  Mode = "declcfgDir"
	ModeDeclCfgFile Mode = "declcfgFile"
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
		return ModeDeclCfgDir, nil
	}

	t, err := filetype.MatchFile(path)
	if err != nil {
		return "", err
	}
	switch t {
	case matchers.TypeSqlite:
		return ModeSqlite, nil
	case matchers.TypeTar:
		return ModeDeclCfgTar, nil
	case typeJSON:
		return ModeDeclCfgFile, nil
	}
	return "", fmt.Errorf("cannot use filetype %q as registry source", t)
}
