package halalcloud

import (
	"context"
	"github.com/alist-org/alist/v3/internal/model"
	"github.com/alist-org/alist/v3/pkg/http_range"
	"github.com/alist-org/alist/v3/pkg/utils"
	"github.com/city404/v6-public-rpc-proto/go/v6/common"
	pubUserFile "github.com/city404/v6-public-rpc-proto/go/v6/userfile"
	"google.golang.org/grpc"
	"io"
	"time"
)

type AuthService struct {
	appID          string
	appVersion     string
	appSecret      string
	grpcConnection *grpc.ClientConn
	dopts          halalOptions
	tr             *TokenResp
}

type TokenResp struct {
	AccessToken           string `json:"accessToken,omitempty"`
	AccessTokenExpiredAt  int64  `json:"accessTokenExpiredAt,omitempty"`
	RefreshToken          string `json:"refreshToken,omitempty"`
	RefreshTokenExpiredAt int64  `json:"refreshTokenExpiredAt,omitempty"`
}

type UserInfo struct {
	Identity string `json:"identity,omitempty"`
	UpdateTs int64  `json:"updateTs,omitempty"`
	Name     string `json:"name,omitempty"`
	CreateTs int64  `json:"createTs,omitempty"`
}

type OrderByInfo struct {
	Field string `json:"field,omitempty"`
	Asc   bool   `json:"asc,omitempty"`
}

type ListInfo struct {
	Token   string         `json:"token,omitempty"`
	Limit   int64          `json:"limit,omitempty"`
	OrderBy []*OrderByInfo `json:"order_by,omitempty"`
	Version int32          `json:"version,omitempty"`
}

type FilesList struct {
	Files    []*Files                `json:"files,omitempty"`
	ListInfo *common.ScanListRequest `json:"list_info,omitempty"`
}

var _ model.Obj = (*Files)(nil)

type Files struct {
	Identity        string `json:"identity,omitempty"`
	Parent          string `json:"parent,omitempty"`
	Name            string `json:"name,omitempty"`
	Path            string `json:"path,omitempty"`
	MimeType        string `json:"mime_type,omitempty"`
	Size            int64  `json:"size,omitempty"`
	Type            int64  `json:"type,omitempty"`
	CreateTs        int64  `json:"create_ts,omitempty"`
	UpdateTs        int64  `json:"update_ts,omitempty"`
	DeleteTs        int64  `json:"delete_ts,omitempty"`
	Deleted         bool   `json:"deleted,omitempty"`
	Dir             bool   `json:"dir,omitempty"`
	Hidden          bool   `json:"hidden,omitempty"`
	Locked          bool   `json:"locked,omitempty"`
	Shared          bool   `json:"shared,omitempty"`
	Starred         bool   `json:"starred,omitempty"`
	Trashed         bool   `json:"trashed,omitempty"`
	LockedAt        int64  `json:"locked_at,omitempty"`
	LockedBy        string `json:"locked_by,omitempty"`
	SharedAt        int64  `json:"shared_at,omitempty"`
	Flag            int64  `json:"flag,omitempty"`
	Unique          string `json:"unique,omitempty"`
	ContentIdentity string `json:"content_identity,omitempty"`
	Label           int64  `json:"label,omitempty"`
	StoreType       int64  `json:"store_type,omitempty"`
	Version         int64  `json:"version,omitempty"`
}

func (f *Files) GetSize() int64 {
	return f.Size
}

func (f *Files) GetName() string {
	return f.Name
}

func (f *Files) ModTime() time.Time {
	return time.UnixMilli(f.UpdateTs)
}

func (f *Files) CreateTime() time.Time {
	return time.UnixMilli(f.UpdateTs)
}

func (f *Files) IsDir() bool {
	return f.Dir
}

func (f *Files) GetHash() utils.HashInfo {
	return utils.HashInfo{}
}

func (f *Files) GetID() string {
	if len(f.Identity) == 0 {
		f.Identity = "/"
	}
	return f.Identity
}

func (f *Files) GetPath() string {
	return f.Path
}

type ChunkedRangeReadCloser struct {
	chunks []*pubUserFile.SliceDownloadInfo // 分片下载地址列表
	index  int                              // 当前正在读取的分片的索引
	utils.Closers
}

func (c *ChunkedRangeReadCloser) RangeReadr(ctx context.Context, httpRange http_range.Range) (io.ReadCloser, error) {

	pipeReader, pipeWriter := io.Pipe()

	// 在一个新的 goroutine 中读取数据
	go func() {
		defer c.Closers.Close()
		defer pipeWriter.Close()

		for _, addr := range c.chunks {
			// 从分片下载地址下载数据
			dataBytes, err := tryAndGetRawFiles(addr)

			if err != nil {
				// 如果下载失败，返回错误
				pipeWriter.CloseWithError(err)
				return
			}

			// 将下载的数据写入管道
			if _, err := pipeWriter.Write(dataBytes); err != nil {
				pipeWriter.CloseWithError(err)
				return
			}
		}
	}()

	// 将管道的读取端作为结果返回
	c.Closers.Add(pipeReader)
	return pipeReader, nil
}

func fileToObj(f Files) *model.ObjThumb {
	return &model.ObjThumb{
		Object: model.Object{
			ID:       f.Identity,
			Path:     f.Path,
			Name:     f.Name,
			Size:     f.Size,
			Modified: f.ModTime(),
			Ctime:    f.CreateTime(),
			IsFolder: f.IsDir(),
		},
	}
}

type SteamFile struct {
	file model.File
}

func (s *SteamFile) Read(p []byte) (n int, err error) {
	return s.file.Read(p)
}

func (s *SteamFile) Close() error {
	return s.file.Close()
}
