package storagex_test

import (
	"context"
	"strings"
	"testing"
	"time"

	storagesvc "github.com/xiongpan828/beidou-layout/api/gen/go/service/storage_service"
	"github.com/xiongpan828/beidou-layout/pkg/storagex"
	_ "github.com/xiongpan828/beidou-layout/pkg/storagex/minio"
	_ "github.com/xiongpan828/beidou-layout/pkg/storagex/s3"
)

const minioJSON = `{"endpoint":"127.0.0.1:9000","access_key_id":"ak","secret_access_key":"sk-old","bucket":"files","region":"us-east-1"}`

func TestNormalizeAndRedact(t *testing.T) {
	// 未知类型 / 坏 JSON / 未知字段 / 缺必填 都在解码 + buf validate 阶段拦下
	for _, tc := range []struct {
		typ, cfg string
		is       func(error) bool
	}{
		{"oss", minioJSON, storagesvc.IsProviderTypeNotSupported},
		{"minio", `{`, storagesvc.IsProviderConfigInvalid},
		{"minio", `{"endpoint":"x","typo":1}`, storagesvc.IsProviderConfigInvalid},
		{"minio", `{"endpoint":"x","access_key_id":"ak","secret_access_key":"sk"}`, storagesvc.IsProviderConfigInvalid},
		{"s3", `{"access_key_id":"ak","secret_access_key":"sk","bucket":"b"}`, storagesvc.IsProviderConfigInvalid},
	} {
		if _, err := storagex.Normalize(tc.typ, tc.cfg, ""); !tc.is(err) {
			t.Errorf("%s %s: err = %v", tc.typ, tc.cfg, err)
		}
	}

	stored, err := storagex.Normalize("minio", minioJSON, "")
	if err != nil {
		t.Fatal(err)
	}
	// 回显不带密钥
	redacted := storagex.Redact("minio", stored)
	if strings.Contains(redacted, "sk-old") || !strings.Contains(redacted, `"bucket":"files"`) {
		t.Fatalf("redacted = %s", redacted)
	}
	// 更新时密钥留空 = 沿用旧值；填了就用新值
	kept, err := storagex.Normalize("minio", strings.Replace(redacted, `"files"`, `"files-2"`, 1), stored)
	if err != nil || !strings.Contains(kept, "sk-old") || !strings.Contains(kept, "files-2") {
		t.Fatalf("keep secret: %s %v", kept, err)
	}
	changed, err := storagex.Normalize("minio", strings.Replace(minioJSON, "sk-old", "sk-new", 1), stored)
	if err != nil || !strings.Contains(changed, "sk-new") {
		t.Fatalf("change secret: %s %v", changed, err)
	}
	// 新建时密钥必填
	if _, err := storagex.Normalize("minio", redacted, ""); !storagesvc.IsProviderConfigInvalid(err) {
		t.Fatalf("create without secret: %v", err)
	}
}

// 预签名是本地计算（配置了 region 就不查桶位置），不需要真实的对象存储
func TestPresign(t *testing.T) {
	for typ, cfg := range map[string]string{
		"minio": minioJSON,
		"s3":    `{"endpoint":"http://127.0.0.1:9000","access_key_id":"ak","secret_access_key":"sk","bucket":"files","region":"us-east-1","use_path_style":true}`,
	} {
		p, err := storagex.New(typ, cfg)
		if err != nil {
			t.Fatalf("%s: %v", typ, err)
		}
		put, err := p.PresignPut(context.Background(), "a/b.jpg", time.Minute)
		if err != nil || !strings.HasPrefix(put, "http://127.0.0.1:9000/files/a/b.jpg?") || !strings.Contains(put, "X-Amz-Signature=") {
			t.Fatalf("%s put: %s %v", typ, put, err)
		}
		if strings.Contains(put, "checksum") {
			t.Fatalf("%s put url must not pin a checksum: %s", typ, put)
		}
		get, err := p.PresignGet(context.Background(), "a/b.jpg", time.Minute)
		if err != nil || !strings.Contains(get, "X-Amz-Expires=60") {
			t.Fatalf("%s get: %s %v", typ, get, err)
		}
	}
}
