package registry

import (
	"context"
	"fmt"
	"io/ioutil"
	"path"
	"path/filepath"
	"sort"

	"github.com/operator-framework/api/pkg/operators"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/yaml"

	"github.com/operator-framework/operator-registry/pkg/api"
	"github.com/operator-framework/operator-registry/pkg/model"
)

type Querier struct {
	pkgs    model.Model
	objects map[string]bundleObjects
}

type bundleObjects struct {
	csv     string
	objects []string
}

var _ GRPCQuery = &Querier{}

func NewQuerier(packages model.Model) *Querier {
	return &Querier{
		pkgs:    packages,
		objects: map[string]bundleObjects{},
	}
}

func (q *Querier) LoadBundleObjects(dir string) error {
	for _, pkg := range q.pkgs {
		pkgDir := filepath.Join(dir, pkg.Name)
		for _, ch := range pkg.Channels {
			for _, b := range ch.Bundles {
				if _, ok := q.objects[b.Name]; ok {
					continue
				}
				bundleDir := filepath.Join(pkgDir, b.Name)
				bObjs, err := readBundleDirectory(bundleDir)
				if err != nil {
					return err
				}
				q.objects[b.Name] = *bObjs
			}
		}
	}
	return nil
}

func (q Querier) ListPackages(_ context.Context) ([]string, error) {
	var packages []string
	for pkgName := range q.pkgs {
		packages = append(packages, pkgName)
	}
	return packages, nil
}

func (q Querier) ListBundles(_ context.Context) ([]*api.Bundle, error) {
	var bundles []*api.Bundle

	for _, pkg := range q.pkgs {
		for _, ch := range pkg.Channels {
			for _, b := range ch.Bundles {
				apiBundle, err := api.ConvertModelBundleToAPIBundle(*b)
				if err != nil {
					return nil, fmt.Errorf("convert bundle %q: %v", b.Name, err)
				}
				apiBundle.CsvJson = q.objects[b.Name].csv
				apiBundle.Object = q.objects[b.Name].objects
				bundles = append(bundles, apiBundle)
			}
		}
	}
	return bundles, nil
}

func (q Querier) GetPackage(_ context.Context, name string) (*PackageManifest, error) {
	pkg, ok := q.pkgs[name]
	if !ok {
		return nil, fmt.Errorf("package %q not found", name)
	}

	var channels []PackageChannel
	for _, ch := range pkg.Channels {
		head, err := ch.Head()
		if err != nil {
			return nil, fmt.Errorf("package %q, channel %q has invalid head: %v", name, ch.Name, err)
		}
		channels = append(channels, PackageChannel{
			Name:           ch.Name,
			CurrentCSVName: head.Name,
		})
	}
	return &PackageManifest{
		PackageName:        pkg.Name,
		Channels:           channels,
		DefaultChannelName: pkg.DefaultChannel.Name,
	}, nil
}

func (q Querier) GetBundle(_ context.Context, pkgName, channelName, csvName string) (*api.Bundle, error) {
	pkg, ok := q.pkgs[pkgName]
	if !ok {
		return nil, fmt.Errorf("package %q not found", pkgName)
	}
	ch, ok := pkg.Channels[channelName]
	if !ok {
		return nil, fmt.Errorf("package %q, channel %q not found", pkgName, channelName)
	}
	b, ok := ch.Bundles[csvName]
	if !ok {
		return nil, fmt.Errorf("package %q, channel %q, bundle %q not found", pkgName, channelName, csvName)
	}
	apiBundle, err := api.ConvertModelBundleToAPIBundle(*b)
	if err != nil {
		return nil, fmt.Errorf("convert bundle %q: %v", b.Name, err)
	}
	apiBundle.CsvJson = q.objects[b.Name].csv
	apiBundle.Object = q.objects[b.Name].objects

	// unset Replaces and Skips (sqlite query does not populate these fields)
	// TODO(joelanford): should these fields be populated?
	apiBundle.Replaces = ""
	apiBundle.Skips = nil
	return apiBundle, nil
}

func (q Querier) GetBundleForChannel(_ context.Context, pkgName string, channelName string) (*api.Bundle, error) {
	pkg, ok := q.pkgs[pkgName]
	if !ok {
		return nil, fmt.Errorf("package %q not found", pkgName)
	}
	ch, ok := pkg.Channels[channelName]
	if !ok {
		return nil, fmt.Errorf("package %q, channel %q not found", pkgName, channelName)
	}
	head, err := ch.Head()
	if err != nil {
		return nil, fmt.Errorf("package %q, channel %q has invalid head: %v", pkgName, channelName, err)
	}
	apiBundle, err := api.ConvertModelBundleToAPIBundle(*head)
	if err != nil {
		return nil, fmt.Errorf("convert bundle %q: %v", head.Name, err)
	}
	apiBundle.CsvJson = q.objects[head.Name].csv
	apiBundle.Object = q.objects[head.Name].objects

	// unset Replaces and Skips (sqlite query does not populate these fields)
	// TODO(joelanford): should these fields be populated?
	apiBundle.Replaces = ""
	apiBundle.Skips = nil
	return apiBundle, nil
}

