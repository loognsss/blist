package template

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/alist-org/alist/v3/internal/conf"
	"github.com/alist-org/alist/v3/internal/driver"
	"github.com/alist-org/alist/v3/internal/errs"
	"github.com/alist-org/alist/v3/internal/model"
	"github.com/pkg/errors"
	"golang.org/x/sync/errgroup"
)

// GithubRelease implements a driver for GitHub Release
type GithubRelease struct {
	model.Storage
	Addition

	api  *ApiContext
	repo repository
}

// Config returns the driver config
func (d *GithubRelease) Config() driver.Config {
	return config
}

func (d *GithubRelease) GetAddition() driver.Additional {
	return &d.Addition
}

// validate checks if the driver configuration is valid
func (d *GithubRelease) validate() error {
	if d.Addition.Token == "" {
		return errs.EmptyToken
	}

	if d.Addition.MaxReleases < 1 {
		return errors.New("max_releases must be greater than 0")
	}

	if d.Addition.MaxReleases > 100 {
		d.Addition.MaxReleases = 100
	}

	return nil
}

// Init initializes the driver
func (d *GithubRelease) Init(ctx context.Context) error {
	if err := d.validate(); err != nil {
		return err
	}

	d.api = NewApiContext(d.Addition.Token, nil)

	repo, err := newRepository(d.Addition.Repo)
	if err != nil {
		return errors.Wrap(err, "failed to create repository")
	}
	d.repo = repo

	return nil
}

// Drop deletes this driver
func (d *GithubRelease) Drop(ctx context.Context) error {
	return nil
}

// listReleases gets all releases
func (d *GithubRelease) listReleases(ctx context.Context, dir model.Obj, args model.ListArgs) ([]model.Obj, error) {
	g, ctx := errgroup.WithContext(ctx)

	var releases []model.Obj
	var latest model.Obj

	// Get latest release if enabled
	if d.Addition.ShowLatest {
		g.Go(func() error {
			release, err := d.api.GetLatestRelease(ctx, d.repo)
			if err != nil {
				if err == ErrNoRelease {
					// for no release, just return
					return nil
				}
				return errors.Wrap(err, "failed to get latest release")
			}
			latest = release
			return nil
		})
	}

	// Get all releases
	g.Go(func() error {
		r, err := d.api.GetReleases(ctx, d.repo, d.Addition.MaxReleases)
		if err != nil {
			return errors.Wrap(err, "failed to get releases")
		}
		releases = r
		return nil
	})

	// Wait for all goroutines to complete
	if err := g.Wait(); err != nil {
		return nil, err
	}

	// Add latest release to the top if available
	if latest != nil && releases != nil {
		releases = append([]model.Obj{latest}, releases...)
	}

	return releases, nil
}

func (d *GithubRelease) listReleaseAssets(ctx context.Context, dir model.Obj, args model.ListArgs) ([]model.Obj, error) {
	idStr := dir.GetID()
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		return nil, errors.Wrapf(err, "list release %s failed, id is not a number", idStr)
	}
	release, err := d.api.GetRelease(ctx, d.repo, id)
	if err != nil {
		return nil, err
	}
	return release.Children()
}

// List returns the objects in the given directory
func (d *GithubRelease) List(ctx context.Context, dir model.Obj, args model.ListArgs) ([]model.Obj, error) {
	// If dir is root, return all releases
	if dir.GetPath() == "" {
		return d.listReleases(ctx, dir, args)
	}

	// Otherwise return release assets
	idStr := dir.GetID()
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		return nil, errors.Wrapf(err, "list release %s failed, id is not a number", idStr)
	}

	release, err := d.api.GetRelease(ctx, d.repo, id)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get release")
	}

	return release.Children()
}

// proxyDownload checks if download should be proxied
func (d *GithubRelease) proxyDownload(file model.Obj, args model.LinkArgs) bool {
	// Must proxy if configured
	if d.Config().MustProxy() || d.GetStorage().WebProxy {
		return true
	}

	// Check if request path indicates proxy is needed
	if req := args.HttpReq; req != nil && req.URL != nil {
		proxyPath := fmt.Sprintf("/p%s", d.GetStorage().MountPath)
		return strings.HasPrefix(req.URL.Path, proxyPath)
	}

	return false
}

// Link returns the download link for a file
func (d *GithubRelease) Link(ctx context.Context, file model.Obj, args model.LinkArgs) (*model.Link, error) {
	idStr := file.GetID()
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		return nil, errors.Wrapf(err, "get link of file %s failed, id is not a number", idStr)
	}

	asset, err := d.api.GetReleaseAsset(ctx, d.repo, id)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get release asset")
	}

	if d.proxyDownload(file, args) {
		header := http.Header{
			"User-Agent": {"Alist/" + conf.VERSION},
			"Accept":     {"application/octet-stream"},
		}
		d.api.SetAuthHeader(header)

		return &model.Link{
			URL:    asset.URL,
			Header: header,
		}, nil
	}

	return &model.Link{
		URL: asset.BrowserDownloadURL,
	}, nil
}

// MakeDir is not supported
func (d *GithubRelease) MakeDir(ctx context.Context, parentDir model.Obj, dirName string) (model.Obj, error) {
	return nil, errs.NotSupport
}

// Move is not supported
func (d *GithubRelease) Move(ctx context.Context, srcObj, dstDir model.Obj) (model.Obj, error) {
	return nil, errs.NotSupport
}

// Rename is not supported
func (d *GithubRelease) Rename(ctx context.Context, srcObj model.Obj, newName string) (model.Obj, error) {
	return nil, errs.NotSupport
}

// Copy is not supported
func (d *GithubRelease) Copy(ctx context.Context, srcObj, dstDir model.Obj) (model.Obj, error) {
	return nil, errs.NotSupport
}

// Remove is not supported
func (d *GithubRelease) Remove(ctx context.Context, obj model.Obj) error {
	return errs.NotSupport
}

// Put is not supported
func (d *GithubRelease) Put(ctx context.Context, dstDir model.Obj, stream model.FileStreamer, up driver.UpdateProgress) (model.Obj, error) {
	return nil, errs.NotSupport
}

// Other implements custom operations
func (d *GithubRelease) Other(ctx context.Context, args model.OtherArgs) (interface{}, error) {
	return nil, errs.NotSupport
}

var _ driver.Driver = (*GithubRelease)(nil)
