package template

import (
	"github.com/alist-org/alist/v3/internal/model"
	"io"
	"os"
	"time"

	"github.com/xhofe/wopan-sdk-go"
)

// do others that not defined in Driver interface

func (d *Wopan) getSortRule() int {
	switch d.SortRule {
	case "name_asc":
		return wopan.SortNameAsc
	case "name_desc":
		return wopan.SortNameDesc
	case "time_asc":
		return wopan.SortTimeAsc
	case "time_desc":
		return wopan.SortTimeDesc
	case "size_asc":
		return wopan.SortSizeAsc
	case "size_desc":
		return wopan.SortSizeDesc
	default:
		return wopan.SortNameAsc
	}
}

func (d *Wopan) getSpaceType() string {
	if d.FamilyID == "" {
		return wopan.SpaceTypePersonal
	}
	return wopan.SpaceTypeFamily
}

// 20230607214351
func getTime(str string) (time.Time, error) {
	return time.Parse("20060102150405", str)
}

// 将 FileStreamer 转换为 *os.File
func convertToFile(stream model.FileStreamer) (*os.File, error) {
	// 创建一个临时文件
	tmpFile, err := os.CreateTemp("", "stream-*.tmp")
	if err != nil {
		return nil, err
	}

	// 将 stream 的内容复制到临时文件中
	if _, err := io.Copy(tmpFile, stream); err != nil {
		tmpFile.Close()
		return nil, err
	}

	// 重新打开临时文件，以便读取
	if _, err := tmpFile.Seek(0, io.SeekStart); err != nil {
		tmpFile.Close()
		return nil, err
	}

	return tmpFile, nil
}
