package halalcloud

import (
	"bytes"
	"context"
	"crypto/md5"
	"crypto/tls"
	"encoding/hex"
	"errors"
	"fmt"
	"github.com/alist-org/alist/v3/internal/model"
	"github.com/alist-org/alist/v3/pkg/utils"
	pbPublicUser "github.com/city404/v6-public-rpc-proto/go/v6/user"
	pubUserFile "github.com/city404/v6-public-rpc-proto/go/v6/userfile"
	"github.com/google/uuid"
	"github.com/ipfs/go-cid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"hash"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	AppID      = "devDebugger/1.0"
	AppVersion = "1.0.0"
	AppSecret  = "Nkx3Y2xvZ2luLmNu"
)

const (
	grpcServer     = "grpcuserapi.2dland.cn:443"
	grpcServerAuth = "grpcuserapi.2dland.cn"
)

func (d *HalalCloud) NewAuthServiceWithOauth(options ...HalalOption) (*AuthService, error) {

	aService := &AuthService{}
	err2 := errors.New("")

	svc := d.HalalCommon.AuthService
	for _, opt := range options {
		opt.apply(&svc.dopts)
	}

	grpcOptions := svc.dopts.grpcOptions
	grpcOptions = append(grpcOptions, grpc.WithAuthority(grpcServerAuth), grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{})), grpc.WithUnaryInterceptor(func(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		ctxx := svc.signContext(method, ctx)
		err := invoker(ctxx, method, req, reply, cc, opts...) // invoking RPC method
		return err
	}))

	grpcConnection, err := grpc.NewClient(grpcServer, grpcOptions...)
	if err != nil {
		return nil, err
	}
	defer grpcConnection.Close()
	userClient := pbPublicUser.NewPubUserClient(grpcConnection)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	stateString := uuid.New().String()
	// queryValues.Add("callback", oauthToken.Callback)
	oauthToken, err := userClient.CreateAuthToken(ctx, &pbPublicUser.LoginRequest{
		ReturnType: 2,
		State:      stateString,
		ReturnUrl:  "",
	})
	if err != nil {
		return nil, err
	}
	if len(oauthToken.State) < 1 {
		oauthToken.State = stateString
	}

	if oauthToken.Url != "" {
		/*		resultChan := make(chan *AuthService, 1)
				errorChan := make(chan error, 1)

				go func() {
					aService, err := d.GetRefreshToken(svc, &userClient, oauthToken)
					if err != nil {
						errorChan <- err
					} else {
						resultChan <- aService
					}
				}()*/
		return nil, fmt.Errorf(`need verify: <a target="_blank" href="%s">Click Here</a>`, oauthToken.Url)
	}

	return aService, err2

}

