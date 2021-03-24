package mode

import (
	"fmt"
	"os"

	"github.com/h2non/filetype"
	"github.com/h2non/filetype/matchers"
)

type Mode string

const (
	ModeSqlite     Mode = "sqlite"
	ModeDeclCfgDir Mode = "declcfgDir"
)

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
	}
	return "", fmt.Errorf("cannot use filetype %q as registry source", t)
}
