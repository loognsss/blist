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
)

type GithubRelease struct {
	model.Storage
	Addition

	api  *ApiContext
	repo repository
}

func (d *GithubRelease) Config() driver.Config {
	return config
}

func (d *GithubRelease) GetAddition() driver.Additional {
	return &d.Addition
}

func (d *GithubRelease) Init(ctx context.Context) error {
	token := d.Addition.Token
	if token == "" {
		return errs.EmptyToken
	}

	if d.Addition.MaxReleases < 1 {
		return errors.New("max_releases must be greater than 0")
	}

	if d.Addition.MaxReleases > 100 {
		d.Addition.MaxReleases = 100
	}

	d.api = NewApiContext(token, nil)

	repo, err := newRepository(d.Addition.Repo)
	if err != nil {
		return err
	}
	d.repo = repo

	return nil
}

// Drop Delete this driver
func (d *GithubRelease) Drop(ctx context.Context) error {
	return nil
}

func (d *GithubRelease) List(ctx context.Context, dir model.Obj, args model.ListArgs) ([]model.Obj, error) {
	repo, err := newRepository(d.Addition.Repo)
	if err != nil {
		return nil, err
	}

	// 判断 dir 是不是挂在点。如果 dir 是挂载点，则返回所有的 release；
	// 如果 dir 不是挂载点，则返回 dir 下的 release。
	if dir.GetPath() == "" {
		releases, err := d.api.GetReleases(repo, d.Addition.MaxReleases)
		if err != nil {
			return nil, err
		}
		return releases, nil
	}

	idStr := dir.GetID()
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		return nil, errors.Wrapf(err, "list release %s failed, id is not a number", idStr)
	}

	release, err := d.api.GetRelease(repo, id)
	if err != nil {
		return nil, err
	}

	return release.Children()
}

func (d *GithubRelease) proxyDownload(file model.Obj, args model.LinkArgs) bool {
	if d.Config().MustProxy() || d.GetStorage().WebProxy {
		return true
	}

	req := args.HttpReq
	if args.HttpReq != nil &&
		req.URL != nil &&
		strings.HasPrefix(req.URL.Path, fmt.Sprintf("/p%s", d.GetStorage().MountPath)) {
		return true
	}

	return false
}

func (d *GithubRelease) Link(ctx context.Context, file model.Obj, args model.LinkArgs) (*model.Link, error) {
	idStr := file.GetID()
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		return nil, errors.Wrapf(err, "get link of file %s failed, id is not a number", idStr)
	}
	asset, err := d.api.GetReleaseAsset(d.repo, id)
	if err != nil {
		return nil, err
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

func (d *GithubRelease) MakeDir(ctx context.Context, parentDir model.Obj, dirName string) (model.Obj, error) {
	return nil, errs.NotSupport
}

func (d *GithubRelease) Move(ctx context.Context, srcObj, dstDir model.Obj) (model.Obj, error) {
	return nil, errs.NotSupport
}

func (d *GithubRelease) Rename(ctx context.Context, srcObj model.Obj, newName string) (model.Obj, error) {
	// TODO rename obj, optional
	return nil, errs.NotImplement
}

func (d *GithubRelease) Copy(ctx context.Context, srcObj, dstDir model.Obj) (model.Obj, error) {
	return nil, errs.NotSupport
}

func (d *GithubRelease) Remove(ctx context.Context, obj model.Obj) error {
	return errs.NotSupport
}

func (d *GithubRelease) Put(ctx context.Context, dstDir model.Obj, stream model.FileStreamer, up driver.UpdateProgress) (model.Obj, error) {
	return nil, errs.NotSupport
}

//func (d *Template) Other(ctx context.Context, args model.OtherArgs) (interface{}, error) {
//	return nil, errs.NotSupport
//}

var _ driver.Driver = (*GithubRelease)(nil)
