package template

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/alist-org/alist/v3/internal/model"
	"github.com/alist-org/alist/v3/pkg/utils"
	"github.com/pkg/errors"
)

type repository struct {
	owner string
	name  string
}

func newRepository(name string) (repository, error) {
	parts := strings.Split(name, "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return repository{}, errors.New("repo name must be in the format of owner/repo")
	}

	return repository{
		owner: parts[0],
		name:  parts[1],
	}, nil
}

func (r *repository) String() string {
	return fmt.Sprintf("%s/%s", r.owner, r.name)
}

func (r *repository) UrlEncode() string {
	ownerPart := url.QueryEscape(r.owner)
	namePart := url.QueryEscape(r.name)
	return fmt.Sprintf("%s/%s", ownerPart, namePart)
}

type Release struct {
	URL             string    `json:"url"`
	HTMLURL         string    `json:"html_url"`
	AssetsURL       string    `json:"assets_url"`
	UploadURL       string    `json:"upload_url"`
	TarballURL      string    `json:"tarball_url"`
	ZipballURL      string    `json:"zipball_url"`
	ID              int64     `json:"id"`
	NodeID          string    `json:"node_id"`
	TagName         string    `json:"tag_name"`
	TargetCommitish string    `json:"target_commitish"`
	Name            string    `json:"name"`
	Body            string    `json:"body"`
	Draft           bool      `json:"draft"`
	Prerelease      bool      `json:"prerelease"`
	CreatedAt       time.Time `json:"created_at"`
	PublishedAt     time.Time `json:"published_at"`
	Author          User      `json:"author"`
	Assets          []Asset   `json:"assets"`
	BodyHTML        string    `json:"body_html"`
	BodyText        string    `json:"body_text"`
	MentionsCount   int       `json:"mentions_count"`
	DiscussionURL   string    `json:"discussion_url"`

	latest bool
}

func (r *Release) UnmarshalJSON(data []byte) error {
	type alias Release
	aux := struct {
		CreatedAt   string `json:"created_at"`
		PublishedAt string `json:"published_at"`
		*alias
	}{
		alias: (*alias)(r),
	}

	if err := utils.Json.Unmarshal(data, &aux); err != nil {
		return errors.Wrap(err, "failed to unmarshal release")
	}

	createdAt, err := time.Parse(time.RFC3339, aux.CreatedAt)
	if err != nil {
		utils.Log.Error("failed to parse created_at in release", "error", err)
		createdAt = time.Time{}
	} else {
		r.CreatedAt = createdAt
	}

	publishedAt, err := time.Parse(time.RFC3339, aux.PublishedAt)
	if err != nil {
		utils.Log.Error("failed to parse published_at in release", "error", err)
		publishedAt = time.Time{}
	} else {
		r.PublishedAt = publishedAt
	}

	return nil
}

func (r *Release) GetSize() int64 {
	return 0
}

func (r *Release) SetLatestFlag(flag bool) {
	r.latest = flag
}

func (r *Release) GetName() string {
	if r.latest {
		return fmt.Sprintf("latest(%s)", r.TagName)
	}
	return r.TagName
}

func (r *Release) ModTime() time.Time {
	return r.PublishedAt
}

func (r *Release) CreateTime() time.Time {
	return r.CreatedAt
}

func (r *Release) IsDir() bool {
	return true
}

func (r *Release) GetHash() utils.HashInfo {
	return utils.HashInfo{}
}

func (r *Release) GetID() string {
	return fmt.Sprintf("%d", r.ID)
}

func (r *Release) GetPath() string {
	return r.TagName
}

func (r *Release) Children() ([]model.Obj, error) {
	return utils.SliceConvert(r.Assets, func(src Asset) (model.Obj, error) {
		return &src, nil
	})
}

type Asset struct {
	URL                string     `json:"url"`
	BrowserDownloadURL string     `json:"browser_download_url"`
	ID                 int64      `json:"id"`
	NodeID             string     `json:"node_id"`
	Name               string     `json:"name"`
	Label              string     `json:"label"`
	State              string     `json:"state"`
	ContentType        string     `json:"content_type"`
	Size               int64      `json:"size"`
	DownloadCount      int64      `json:"download_count"`
	CreatedAt          *time.Time `json:"created_at"`
	UpdatedAt          *time.Time `json:"updated_at"`
	Uploader           *User      `json:"uploader"`
}

func (a *Asset) UnmarshalJSON(data []byte) error {
	type alias Asset
	aux := struct {
		CreatedAt string `json:"created_at"`
		UpdatedAt string `json:"updated_at"`
		*alias
	}{
		alias: (*alias)(a),
	}

	if err := utils.Json.Unmarshal(data, &aux); err != nil {
		return errors.Wrap(err, "failed to unmarshal asset")
	}

	createdAt, err := time.Parse(time.RFC3339, aux.CreatedAt)
	if err != nil {
		return errors.Wrap(err, "failed to parse created_at in asset")
	}

	a.CreatedAt = &createdAt

	updatedAt, err := time.Parse(time.RFC3339, aux.UpdatedAt)
	if err != nil {
		return errors.Wrap(err, "failed to parse updated_at in asset")
	}

	a.UpdatedAt = &updatedAt

	return nil
}
func (a *Asset) GetSize() (_ int64) {
	return a.Size
}

func (a *Asset) GetName() (_ string) {
	return a.Name
}

func (a *Asset) ModTime() (_ time.Time) {
	if a.UpdatedAt == nil {
		return time.Time{}
	}
	return *a.UpdatedAt
}

func (a *Asset) CreateTime() (_ time.Time) {
	if a.CreatedAt == nil {
		return time.Time{}
	}
	return *a.CreatedAt
}

func (a *Asset) IsDir() bool {
	return false
}

// GetHash 获取文件的哈希值. github release api 不提供哈希值
func (a *Asset) GetHash() utils.HashInfo {
	return utils.HashInfo{}
}

func (a *Asset) GetID() string {
	return fmt.Sprintf("%d", a.ID)
}

// GetPath 获取路径. 通过解析 Asset.BrowserDownloadURL 获取
func (a *Asset) GetPath() string {
	pattern := `https://github.com/[^/]+/[^/]+/releases/download/([^/]+)/([^/]+)`
	re := regexp.MustCompile(pattern)
	matches := re.FindStringSubmatch(a.BrowserDownloadURL)
	if len(matches) != 3 {
		return ""
	}
	tag := matches[1]
	assetName := matches[2]
	return fmt.Sprintf("%s/%s", tag, assetName)
}

type User struct {
	Name              string `json:"name"`
	Email             string `json:"email"`
	Login             string `json:"login"`
	ID                int64  `json:"id"`
	NodeID            string `json:"node_id"`
	AvatarURL         string `json:"avatar_url"`
	GravatarID        string `json:"gravatar_id"`
	URL               string `json:"url"`
	HTMLURL           string `json:"html_url"`
	FollowersURL      string `json:"followers_url"`
	FollowingURL      string `json:"following_url"`
	GistsURL          string `json:"gists_url"`
	StarredURL        string `json:"starred_url"`
	SubscriptionsURL  string `json:"subscriptions_url"`
	OrganizationsURL  string `json:"organizations_url"`
	ReposURL          string `json:"repos_url"`
	EventsURL         string `json:"events_url"`
	ReceivedEventsURL string `json:"received_events_url"`
	Type              string `json:"type"`
	SiteAdmin         bool   `json:"site_admin"`
}
