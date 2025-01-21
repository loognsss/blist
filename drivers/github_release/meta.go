package template

import (
	"github.com/alist-org/alist/v3/internal/driver"
	"github.com/alist-org/alist/v3/internal/op"
)

type Addition struct {
	driver.RootID
	// define other
	Repo        string `json:"repo" required:"true" default:"AlistGo/alist"`
	Token       string `json:"token" required:"true" default:""`
	MaxReleases int    `json:"max_releases" required:"true" type:"number" default:"30" help:"max releases to list"`
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
