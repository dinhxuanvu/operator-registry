package model

import (
	"context"
	"fmt"
	"sort"

	"github.com/operator-framework/operator-registry/pkg/api"
	"github.com/operator-framework/operator-registry/pkg/registry"
)

type Querier struct {
	model Model
}

var _ registry.GRPCQuery = &Querier{}

func NewQuerier(packages Model) *Querier {
	return &Querier{
		packages,
	}
}

func (q Querier) ListPackages(ctx context.Context) ([]string, error) {
	var packages []string
	for pkgName := range q.model {
		packages = append(packages, pkgName)
	}
	return packages, nil
}

func (q Querier) ListBundles(ctx context.Context) ([]*api.Bundle, error) {
	var bundles []*api.Bundle

	for _, pkg := range q.model {
		for _, ch := range pkg.Channels {
			for _, b := range ch.Bundles {
				bundles = append(bundles, b.ConvertToAPI())
			}
		}
	}
	return bundles, nil
}

func (q Querier) GetPackage(ctx context.Context, name string) (*registry.PackageManifest, error) {
	pkg, ok := q.model[name]
	if !ok {
		return nil, fmt.Errorf("package %q not found", name)
	}

	var channels []registry.PackageChannel
	for _, ch := range pkg.Channels {
		channels = append(channels, registry.PackageChannel{
			Name:           ch.Name,
			CurrentCSVName: ch.Head.Name,
		})
	}
	return &registry.PackageManifest{
		PackageName:        pkg.Name,
		Channels:           channels,
		DefaultChannelName: pkg.DefaultChannel.Name,
	}, nil
}

func (q Querier) GetBundle(ctx context.Context, pkgName, channelName, csvName string) (*api.Bundle, error) {
	pkg, ok := q.model[pkgName]
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
	return b.ConvertToAPI(), nil
}

func (q Querier) GetBundleForChannel(ctx context.Context, pkgName string, channelName string) (*api.Bundle, error) {
	pkg, ok := q.model[pkgName]
	if !ok {
		return nil, fmt.Errorf("package %q not found", pkgName)
	}
	ch, ok := pkg.Channels[channelName]
	if !ok {
		return nil, fmt.Errorf("package %q, channel %q not found", pkgName, channelName)
	}
	return ch.Head.ConvertToAPI(), nil
}

func (q Querier) GetChannelEntriesThatReplace(ctx context.Context, name string) ([]*registry.ChannelEntry, error) {
	var entries []*registry.ChannelEntry

	for _, pkg := range q.model {
		for _, ch := range pkg.Channels {
			for _, b := range ch.Bundles {
				if b.Replaces == name {
					entries = append(entries, &registry.ChannelEntry{
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

func (q Querier) GetBundleThatReplaces(ctx context.Context, name, pkgName, channelName string) (*api.Bundle, error) {
	pkg, ok := q.model[pkgName]
	if !ok {
		return nil, fmt.Errorf("package %s not found", pkgName)
	}
	ch, ok := pkg.Channels[channelName]
	if !ok {
		return nil, fmt.Errorf("package %q, channel %q not found", pkgName, channelName)
	}
	for _, b := range ch.Bundles {
		if b.Replaces == name {
			return b.ConvertToAPI(), nil
		}
	}
	return nil, fmt.Errorf("no entry found for package %q, channel %q", pkgName, channelName)
}

func (q Querier) GetChannelEntriesThatProvide(ctx context.Context, group, version, kind string) ([]*registry.ChannelEntry, error) {
	var entries []*registry.ChannelEntry

	for _, pkg := range q.model {
		for _, ch := range pkg.Channels {
			for _, b := range ch.Bundles {
				if b.Provides(group, version, kind) {
					entries = append(entries, &registry.ChannelEntry{
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

func (q Querier) GetLatestChannelEntriesThatProvide(ctx context.Context, group, version, kind string) ([]*registry.ChannelEntry, error) {
	var entries []*registry.ChannelEntry

	for _, pkg := range q.model {
		for _, ch := range pkg.Channels {
			b := ch.Head
			for b != nil {
				if b.Provides(group, version, kind) {
					entries = append(entries, &registry.ChannelEntry{
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
		pkg, ok := q.model[entry.PackageName]
		if !ok {
			// This should never happen because the latest entries were
			// collected based on iterating over the packages in q.model.
			continue
		}
		if entry.ChannelName == pkg.DefaultChannel.Name {
			return q.GetBundle(ctx, entry.PackageName, entry.ChannelName, entry.BundleName)
		}
	}
	return nil, fmt.Errorf("no entry found that provides group:%q version:%q kind:%q", group, version, kind)
}
