package biz

import (
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"testing"
	"time"

	"google.golang.org/protobuf/types/known/durationpb"

	conf "github.com/xiongpan828/beidou-layout/api/gen/go/config/storage_service"
	common "github.com/xiongpan828/beidou-layout/api/gen/go/service/common"
	storagesvc "github.com/xiongpan828/beidou-layout/api/gen/go/service/storage_service"
	"github.com/xiongpan828/beidou-layout/app/service/storage-service/model"
)

func testBiz(baseURL string) *biz {
	return &biz{cfg: &conf.Config{
		BaseUrl: baseURL,
		SignKeys: []*conf.SignKey{
			{Id: "new", Secret: "new-secret-0123456789"},
			{Id: "old", Secret: "old-secret-0123456789"},
		},
		Scopes: map[string]*conf.Scope{
			"avatar":     {PublicRead: true},
			"attachment": {UrlTtl: durationpb.New(time.Hour)},
		},
	}}
}

// 库里存的状态就是 proto 枚举名
func TestFileStatusNames(t *testing.T) {
	if model.FileStatusPending != common.FileStatus_FILE_STATUS_PENDING.String() || model.FileStatusUploaded != common.FileStatus_FILE_STATUS_UPLOADED.String() {
		t.Fatal("file status constants must equal common.FileStatus names")
	}
}

func TestNewKey(t *testing.T) {
	// 日期目录按 UTC：东八区 10-29 00:30 = UTC 10-28 16:30
	now := time.Date(2026, 10, 29, 0, 30, 0, 0, time.FixedZone("UTC+8", 8*3600))
	k1, err := newKey("attachment", ".pdf", now)
	if err != nil {
		t.Fatal(err)
	}
	if !regexp.MustCompile(`^attachment/2026/10/28/[0-9a-f]{32}\.pdf$`).MatchString(k1) {
		t.Fatalf("key = %q", k1)
	}
	if k2, _ := newKey("attachment", ".pdf", now); k1 == k2 {
		t.Fatal("keys must be random")
	}
}

func TestAlignedExpire(t *testing.T) {
	ttl := 10 * time.Minute
	base := time.Unix(1_800_000_000, 0)
	e1, e2 := alignedExpire(base, ttl), alignedExpire(base.Add(time.Second), ttl)
	if e1 != e2 {
		t.Fatalf("same window must give same expire: %d %d", e1, e2)
	}
	if d := e1 - base.Unix(); d <= int64(ttl/time.Second) || d > 2*int64(ttl/time.Second) {
		t.Fatalf("expire in %ds, want (ttl, 2ttl]", d)
	}
}

func TestSignedURLAndVerify(t *testing.T) {
	b := testBiz("https://example.com/")
	const key = "attachment/2026/10/29/abc.pdf"

	// 公有读：稳定地址，不带签名
	if u := b.FileURL(&model.File{Key: "avatar/a.jpg", Scope: "avatar"}); u != "https://example.com/api/v1/storage/file/avatar/a.jpg" {
		t.Fatalf("public url = %s", u)
	}
	// 私有：签名地址，用第一个密钥签发
	u := b.FileURL(&model.File{Key: key, Scope: "attachment"})
	parsed, err := url.Parse(u)
	if err != nil || parsed.Path != "/api/v1/storage/file/"+key || parsed.Query().Get("key_id") != "new" {
		t.Fatalf("private url = %s", u)
	}
	q := parsed.Query()
	expire, _ := strconv.ParseInt(q.Get("expire"), 10, 64)
	sig := model.Signature{Expire: expire, KeyID: q.Get("key_id"), Sign: q.Get("sign")}
	if err := b.VerifySignature(http.MethodGet, key, sig); err != nil {
		t.Fatalf("verify: %v", err)
	}
	// 下载签名不能拿来上传；改 key / 过期 / 未知密钥都不行
	for name, err := range map[string]error{
		"method":  b.VerifySignature(http.MethodPut, key, sig),
		"key":     b.VerifySignature(http.MethodGet, key+"x", sig),
		"expired": b.VerifySignature(http.MethodGet, key, model.Signature{Expire: 1, KeyID: sig.KeyID, Sign: sig.Sign}),
		"key_id":  b.VerifySignature(http.MethodGet, key, model.Signature{Expire: sig.Expire, KeyID: "nope", Sign: sig.Sign}),
	} {
		if !storagesvc.IsSignatureInvalid(err) {
			t.Errorf("%s: err = %v", name, err)
		}
	}
	// 轮换：旧密钥签的地址在过期前仍然有效
	old := testBiz("")
	old.cfg.SignKeys = old.cfg.GetSignKeys()[1:]
	ou := old.FileURL(&model.File{Key: key, Scope: "attachment"})
	op, _ := url.Parse(ou)
	if op.Host != "" || op.Path != "/api/v1/storage/file/"+key {
		t.Fatalf("empty base_url should give a relative url: %s", ou)
	}
	oe, _ := strconv.ParseInt(op.Query().Get("expire"), 10, 64)
	if err := b.VerifySignature(http.MethodGet, key, model.Signature{Expire: oe, KeyID: "old", Sign: op.Query().Get("sign")}); err != nil {
		t.Fatalf("old key: %v", err)
	}
	// scope 从配置删掉的历史文件按私有处理
	if b.PublicRead(&model.File{Scope: "gone"}) {
		t.Fatal("unknown scope must be private")
	}
}
