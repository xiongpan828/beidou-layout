// Package storagex 对象存储供应商（provider）抽象与注册中心。
//
// 各实现（minio / s3）在 init 里 Register 自己：类型名 + 配置 proto + 构造函数。
// 供应商配置以 JSON 存库（provider.config），结构是 storage_service 里的 MinioConfig / S3Config，
// 本包负责 JSON 解码 + buf validate 校验，不手写字段校验。
package storagex

import (
	"context"
	"strings"
	"time"

	"buf.build/go/protovalidate"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"

	storagesvc "github.com/xiongpan828/beidou-layout/api/gen/go/service/storage_service"
)

// ObjectInfo 对象信息
type ObjectInfo struct {
	Size        int64
	ETag        string
	ContentType string
}

// Provider 对象存储能力（与具体 SDK 解耦）。
// 客户端不直连供应商：storage-service 拿预签名地址在服务端代理读写，签名地址只在服务内部使用
type Provider interface {
	// PresignPut 预签名上传地址（PUT）
	PresignPut(ctx context.Context, key string, expire time.Duration) (string, error)
	// PresignGet 预签名下载地址（GET）
	PresignGet(ctx context.Context, key string, expire time.Duration) (string, error)
	// Stat 对象信息；对象不存在返回 FILE_NOT_FOUND
	Stat(ctx context.Context, key string) (*ObjectInfo, error)
}

type factory struct {
	newConfig   func() proto.Message
	newProvider func(proto.Message) (Provider, error)
}

// registry 只在各实现包的 init 里写入，之后只读，不需要加锁
var registry = map[string]factory{}

// Register 注册供应商类型（只在 init 里调用）
func Register[T proto.Message](typ string, newConfig func() T, newProvider func(T) (Provider, error)) {
	registry[typ] = factory{
		newConfig:   func() proto.Message { return newConfig() },
		newProvider: func(m proto.Message) (Provider, error) { return newProvider(m.(T)) },
	}
}

// New 按类型和配置 JSON 创建 Provider（解码 + 校验）
func New(typ, configJSON string) (Provider, error) {
	f, cfg, err := decode(typ, configJSON)
	if err != nil {
		return nil, err
	}
	if err := validate(typ, cfg); err != nil {
		return nil, err
	}
	return f.newProvider(cfg)
}

// Normalize 校验配置 JSON 并返回规范化的 JSON（入库用）。
// oldJSON 非空（更新）时，新配置里为空的密钥字段沿用旧值：列表不回显密钥，前端改别的字段不用重填
func Normalize(typ, configJSON, oldJSON string) (string, error) {
	_, cfg, err := decode(typ, configJSON)
	if err != nil {
		return "", err
	}
	if oldJSON != "" {
		if _, old, err := decode(typ, oldJSON); err == nil {
			eachSecret(cfg, func(fd protoreflect.FieldDescriptor) {
				if cfg.ProtoReflect().Get(fd).String() == "" {
					cfg.ProtoReflect().Set(fd, old.ProtoReflect().Get(fd))
				}
			})
		}
	}
	if err := validate(typ, cfg); err != nil {
		return "", err
	}
	return marshal(cfg), nil
}

// Redact 去掉密钥字段（接口回显用）；解析失败返回 "{}"
func Redact(typ, configJSON string) string {
	_, cfg, err := decode(typ, configJSON)
	if err != nil {
		return "{}"
	}
	eachSecret(cfg, func(fd protoreflect.FieldDescriptor) { cfg.ProtoReflect().Clear(fd) })
	return marshal(cfg)
}

func decode(typ, configJSON string) (factory, proto.Message, error) {
	f, ok := registry[typ]
	if !ok {
		return f, nil, storagesvc.ErrorProviderTypeNotSupported("provider type %q not supported", typ)
	}
	cfg := f.newConfig()
	if err := protojson.Unmarshal([]byte(configJSON), cfg); err != nil {
		return f, nil, storagesvc.ErrorProviderConfigInvalid("%s config: invalid json: %v", typ, err)
	}
	return f, cfg, nil
}

func validate(typ string, cfg proto.Message) error {
	if err := protovalidate.Validate(cfg); err != nil {
		return storagesvc.ErrorProviderConfigInvalid("%s config: %v", typ, err)
	}
	return nil
}

func marshal(cfg proto.Message) string {
	b, _ := protojson.MarshalOptions{UseProtoNames: true}.Marshal(cfg)
	return string(b)
}

// eachSecret 配置里的密钥字段：约定字段名含 secret（secret_access_key 等）
func eachSecret(cfg proto.Message, fn func(protoreflect.FieldDescriptor)) {
	fields := cfg.ProtoReflect().Descriptor().Fields()
	for i := 0; i < fields.Len(); i++ {
		if fd := fields.Get(i); fd.Kind() == protoreflect.StringKind && strings.Contains(string(fd.Name()), "secret") {
			fn(fd)
		}
	}
}
