package thunder_browser

import (
	"context"
	"fmt"
	"github.com/alist-org/alist/v3/drivers/base"
	"github.com/alist-org/alist/v3/internal/driver"
	"github.com/alist-org/alist/v3/internal/errs"
	"github.com/alist-org/alist/v3/internal/model"
	"github.com/alist-org/alist/v3/internal/op"
	"github.com/alist-org/alist/v3/pkg/utils"
	hash_extend "github.com/alist-org/alist/v3/pkg/utils/hash"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/go-resty/resty/v2"
	"net/http"
	"regexp"
	"strings"
)

type ThunderBrowser struct {
	*XunLeiBrowserCommon
	model.Storage
	Addition

	identity string
}

func (x *ThunderBrowser) Config() driver.Config {
	return config
}

func (x *ThunderBrowser) GetAddition() driver.Additional {
	return &x.Addition
}

func (x *ThunderBrowser) Init(ctx context.Context) (err error) {
	// 初始化所需参数
	if x.XunLeiBrowserCommon == nil {
		x.XunLeiBrowserCommon = &XunLeiBrowserCommon{
			Common: &Common{
				client: base.NewRestyClient(),
				Algorithms: []string{
					"x+I5XiTByg",
					"6QU1x5DqGAV3JKg6h",
					"VI1vL1WXr7st0es",
					"n+/3yhlrnKs4ewhLgZhZ5ITpt554",
					"UOip2PE7BLIEov/ZX6VOnsz",
					"Q70h9lpViNCOC8sGVkar9o22LhBTjfP",
					"IVHFuB1JcMlaZHnW",
					"bKE",
					"HZRbwxOiQx+diNopi6Nu",
					"fwyasXgYL3rP314331b",
					"LWxXAiSW4",
					"UlWIjv1HGrC6Ngmt4Nohx",
					"FOa+Lc0bxTDpTwIh2",
					"0+RY",
					"xmRVMqokHHpvsiH0",
				},
				DeviceID:          utils.GetMD5EncodeStr(x.Username + x.Password),
				ClientID:          "ZUBzD9J_XPXfn7f7",
				ClientSecret:      "yESVmHecEe6F0aou69vl-g",
				ClientVersion:     "1.0.7.1938",
				PackageName:       "com.xunlei.browser",
				UserAgent:         "ANDROID-com.xunlei.browser/1.0.7.1938 netWorkType/5G appid/22062 deviceName/Xiaomi_M2004j7ac deviceModel/M2004J7AC OSVersion/12 protocolVersion/301 platformVersion/10 sdkVersion/233100 Oauth2Client/0.9 (Linux 4_14_186-perf-gddfs8vbb238b) (JAVA 0)",
				DownloadUserAgent: "AndroidDownloadManager/12 (Linux; U; Android 12; M2004J7AC Build/SP1A.210812.016)",
				UseVideoUrl:       x.UseVideoUrl,

				refreshCTokenCk: func(token string) {
					x.CaptchaToken = token
					op.MustSaveDriverStorage(x)
				},
			},
			refreshTokenFunc: func() error {
				// 通过RefreshToken刷新
				token, err := x.RefreshToken(x.TokenResp.RefreshToken)
				if err != nil {
					// 重新登录
					token, err = x.Login(x.Username, x.Password)
					if err != nil {
						x.GetStorage().SetStatus(fmt.Sprintf("%+v", err.Error()))
						op.MustSaveDriverStorage(x)
					}
				}
				x.SetTokenResp(token)
				return err
			},
		}
	}

	// 自定义验证码token
	ctoekn := strings.TrimSpace(x.CaptchaToken)
	if ctoekn != "" {
		x.SetCaptchaToken(ctoekn)
	}
	x.XunLeiBrowserCommon.UseVideoUrl = x.UseVideoUrl
	x.Addition.RootFolderID = x.RootFolderID
	// 防止重复登录
	identity := x.GetIdentity()
	if x.identity != identity || !x.IsLogin() {
		x.identity = identity
		// 登录
		token, err := x.Login(x.Username, x.Password)
		if err != nil {
			return err
		}
		x.SetTokenResp(token)
	}
	return nil
}

func (x *ThunderBrowser) Drop(ctx context.Context) error {
	return nil
}

type ThunderBrowserExpert struct {
	*XunLeiBrowserCommon
	model.Storage
	ExpertAddition

	identity string
}

func (x *ThunderBrowserExpert) Config() driver.Config {
	return configExpert
}

