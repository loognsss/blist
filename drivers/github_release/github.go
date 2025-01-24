package template

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/alist-org/alist/v3/internal/model"
	"github.com/alist-org/alist/v3/pkg/utils"
	"github.com/pkg/errors"
)

const GITHUB_API_VERSION = "2022-11-28"

type ApiContext struct {
	token   string
	version string
	client  *http.Client
}

func NewApiContext(token string, client *http.Client) *ApiContext {
	ret := ApiContext{
		token:   token,
		version: GITHUB_API_VERSION,
		client:  client,
	}

	if ret.client == nil {
		ret.client = http.DefaultClient
	}

	return &ret
}

// parseHTTPError 解析 HTTP 错误.
func parseHTTPError(body []byte) error {
	var v map[string]interface{}
	err := utils.Json.Unmarshal(body, &v)
	if err != nil {
		return errors.New(string(body))
	}

	iface, ok := v["message"]
	if !ok {
		return errors.New(string(body))
	}

	message, ok := iface.(string)
	if !ok {
		return errors.New(string(body))
	}

	return errors.New(message)
}

// sleepWithContext 在指定的时间内等待, 如果 context 被取消则提前返回.
func sleepWithContext(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

// getWithRetry 获取 GitHub API 并重试.
func (a *ApiContext) getWithRetry(ctx context.Context, url string) (*http.Response, error) {
	backoff := Backoff{}

	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		response, err := a.get(ctx, url)

		// non-2xx code does not cause error
		if err != nil {
			// retry when error is not nil
			p, retryAgain := backoff.Pause()
			if !retryAgain {
				return nil, errors.Wrap(err, "request failed")
			}
			utils.Log.Debugf("query github api error: %s, retry after %s", err, p)

			if err := sleepWithContext(ctx, p); err != nil {
				return nil, err
			}
			continue
		}

		// defensive check
		if response == nil {
			utils.Log.Errorf("query github api error: %s, will not retry", err)
			return nil, errors.New("request failed: response is nil")
		}

		if response.StatusCode == http.StatusOK {
			return response, nil
		}

		// We won't return the response to the caller here, but it's still better to read the response.Body completely even if we don't use it.
		// see https://pkg.go.dev/net/http#Client.Do
		body, err := io.ReadAll(response.Body)
		if err != nil {
			return nil, errors.Wrap(err, "failed to read response body")
		}

		if response.StatusCode >= 500 && response.StatusCode <= 599 {
			// retry when server error
			p, retryAgain := backoff.Pause()
			if !retryAgain {
				return nil, parseHTTPError(body)
			}
			utils.Log.Debugf("query github api error: status code %d, retry after %s", response.StatusCode, p)

			if err := sleepWithContext(ctx, p); err != nil {
				return nil, err
			}
			continue
		}

		return nil, parseHTTPError(body)
	}
}

// SetAuthHeader 为请求头添加 GitHub API 所需的认证头.
// 这是一个副作用函数, 会直接修改传入的 header.
func (a *ApiContext) SetAuthHeader(header http.Header) {
	header.Set("Authorization", fmt.Sprintf("Bearer %s", a.token))
}

// get 获取 GitHub API.
func (a *ApiContext) get(ctx context.Context, url string) (*http.Response, error) {
	request, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	request.Header.Set("Accept", "application/vnd.github+json")
	a.SetAuthHeader(request.Header)

	response, err := a.client.Do(request)
	if err != nil {
		return nil, err
	}

	return response, nil
}

// GetReleases 获取仓库信息.
func (a *ApiContext) GetReleases(ctx context.Context, repo repository, perPage int) ([]model.Obj, error) {
	if perPage < 1 {
		perPage = 30
	}
	url := fmt.Sprintf("https://api.github.com/repos/%s/releases?per_page=%d", repo.UrlEncode(), perPage)
	response, err := a.getWithRetry(ctx, url)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	body, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, errors.Wrap(err, "failed to read response body")
	}

	if response.StatusCode != http.StatusOK {
		return nil, parseHTTPError(body)
	}

	releases := []Release{}
	err = utils.Json.Unmarshal(body, &releases)
	if err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal releases")
	}

	tree := make([]model.Obj, 0, len(releases))
	for _, release := range releases {
		tree = append(tree, &release)
	}
	return tree, nil
}

// GetRelease 获取指定 tag 的 release.
func (a *ApiContext) GetRelease(ctx context.Context, repo repository, id int64) (*Release, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/releases/%d", repo.UrlEncode(), id)
	response, err := a.getWithRetry(ctx, url)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	body, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, errors.Wrap(err, "failed to read response body")
	}

	if response.StatusCode != http.StatusOK {
		return nil, parseHTTPError(body)
	}

	release := Release{}
	err = utils.Json.Unmarshal(body, &release)
	if err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal release")
	}

	return &release, nil
}

// GetReleaseAsset 获取指定 tag 的 release 的 assets.
func (a *ApiContext) GetReleaseAsset(ctx context.Context, repo repository, ID int64) (*Asset, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/releases/assets/%d", repo.UrlEncode(), ID)
	response, err := a.getWithRetry(ctx, url)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	body, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, errors.Wrap(err, "failed to read response body")
	}

	if response.StatusCode != http.StatusOK {
		return nil, parseHTTPError(body)
	}

	asset := Asset{}
	err = utils.Json.Unmarshal(body, &asset)
	if err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal asset")
	}

	return &asset, nil
}

var (
	ErrNoRelease = errors.New("no release found")
)

// GetLatestRelease 获取最新 release.
func (a *ApiContext) GetLatestRelease(ctx context.Context, repo repository) (model.Obj, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/releases/latest", repo.UrlEncode())
	response, err := a.getWithRetry(ctx, url)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	body, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, errors.Wrap(err, "get latest release failed")
	}

	if response.StatusCode != http.StatusOK {
		if response.StatusCode == http.StatusNotFound {
			// identify no release
			return nil, ErrNoRelease
		}
		return nil, parseHTTPError(body)
	}

	release := Release{}
	err = utils.Json.Unmarshal(body, &release)
	if err != nil {
		return nil, errors.Wrap(err, "get latest release failed")
	}

	release.SetLatestFlag(true)

	return &release, nil
}
