package declcfg

import (
	"archive/tar"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"
)

const (
	tarDirName = "index"
	globalName = "__global"
)

func WriteDir(cfg DeclarativeConfig, configDir string) error {
	entries, err := ioutil.ReadDir(configDir)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if len(entries) > 0 {
		return fmt.Errorf("config dir %q must be empty", configDir)
	}

	return writeToFS(cfg, &diskWriter{}, configDir)
}

func WriteTar(cfg DeclarativeConfig, tarFile string) error {
	f, err := os.OpenFile(tarFile, os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	tw := tar.NewWriter(f)
	defer tw.Close()

	return writeToFS(cfg, &tarWriter{tw}, tarDirName)
}

func WriteFile(cfg DeclarativeConfig, configFile string) error {
	f, err := os.OpenFile(configFile, os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()
	return writeJSON(cfg, f)
}

type fsWriter interface {
	MkdirAll(path string, mode os.FileMode) error
	WriteFile(path string, data []byte, mode os.FileMode) error
}

var _ fsWriter = &diskWriter{}

type diskWriter struct{}

func (w diskWriter) MkdirAll(path string, mode os.FileMode) error {
	return os.MkdirAll(path, mode)
}

func (w diskWriter) WriteFile(path string, data []byte, mode os.FileMode) error {
	return ioutil.WriteFile(path, data, mode)
}

var _ fsWriter = &tarWriter{}

type tarWriter struct {
	tw *tar.Writer
}

func (w tarWriter) MkdirAll(path string, mode os.FileMode) error {
	if path == "" {
		return nil
	}
	dir, _ := filepath.Split(path)
	if err := w.MkdirAll(dir, mode); err != nil {
		return err
	}
	return w.tw.WriteHeader(&tar.Header{
		Name:       path,
		Mode:       int64(mode),
		AccessTime: time.Now(),
		ChangeTime: time.Now(),
		ModTime:    time.Now(),
		Typeflag:   tar.TypeDir,
	})
}

func (w tarWriter) WriteFile(path string, data []byte, mode os.FileMode) error {
	if err := w.tw.WriteHeader(&tar.Header{
		Name:       path,
		Size:       int64(len(data)),
		Mode:       int64(mode),
		AccessTime: time.Now(),
		ChangeTime: time.Now(),
		ModTime:    time.Now(),
		Typeflag:   tar.TypeReg,
	}); err != nil {
		return err
	}
	if _, err := w.tw.Write(data); err != nil {
		return err
	}
	return nil
}

func writeToFS(cfg DeclarativeConfig, w fsWriter, rootDir string) error {
	bundlesByPackage := map[string][]bundle{}
	for _, b := range cfg.Bundles {
		bundlesByPackage[b.Package] = append(bundlesByPackage[b.Package], b)
	}
	othersByPackage := map[string][]meta{}
	for _, o := range cfg.others {
		pkgName := o.Package()
		if pkgName == "" {
			pkgName = globalName
		}
		othersByPackage[pkgName] = append(othersByPackage[pkgName], o)
	}

	if err := w.MkdirAll(rootDir, 0755); err != nil {
		return err
	}

	for _, p := range cfg.Packages {
		fcfg := DeclarativeConfig{
			Packages: []pkg{p},
			Bundles:  bundlesByPackage[p.Name],
			others:   othersByPackage[p.Name],
		}
		filename := filepath.Join(rootDir, fmt.Sprintf("%s.json", p.Name))

		buf := &bytes.Buffer{}
		if err := writeJSON(fcfg, buf); err != nil {
			return fmt.Errorf("write to buffer for %q: %v", filename, err)
		}
		if err := w.WriteFile(filename, buf.Bytes(), 0644); err != nil {
			return err
		}
	}

	if globals, ok := othersByPackage[globalName]; ok {
		gcfg := DeclarativeConfig{
			others: globals,
		}
		filename := filepath.Join(rootDir, fmt.Sprintf("%s.json", globalName))
		buf := &bytes.Buffer{}
		if err := writeJSON(gcfg, buf); err != nil {
			return fmt.Errorf("write to buffer for %q: %v", filename, err)
		}
		if err := w.WriteFile(filename, buf.Bytes(), 0644); err != nil {
			return err
		}
	}
	return nil
}

func writeJSON(cfg DeclarativeConfig, w io.Writer) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "    ")
	enc.SetEscapeHTML(false)

	bundlesByPackage := map[string][]bundle{}
	for _, b := range cfg.Bundles {
		pkgName := b.Package
		bundlesByPackage[pkgName] = append(bundlesByPackage[pkgName], b)
	}

	for _, p := range cfg.Packages {
		if err := enc.Encode(p); err != nil {
			return err
		}
		for _, b := range bundlesByPackage[p.Name] {
			if err := enc.Encode(b); err != nil {
				return err
			}
		}
	}
	for _, o := range cfg.others {
		if err := enc.Encode(o); err != nil {
			return err
		}
	}
	return nil
}