/*
	func (d *HalalCloud) GetRefreshToken(svc *AuthService, userClient *pbPublicUser.PubUserClient, oauthToken *pbPublicUser.OauthTokenResponse) (*AuthService, error) {
		checkTimer := time.NewTicker(5 * time.Second)
		defer checkTimer.Stop()
		for {
			select {
			case <-checkTimer.C:

				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				checkLoginResponse, err := (*userClient).VerifyAuthToken(ctx, &pbPublicUser.LoginRequest{
					State:      oauthToken.State,
					Callback:   oauthToken.Callback,
					ReturnType: 2,
				})
				if err != nil {
					return nil, err
				}
				if checkLoginResponse.Status == 6 {
					login := checkLoginResponse.Login
					if login == nil {
						return nil, fmt.Errorf("login is nil")
					}
					if login.User != nil && len(login.Token.RefreshToken) > 0 {
						// checkLoginResponse = checkLoginResponse
						_ = d.refreshTokenFunc(login.Token.RefreshToken)
						svc.OnAccessTokenRefreshed(login.Token.AccessToken, login.Token.AccessTokenExpireTs, login.Token.RefreshToken, login.Token.RefreshTokenExpireTs)
						newAuthService, err := d.NewAuthService(login.Token.RefreshToken)
						if err != nil {
							return nil, err
						}
						return newAuthService, nil
						// break
					}
				}

				// reset timer
				checkTimer.Reset(5 * time.Second)

			}
		}
	}
*/
func (d *HalalCloud) NewAuthService(refreshToken string, options ...HalalOption) (*AuthService, error) {
	svc := d.HalalCommon.AuthService

	if len(refreshToken) < 1 {
		refreshToken = d.Addition.RefreshToken
	}

	for _, opt := range options {
		opt.apply(&svc.dopts)
	}

	grpcOptions := svc.dopts.grpcOptions
	grpcOptions = append(grpcOptions, grpc.WithAuthority(grpcServerAuth), grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{})), grpc.WithUnaryInterceptor(func(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		ctxx := svc.signContext(method, ctx)
		err := invoker(ctxx, method, req, reply, cc, opts...) // invoking RPC method
		if err != nil {
			grpcStatus, ok := status.FromError(err)
			// if error is grpc error and error code is unauthenticated and error message contains "invalid accesstoken" and refresh token is not empty
			// then refresh access token and retry
			if ok && grpcStatus.Code() == codes.Unauthenticated && strings.Contains(grpcStatus.Err().Error(), "invalid accesstoken") && len(refreshToken) > 0 {
				// refresh token
				refreshResponse, err := pbPublicUser.NewPubUserClient(cc).Refresh(ctx, &pbPublicUser.Token{
					RefreshToken: refreshToken,
				})
				if err != nil {
					return err
				}
				if len(refreshResponse.AccessToken) > 0 {
					svc.tr.AccessToken = refreshResponse.AccessToken
					svc.tr.AccessTokenExpiredAt = refreshResponse.AccessTokenExpireTs
					svc.OnAccessTokenRefreshed(refreshResponse.AccessToken, refreshResponse.AccessTokenExpireTs, refreshResponse.RefreshToken, refreshResponse.RefreshTokenExpireTs)
				}
				// retry
				ctxx := svc.signContext(method, ctx)
				err = invoker(ctxx, method, req, reply, cc, opts...) // invoking RPC method
				if err != nil {
					return err
				} else {
					return nil
				}
			}
		}
		// post-processing
		return err
	}))
	grpcConnection, err := grpc.NewClient(grpcServer, grpcOptions...)

	if err != nil {
		return nil, err
	}

	svc.grpcConnection = grpcConnection
	return svc, err
}

func (s *AuthService) OnAccessTokenRefreshed(accessToken string, accessTokenExpiredAt int64, refreshToken string, refreshTokenExpiredAt int64) {
	s.tr.AccessToken = accessToken
	s.tr.AccessTokenExpiredAt = accessTokenExpiredAt
	s.tr.RefreshToken = refreshToken
	s.tr.RefreshTokenExpiredAt = refreshTokenExpiredAt

	if s.dopts.onTokenRefreshed != nil {
		s.dopts.onTokenRefreshed(accessToken, accessTokenExpiredAt, refreshToken, refreshTokenExpiredAt)
	}

}

func (s *AuthService) GetGrpcConnection() *grpc.ClientConn {
	return s.grpcConnection
}

func (s *AuthService) Close() {
	s.grpcConnection.Close()
}

func (s *AuthService) signContext(method string, ctx context.Context) context.Context {
	var kvString []string
	currentTimeStamp := strconv.FormatInt(time.Now().UnixMilli(), 10)
	bufferedString := bytes.NewBufferString(method)
	kvString = append(kvString, "timestamp", currentTimeStamp)
	bufferedString.WriteString(currentTimeStamp)
	kvString = append(kvString, "appid", AppID)
	bufferedString.WriteString(AppID)
	kvString = append(kvString, "appversion", AppVersion)
	bufferedString.WriteString(AppVersion)
	if s.tr != nil && len(s.tr.AccessToken) > 0 {
		authorization := "Bearer " + s.tr.AccessToken
		kvString = append(kvString, "authorization", authorization)
		bufferedString.WriteString(authorization)
	}
	bufferedString.WriteString(AppSecret)
	sign := GetMD5Hash(bufferedString.String())
	kvString = append(kvString, "sign", sign)
	return metadata.AppendToOutgoingContext(ctx, kvString...)
}

func (d *HalalCloud) GetCurrentOpDir(dir model.Obj, args []string, index int) string {
	currentDir := dir.GetPath()
	if len(currentDir) == 0 {
		currentDir = "/"
	}
	opPath := currentDir + "/" + args[index]
	if strings.HasPrefix(args[index], "/") {
		opPath = args[index]
	}
	return opPath
}

func (d *HalalCloud) GetCurrentDir(dir model.Obj) string {
	currentDir := dir.GetPath()
	if len(currentDir) == 0 {
		currentDir = "/"
	}
	return currentDir
}

type Common struct {
}