func (q Querier) GetChannelEntriesThatReplace(_ context.Context, name string) ([]*ChannelEntry, error) {
	var entries []*ChannelEntry

	for _, pkg := range q.pkgs {
		for _, ch := range pkg.Channels {
			for _, b := range ch.Bundles {
				entries = append(entries, channelEntriesThatReplace(*b, name)...)
			}
		}
	}
	if len(entries) == 0 {
		return nil, fmt.Errorf("no channel entries found that replace %s", name)
	}
	return entries, nil
}

// TODO(joelanford): What if multiple bundles replace this one?
func (q Querier) GetBundleThatReplaces(_ context.Context, name, pkgName, channelName string) (*api.Bundle, error) {
	pkg, ok := q.pkgs[pkgName]
	if !ok {
		return nil, fmt.Errorf("package %s not found", pkgName)
	}
	ch, ok := pkg.Channels[channelName]
	if !ok {
		return nil, fmt.Errorf("package %q, channel %q not found", pkgName, channelName)
	}
	for _, b := range ch.Bundles {
		if bundleReplaces(*b, name) {
			apiBundle, err := api.ConvertModelBundleToAPIBundle(*b)
			if err != nil {
				return nil, fmt.Errorf("convert bundle %q: %v", b.Name, err)
			}
			apiBundle.CsvJson = q.objects[b.Name].csv
			apiBundle.Object = q.objects[b.Name].objects

			// unset Replaces and Skips (sqlite query does not populate these fields)
			// TODO(joelanford): should these fields be populated?
			apiBundle.Replaces = ""
			apiBundle.Skips = nil
			return apiBundle, nil
		}
	}
	return nil, fmt.Errorf("no entry found for package %q, channel %q", pkgName, channelName)
}

func (q Querier) GetChannelEntriesThatProvide(_ context.Context, group, version, kind string) ([]*ChannelEntry, error) {
	var entries []*ChannelEntry

	for _, pkg := range q.pkgs {
		for _, ch := range pkg.Channels {
			for _, b := range ch.Bundles {
				provides, err := doesModelBundleProvide(*b, group, version, kind)
				if err != nil {
					return nil, err
				}
				if provides {
					entries = append(entries, channelEntriesForBundle(*b)...)
				}
			}
		}
	}
	if len(entries) == 0 {
		return nil, fmt.Errorf("no channel entries found that provide group:%q version:%q kind:%q", group, version, kind)
	}
	return entries, nil
}

// TODO(joelanford): Need to review the expected functionality of this function. I ran
//   some experiments with the sqlite version of this function and it seems to only return
//   channel heads that provide the GVK (rather than searching down the graph if parent bundles
//   don't provide the API). Based on that, this function currently looks at channel heads only.
//   ---
//   Separate, but possibly related, I noticed there are several channels in the channel entry
//   table who's minimum depth is 1. What causes 1 to be minimum depth in some cases and 0 in others?
func (q Querier) GetLatestChannelEntriesThatProvide(_ context.Context, group, version, kind string) ([]*ChannelEntry, error) {
	var entries []*ChannelEntry

	for _, pkg := range q.pkgs {
		for _, ch := range pkg.Channels {
			b, err := ch.Head()
			if err != nil {
				return nil, fmt.Errorf("package %q, channel %q has invalid head: %v", pkg.Name, ch.Name, err)
			}

			provides, err := doesModelBundleProvide(*b, group, version, kind)
			if err != nil {
				return nil, err
			}
			if provides {
				entries = append(entries, latestChannelEntriesForBundle(*b)...)
			}
		}
	}
	if len(entries) == 0 {
		return nil, fmt.Errorf("no channel entries found that provide group:%q version:%q kind:%q", group, version, kind)
	}
	return entries, nil
}

