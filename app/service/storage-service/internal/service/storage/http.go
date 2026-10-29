package storage

// 文件读写只有一个入口 {base_url}/api/v1/storage/file/{key}（网关 /api/v1/storage/* 转发，不经过 BFF，靠 URL 签名）：
//   - PUT：必须带上传签名 → 占位 UPLOADING → 服务端 PUT 到供应商预签名地址（转发请求体）→ UPLOADED，返回 File JSON
//   - GET：公有读 scope 不校验签名，长缓存；私有文件必须带下载签名 → 服务端 GET 供应商并回传字节
// 客户端从不直连供应商：没有跨域 / mixed-content 问题，供应商地址和密钥不外泄，换供应商客户端无感。

import (
	"fmt"
	"io"
	"mime"
	"net/http"
	"path"
	"strconv"
	"strings"
	"time"

	khttp "github.com/go-kratos/kratos/v3/transport/http"

	storagesvc "github.com/xiongpan828/beidou-layout/api/gen/go/service/storage_service"
	"github.com/xiongpan828/beidou-layout/app/service/storage-service/model"
	"github.com/xiongpan828/beidou-layout/pkg/codecx"
)

// proxyClient 代理读写供应商（大文件在慢网络下要几十秒；总时长由 HTTP server 的超时兜底）
var proxyClient = &http.Client{}

// publicCacheControl 公有读文件：key 只写一次，内容不会变
const publicCacheControl = "public, max-age=31536000, immutable"

// 下载时从供应商响应透传的头（Content-Type 是上传时服务端按扩展名定的，见 contentType）
var passHeaders = []string{"Content-Type", "Content-Length", "Content-Range", "Accept-Ranges", "ETag", "Last-Modified"}

// contentType 按 key 的扩展名定类型（扩展名已过 scope.exts 白名单），不信任客户端传的 Content-Type：
// 文件和运营后台同域，客户端能指定 text/html 就能让上传的内容在本域执行
func contentType(key string) string {
	if t := mime.TypeByExtension(path.Ext(key)); t != "" {
		return t
	}
	return "application/octet-stream"
}

// UploadFile PUT：校验上传签名 → 代理写入供应商 → 标记 UPLOADED，返回 File（含访问地址）
func (s *StorageService) UploadFile(ctx khttp.Context) error {
	key := ctx.Vars().Get("key")
	if err := s.biz.VerifySignature(http.MethodPut, key, signature(ctx)); err != nil {
		return err
	}
	r := ctx.Request()
	// S3 兼容存储的预签名 PUT 不接受 chunked，必须知道长度（浏览器 fetch / XHR 发 Blob 会自动带）
	if r.ContentLength <= 0 {
		return storagesvc.ErrorFileContentLengthRequired("content-length required")
	}
	file, target, err := s.biz.PrepareUpload(ctx, key)
	if err != nil {
		return err
	}
	ct := contentType(key)
	res, err := putObject(ctx, target, r, ct)
	if err != nil {
		// 没写进去：放开占位，签名没过期时可以重试
		s.biz.AbortUpload(ctx, file)
		return storagesvc.ErrorObjectStorageFail("upload %s: %v", key, err)
	}
	// 写入成功即更新状态；这一步失败也不丢数据：之后读到 PENDING 会去对象存储查一次再补上
	file, err = s.biz.MarkUploaded(ctx, file, model.UploadedObject{
		Size:        r.ContentLength,
		Etag:        strings.Trim(res.Get("ETag"), `"`),
		ContentType: ct,
	})
	if err != nil {
		return err
	}
	body, err := codecx.Marshal(s.toProtoFile(file))
	if err != nil {
		return err
	}
	return ctx.Blob(http.StatusOK, "application/json", body)
}

// putObject 把请求体转发到供应商预签名地址，返回供应商响应头（2xx 以外算失败）
func putObject(ctx khttp.Context, target string, r *http.Request, ct string) (http.Header, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, target, r.Body)
	if err != nil {
		return nil, err
	}
	req.ContentLength = r.ContentLength
	req.Header.Set("Content-Type", ct)
	res, err := proxyClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode/100 != 2 {
		msg, _ := io.ReadAll(io.LimitReader(res.Body, 512))
		return nil, fmt.Errorf("provider status %d: %s", res.StatusCode, msg)
	}
	return res.Header, nil
}

// DownloadFile GET：公有读直接回传；私有文件（以及不存在的文件）先验签，没有签名探测不出私有文件在不在
func (s *StorageService) DownloadFile(ctx khttp.Context) error {
	key := ctx.Vars().Get("key")
	file, err := s.biz.GetFile(ctx, key)
	if err == nil && s.biz.PublicRead(file) {
		return s.proxyDownload(ctx, file, publicCacheControl)
	}
	sig := signature(ctx)
	if verr := s.biz.VerifySignature(http.MethodGet, key, sig); verr != nil {
		return verr
	}
	if err != nil {
		return err
	}
	// 签名地址过期前客户端可缓存；private：不让 CDN / 中间代理存
	return s.proxyDownload(ctx, file, "private, max-age="+strconv.FormatInt(max(sig.Expire-time.Now().Unix(), 0), 10))
}

// proxyDownload 服务端 GET 供应商并回传（透传 Range，视频可拖动）
func (s *StorageService) proxyDownload(ctx khttp.Context, file *model.File, cacheControl string) error {
	target, err := s.biz.ProviderDownloadURL(ctx, file)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return storagesvc.ErrorObjectStorageFail("build download request: %v", err)
	}
	if rg := ctx.Request().Header.Get("Range"); rg != "" {
		req.Header.Set("Range", rg)
	}
	res, err := proxyClient.Do(req)
	if err != nil {
		return storagesvc.ErrorObjectStorageFail("download %s: %v", file.Key, err)
	}
	defer func() { _ = res.Body.Close() }()
	switch res.StatusCode {
	case http.StatusOK, http.StatusPartialContent, http.StatusRequestedRangeNotSatisfiable: // 416 带 Content-Range 原样回给客户端
	case http.StatusNotFound:
		return storagesvc.ErrorFileNotFound("object %s not found", file.Key)
	default:
		return storagesvc.ErrorObjectStorageFail("download %s: provider status %d", file.Key, res.StatusCode)
	}
	h := ctx.Response().Header()
	for _, name := range passHeaders {
		if v := res.Header.Get(name); v != "" {
			h.Set(name, v)
		}
	}
	h.Set("Cache-Control", cacheControl)
	// 用户上传的内容与运营后台同域：禁止嗅探类型；sandbox 让内容即使是 HTML / SVG 也不能执行脚本、访问本域；
	// 图片以外一律下载，不在浏览器里打开
	h.Set("X-Content-Type-Options", "nosniff")
	h.Set("Content-Security-Policy", "sandbox")
	if !strings.HasPrefix(h.Get("Content-Type"), "image/") {
		h.Set("Content-Disposition", "attachment")
	}
	ctx.Response().WriteHeader(res.StatusCode)
	// 响应头已发出，中途断开（多半是客户端取消）没法再改成错误响应
	_, _ = io.Copy(ctx.Response(), res.Body)
	return nil
}

// signature URL 上的签名参数（缺失 / 格式错误时为零值，验签失败）
func signature(ctx khttp.Context) model.Signature {
	q := ctx.Query()
	expire, _ := strconv.ParseInt(q.Get("expire"), 10, 64)
	return model.Signature{Expire: expire, KeyID: q.Get("key_id"), Sign: q.Get("sign")}
}
