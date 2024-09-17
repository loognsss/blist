package _115

import (
	"github.com/alist-org/alist/v3/internal/model"
	"github.com/alist-org/alist/v3/pkg/utils"
	"github.com/deadblue/elevengo"
	"time"
)

var _ model.Obj = (*FileObj)(nil)

type FileObj struct {
	elevengo.File
}

func (f *FileObj) GetSize() int64 {
	return f.File.Size
}

func (f *FileObj) GetName() string {
	return f.File.Name
}

func (f *FileObj) ModTime() time.Time {
	return f.File.ModifiedTime
}

func (f *FileObj) IsDir() bool {
	return f.File.IsDirectory
}

func (f *FileObj) GetID() string {
	return f.File.FileId
}

func (f *FileObj) GetPath() string {
	return ""
}

func (f *FileObj) CreateTime() time.Time {
	return f.File.ModifiedTime
}

func (f *FileObj) GetHash() utils.HashInfo {
	return utils.NewHashInfo(utils.SHA1, f.File.Sha1)
}
