package declcfg

import (
	"path/filepath"
	"testing"
)

var (
	configsDir = filepath.Join("testdata", "configs")
	configFile = filepath.Join(configsDir, "etcd.json")
)

func TestLoadFile(t *testing.T) {
	_ = configFile
	t.Skipf("pending")
}

func TestLoadDir(t *testing.T) {
	_ = configsDir
	t.Skipf("pending")
}
