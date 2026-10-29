// Package minio MinIO 供应商（minio-go）。
package minio

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"

	storagesvc "github.com/xiongpan828/beidou-layout/api/gen/go/service/storage_service"
	"github.com/xiongpan828/beidou-layout/pkg/storagex"
)

func init() {
	storagex.Register("minio", func() *storagesvc.MinioConfig { return &storagesvc.MinioConfig{} }, New)
}

type provider struct {
	client *minio.Client
	bucket string
}

// New 创建 MinIO Provider（配置已校验；不连网络）
func New(c *storagesvc.MinioConfig) (storagex.Provider, error) {
	client, err := minio.New(c.GetEndpoint(), &minio.Options{
		Creds:  credentials.NewStaticV4(c.GetAccessKeyId(), c.GetSecretAccessKey(), ""),
		Secure: c.GetSecure(),
		Region: c.GetRegion(),
	})
	if err != nil {
		return nil, storagesvc.ErrorProviderConfigInvalid("minio: %v", err)
	}
	return &provider{client: client, bucket: c.GetBucket()}, nil
}

func (p *provider) PresignPut(ctx context.Context, key string, expire time.Duration) (string, error) {
	u, err := p.client.PresignedPutObject(ctx, p.bucket, key, expire)
	if err != nil {
		return "", storagesvc.ErrorObjectStorageFail("minio presign put: %v", err)
	}
	return u.String(), nil
}

func (p *provider) PresignGet(ctx context.Context, key string, expire time.Duration) (string, error) {
	u, err := p.client.PresignedGetObject(ctx, p.bucket, key, expire, nil)
	if err != nil {
		return "", storagesvc.ErrorObjectStorageFail("minio presign get: %v", err)
	}
	return u.String(), nil
}

func (p *provider) Stat(ctx context.Context, key string) (*storagex.ObjectInfo, error) {
	s, err := p.client.StatObject(ctx, p.bucket, key, minio.StatObjectOptions{})
	if err != nil {
		if minio.ToErrorResponse(err).StatusCode == http.StatusNotFound {
			return nil, storagesvc.ErrorFileNotFound("object %q not found", key)
		}
		return nil, storagesvc.ErrorObjectStorageFail("minio stat %q: %v", key, err)
	}
	return &storagex.ObjectInfo{Size: s.Size, ETag: strings.Trim(s.ETag, `"`), ContentType: s.ContentType}, nil
}
