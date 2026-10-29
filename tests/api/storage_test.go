//go:build integration

package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"
)

// ensureProvider scope 都指向 code=default 的供应商：没有就用 TEST_STORAGE_PROVIDER_CONFIG（minio 配置 JSON）创建，否则跳过
func ensureProvider(t *testing.T) {
	t.Helper()
	code, body := postJSON(t, "/api/v1/admin/Storage/ListProvider", map[string]any{})
	if code != http.StatusOK {
		t.Fatalf("list provider: %d %s", code, body)
	}
	if strings.Contains(string(body), `"code":"default"`) {
		return
	}
	cfg := os.Getenv("TEST_STORAGE_PROVIDER_CONFIG")
	if cfg == "" {
		t.Skip(`no storage provider "default": create it in admin, or set TEST_STORAGE_PROVIDER_CONFIG='{"endpoint":"127.0.0.1:9000","access_key_id":"…","secret_access_key":"…","bucket":"…"}'`)
	}
	code, body = postJSON(t, "/api/v1/admin/Storage/CreateProvider", map[string]string{
		"code": "default", "name": "default", "type": getenv("TEST_STORAGE_PROVIDER_TYPE", "minio"), "config": cfg,
	})
	if code != http.StatusOK {
		t.Fatalf("create provider: %d %s", code, body)
	}
}

// do 直接请求签名地址（相对路径拼网关地址）
func do(t *testing.T, method, url, contentType, body string) (int, http.Header, string) {
	t.Helper()
	if strings.HasPrefix(url, "/") {
		url = baseURL() + url
	}
	req, err := http.NewRequest(method, url, strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	res, err := (&http.Client{Timeout: 30 * time.Second}).Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = res.Body.Close() }()
	out, _ := io.ReadAll(res.Body)
	return res.StatusCode, res.Header, string(out)
}

type uploadURL struct {
	Key       string `json:"key"`
	UploadURL string `json:"upload_url"`
}

type file struct {
	Key    string `json:"key"`
	Size   string `json:"size"` // int64 → protojson 字符串
	Status string `json:"status"`
	URL    string `json:"url"`
}

// 签发上传地址 → PUT 字节（网关 → storage-service 验签 → 代理写入供应商）→ 读回 → GetFile 看到同一个文件
func TestStorageUploadRoundTrip(t *testing.T) {
	ensureProvider(t)

	// scope 与扩展名校验
	if code, body := postJSON(t, "/api/v1/admin/Storage/CreateUploadURL", map[string]string{"scope": "nope", "filename": "a.txt"}); code != http.StatusBadRequest {
		t.Fatalf("unknown scope: %d %s", code, body)
	}
	if code, body := postJSON(t, "/api/v1/admin/Storage/CreateUploadURL", map[string]string{"scope": "attachment", "filename": "a.exe"}); code != http.StatusBadRequest || reason([]byte(body)) != "FILE_EXT_NOT_ALLOWED" {
		t.Fatalf("ext: %d %s", code, body)
	}

	// 私有附件
	code, body := postJSON(t, "/api/v1/admin/Storage/CreateUploadURL", map[string]string{"scope": "attachment", "filename": "note.txt"})
	var up uploadURL
	if err := json.Unmarshal(body, &up); code != http.StatusOK || err != nil || up.UploadURL == "" {
		t.Fatalf("create upload url: %d %s", code, body)
	}
	const content = "hello storage"
	status, _, reply := do(t, http.MethodPut, up.UploadURL, "text/plain", content)
	var f file
	if err := json.Unmarshal([]byte(reply), &f); status != http.StatusOK || err != nil || f.Status != "FILE_STATUS_UPLOADED" || f.Size != fmt.Sprint(len(content)) {
		t.Fatalf("upload: %d %s", status, reply)
	}
	if status, _, reply = do(t, http.MethodPut, up.UploadURL, "text/plain", "again"); status != http.StatusConflict {
		t.Fatalf("second upload: %d %s", status, reply)
	}
	if status, _, reply = do(t, http.MethodGet, f.URL, "", ""); status != http.StatusOK || reply != content {
		t.Fatalf("signed download: %d %s", status, reply)
	}
	unsigned := strings.SplitN(f.URL, "?", 2)[0]
	if status, _, reply = do(t, http.MethodGet, unsigned, "", ""); status != http.StatusForbidden || reason([]byte(reply)) != "SIGNATURE_INVALID" {
		t.Fatalf("unsigned private download: %d %s", status, reply)
	}
	code, body = postJSON(t, "/api/v1/admin/Storage/GetFile", map[string]string{"key": up.Key})
	var got struct {
		File file `json:"file"`
	}
	if err := json.Unmarshal(body, &got); code != http.StatusOK || err != nil || got.File.Key != up.Key || got.File.URL == "" {
		t.Fatalf("admin get file: %d %s", code, body)
	}

	// 公有读头像：不带签名可读，长缓存
	code, body = postJSON(t, "/api/v1/admin/Storage/CreateUploadURL", map[string]string{"scope": "avatar", "filename": "me.png"})
	if err := json.Unmarshal(body, &up); code != http.StatusOK || err != nil {
		t.Fatalf("avatar upload url: %d %s", code, body)
	}
	if status, _, reply = do(t, http.MethodPut, up.UploadURL, "image/png", "png-bytes"); status != http.StatusOK {
		t.Fatalf("avatar upload: %d %s", status, reply)
	}
	_ = json.Unmarshal([]byte(reply), &f)
	status, header, reply := do(t, http.MethodGet, f.URL, "", "")
	if status != http.StatusOK || reply != "png-bytes" || strings.Contains(f.URL, "sign=") || !strings.HasPrefix(header.Get("Cache-Control"), "public") {
		t.Fatalf("public avatar: %d %v %s", status, header, reply)
	}
}
