package github_release

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/alist-org/alist/v3/internal/model"
	"github.com/alist-org/alist/v3/pkg/utils"
	"github.com/pkg/errors"
)

const (
	GITHUB_API_VERSION = "2022-11-28"
	DEFAULT_TIMEOUT    = 10 * time.Second
)

var ErrRateLimitExceeded = errors.New("rate limit exceeded")

// RateLimit 表示 GitHub API 的速率限制信息
type RateLimit struct {
	Limit     uint
	Remaining uint
	Reset     time.Time
}

// GitHubError 表示 GitHub API 返回的错误信息
type GitHubError struct {
	Message          string `json:"message"`
	DocumentationURL string `json:"documentation_url"`
	StatusCode       int
}

func (e *GitHubError) Error() string {
	return fmt.Sprintf("github api error: %s (status: %d)", e.Message, e.StatusCode)
}

// parseHTTPError 解析 GitHub API 的错误响应
func parseHTTPError(statusCode int, body []byte) error {
	var v GitHubError
	err := utils.Json.Unmarshal(body, &v)
	if err != nil {
		return &GitHubError{
			Message:    string(body),
			StatusCode: statusCode,
		}
	}
	v.StatusCode = statusCode
	return &v
}

// parseRateLimit 从响应头中解析速率限制信息
func parseRateLimit(header http.Header) *RateLimit {
	limit, _ := strconv.Atoi(header.Get("X-RateLimit-Limit"))
	remaining, _ := strconv.Atoi(header.Get("X-RateLimit-Remaining"))
	reset, _ := strconv.ParseInt(header.Get("X-RateLimit-Reset"), 10, 64)

	return &RateLimit{
		Limit:     uint(limit),
		Remaining: uint(remaining),
		Reset:     time.Unix(reset, 0),
	}
}

// APIContext 表示 GitHub API 的上下文信息
type APIContext struct {
	token          string
	version        string
	client         *http.Client
	defaultTimeout time.Duration
	rateLimit      *RateLimit
}

// NewAPIContext 创建一个新的 GitHub API 上下文
func NewAPIContext(token string, client *http.Client) *APIContext {
	ret := APIContext{
		token:          token,
		version:        GITHUB_API_VERSION,
		client:         client,
		defaultTimeout: DEFAULT_TIMEOUT,
	}

	if ret.client == nil {
		ret.client = &http.Client{
			Timeout: ret.defaultTimeout,
		}
	}

	return &ret
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
func (a *APIContext) getWithRetry(ctx context.Context, url string) (*http.Response, error) {
	backoff := Backoff{}

	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		response, err := a.get(ctx, url)

		// non-2xx code does not cause error
		if err != nil {
			// 如果错误是速率限制错误, 则直接返回
			if errors.Is(err, ErrRateLimitExceeded) {
				return nil, err
			}

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
				return nil, parseHTTPError(response.StatusCode, body)
			}
			utils.Log.Debugf("query github api error: status code %d, retry after %s", response.StatusCode, p)

			if err := sleepWithContext(ctx, p); err != nil {
				return nil, err
			}
			continue
		}

		return nil, parseHTTPError(response.StatusCode, body)
	}
}

// SetAuthHeader 为请求头添加 GitHub API 所需的认证头.
// 这是一个副作用函数, 会直接修改传入的 header.
func (a *APIContext) SetAuthHeader(header http.Header) {
	header.Set("Authorization", fmt.Sprintf("Bearer %s", a.token))
}

// get 获取 GitHub API.
func (a *APIContext) get(ctx context.Context, url string) (*http.Response, error) {
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

	// 更新速率限制信息
	a.rateLimit = parseRateLimit(response.Header)

	// 如果剩余请求数为 0, 等待到重置时间
	if a.rateLimit.Remaining == 0 {
		waitTime := time.Until(a.rateLimit.Reset)
		utils.Log.Warnf("rate limit exceeded, will wait for %s", waitTime)
		return nil, ErrRateLimitExceeded
	}

	return response, nil
}

// GetReleases 获取仓库信息.
func (a *APIContext) GetReleases(ctx context.Context, repo repository, perPage int) ([]model.Obj, error) {
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

// GetLatestRelease 获取最新 release.
func (a *APIContext) GetLatestRelease(ctx context.Context, repo repository) (model.Obj, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/releases/latest", repo.UrlEncode())
	response, err := a.getWithRetry(ctx, url)
	if err != nil {
		var githubErr *GitHubError
		if errors.As(err, &githubErr) && githubErr.StatusCode == http.StatusNotFound {
			return nil, ErrNoRelease
		}
		return nil, errors.Wrap(err, "get latest release")
	}
	defer response.Body.Close()

	body, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, errors.Wrap(err, "read response body")
	}

	if response.StatusCode == http.StatusNotFound {
		return nil, ErrNoRelease
	}

	if response.StatusCode != http.StatusOK {
		err := parseHTTPError(response.StatusCode, body)
		var githubErr *GitHubError
		if errors.As(err, &githubErr) && githubErr.StatusCode == http.StatusNotFound {
			return nil, ErrNoRelease
		}
		return nil, err
	}

	var release Release
	if err := utils.Json.Unmarshal(body, &release); err != nil {
		return nil, errors.Wrap(err, "unmarshal release data")
	}

	release.SetLatestFlag(true)
	return &release, nil
}

// GetRelease 获取指定 tag 的 release.
func (a *APIContext) GetRelease(ctx context.Context, repo repository, id int64) (*Release, error) {
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

	release := Release{}
	err = utils.Json.Unmarshal(body, &release)
	if err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal release")
	}

	return &release, nil
}

// GetReleaseAsset 获取指定 tag 的 release 的 assets.
func (a *APIContext) GetReleaseAsset(ctx context.Context, repo repository, ID int64) (*Asset, error) {
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
