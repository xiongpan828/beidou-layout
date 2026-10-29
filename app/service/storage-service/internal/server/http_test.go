package server

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	conf "github.com/xiongpan828/beidou-layout/api/gen/go/config/storage_service"
	storagesvc "github.com/xiongpan828/beidou-layout/api/gen/go/service/storage_service"
	"github.com/xiongpan828/beidou-layout/app/service/storage-service/internal/service/storage"
	"github.com/xiongpan828/beidou-layout/app/service/storage-service/model"
	"github.com/xiongpan828/beidou-layout/pkg/signaturex"
)

const testSecret = "test-secret-0123456789"

// fakeBiz 只实现文件 HTTP 用到的方法；provider 是测试里的「对象存储」
type fakeBiz struct {
	model.IStorageBiz
	mu       sync.Mutex
	files    map[string]*model.File
	provider string
}

func (f *fakeBiz) VerifySignature(method, key string, sig model.Signature) error {
	if sig.Expire < time.Now().Unix() || sig.KeyID != "k1" || !signaturex.Valid(testSecret, method, key, sig.Expire, sig.Sign) {
		return storagesvc.ErrorSignatureInvalid("bad signature")
	}
	return nil
}

func (f *fakeBiz) file(key string) (*model.File, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if file, ok := f.files[key]; ok {
		cp := *file
		return &cp, nil
	}
	return nil, storagesvc.ErrorFileNotFound("not found")
}

func (f *fakeBiz) GetFile(_ context.Context, key string) (*model.File, error) {
	file, err := f.file(key)
	if err == nil && file.Status != model.FileStatusUploaded {
		return nil, storagesvc.ErrorFileNotFound("pending")
	}
	return file, err
}

func (f *fakeBiz) PrepareUpload(_ context.Context, key string) (*model.File, string, error) {
	file, err := f.file(key)
	if err != nil {
		return nil, "", err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.files[key].Status != model.FileStatusPending {
		return nil, "", storagesvc.ErrorFileAlreadyUploaded("uploaded")
	}
	f.files[key].Status = model.FileStatusUploading
	return file, f.provider + "/" + key, nil
}

func (f *fakeBiz) MarkUploaded(_ context.Context, file *model.File, obj model.UploadedObject) (*model.File, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	stored := f.files[file.Key]
	stored.Status, stored.Size, stored.Etag, stored.ContentType = model.FileStatusUploaded, obj.Size, obj.Etag, obj.ContentType
	cp := *stored
	return &cp, nil
}

func (f *fakeBiz) PublicRead(file *model.File) bool { return file.Scope == "avatar" }

func (f *fakeBiz) FileURL(file *model.File) string {
	return "/api/v1/storage/file/" + file.Key
}

func (f *fakeBiz) AbortUpload(_ context.Context, file *model.File) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.files[file.Key].Status = model.FileStatusPending
}

func (f *fakeBiz) ProviderDownloadURL(_ context.Context, file *model.File) (string, error) {
	return f.provider + "/" + file.Key, nil
}

