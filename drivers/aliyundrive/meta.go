package aliyundrive

import (
	"github.com/alist-org/alist/v3/internal/driver"
	"github.com/alist-org/alist/v3/internal/op"
)

type Addition struct {
	driver.RootID
	RefreshToken string `json:"refresh_token" required:"true"`
	//DeviceID       string `json:"device_id" required:"true"`
	OrderBy        string `json:"order_by" type:"select" options:"name,size,updated_at,created_at"`
	OrderDirection string `json:"order_direction" type:"select" options:"ASC,DESC"`
	RapidUpload    bool   `json:"rapid_upload"`
	InternalUpload bool   `json:"internal_upload"`
	DriveType      string `json:"drive_type" type:"select" options:"default,resource,backup" default:"default"`
	HookAddress    string `json:"hook_address"`
	UserAgent      string `json:"user_agent" default:"AliApp(AYSD/6.4.0) com.alicloud.databox/40180722 Channel/36176927979800@rimet_android_6.4.0 language/zh-CN /Android Mobile/samsung samsung+SM-G9810"`
	XCanary        string `json:"x_canary" default:"client=Android,app=adrive,version=v6.4.0"`
}

var config = driver.Config{
	Name:        "Aliyundrive",
	DefaultRoot: "root",
	Alert: `warning|There may be an infinite loop bug in this driver.
Deprecated, no longer maintained and will be removed in a future version.
We recommend using the official driver AliyundriveOpen.`,
}

func init() {
	op.RegisterDriver(func() driver.Driver {
		return &AliDrive{}
	})
}
