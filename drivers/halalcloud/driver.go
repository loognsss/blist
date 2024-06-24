package halalcloud

import (
	"context"
	"crypto/sha1"
	"fmt"
	"github.com/alist-org/alist/v3/internal/driver"
	"github.com/alist-org/alist/v3/internal/model"
	"github.com/alist-org/alist/v3/internal/op"
	"github.com/alist-org/alist/v3/pkg/http_range"
	"github.com/alist-org/alist/v3/pkg/utils"
	bauth "github.com/baidubce/bce-sdk-go/auth"
	"github.com/baidubce/bce-sdk-go/bce"
	"github.com/baidubce/bce-sdk-go/services/bos"
	"github.com/baidubce/bce-sdk-go/services/bos/api"
	"github.com/city404/v6-public-rpc-proto/go/v6/common"
	pbPublicUser "github.com/city404/v6-public-rpc-proto/go/v6/user"
	pubUserFile "github.com/city404/v6-public-rpc-proto/go/v6/userfile"
	"github.com/jinzhu/copier"
	"github.com/rclone/rclone/lib/readers"
	"github.com/zzzhr1990/go-common-entity/userfile"
	"io"
	"strconv"
	"strings"
	"time"
)

type HalalCloud struct {
	*HalalCommon
	model.Storage
	Addition

	uploadThread int
	fileStatus   int // 文件状态 类型，0小文件(1M)、1中型文件(16M)、2大型文件(32M)
}

func (d *HalalCloud) Config() driver.Config {
	return config
}

func (d *HalalCloud) GetAddition() driver.Additional {
	return &d.Addition
}

func (d *HalalCloud) Init(ctx context.Context) error {
	d.uploadThread, _ = strconv.Atoi(d.UploadThread)
	if d.uploadThread < 1 || d.uploadThread > 32 {
		d.uploadThread, d.UploadThread = 3, "3"
	}

	if d.HalalCommon == nil {
		d.HalalCommon = &HalalCommon{
			Common: &Common{},
			AuthService: &AuthService{
				appID:      AppID,
				appVersion: AppVersion,
				appSecret:  AppSecret,
				tr: &TokenResp{
					RefreshToken: d.Addition.RefreshToken,
				},
			},
			UserInfo: &UserInfo{},
			refreshTokenFunc: func(token string) error {
				d.Addition.RefreshToken = token
				op.MustSaveDriverStorage(d)
				return nil
			},
		}
	}

	// 防止重复登录
	if d.Addition.RefreshToken == "" || !d.IsLogin() {
		as, err := d.NewAuthServiceWithOauth()
		if err != nil {
			d.GetStorage().SetStatus(fmt.Sprintf("%+v", err.Error()))
			return err
		}
		d.HalalCommon.AuthService = as
		d.SetTokenResp(as.tr)
		op.MustSaveDriverStorage(d)
	}
	var err error
	d.HalalCommon.serv, err = d.NewAuthService(d.Addition.RefreshToken)
	if err != nil {
		return err
	}

	return nil
}

func (d *HalalCloud) Drop(ctx context.Context) error {
	return nil
}

func (d *HalalCloud) List(ctx context.Context, dir model.Obj, args model.ListArgs) ([]model.Obj, error) {
	return d.getFiles(ctx, dir)
}

func (d *HalalCloud) Link(ctx context.Context, file model.Obj, args model.LinkArgs) (*model.Link, error) {
	return d.getLink(ctx, file, args)
}

func (d *HalalCloud) MakeDir(ctx context.Context, parentDir model.Obj, dirName string) (model.Obj, error) {
	return d.makeDir(ctx, parentDir, dirName)
}

func (d *HalalCloud) Move(ctx context.Context, srcObj, dstDir model.Obj) (model.Obj, error) {
	return d.move(ctx, srcObj, dstDir)
}

func (d *HalalCloud) Rename(ctx context.Context, srcObj model.Obj, newName string) (model.Obj, error) {
	return d.rename(ctx, srcObj, newName)
}

func (d *HalalCloud) Copy(ctx context.Context, srcObj, dstDir model.Obj) (model.Obj, error) {
	return d.copy(ctx, srcObj, dstDir)
}

func (d *HalalCloud) Remove(ctx context.Context, obj model.Obj) error {
	return d.remove(ctx, obj)
}

