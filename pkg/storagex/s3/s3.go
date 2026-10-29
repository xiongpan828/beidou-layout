// Package s3 AWS S3 及兼容存储供应商（aws-sdk-go-v2）。
package s3

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"

	storagesvc "github.com/xiongpan828/beidou-layout/api/gen/go/service/storage_service"
	"github.com/xiongpan828/beidou-layout/pkg/storagex"
)

func init() {
	storagex.Register("s3", func() *storagesvc.S3Config { return &storagesvc.S3Config{} }, New)
}

type provider struct {
	client    *s3.Client
	presigner *s3.PresignClient
	bucket    string
}

// New 创建 S3 Provider（配置已校验；只用配置里的静态密钥，不读环境变量 / 实例角色）
func New(c *storagesvc.S3Config) (storagex.Provider, error) {
	client := s3.New(s3.Options{
		Region:       c.GetRegion(),
		Credentials:  credentials.NewStaticCredentialsProvider(c.GetAccessKeyId(), c.GetSecretAccessKey(), ""),
		UsePathStyle: c.GetUsePathStyle(),
		BaseEndpoint: nilIfEmpty(c.GetEndpoint()),
		// 默认会给预签名 PUT 加校验和参数（按空 body 算），代理转发真实内容时对不上；兼容存储也多数不支持
		RequestChecksumCalculation: aws.RequestChecksumCalculationWhenRequired,
	})
	return &provider{client: client, presigner: s3.NewPresignClient(client), bucket: c.GetBucket()}, nil
}

func nilIfEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return aws.String(s)
}

func (p *provider) PresignPut(ctx context.Context, key string, expire time.Duration) (string, error) {
	r, err := p.presigner.PresignPutObject(ctx, &s3.PutObjectInput{Bucket: &p.bucket, Key: &key}, s3.WithPresignExpires(expire))
	if err != nil {
		return "", storagesvc.ErrorObjectStorageFail("s3 presign put: %v", err)
	}
	return r.URL, nil
}

func (p *provider) PresignGet(ctx context.Context, key string, expire time.Duration) (string, error) {
	r, err := p.presigner.PresignGetObject(ctx, &s3.GetObjectInput{Bucket: &p.bucket, Key: &key}, s3.WithPresignExpires(expire))
	if err != nil {
		return "", storagesvc.ErrorObjectStorageFail("s3 presign get: %v", err)
	}
	return r.URL, nil
}

func (p *provider) Stat(ctx context.Context, key string) (*storagex.ObjectInfo, error) {
	out, err := p.client.HeadObject(ctx, &s3.HeadObjectInput{Bucket: &p.bucket, Key: &key})
	if err != nil {
		var notFound *types.NotFound
		if errors.As(err, &notFound) {
			return nil, storagesvc.ErrorFileNotFound("object %q not found", key)
		}
		return nil, storagesvc.ErrorObjectStorageFail("s3 head %q: %v", key, err)
	}
	return &storagex.ObjectInfo{
		Size:        aws.ToInt64(out.ContentLength),
		ETag:        strings.Trim(aws.ToString(out.ETag), `"`),
		ContentType: aws.ToString(out.ContentType),
	}, nil
}