func (q Querier) GetBundleThatProvides(ctx context.Context, group, version, kind string) (*api.Bundle, error) {
	latestEntries, err := q.GetLatestChannelEntriesThatProvide(ctx, group, version, kind)
	if err != nil {
		return nil, err
	}

	// It's possible for multiple packages to provide an API, but this function is forced to choose one.
	// To do that deterministically, we'll pick the the bundle based on a lexicographical sort of its
	// package name.
	sort.Slice(latestEntries, func(i, j int) bool {
		return latestEntries[i].PackageName < latestEntries[j].PackageName
	})

	for _, entry := range latestEntries {
		pkg, ok := q.pkgs[entry.PackageName]
		if !ok {
			// This should never happen because the latest entries were
			// collected based on iterating over the packages in q.pkgs.
			continue
		}
		if entry.ChannelName == pkg.DefaultChannel.Name {
			return q.GetBundle(ctx, entry.PackageName, entry.ChannelName, entry.BundleName)
		}
	}
	return nil, fmt.Errorf("no entry found that provides group:%q version:%q kind:%q", group, version, kind)
}

func doesModelBundleProvide(b model.Bundle, group, version, kind string) (bool, error) {
	apiBundle, err := api.ConvertModelBundleToAPIBundle(b)
	if err != nil {
		return false, fmt.Errorf("convert bundle %q: %v", b.Name, err)
	}
	for _, gvk := range apiBundle.ProvidedApis {
		if gvk.Group == group && gvk.Version == version && gvk.Kind == kind {
			return true, nil
		}
	}
	return false, nil
}

func bundleReplaces(b model.Bundle, name string) bool {
	if b.Replaces == name {
		return true
	}
	for _, s := range b.Skips {
		if s == name {
			return true
		}
	}
	return false
}

func channelEntriesThatReplace(b model.Bundle, name string) []*ChannelEntry {
	var entries []*ChannelEntry
	if b.Replaces == name {
		entries = append(entries, &ChannelEntry{
			PackageName: b.Package.Name,
			ChannelName: b.Channel.Name,
			BundleName:  b.Name,
			Replaces:    b.Replaces,
		})
	}
	for _, s := range b.Skips {
		if s == name && s != b.Replaces {
			entries = append(entries, &ChannelEntry{
				PackageName: b.Package.Name,
				ChannelName: b.Channel.Name,
				BundleName:  b.Name,
				Replaces:    b.Replaces,
			})
		}
	}
	return entries
}

func latestChannelEntriesForBundle(b model.Bundle) []*ChannelEntry {
	entries := []*ChannelEntry{{
		PackageName: b.Package.Name,
		ChannelName: b.Channel.Name,
		BundleName:  b.Name,
		Replaces:    b.Replaces,
	}}
	for _, s := range b.Skips {
		// If the skipped bundle is in this channel AND it isn't what
		// this bundle replaces add a channel entry for it.
		if _, ok := b.Channel.Bundles[s]; ok && s != b.Replaces {
			entries = append(entries, &ChannelEntry{
				PackageName: b.Package.Name,
				ChannelName: b.Channel.Name,
				BundleName:  b.Name,
				Replaces:    s,
			})
		}
	}
	return entries
}

func channelEntriesForBundle(b model.Bundle) []*ChannelEntry {
	entries := []*ChannelEntry{{
		PackageName: b.Package.Name,
		ChannelName: b.Channel.Name,
		BundleName:  b.Name,
		Replaces:    b.Replaces,
	}}
	for _, s := range b.Skips {
		// If the skipped bundle isn't what this bundle replaces add a
		// channel entry for it.
		//
		// TODO(joelanford): It seems like the SQLite query returns
		//   invalid entries (i.e. where bundle `Replaces` isn't actually
		//   in channel `ChannelName`). Is that a bug? For now, this mimics
		//   the sqlite server and returns seemingly invalid channel entries.
		if s != b.Replaces {
			entries = append(entries, &ChannelEntry{
				PackageName: b.Package.Name,
				ChannelName: b.Channel.Name,
				BundleName:  b.Name,
				Replaces:    s,
			})
		}
	}
	return entries
}

func readBundleDirectory(dir string) (*bundleObjects, error) {
	infos, err := ioutil.ReadDir(dir)
	if err != nil {
		return &bundleObjects{}, nil
	}

	bObjs := bundleObjects{}
	for _, info := range infos {
		if info.IsDir() {
			continue
		}
		filedata, err := ioutil.ReadFile(path.Join(dir, info.Name()))
		if err != nil {
			continue
		}

		u := unstructured.Unstructured{}
		if err := yaml.Unmarshal(filedata, &u); err != nil {
			continue
		}

		bObjs.objects = append(bObjs.objects, string(filedata))
		if u.GetKind() != operators.ClusterServiceVersionKind {
			continue
		}

		if len(bObjs.csv) > 0 {
			return nil, fmt.Errorf("more than one ClusterServiceVersion is found in bundle")
		}
		bObjs.csv = string(filedata)
	}
	return &bObjs, nil
}
