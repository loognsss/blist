package github_release

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestParseRateLimit(t *testing.T) {
	header := http.Header{}
	header.Set("X-RateLimit-Limit", "60")
	header.Set("X-RateLimit-Remaining", "59")
	header.Set("X-RateLimit-Reset", "1735689600") // 2025-01-01 00:00:00 UTC

	rateLimit := parseRateLimit(header)

	assert.Equal(t, uint(60), rateLimit.Limit)
	assert.Equal(t, uint(59), rateLimit.Remaining)
	assert.Equal(t, time.Unix(1735689600, 0), rateLimit.Reset)
}

func TestGitHubError(t *testing.T) {
	err := &GitHubError{
		Message:    "API rate limit exceeded",
		StatusCode: 403,
	}

	assert.Equal(t, "github api error: API rate limit exceeded (status: 403)", err.Error())
}

func TestNewAPIContext(t *testing.T) {
	token := "test-token"
	client := &http.Client{}
	ctx := NewAPIContext(token, client)

	assert.Equal(t, token, ctx.token)
	assert.Equal(t, GITHUB_API_VERSION, ctx.version)
	assert.Equal(t, client, ctx.client)
	assert.Equal(t, DEFAULT_TIMEOUT, ctx.defaultTimeout)
}

func TestAPIContext_SetAuthHeader(t *testing.T) {
	token := "test-token"
	ctx := NewAPIContext(token, nil)
	header := http.Header{}

	ctx.SetAuthHeader(header)
	assert.Equal(t, "Bearer "+token, header.Get("Authorization"))
}

func TestAPIContext_GetWithRetry_RateLimit(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-RateLimit-Limit", "60")
		w.Header().Set("X-RateLimit-Remaining", "0")
		w.Header().Set("X-RateLimit-Reset", "1735689600")
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte(`{"message": "API rate limit exceeded"}`))
	}))
	defer server.Close()

	ctx := NewAPIContext("test-token", server.Client())
	_, err := ctx.getWithRetry(context.Background(), server.URL)

	assert.ErrorIs(t, err, ErrRateLimitExceeded)
}

type testRoundTripper struct {
	handler http.HandlerFunc
}

func (t *testRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	// 创建一个响应记录器
	w := httptest.NewRecorder()
	// 调用处理函数
	t.handler.ServeHTTP(w, req)
	// 将响应记录器转换为响应
	return w.Result(), nil
}

func TestAPIContext_GetLatestRelease(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 验证请求路径
		assert.Equal(t, "/repos/test-owner/test-repo/releases/latest", r.URL.Path)

		// 验证请求头
		assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))
		assert.Equal(t, "application/vnd.github+json", r.Header.Get("Accept"))

		// 设置速率限制头部
		w.Header().Set("X-RateLimit-Limit", "60")
		w.Header().Set("X-RateLimit-Remaining", "59")
		w.Header().Set("X-RateLimit-Reset", "1735689600")

		// 设置响应头和内容
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"id": 1,
			"tag_name": "v1.0.0",
			"name": "Release 1.0.0",
			"published_at": "2025-01-01T00:00:00Z",
			"created_at": "2025-01-01T00:00:00Z",
			"assets": []
		}`))
	})

	// 创建一个自定义的 HTTP 客户端
	client := &http.Client{
		Transport: &testRoundTripper{handler: handler},
	}

	ctx := NewAPIContext("test-token", client)
	repo := repository{owner: "test-owner", name: "test-repo"}
	release, err := ctx.GetLatestRelease(context.Background(), repo)

	if assert.NoError(t, err) {
		assert.NotNil(t, release)
		assert.Equal(t, "latest(v1.0.0)", release.GetName())
	}
}

func TestAPIContext_GetLatestRelease_NoRelease(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 验证请求路径
		assert.Equal(t, "/repos/test-owner/test-repo/releases/latest", r.URL.Path)

		// 验证请求头
		assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))
		assert.Equal(t, "application/vnd.github+json", r.Header.Get("Accept"))

		// 设置速率限制头部
		w.Header().Set("X-RateLimit-Limit", "60")
		w.Header().Set("X-RateLimit-Remaining", "59")
		w.Header().Set("X-RateLimit-Reset", "1735689600")

		// 返回 404 状态码
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"message": "Not Found"}`))
	})

	// 创建一个自定义的 HTTP 客户端
	client := &http.Client{
		Transport: &testRoundTripper{handler: handler},
	}

	ctx := NewAPIContext("test-token", client)
	repo := repository{owner: "test-owner", name: "test-repo"}
	_, err := ctx.GetLatestRelease(context.Background(), repo)

	assert.ErrorIs(t, err, ErrNoRelease)
}