// 读写同一个入口：上传一律验签、一个 key 只写一次；公有读不验签、长缓存；私有文件和不存在的文件先验签
func TestFileRoutes(t *testing.T) {
	objects := map[string]string{}
	var mu sync.Mutex
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		if r.Method == http.MethodPut {
			b, _ := io.ReadAll(r.Body)
			objects[r.URL.Path] = r.Header.Get("Content-Type") + "|" + string(b)
			w.Header().Set("ETag", `"etag-1"`)
			return
		}
		v, ok := objects[r.URL.Path]
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		ct, body, _ := strings.Cut(v, "|")
		w.Header().Set("Content-Type", ct)
		_, _ = io.WriteString(w, body)
	}))
	defer provider.Close()

	const (
		avatar  = "avatar/2026/10/29/aaaa.png"
		doc     = "attachment/2026/10/29/bbbb.pdf"
		missing = "attachment/2026/10/29/cccc.pdf"
	)
	biz := &fakeBiz{provider: provider.URL, files: map[string]*model.File{
		avatar: {Key: avatar, Scope: "avatar", Status: model.FileStatusPending},
		doc:    {Key: doc, Scope: "attachment", Status: model.FileStatusPending},
	}}
	cfg := &conf.Config{HttpAddr: ":0"}
	srv := httptest.NewServer(NewHTTPServer(cfg, storage.NewStorageService(biz)))
	defer srv.Close()

	plain := func(key string) string { return srv.URL + "/api/v1/storage/file/" + key }
	signed := func(method, key string) string {
		expire := time.Now().Add(time.Hour).Unix()
		q := url.Values{"expire": {strconv.FormatInt(expire, 10)}, "key_id": {"k1"}, "sign": {signaturex.Sign(testSecret, method, key, expire)}}
		return plain(key) + "?" + q.Encode()
	}
	do := func(method, u, body string) (int, http.Header, string) {
		var rd io.Reader
		if body != "" {
			rd = strings.NewReader(body)
		}
		req, _ := http.NewRequest(method, u, rd)
		if body != "" {
			// 客户端声明的类型一律忽略：服务端按扩展名定类型（否则能把上传内容伪装成 HTML 在本域执行）
			req.Header.Set("Content-Type", "text/html")
		}
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = res.Body.Close() }()
		b, _ := io.ReadAll(res.Body)
		return res.StatusCode, res.Header, string(b)
	}

	steps := []struct {
		name, method, url, body string
		status                  int
		cache, want             string
	}{
		{"上传不带签名", http.MethodPut, plain(doc), "pdf-bytes", 403, "", ""},
		{"下载签名不能上传", http.MethodPut, signed(http.MethodGet, doc), "pdf-bytes", 403, "", ""},
		{"空 body", http.MethodPut, signed(http.MethodPut, doc), "", 411, "", ""},
		{"上传私有文件", http.MethodPut, signed(http.MethodPut, doc), "pdf-bytes", 200, "", `"status":"FILE_STATUS_UPLOADED"`},
		{"同一个 key 不能再写", http.MethodPut, signed(http.MethodPut, doc), "again", 409, "", ""},
		{"上传头像", http.MethodPut, signed(http.MethodPut, avatar), "png-bytes", 200, "", `"size":"9"`},
		{"公有读不带签名", http.MethodGet, plain(avatar), "", 200, "public, max-age=31536000, immutable", "png-bytes"},
		{"私有不带签名", http.MethodGet, plain(doc), "", 403, "", ""},
		{"私有用上传签名", http.MethodGet, signed(http.MethodPut, doc), "", 403, "", ""},
		{"私有带签名", http.MethodGet, signed(http.MethodGet, doc), "", 200, "private, max-age=", "pdf-bytes"},
		{"不存在且不带签名：探测不出", http.MethodGet, plain(missing), "", 403, "", ""},
		{"不存在带签名", http.MethodGet, signed(http.MethodGet, missing), "", 404, "", ""},
	}
	for _, s := range steps {
		status, header, body := do(s.method, s.url, s.body)
		if status != s.status {
			t.Fatalf("%s: status = %d, want %d, body = %s", s.name, status, s.status, body)
		}
		if s.cache != "" && !strings.HasPrefix(header.Get("Cache-Control"), s.cache) {
			t.Fatalf("%s: Cache-Control = %q", s.name, header.Get("Cache-Control"))
		}
		if s.want != "" && !strings.Contains(body, s.want) {
			t.Fatalf("%s: body = %s, want %s", s.name, body, s.want)
		}
	}

	// 类型按扩展名写进对象存储，下载时原样返回；图片以外强制下载，内容一律 sandbox
	if objects["/"+doc] != "application/pdf|pdf-bytes" {
		t.Fatalf("stored object = %q", objects["/"+doc])
	}
	_, header, _ := do(http.MethodGet, signed(http.MethodGet, doc), "")
	if header.Get("Content-Type") != "application/pdf" || header.Get("X-Content-Type-Options") != "nosniff" ||
		header.Get("Content-Security-Policy") != "sandbox" || header.Get("Content-Disposition") != "attachment" {
		t.Fatalf("download headers = %v", header)
	}
	if _, header, _ = do(http.MethodGet, plain(avatar), ""); header.Get("Content-Type") != "image/png" || header.Get("Content-Disposition") != "" {
		t.Fatalf("avatar headers = %v", header)
	}
	// 上传返回的 File JSON：snake_case，带访问地址
	biz.files[missing] = &model.File{Key: missing, Scope: "attachment", Status: model.FileStatusPending}
	_, _, body := do(http.MethodPut, signed(http.MethodPut, missing), "x")
	var f struct {
		Key         string `json:"key"`
		ContentType string `json:"content_type"`
		URL         string `json:"url"`
	}
	if err := json.Unmarshal([]byte(body), &f); err != nil || f.Key != missing || f.ContentType != "application/pdf" || f.URL != "/api/v1/storage/file/"+missing {
		t.Fatalf("upload reply = %s", body)
	}
}