func (x *ThunderBrowserExpert) GetAddition() driver.Additional {
	return &x.ExpertAddition
}

func (x *ThunderBrowserExpert) Init(ctx context.Context) (err error) {
	// 防止重复登录
	identity := x.GetIdentity()
	if identity != x.identity || !x.IsLogin() {
		x.identity = identity
		x.XunLeiBrowserCommon = &XunLeiBrowserCommon{
			Common: &Common{
				client: base.NewRestyClient(),

				DeviceID: func() string {
					if len(x.DeviceID) != 32 {
						return utils.GetMD5EncodeStr(x.DeviceID)
					}
					return x.DeviceID
				}(),
				ClientID:          x.ClientID,
				ClientSecret:      x.ClientSecret,
				ClientVersion:     x.ClientVersion,
				PackageName:       x.PackageName,
				UserAgent:         x.UserAgent,
				DownloadUserAgent: x.DownloadUserAgent,
				UseVideoUrl:       x.UseVideoUrl,

				refreshCTokenCk: func(token string) {
					x.CaptchaToken = token
					op.MustSaveDriverStorage(x)
				},
			},
		}

		if x.CaptchaToken != "" {
			x.SetCaptchaToken(x.CaptchaToken)
		}
		x.XunLeiBrowserCommon.UseVideoUrl = x.UseVideoUrl
		x.ExpertAddition.RootFolderID = x.RootFolderID
		// 签名方法
		if x.SignType == "captcha_sign" {
			x.Common.Timestamp = x.Timestamp
			x.Common.CaptchaSign = x.CaptchaSign
		} else {
			x.Common.Algorithms = strings.Split(x.Algorithms, ",")
		}

		// 登录方式
		if x.LoginType == "refresh_token" {
			// 通过RefreshToken登录
			token, err := x.XunLeiBrowserCommon.RefreshToken(x.ExpertAddition.RefreshToken)
			if err != nil {
				return err
			}
			x.SetTokenResp(token)

			// 刷新token方法
			x.SetRefreshTokenFunc(func() error {
				token, err := x.XunLeiBrowserCommon.RefreshToken(x.TokenResp.RefreshToken)
				if err != nil {
					x.GetStorage().SetStatus(fmt.Sprintf("%+v", err.Error()))
				}
				x.SetTokenResp(token)
				op.MustSaveDriverStorage(x)
				return err
			})
		} else {
			// 通过用户密码登录
			token, err := x.Login(x.Username, x.Password)
			if err != nil {
				return err
			}
			x.SetTokenResp(token)
			x.SetRefreshTokenFunc(func() error {
				token, err := x.XunLeiBrowserCommon.RefreshToken(x.TokenResp.RefreshToken)
				if err != nil {
					token, err = x.Login(x.Username, x.Password)
					if err != nil {
						x.GetStorage().SetStatus(fmt.Sprintf("%+v", err.Error()))
					}
				}
				x.SetTokenResp(token)
				op.MustSaveDriverStorage(x)
				return err
			})
		}
	} else {
		// 仅修改验证码token
		if x.CaptchaToken != "" {
			x.SetCaptchaToken(x.CaptchaToken)
		}
		x.XunLeiBrowserCommon.UserAgent = x.UserAgent
		x.XunLeiBrowserCommon.DownloadUserAgent = x.DownloadUserAgent
		x.XunLeiBrowserCommon.UseVideoUrl = x.UseVideoUrl
		x.ExpertAddition.RootFolderID = x.RootFolderID
	}
	return nil
}

func (x *ThunderBrowserExpert) Drop(ctx context.Context) error {
	return nil
}

func (x *ThunderBrowserExpert) SetTokenResp(token *TokenResp) {
	x.XunLeiBrowserCommon.SetTokenResp(token)
	if token != nil {
		x.ExpertAddition.RefreshToken = token.RefreshToken
	}
}

type XunLeiBrowserCommon struct {
	*Common
	*TokenResp // 登录信息

	refreshTokenFunc func() error
}

func (xc *XunLeiBrowserCommon) List(ctx context.Context, dir model.Obj, args model.ListArgs) ([]model.Obj, error) {
	return xc.getFiles(ctx, dir.GetID(), args.ReqPath)
}