func tryAndGetRawFiles(addr *pubUserFile.SliceDownloadInfo) ([]byte, error) {
	tryTimes := 0
	for {
		tryTimes++
		dataBytes, err := getRawFiles(addr)
		if err != nil {
			if tryTimes > 3 {
				return nil, err
			}
			continue
		}
		return dataBytes, nil
	}
}

func getRawFiles(addr *pubUserFile.SliceDownloadInfo) ([]byte, error) {

	if addr == nil {
		return nil, errors.New("addr is nil")
	}

	client := http.Client{
		Timeout: time.Duration(60 * time.Second), // Set timeout to 5 seconds
	}
	resp, err := client.Get(addr.DownloadAddress)
	if err != nil {

		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("bad status: %s, body: %s", resp.Status, body)
	}

	if addr.Encrypt > 0 {
		cd := uint8(addr.Encrypt)
		for idx := 0; idx < len(body); idx++ {
			body[idx] = body[idx] ^ cd
		}
	}

	sourceCid, err := cid.Decode(addr.Identity)
	if err != nil {
		return nil, err
	}
	checkCid, err := sourceCid.Prefix().Sum(body)
	if err != nil {
		return nil, err
	}
	if !checkCid.Equals(sourceCid) {
		return nil, fmt.Errorf("bad cid: %s, body: %s", checkCid.String(), body)
	}

	return body, nil

	// Process response body

	// Do something with the response body
}

// do others that not defined in Driver interface
// openObject represents a download in progress
type openObject struct {
	ctx     context.Context
	mu      sync.Mutex
	d       []*pubUserFile.SliceDownloadInfo
	id      int
	skip    int64
	chunk   []byte
	closed  bool
	sha     string
	shaTemp hash.Hash
}

// get the next chunk
func (oo *openObject) getChunk(ctx context.Context) (err error) {
	if oo.id >= len(oo.d) {
		return io.EOF
	}
	var chunk []byte
	err = utils.Retry(3, time.Second, func() (err error) {
		chunk, err = getRawFiles(oo.d[oo.id])
		return err
	})
	if err != nil {
		return err
	}
	oo.id++
	oo.chunk = chunk
	return nil
}

// Read reads up to len(p) bytes into p.
func (oo *openObject) Read(p []byte) (n int, err error) {
	oo.mu.Lock()
	defer oo.mu.Unlock()
	if oo.closed {
		return 0, fmt.Errorf("read on closed file")
	}
	// Skip data at the start if requested
	for oo.skip > 0 {
		size := 1024 * 1024
		if oo.skip < int64(size) {
			break
		}
		oo.id++
		oo.skip -= int64(size)
	}
	if len(oo.chunk) == 0 {
		err = oo.getChunk(oo.ctx)
		if err != nil {
			return 0, err
		}
		if oo.skip > 0 {
			oo.chunk = oo.chunk[oo.skip:]
			oo.skip = 0
		}
	}
	n = copy(p, oo.chunk)
	oo.chunk = oo.chunk[n:]

	oo.shaTemp.Write(oo.chunk)

	return n, nil
}

// Close closed the file - MAC errors are reported here
func (oo *openObject) Close() (err error) {
	oo.mu.Lock()
	defer oo.mu.Unlock()
	if oo.closed {
		return nil
	}
	//err = utils.Retry(3, 500*time.Millisecond, func() (err error) {
	//	return oo.d.Finish()
	//})
	// 校验Sha1
	if string(oo.shaTemp.Sum(nil)) != oo.sha {
		return fmt.Errorf("failed to finish download: %w", err)
	}

	oo.closed = true
	return nil
}

func GetMD5Hash(text string) string {
	hash := md5.Sum([]byte(text))
	return hex.EncodeToString(hash[:])
}

const (
	SmallSliceSize  int64 = 1 * utils.MB
	MediumSliceSize       = 16 * utils.MB
	LargeSliceSize        = 32 * utils.MB
)

func (d *HalalCloud) getSliceSize() int64 {
	if d.CustomUploadPartSize != 0 {
		return d.CustomUploadPartSize
	}
	switch d.fileStatus {
	case 0:
		return SmallSliceSize
	case 1:
		return MediumSliceSize
	case 2:
		return LargeSliceSize
	default:
		return SmallSliceSize
	}
}

func (d *HalalCloud) setFileStatus(fileSize int64) {
	if fileSize <= 32*utils.MB {
		d.fileStatus = 0
	} else if fileSize <= 512*utils.MB {
		d.fileStatus = 1
	} else {
		d.fileStatus = 2
	}
}