func (d *HalalCloud) Put(ctx context.Context, dstDir model.Obj, stream model.FileStreamer, up driver.UpdateProgress) (model.Obj, error) {
	return d.put(ctx, dstDir, stream, up)
}

func (d *HalalCloud) IsLogin() bool {
	if d.AuthService.tr == nil {
		return false
	}
	serv, err := d.NewAuthService(d.Addition.RefreshToken)
	if err != nil {
		return false
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()
	result, err := pbPublicUser.NewPubUserClient(serv.GetGrpcConnection()).Get(ctx, &pbPublicUser.User{
		Identity: "",
	})
	if result == nil || err != nil {
		return false
	}
	d.UserInfo.Identity = result.Identity
	d.UserInfo.CreateTs = result.CreateTs
	d.UserInfo.Name = result.Name
	d.UserInfo.UpdateTs = result.UpdateTs
	return true
}

type HalalCommon struct {
	*Common
	*AuthService     // 登录信息
	*UserInfo        // 用户信息
	refreshTokenFunc func(token string) error
	serv             *AuthService
}

func (d *HalalCloud) SetTokenResp(tr *TokenResp) {
	d.Addition.RefreshToken = tr.RefreshToken
}

func (d *HalalCloud) getFiles(ctx context.Context, dir model.Obj) ([]model.Obj, error) {

	files := make([]model.Obj, 0)
	limit := int64(10)
	token := ""
	client := pubUserFile.NewPubUserFileClient(d.HalalCommon.serv.GetGrpcConnection())

	opDir := d.GetCurrentDir(dir)
	//if len(args) > 0 {
	//	opDir = d.GetCurrentOpDir(dir, args, 0)
	//}

	for {
		filesList := FilesList{}
		ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
		defer cancel()
		result, err := client.List(ctx, &pubUserFile.FileListRequest{
			Parent: &pubUserFile.File{Path: opDir},
			ListInfo: &common.ScanListRequest{
				Limit: limit,
				Token: token,
			},
		})
		if err != nil {
			return nil, err
		}
		err = copier.Copy(&filesList, result)
		if err != nil {
			return nil, err
		}

		if filesList.Files != nil && len(filesList.Files) > 0 {
			for i := 0; i < len(filesList.Files); i++ {
				files = append(files, filesList.Files[i])
			}
		}

		if result.ListInfo == nil || result.ListInfo.Token == "" {
			break
		}
		token = result.ListInfo.Token

	}
	return files, nil
}

func (d *HalalCloud) getLink(ctx context.Context, file model.Obj, args model.LinkArgs) (*model.Link, error) {
	id := file.GetID()

	newPath := userfile.NewFormattedPath(d.GetCurrentDir(file)).GetPath()
	client := pubUserFile.NewPubUserFileClient(d.HalalCommon.serv.GetGrpcConnection())
	if len(id) > 0 {
		newPath = ""
	}
	result, err := client.ParseFileSlice(ctx, &pubUserFile.File{

		Path:     newPath,
		Identity: id,
	})
	if err != nil {
		return nil, err
	}
	fileAddrs := []*pubUserFile.SliceDownloadInfo{}
	var addressDuration int64
	batchRequest := []string{}
	for _, slice := range result.RawNodes {
		batchRequest = append(batchRequest, slice)
		if len(batchRequest) >= 200 {
			sliceAddress, err := client.GetSliceDownloadAddress(ctx, &pubUserFile.SliceDownloadAddressRequest{
				Identity: batchRequest,
				Version:  1,
			})
			if err != nil {
				return nil, err
			}
			fileAddrs = append(fileAddrs, sliceAddress.Addresses...)
			batchRequest = []string{}
			addressDuration = sliceAddress.ExpireAt
		}
	}
	if len(batchRequest) > 0 {
		sliceAddress, err := client.GetSliceDownloadAddress(ctx, &pubUserFile.SliceDownloadAddressRequest{
			Identity: batchRequest,
			Version:  1,
		})
		if err != nil {
			return nil, err
		}
		fileAddrs = append(fileAddrs, sliceAddress.Addresses...)
	}
	size := result.FileSize
	var finalClosers utils.Closers
	resultRangeReader := func(ctx context.Context, httpRange http_range.Range) (io.ReadCloser, error) {
		length := httpRange.Length
		if httpRange.Length >= 0 && httpRange.Start+httpRange.Length >= size {
			length = -1
		}
		if err != nil {
			return nil, fmt.Errorf("open download file failed: %w", err)
		}
		oo := &openObject{
			ctx:     ctx,
			d:       fileAddrs,
			chunks:  getChunkSizes(result.Sizes),
			skip:    httpRange.Start,
			sha:     result.Sha1,
			shaTemp: sha1.New(),
		}
		finalClosers.Add(oo)

		return readers.NewLimitedReadCloser(oo, length), nil
	}

	var duration time.Duration
	if addressDuration != 0 {
		duration = time.Until(time.UnixMilli(addressDuration))
	} else {
		duration = time.Until(time.Now().Add(time.Hour))
	}

	resultRangeReadCloser := &model.RangeReadCloser{RangeReader: resultRangeReader, Closers: finalClosers}
	return &model.Link{
		RangeReadCloser: resultRangeReadCloser,
		Expiration:      &duration,
	}, nil
}

func (d *HalalCloud) makeDir(ctx context.Context, dir model.Obj, name string) (model.Obj, error) {
	newDir := userfile.NewFormattedPath(d.GetCurrentOpDir(dir, []string{name}, 0)).GetPath()
	_, err := pubUserFile.NewPubUserFileClient(d.HalalCommon.serv.GetGrpcConnection()).Create(ctx, &pubUserFile.File{
		// Parent: &pubUserFile.File{Path: currentDir},
		Path: newDir,
	})
	return nil, err
}

func (d *HalalCloud) move(ctx context.Context, obj model.Obj, dir model.Obj) (model.Obj, error) {
	oldDir := userfile.NewFormattedPath(d.GetCurrentDir(obj)).GetPath()
	newDir := userfile.NewFormattedPath(d.GetCurrentDir(dir)).GetPath()
	_, err := pubUserFile.NewPubUserFileClient(d.HalalCommon.serv.GetGrpcConnection()).Move(ctx, &pubUserFile.BatchOperationRequest{
		Source: []*pubUserFile.File{
			{
				Identity: obj.GetID(),
				Path:     oldDir,
			},
		},
		Dest: &pubUserFile.File{
			Identity: dir.GetID(),
			Path:     newDir,
		},
	})
	return nil, err
}

func (d *HalalCloud) rename(ctx context.Context, obj model.Obj, name string) (model.Obj, error) {
	id := obj.GetID()
	newPath := userfile.NewFormattedPath(d.GetCurrentOpDir(obj, []string{name}, 0)).GetPath()
	if len(id) > 0 {
		newPath = ""
	}
	_, err := pubUserFile.NewPubUserFileClient(d.HalalCommon.serv.GetGrpcConnection()).Rename(ctx, &pubUserFile.File{
		Path:     newPath,
		Identity: id,
		Name:     name,
	})
	return nil, err
}

func (d *HalalCloud) copy(ctx context.Context, obj model.Obj, dir model.Obj) (model.Obj, error) {
	id := obj.GetID()
	sourcePath := userfile.NewFormattedPath(d.GetCurrentDir(obj)).GetPath()
	if len(id) > 0 {
		sourcePath = ""
	}
	dest := &pubUserFile.File{
		Identity: dir.GetID(),
		Path:     userfile.NewFormattedPath(d.GetCurrentDir(dir)).GetPath(),
	}
	_, err := pubUserFile.NewPubUserFileClient(d.HalalCommon.serv.GetGrpcConnection()).Copy(ctx, &pubUserFile.BatchOperationRequest{
		Source: []*pubUserFile.File{
			{
				Path:     sourcePath,
				Identity: id,
			},
		},
		Dest: dest,
	})
	return nil, err
}

func (d *HalalCloud) remove(ctx context.Context, obj model.Obj) error {
	id := obj.GetID()
	newPath := userfile.NewFormattedPath(d.GetCurrentDir(obj)).GetPath()
	if len(id) > 0 {
		newPath = ""
	}
	_, err := pubUserFile.NewPubUserFileClient(d.HalalCommon.serv.GetGrpcConnection()).Delete(ctx, &pubUserFile.BatchOperationRequest{
		Source: []*pubUserFile.File{
			{
				Path:     newPath,
				Identity: id,
			},
		},
	})
	return err
}

func (d *HalalCloud) put(ctx context.Context, dstDir model.Obj, fileStream model.FileStreamer, up driver.UpdateProgress) (model.Obj, error) {

	tempFile, err := fileStream.CacheFullInTempFile()
	if err != nil {
		return nil, err
	}

	newDir := userfile.NewFormattedPath(d.GetCurrentDir(dstDir)).GetPath()
	newDir = strings.TrimSuffix(newDir, "/") + "/" + fileStream.GetName()
	result, err := pubUserFile.NewPubUserFileClient(d.HalalCommon.serv.GetGrpcConnection()).CreateUploadToken(ctx, &pubUserFile.File{
		Path: newDir,
	})
	if err != nil {
		return nil, err
	}
	clientConfig := bos.BosClientConfiguration{
		Ak:               result.AccessKey,
		Sk:               result.SecretKey,
		Endpoint:         result.Endpoint,
		RedirectDisabled: false,
		//SessionToken:     result.SessionToken,
	}

	// 初始化一个BosClient
	bosClient, err := bos.NewClientWithConfig(&clientConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create bos client: %w", err)
	}
	stsCredential, err := bauth.NewSessionBceCredentials(
		result.AccessKey,
		result.SecretKey,
		result.Token)
	if err != nil {
		return nil, fmt.Errorf("failed to create sts credential: %w", err)
	}
	bosClient.Config.Credentials = stsCredential
	bosClient.MaxParallel = int64(d.uploadThread)

	d.setFileStatus(fileStream.GetSize()) // 设置文件状态

	bosClient.MultipartSize = d.getSliceSize()

	if fileStream.GetSize() < 1*utils.MB {
		partBody, _ := bce.NewBodyFromSizedReader(tempFile, fileStream.GetSize())
		_, err := bosClient.PutObject(result.Bucket, result.Key, partBody, nil)
		//_, err = bosClient.PutObjectFromStream(result.GetBucket(), fileStream.GetName(), tempFile, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to upload file: %v ===> %s/%s", err, clientConfig.Ak, clientConfig.Sk)
		}
		up(100)
	} else {
		res, err := bosClient.BasicInitiateMultipartUpload(result.Bucket, result.Key)
		// 分块大小按MULTIPART_ALIGN=1MB对齐
		partSize := (bosClient.MultipartSize +
			bos.MULTIPART_ALIGN - 1) / bos.MULTIPART_ALIGN * bos.MULTIPART_ALIGN

		// 获取文件大小，并计算分块数目，最大分块数MAX_PART_NUMBER=10000
		fileSize := fileStream.GetSize()
		partNum := (fileSize + partSize - 1) / partSize
		if partNum > bos.MAX_PART_NUMBER { // 超过最大分块数，需调整分块大小
			partSize = (fileSize + bos.MAX_PART_NUMBER + 1) / bos.MAX_PART_NUMBER
			partSize = (partSize + bos.MULTIPART_ALIGN - 1) / bos.MULTIPART_ALIGN * bos.MULTIPART_ALIGN
			partNum = (fileSize + partSize - 1) / partSize
		}
		// 创建保存每个分块上传后的ETag和PartNumber信息的列表
		partEtags := make([]api.UploadInfoType, 0)

		// 逐个分块上传
		for i := int64(1); i <= partNum; i++ {
			// 计算偏移offset和本次上传的大小uploadSize
			uploadSize := partSize
			offset := partSize * (i - 1)
			left := fileSize - offset
			if left < partSize {
				uploadSize = left
			}

			// 创建指定大小的文件流
			partBody, _ := bce.NewBodyFromSizedReader(tempFile, uploadSize)

			// 上传当前分块
			etag, _ := bosClient.BasicUploadPart(result.Bucket, result.Key, res.UploadId, int(i), partBody)

			// 保存当前分块上传成功后返回的序号和ETag
			partEtags = append(partEtags, api.UploadInfoType{int(i), etag})

			up(float64(i) / float64(partNum) * 100)
		}

		completeArgs := &api.CompleteMultipartUploadArgs{Parts: partEtags}
		_, err = bosClient.CompleteMultipartUploadFromStruct(
			result.Bucket, result.Key, res.UploadId, completeArgs)
		if err != nil {
			return nil, err
		}
	}
	return nil, nil
}

var _ driver.Driver = (*HalalCloud)(nil)
