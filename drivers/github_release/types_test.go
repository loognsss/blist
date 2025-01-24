package github_release

import (
	"testing"
	"time"

	"github.com/alist-org/alist/v3/pkg/utils"
	"github.com/stretchr/testify/assert"
)

func TestNewRepository(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    repository
		wantErr bool
	}{
		{
			name:  "正常的仓库名称",
			input: "alist-org/alist",
			want: repository{
				owner: "alist-org",
				name:  "alist",
			},
			wantErr: false,
		},
		{
			name:    "缺少斜杠的仓库名称",
			input:   "alist-org",
			want:    repository{},
			wantErr: true,
		},
		{
			name:    "空的所有者",
			input:   "/alist",
			want:    repository{},
			wantErr: true,
		},
		{
			name:    "空的仓库名",
			input:   "alist-org/",
			want:    repository{},
			wantErr: true,
		},
		{
			name:    "完全空的输入",
			input:   "",
			want:    repository{},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := newRepository(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Equal(t, repository{}, got)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func TestRepository_String(t *testing.T) {
	repo := repository{
		owner: "alist-org",
		name:  "alist",
	}
	assert.Equal(t, "alist-org/alist", repo.String())
}

func TestRepository_UrlEncode(t *testing.T) {
	tests := []struct {
		name string
		repo repository
		want string
	}{
		{
			name: "普通仓库名称",
			repo: repository{
				owner: "alist-org",
				name:  "alist",
			},
			want: "alist-org/alist",
		},
		{
			name: "包含特殊字符的仓库名称",
			repo: repository{
				owner: "user name",
				name:  "repo name",
			},
			want: "user+name/repo+name",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.repo.UrlEncode()
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestRelease_UnmarshalJSON(t *testing.T) {
	tests := []struct {
		name            string
		json            string
		want            *Release
		invalidDatetime bool
	}{
		{
			name: "正常的发布数据",
			json: `{
				"url": "https://api.github.com/repos/alist-org/alist/releases/1",
				"html_url": "https://github.com/alist-org/alist/releases/tag/v1.0.0",
				"tag_name": "v1.0.0",
				"name": "Release v1.0.0",
				"body": "Release notes",
				"created_at": "2023-01-01T12:00:00Z",
				"published_at": "2023-01-01T12:30:00Z",
				"author": {
					"login": "test-user",
					"id": 1
				}
			}`,
			want: &Release{
				URL:     "https://api.github.com/repos/alist-org/alist/releases/1",
				HTMLURL: "https://github.com/alist-org/alist/releases/tag/v1.0.0",
				TagName: "v1.0.0",
				Name:    "Release v1.0.0",
				Body:    "Release notes",
				Author: User{
					Login: "test-user",
					ID:    1,
				},
			},
			invalidDatetime: false,
		},
		{
			name: "无效的时间格式",
			json: `{
				"created_at": "invalid-time",
				"published_at": "invalid-time"
			}`,
			want:            nil,
			invalidDatetime: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var release Release
			err := release.UnmarshalJSON([]byte(tt.json))
			if tt.invalidDatetime {
				assert.True(t, release.CreatedAt.IsZero())
				assert.True(t, release.PublishedAt.IsZero())
			} else {
				assert.NoError(t, err)
				// 验证时间字段
				assert.Equal(t, 2023, release.CreatedAt.Year())
				assert.Equal(t, 2023, release.PublishedAt.Year())
				// 验证其他字段
				assert.Equal(t, tt.want.URL, release.URL)
				assert.Equal(t, tt.want.HTMLURL, release.HTMLURL)
				assert.Equal(t, tt.want.TagName, release.TagName)
				assert.Equal(t, tt.want.Name, release.Name)
				assert.Equal(t, tt.want.Body, release.Body)
				assert.Equal(t, tt.want.Author.Login, release.Author.Login)
				assert.Equal(t, tt.want.Author.ID, release.Author.ID)
			}
		})
	}
}

func TestAsset_UnmarshalJSON(t *testing.T) {
	tests := []struct {
		name    string
		json    string
		want    *Asset
		wantErr bool
	}{
		{
			name: "正常的资源数据",
			json: `{
				"url": "https://api.github.com/repos/alist-org/alist/releases/assets/1",
				"browser_download_url": "https://github.com/alist-org/alist/releases/download/v1.0.0/asset.zip",
				"id": 1,
				"name": "asset.zip",
				"label": "Binary",
				"state": "uploaded",
				"content_type": "application/zip",
				"size": 1024,
				"download_count": 100,
				"created_at": "2023-01-01T12:00:00Z",
				"updated_at": "2023-01-01T12:30:00Z",
				"uploader": {
					"login": "test-user",
					"id": 1
				}
			}`,
			want: &Asset{
				URL:                "https://api.github.com/repos/alist-org/alist/releases/assets/1",
				BrowserDownloadURL: "https://github.com/alist-org/alist/releases/download/v1.0.0/asset.zip",
				ID:                 1,
				Name:               "asset.zip",
				Label:              "Binary",
				State:              "uploaded",
				ContentType:        "application/zip",
				Size:               1024,
				DownloadCount:      100,
				Uploader: &User{
					Login: "test-user",
					ID:    1,
				},
			},
			wantErr: false,
		},
		{
			name: "无效的时间格式",
			json: `{
				"created_at": "invalid-time",
				"updated_at": "2023-01-01T12:30:00Z"
			}`,
			want:    nil,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var asset Asset
			err := asset.UnmarshalJSON([]byte(tt.json))
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				// 验证时间字段
				assert.Equal(t, 2023, asset.CreatedAt.Year())
				assert.Equal(t, 2023, asset.UpdatedAt.Year())
				// 验证其他字段
				assert.Equal(t, tt.want.URL, asset.URL)
				assert.Equal(t, tt.want.BrowserDownloadURL, asset.BrowserDownloadURL)
				assert.Equal(t, tt.want.ID, asset.ID)
				assert.Equal(t, tt.want.Name, asset.Name)
				assert.Equal(t, tt.want.Label, asset.Label)
				assert.Equal(t, tt.want.State, asset.State)
				assert.Equal(t, tt.want.ContentType, asset.ContentType)
				assert.Equal(t, tt.want.Size, asset.Size)
				assert.Equal(t, tt.want.DownloadCount, asset.DownloadCount)
				assert.Equal(t, tt.want.Uploader.Login, asset.Uploader.Login)
				assert.Equal(t, tt.want.Uploader.ID, asset.Uploader.ID)
			}
		})
	}
}

func TestReleases_UnmarshalJSON(t *testing.T) {
	jsonData := `[
		{
			"url": "https://api.github.com/repos/AlistGo/alist/releases/170718825",
			"assets_url": "https://api.github.com/repos/AlistGo/alist/releases/170718825/assets",
			"upload_url": "https://uploads.github.com/repos/AlistGo/alist/releases/170718825/assets{?name,label}",
			"html_url": "https://github.com/AlistGo/alist/releases/tag/beta",
			"id": 170718825,
			"author": {
				"login": "xhofe",
				"id": 36558727,
				"node_id": "MDQ6VXNlcjM2NTU4NzI3",
				"avatar_url": "https://avatars.githubusercontent.com/u/36558727?v=4",
				"url": "https://api.github.com/users/xhofe",
				"html_url": "https://github.com/xhofe",
				"type": "User",
				"site_admin": false
			},
			"node_id": "RE_kwDOE09S284KLPZp",
			"tag_name": "beta",
			"target_commitish": "main",
			"name": "AList Beta Version",
			"draft": false,
			"prerelease": true,
			"created_at": "2025-01-18T15:52:02Z",
			"published_at": "2024-08-17T14:10:08Z",
			"assets": [
				{
					"url": "https://api.github.com/repos/AlistGo/alist/releases/assets/221414212",
					"id": 221414212,
					"name": "alist-android-386.tar.gz",
					"content_type": "application/gzip",
					"state": "uploaded",
					"size": 31186443,
					"download_count": 6,
					"created_at": "2025-01-18T15:58:55Z",
					"updated_at": "2025-01-18T15:58:56Z",
					"browser_download_url": "https://github.com/AlistGo/alist/releases/download/beta/alist-android-386.tar.gz",
					"uploader": {
						"login": "github-actions[bot]",
						"id": 41898282,
						"type": "Bot",
						"site_admin": false
					}
				},
				{
					"url": "https://api.github.com/repos/AlistGo/alist/releases/assets/221414214",
					"id": 221414214,
					"name": "alist-android-amd64.tar.gz",
					"content_type": "application/gzip",
					"state": "uploaded",
					"size": 31586093,
					"download_count": 10,
					"created_at": "2025-01-18T15:58:55Z",
					"updated_at": "2025-01-18T15:58:56Z",
					"browser_download_url": "https://github.com/AlistGo/alist/releases/download/beta/alist-android-amd64.tar.gz",
					"uploader": {
						"login": "github-actions[bot]",
						"id": 41898282,
						"type": "Bot",
						"site_admin": false
					}
				}
			],
			"body": "Test text"
		}
	]`

	var releases []Release
	err := utils.Json.Unmarshal([]byte(jsonData), &releases)
	assert.NoError(t, err)
	assert.Len(t, releases, 1)

	release := releases[0]
	// 验证 Release 基本信息
	assert.Equal(t, int64(170718825), release.ID)
	assert.Equal(t, "beta", release.TagName)
	assert.Equal(t, "AList Beta Version", release.Name)
	assert.Equal(t, "Test text", release.Body)
	assert.False(t, release.Draft)
	assert.True(t, release.Prerelease)

	// 验证时间
	assert.Equal(t, 2025, release.CreatedAt.Year())
	assert.Equal(t, 2024, release.PublishedAt.Year())

	// 验证作者信息
	assert.Equal(t, "xhofe", release.Author.Login)
	assert.Equal(t, int64(36558727), release.Author.ID)
	assert.Equal(t, "User", release.Author.Type)

	// 验证资源信息
	assert.Len(t, release.Assets, 2)

	// 验证第一个资源
	asset1 := release.Assets[0]
	assert.Equal(t, int64(221414212), asset1.ID)
	assert.Equal(t, "alist-android-386.tar.gz", asset1.Name)
	assert.Equal(t, "application/gzip", asset1.ContentType)
	assert.Equal(t, int64(31186443), asset1.Size)
	assert.Equal(t, int64(6), asset1.DownloadCount)
	assert.Equal(t, "uploaded", asset1.State)
	assert.Equal(t, "https://github.com/AlistGo/alist/releases/download/beta/alist-android-386.tar.gz", asset1.BrowserDownloadURL)

	// 验证第一个资源的上传者
	assert.Equal(t, "github-actions[bot]", asset1.Uploader.Login)
	assert.Equal(t, int64(41898282), asset1.Uploader.ID)
	assert.Equal(t, "Bot", asset1.Uploader.Type)

	// 验证第二个资源
	asset2 := release.Assets[1]
	assert.Equal(t, int64(221414214), asset2.ID)
	assert.Equal(t, "alist-android-amd64.tar.gz", asset2.Name)
	assert.Equal(t, int64(31586093), asset2.Size)
	assert.Equal(t, int64(10), asset2.DownloadCount)
}

func TestRelease_InterfaceMethods(t *testing.T) {
	release := &Release{
		ID:          123,
		TagName:     "v1.0.0",
		CreatedAt:   time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC),
		PublishedAt: time.Date(2023, 1, 2, 0, 0, 0, 0, time.UTC),
		Assets: []Asset{
			{Name: "asset1.zip"},
			{Name: "asset2.tar.gz"},
		},
	}

	// 测试基本方法
	t.Run("basic methods", func(t *testing.T) {
		assert.Equal(t, int64(0), release.GetSize())
		assert.Equal(t, "v1.0.0", release.GetName())
		assert.Equal(t, time.Date(2023, 1, 2, 0, 0, 0, 0, time.UTC), release.ModTime())
		assert.Equal(t, time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC), release.CreateTime())
		assert.True(t, release.IsDir())
		assert.Equal(t, utils.HashInfo{}, release.GetHash())
		assert.Equal(t, "123", release.GetID())
		assert.Equal(t, "v1.0.0", release.GetPath())
	})

	// 测试 Children 方法
	t.Run("children", func(t *testing.T) {
		children, err := release.Children()
		assert.NoError(t, err)
		assert.Len(t, children, 2)
		assert.Equal(t, "asset1.zip", children[0].GetName())
		assert.Equal(t, "asset2.tar.gz", children[1].GetName())
	})
}

func TestAsset_InterfaceMethods(t *testing.T) {
	now := time.Now()
	asset := &Asset{
		ID:                 456,
		Name:               "test.zip",
		Size:               12345,
		CreatedAt:          &now,
		UpdatedAt:          &now,
		BrowserDownloadURL: "https://github.com/owner/repo/releases/download/v1.0.0/test.zip",
	}

	t.Run("basic methods", func(t *testing.T) {
		assert.Equal(t, int64(12345), asset.GetSize())
		assert.Equal(t, "test.zip", asset.GetName())
		assert.Equal(t, now, asset.ModTime())
		assert.Equal(t, now, asset.CreateTime())
		assert.False(t, asset.IsDir())
		assert.Equal(t, utils.HashInfo{}, asset.GetHash())
		assert.Equal(t, "456", asset.GetID())
	})

	// 测试空时间的情况
	t.Run("nil time fields", func(t *testing.T) {
		emptyAsset := &Asset{}
		assert.Equal(t, time.Time{}, emptyAsset.ModTime())
		assert.Equal(t, time.Time{}, emptyAsset.CreateTime())
	})
}

func TestAsset_GetPath(t *testing.T) {
	tests := []struct {
		name               string
		browserDownloadURL string
		want               string
	}{
		{
			name:               "valid url",
			browserDownloadURL: "https://github.com/owner/repo/releases/download/v1.0.0/test.zip",
			want:               "v1.0.0/test.zip",
		},
		{
			name:               "invalid url format",
			browserDownloadURL: "https://github.com/invalid/url",
			want:               "",
		},
		{
			name:               "empty url",
			browserDownloadURL: "",
			want:               "",
		},
		{
			name:               "url with special characters",
			browserDownloadURL: "https://github.com/owner/repo/releases/download/v1.0.0-beta/test-file_1.2.3.zip",
			want:               "v1.0.0-beta/test-file_1.2.3.zip",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			asset := &Asset{
				BrowserDownloadURL: tt.browserDownloadURL,
			}
			got := asset.GetPath()
			assert.Equal(t, tt.want, got)
		})
	}
}
