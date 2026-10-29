package model

import (
	"context"
	"time"

	"github.com/xiongpan828/beidou-layout/pkg/datatypex"
)

// File 文件记录：签发上传地址时登记（PENDING），经签名 URL 写入成功后 UPLOADED。
// key 由服务端随机生成、只写一次，文件不删除，所以 key 用普通唯一索引
type File struct {
	datatypex.Model
	Key          string `gorm:"type:varchar(1024);not null;uniqueIndex"`
	Filename     string `gorm:"type:varchar(255);not null;default:''"`
	Size         int64  `gorm:"not null;default:0"`
	ContentType  string `gorm:"type:varchar(100);not null;default:''"`
	Etag         string `gorm:"type:varchar(100);not null;default:''"`
	Status       string `gorm:"type:varchar(100);not null;default:FILE_STATUS_PENDING"` // FileStatus 枚举名
	Scope        string `gorm:"type:varchar(100);not null"`
	ProviderCode string `gorm:"type:varchar(100);not null;index"` // 写入时的供应商（scope 换供应商后历史文件仍从原供应商读）
}

// 文件状态（= common.FileStatus 枚举名）
const (
	FileStatusPending   = "FILE_STATUS_PENDING"
	FileStatusUploaded  = "FILE_STATUS_UPLOADED"
	FileStatusUploading = "FILE_STATUS_UPLOADING"
)

// CreateUploadParam 签发上传地址参数
type CreateUploadParam struct {
	Scope    string
	Filename string
}

// UploadURL 上传地址
type UploadURL struct {
	Key      string
	URL      string
	ExpireAt time.Time
}

// Signature 签名 URL 上的参数（?expire=&key_id=&sign=）
type Signature struct {
	Expire int64
	KeyID  string
	Sign   string
}

// UploadedObject 写入对象存储后的结果
type UploadedObject struct {
	Size        int64
	Etag        string
	ContentType string
}

// IFileBiz 文件业务接口
type IFileBiz interface {
	// CreateUploadURL 登记 PENDING 文件并签发上传地址
	CreateUploadURL(ctx context.Context, param CreateUploadParam) (*UploadURL, error)
	// GetFile 已上传的文件；PENDING 的先查一次对象存储（写入成功但状态没更新时补上），仍不存在返回 FILE_NOT_FOUND
	GetFile(ctx context.Context, key string) (*File, error)
	// FileURL 访问地址：公有读 scope 返回稳定地址，私有返回限时签名地址
	FileURL(file *File) string
	// PublicRead 文件是否公有读（以当前 scope 配置为准）
	PublicRead(file *File) bool
	// VerifySignature 校验签名 URL（method 参与签名，上传 / 下载签名不能互用）
	VerifySignature(method, key string, sig Signature) error
	// PrepareUpload 占位 PENDING → UPLOADING 并返回供应商预签名上传地址（一个 key 只能写一次）
	PrepareUpload(ctx context.Context, key string) (*File, string, error)
	// MarkUploaded 写入成功：PENDING / UPLOADING → UPLOADED，回填大小 / etag / 类型
	MarkUploaded(ctx context.Context, file *File, obj UploadedObject) (*File, error)
	// AbortUpload 写入失败：UPLOADING → PENDING，签名没过期时可以重试
	AbortUpload(ctx context.Context, file *File)
	// ProviderDownloadURL 供应商预签名下载地址（服务端代理读取用，不给客户端）
	ProviderDownloadURL(ctx context.Context, file *File) (string, error)
}