func (xc *XunLeiBrowserCommon) Link(ctx context.Context, file model.Obj, args model.LinkArgs) (*model.Link, error) {
	var lFile Files

	// 对 迅雷云盘 内的文件 特殊处理
	params := map[string]string{
		"_magic":         "2021",
		"space":          "SPACE_BROWSER",
		"thumbnail_size": "SIZE_LARGE",
		"with":           "url",
	}
	if file.GetPath() == "true" {
		params = map[string]string{}
	}

	_, err := xc.Request(FILE_API_URL+"/{fileID}", http.MethodGet, func(r *resty.Request) {
		r.SetContext(ctx)
		r.SetPathParam("fileID", file.GetID())
		r.SetQueryParams(params)
		//r.SetQueryParam("space", "")
	}, &lFile)
	if err != nil {
		return nil, err
	}
	link := &model.Link{
		URL: lFile.WebContentLink,
		Header: http.Header{
			"User-Agent": {xc.DownloadUserAgent},
		},
	}

	if xc.UseVideoUrl {
		for _, media := range lFile.Medias {
			if media.Link.URL != "" {
				link.URL = media.Link.URL
				break
			}
		}
	}
	return link, nil
}

func (xc *XunLeiBrowserCommon) MakeDir(ctx context.Context, parentDir model.Obj, dirName string) error {
	js := base.Json{
		"kind":      FOLDER,
		"name":      dirName,
		"parent_id": parentDir.GetID(),
		"space":     "SPACE_BROWSER",
	}
	if parentDir.GetPath() == "true" {
		js = base.Json{
			"kind":      FOLDER,
			"name":      dirName,
			"parent_id": parentDir.GetID(),
		}
	}
	_, err := xc.Request(FILE_API_URL, http.MethodPost, func(r *resty.Request) {
		r.SetContext(ctx)
		r.SetBody(&js)
	}, nil)
	return err
}

func (xc *XunLeiBrowserCommon) Move(ctx context.Context, srcObj, dstDir model.Obj) error {

	// 对 迅雷云盘 内的文件 特殊处理
	params := map[string]string{
		"_from": "SPACE_BROWSER",
	}
	js := base.Json{
		"to":    base.Json{"parent_id": dstDir.GetID(), "space": "SPACE_BROWSER"},
		"space": "SPACE_BROWSER",
		"ids":   []string{srcObj.GetID()},
	}

	if srcObj.GetPath() == "true" {
		params = map[string]string{}
		js = base.Json{
			"to":  base.Json{"parent_id": dstDir.GetID()},
			"ids": []string{srcObj.GetID()},
		}
	}

	_, err := xc.Request(FILE_API_URL+":batchMove", http.MethodPost, func(r *resty.Request) {
		r.SetContext(ctx)
		r.SetBody(&js)
		r.SetQueryParams(params)
	}, nil)
	return err
}

func (xc *XunLeiBrowserCommon) Rename(ctx context.Context, srcObj model.Obj, newName string) error {

	// 对 迅雷云盘 内的文件 特殊处理
	params := map[string]string{
		"space": "SPACE_BROWSER",
	}

	if srcObj.GetPath() == "true" {
		params = map[string]string{}
	}

	_, err := xc.Request(FILE_API_URL+"/{fileID}", http.MethodPatch, func(r *resty.Request) {
		r.SetContext(ctx)
		r.SetPathParam("fileID", srcObj.GetID())
		r.SetBody(&base.Json{"name": newName})
		r.SetQueryParams(params)
	}, nil)
	return err
}

func (xc *XunLeiBrowserCommon) Copy(ctx context.Context, srcObj, dstDir model.Obj) error {
	// 对 迅雷云盘 内的文件 特殊处理
	params := map[string]string{
		"_from": "SPACE_BROWSER",
	}
	js := base.Json{
		"to":    base.Json{"parent_id": dstDir.GetID(), "space": "SPACE_BROWSER"},
		"space": "SPACE_BROWSER",
		"ids":   []string{srcObj.GetID()},
	}

	if srcObj.GetPath() == "true" {
		params = map[string]string{}
		js = base.Json{
			"to":  base.Json{"parent_id": dstDir.GetID()},
			"ids": []string{srcObj.GetID()},
		}
	}

	_, err := xc.Request(FILE_API_URL+":batchCopy", http.MethodPost, func(r *resty.Request) {
		r.SetContext(ctx)
		r.SetBody(&js)
		r.SetQueryParams(params)
	}, nil)
	return err
}

