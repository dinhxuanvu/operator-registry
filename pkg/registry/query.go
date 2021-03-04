package registry

import (
	"context"
	"fmt"
	"sort"

	"github.com/operator-framework/operator-registry/pkg/api"
	"github.com/operator-framework/operator-registry/pkg/model"
)

type Querier struct {
	pkgs model.Model
}

var _ GRPCQuery = &Querier{}

func NewQuerier(packages model.Model) *Querier {
	return &Querier{
		packages,
	}
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
				bundles = append(bundles, api.BundleFromModel(*b))
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
	return api.BundleFromModel(*b), nil
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
	return api.BundleFromModel(*head), nil
}

func (q Querier) GetChannelEntriesThatReplace(_ context.Context, name string) ([]*ChannelEntry, error) {
	var entries []*ChannelEntry

	for _, pkg := range q.pkgs {
		for _, ch := range pkg.Channels {
			for _, b := range ch.Bundles {
				if b.Replaces == name {
					entries = append(entries, &ChannelEntry{
						PackageName: b.Package.Name,
						ChannelName: b.Channel.Name,
						BundleName:  b.Name,
						Replaces:    b.Replaces,
					})
				}
			}
		}
	}
	if len(entries) == 0 {
		return nil, fmt.Errorf("no channel entries found that replace %s", name)
	}
	return entries, nil
}

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
		if b.Replaces == name {
			return api.BundleFromModel(*b), nil
		}
	}
	return nil, fmt.Errorf("no entry found for package %q, channel %q", pkgName, channelName)
}

func (q Querier) GetChannelEntriesThatProvide(_ context.Context, group, version, kind string) ([]*ChannelEntry, error) {
	var entries []*ChannelEntry

	for _, pkg := range q.pkgs {
		for _, ch := range pkg.Channels {
			for _, b := range ch.Bundles {
				if b.Provides(group, version, kind) {
					entries = append(entries, &ChannelEntry{
						PackageName: b.Package.Name,
						ChannelName: b.Channel.Name,
						BundleName:  b.Name,
						Replaces:    b.Replaces,
					})
				}
			}
		}
	}
	if len(entries) == 0 {
		return nil, fmt.Errorf("no channel entries found that provide group:%q version:%q kind:%q", group, version, kind)
	}
	return entries, nil
}

func (q Querier) GetLatestChannelEntriesThatProvide(_ context.Context, group, version, kind string) ([]*ChannelEntry, error) {
	var entries []*ChannelEntry

	for _, pkg := range q.pkgs {
		for _, ch := range pkg.Channels {
			b, err := ch.Head()
			if err != nil {
				return nil, fmt.Errorf("package %q, channel %q has invalid head: %v", pkg.Name, ch.Name, err)
			}
			for b != nil {
				if b.Provides(group, version, kind) {
					entries = append(entries, &ChannelEntry{
						PackageName: b.Package.Name,
						ChannelName: b.Channel.Name,
						BundleName:  b.Name,
						Replaces:    b.Replaces,
					})
					break
				}
				if b.Replaces == "" {
					break
				}
				b = ch.Bundles[b.Replaces]
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
