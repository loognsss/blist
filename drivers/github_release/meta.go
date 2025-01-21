package template

import (
	"github.com/alist-org/alist/v3/internal/driver"
	"github.com/alist-org/alist/v3/internal/op"
)

type Addition struct {
	driver.RootID
	// define other
	Repo        string `json:"repo" required:"true" default:"AlistGo/alist" help:"Repository name(owner/repo)"`
	Token       string `json:"token" required:"true" default:"" help:"Github personal access token"`
	MaxReleases int    `json:"max_releases" required:"true" type:"number" default:"30" help:"Max releases to list"`
}

var config = driver.Config{
	Name:              "Github Release",
	LocalSort:         false,
	OnlyLocal:         false,
	OnlyProxy:         false,
	NoCache:           false,
	NoUpload:          true,
	NeedMs:            false,
	DefaultRoot:       "0",
	CheckStatus:       false,
	Alert:             "",
	NoOverwriteUpload: false,
}

func init() {
	op.RegisterDriver(func() driver.Driver {
		return &GithubRelease{}
	})
}