func (xc *XunLeiBrowserCommon) Remove(ctx context.Context, obj model.Obj) error {
	// 对 迅雷云盘 内的文件 特殊处理
	js := base.Json{
		"ids":   []string{obj.GetID()},
		"space": "SPACE_BROWSER",
	}

	if obj.GetPath() == "true" {
		js = base.Json{
			"ids": []string{obj.GetID()},
		}
	}

	if xc.RemoveWay == "delete" {
		_, err := xc.Request(FILE_API_URL+"/{fileID}/trash", http.MethodPatch, func(r *resty.Request) {
			r.SetContext(ctx)
			r.SetPathParam("fileID", obj.GetID())
			r.SetBody("{}")
		}, nil)
		return err
	}

	_, err := xc.Request(FILE_API_URL+":batchTrash", http.MethodPost, func(r *resty.Request) {
		r.SetContext(ctx)
		r.SetBody(&js)
	}, nil)
	return err

}

func (xc *XunLeiBrowserCommon) Put(ctx context.Context, dstDir model.Obj, stream model.FileStreamer, up driver.UpdateProgress) error {
	hi := stream.GetHash()
	gcid := hi.GetHash(hash_extend.GCID)
	if len(gcid) < hash_extend.GCID.Width {
		tFile, err := stream.CacheFullInTempFile()
		if err != nil {
			return err
		}

		gcid, err = utils.HashFile(hash_extend.GCID, tFile, stream.GetSize())
		if err != nil {
			return err
		}
	}

	// 对 迅雷云盘 内的文件 特殊处理
	js := base.Json{
		"kind":        FILE,
		"parent_id":   dstDir.GetID(),
		"name":        stream.GetName(),
		"size":        stream.GetSize(),
		"hash":        gcid,
		"upload_type": UPLOAD_TYPE_RESUMABLE,
		"space":       "SPACE_BROWSER",
	}
	if dstDir.GetPath() == "true" {
		js = base.Json{
			"kind":        FILE,
			"parent_id":   dstDir.GetID(),
			"name":        stream.GetName(),
			"size":        stream.GetSize(),
			"hash":        gcid,
			"upload_type": UPLOAD_TYPE_RESUMABLE,
		}
	}

	var resp UploadTaskResponse
	_, err := xc.Request(FILE_API_URL, http.MethodPost, func(r *resty.Request) {
		r.SetContext(ctx)
		r.SetBody(&js)
	}, &resp)
	if err != nil {
		return err
	}

	param := resp.Resumable.Params
	if resp.UploadType == UPLOAD_TYPE_RESUMABLE {
		param.Endpoint = strings.TrimLeft(param.Endpoint, param.Bucket+".")
		s, err := session.NewSession(&aws.Config{
			Credentials: credentials.NewStaticCredentials(param.AccessKeyID, param.AccessKeySecret, param.SecurityToken),
			Region:      aws.String("xunlei"),
			Endpoint:    aws.String(param.Endpoint),
		})
		if err != nil {
			return err
		}
		uploader := s3manager.NewUploader(s)
		if stream.GetSize() > s3manager.MaxUploadParts*s3manager.DefaultUploadPartSize {
			uploader.PartSize = stream.GetSize() / (s3manager.MaxUploadParts - 1)
		}
		_, err = uploader.UploadWithContext(ctx, &s3manager.UploadInput{
			Bucket:  aws.String(param.Bucket),
			Key:     aws.String(param.Key),
			Expires: aws.Time(param.Expiration),
			Body:    stream,
		})
		return err
	}
	return nil
}

func (xc *XunLeiBrowserCommon) getFiles(ctx context.Context, folderId string, path string) ([]model.Obj, error) {
	files := make([]model.Obj, 0)
	var pageToken string
	for {
		var fileList FileList
		params := map[string]string{
			"parent_id":      folderId,
			"page_token":     pageToken,
			"space":          "SPACE_BROWSER",
			"filters":        `{"trashed":{"eq":false}}`,
			"with_audit":     "true",
			"thumbnail_size": "SIZE_LARGE",
		}
		var IsThunderDriveFolder bool
		// 处理特殊目录 “迅雷云盘” 设置特殊的 params 以便正常访问

		pattern := fmt.Sprintf(`^/.*/%s(/.*)?$`, ThunderDriveFolderName)
		match, _ := regexp.MatchString(pattern, path)

		// 如果是 迅雷云盘 内的
		if folderId == ThunderDriveFileID || match {
			params = map[string]string{
				"space":      "",
				"__type":     "drive",
				"refresh":    "true",
				"__sync":     "true",
				"parent_id":  folderId,
				"page_token": pageToken,
				"with_audit": "true",
				"limit":      "100",
				"filters":    `{"phase":{"eq":"PHASE_TYPE_COMPLETE"},"trashed":{"eq":false}}`,
			}
			// 如果不是 迅雷云盘 根目录
			if folderId == ThunderDriveFileID {
				params["parent_id"] = ""
			}
			IsThunderDriveFolder = true
		}

		_, err := xc.Request(FILE_API_URL, http.MethodGet, func(r *resty.Request) {
			r.SetContext(ctx)
			r.SetQueryParams(params)
		}, &fileList)
		if err != nil {
			return nil, err
		}
		// 标记 迅雷网盘内的文件夹
		if IsThunderDriveFolder {
			fileList.ThunderDriveFolder = true
		}

		for i := 0; i < len(fileList.Files); i++ {
			file := &fileList.Files[i]
			// 标记 迅雷网盘内的文件
			if fileList.ThunderDriveFolder {
				file.ThunderDriveFolder = true
			}
			// 处理特殊目录 “迅雷云盘” 设置特殊的文件夹ID
			if file.Name == "迅雷云盘" && file.ID == "" {
				file.ID = ThunderDriveFileID
			}
			files = append(files, file)
		}

		if fileList.NextPageToken == "" {
			break
		}
		pageToken = fileList.NextPageToken
	}
	return files, nil
}

// SetRefreshTokenFunc 设置刷新Token的方法
func (xc *XunLeiBrowserCommon) SetRefreshTokenFunc(fn func() error) {
	xc.refreshTokenFunc = fn
}

// SetTokenResp 设置Token
func (xc *XunLeiBrowserCommon) SetTokenResp(tr *TokenResp) {
	xc.TokenResp = tr
}

// Request 携带Authorization和CaptchaToken的请求
func (xc *XunLeiBrowserCommon) Request(url string, method string, callback base.ReqCallback, resp interface{}) ([]byte, error) {
	data, err := xc.Common.Request(url, method, func(req *resty.Request) {
		req.SetHeaders(map[string]string{
			"Authorization":   xc.Token(),
			"X-Captcha-Token": xc.GetCaptchaToken(),
		})
		if callback != nil {
			callback(req)
		}
	}, resp)

	errResp, ok := err.(*ErrResp)
	if !ok {
		return nil, err
	}

	switch errResp.ErrorCode {
	case 0:
		return data, nil
	case 4122, 4121, 10, 16:
		if xc.refreshTokenFunc != nil {
			if err = xc.refreshTokenFunc(); err == nil {
				break
			}
		}
		return nil, err
	case 9: // 验证码token过期
		if err = xc.RefreshCaptchaTokenAtLogin(GetAction(method, url), xc.UserID); err != nil {
			return nil, err
		}
	default:
		return nil, err
	}
	return xc.Request(url, method, callback, resp)
}

// 刷新Token
func (xc *XunLeiBrowserCommon) RefreshToken(refreshToken string) (*TokenResp, error) {
	var resp TokenResp
	_, err := xc.Common.Request(XLUSER_API_URL+"/auth/token", http.MethodPost, func(req *resty.Request) {
		req.SetBody(&base.Json{
			"grant_type":    "refresh_token",
			"refresh_token": refreshToken,
			"client_id":     xc.ClientID,
			"client_secret": xc.ClientSecret,
		})
	}, &resp)
	if err != nil {
		return nil, err
	}

	if resp.RefreshToken == "" {
		return nil, errs.EmptyToken
	}
	return &resp, nil
}

// 登录
func (xc *XunLeiBrowserCommon) Login(username, password string) (*TokenResp, error) {
	url := XLUSER_API_URL + "/auth/signin"
	err := xc.RefreshCaptchaTokenInLogin(GetAction(http.MethodPost, url), username)
	if err != nil {
		return nil, err
	}

	var resp TokenResp
	_, err = xc.Common.Request(url, http.MethodPost, func(req *resty.Request) {
		req.SetBody(&SignInRequest{
			CaptchaToken: xc.GetCaptchaToken(),
			ClientID:     xc.ClientID,
			ClientSecret: xc.ClientSecret,
			Username:     username,
			Password:     password,
		})
	}, &resp)
	if err != nil {
		return nil, err
	}
	return &resp, nil
}

func (xc *XunLeiBrowserCommon) IsLogin() bool {
	if xc.TokenResp == nil {
		return false
	}
	_, err := xc.Request(XLUSER_API_URL+"/user/me", http.MethodGet, nil, nil)
	return err == nil
}
