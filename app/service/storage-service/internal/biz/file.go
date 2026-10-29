package biz

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"net/http"
	"net/url"
	"path"
	"slices"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"

	conf "github.com/xiongpan828/beidou-layout/api/gen/go/config/storage_service"
	storagesvc "github.com/xiongpan828/beidou-layout/api/gen/go/service/storage_service"
	"github.com/xiongpan828/beidou-layout/app/service/storage-service/model"
	"github.com/xiongpan828/beidou-layout/app/service/storage-service/pkg/constants"
	"github.com/xiongpan828/beidou-layout/pkg/signaturex"
)

// defaultURLTTL scope 没配 url_ttl 时签名地址的有效期
const defaultURLTTL = 10 * time.Minute

// CreateUploadURL 登记 PENDING 文件并签发上传地址（PUT 到本服务，签名校验后代理写入供应商）
func (b *biz) CreateUploadURL(ctx context.Context, param model.CreateUploadParam) (*model.UploadURL, error) {
	sc, err := b.scope(param.Scope)
	if err != nil {
		return nil, err
	}
	ext := strings.ToLower(path.Ext(param.Filename))
	if !slices.Contains(sc.GetExts(), ext) {
		return nil, storagesvc.ErrorFileExtNotAllowed("ext %q not allowed in scope %s, allowed: %v", ext, param.Scope, sc.GetExts())
	}
	// 供应商不存在 / 配置坏了提前失败，不留一条写不进去的记录
	if _, err := b.provider(ctx, sc.GetProviderCode()); err != nil {
		return nil, err
	}
	now := time.Now()
	key, err := newKey(sc.GetPrefix(), ext, now)
	if err != nil {
		return nil, err
	}
	f := &model.File{
		Key:          key,
		Filename:     param.Filename,
		Status:       model.FileStatusPending,
		Scope:        param.Scope,
		ProviderCode: sc.GetProviderCode(),
	}
	if err := b.WithContext(ctx).File.Create(f); err != nil {
		return nil, storagesvc.ErrorFileCreateFail("create file %s: %v", key, err)
	}
	expireAt := now.Add(urlTTL(sc))
	return &model.UploadURL{Key: key, URL: b.signedURL(http.MethodPut, key, expireAt.Unix()), ExpireAt: expireAt}, nil
}

// GetFile 已上传的文件。还没标记 UPLOADED 的先查一次对象存储：写入成功但状态没来得及更新时在这里补上（不需要定时任务）
func (b *biz) GetFile(ctx context.Context, key string) (*model.File, error) {
	f, err := b.fileByKey(ctx, key)
	if err != nil {
		return nil, err
	}
	if f.Status == model.FileStatusUploaded {
		return f, nil
	}
	p, err := b.provider(ctx, f.ProviderCode)
	if err != nil {
		return nil, err
	}
	info, err := p.Stat(ctx, f.Key)
	if err != nil {
		return nil, err // 对象不存在 → FILE_NOT_FOUND
	}
	return b.MarkUploaded(ctx, f, model.UploadedObject{Size: info.Size, Etag: info.ETag, ContentType: info.ContentType})
}

// PrepareUpload 占位后返回供应商预签名上传地址：PENDING → UPLOADING 是条件更新，
// 并发或重复的 PUT 只有一个能占到，其余 409（一个 key 只写一次）
func (b *biz) PrepareUpload(ctx context.Context, key string) (*model.File, string, error) {
	f, err := b.fileByKey(ctx, key)
	if err != nil {
		return nil, "", err
	}
	p, err := b.provider(ctx, f.ProviderCode)
	if err != nil {
		return nil, "", err
	}
	q := b.Q.File
	res, err := b.WithContext(ctx).File.Where(q.ID.Eq(f.ID), q.Status.Eq(model.FileStatusPending)).UpdateSimple(q.Status.Value(model.FileStatusUploading))
	if err != nil {
		return nil, "", storagesvc.ErrorFileUpdateFail("lock file %s: %v", key, err)
	}
	if res.RowsAffected != 1 {
		return nil, "", storagesvc.ErrorFileAlreadyUploaded("file %s already uploaded or uploading", key)
	}
	u, err := p.PresignPut(ctx, f.Key, time.Minute)
	if err != nil {
		b.AbortUpload(ctx, f)
		return nil, "", err
	}
	return f, u, nil
}

// AbortUpload 写入失败：UPLOADING → PENDING（尽力而为；失败时文件停在 UPLOADING，读取路径仍会按对象存储补齐状态）
func (b *biz) AbortUpload(ctx context.Context, file *model.File) {
	q := b.Q.File
	_, _ = b.WithContext(ctx).File.Where(q.ID.Eq(file.ID), q.Status.Eq(model.FileStatusUploading)).UpdateSimple(q.Status.Value(model.FileStatusPending))
}

// MarkUploaded PENDING / UPLOADING → UPLOADED 并回填对象信息（条件更新，重复调用无副作用）
func (b *biz) MarkUploaded(ctx context.Context, file *model.File, obj model.UploadedObject) (*model.File, error) {
	q := b.Q.File
	if _, err := b.WithContext(ctx).File.Where(q.ID.Eq(file.ID), q.Status.In(model.FileStatusPending, model.FileStatusUploading)).UpdateSimple(
		q.Status.Value(model.FileStatusUploaded),
		q.Size.Value(obj.Size),
		q.Etag.Value(obj.Etag),
		q.ContentType.Value(obj.ContentType),
	); err != nil {
		return nil, storagesvc.ErrorFileUpdateFail("mark file %s uploaded: %v", file.Key, err)
	}
	return b.fileByKey(ctx, file.Key)
}

// ProviderDownloadURL 供应商预签名下载地址（只在服务端代理读取时用）
func (b *biz) ProviderDownloadURL(ctx context.Context, file *model.File) (string, error) {
	p, err := b.provider(ctx, file.ProviderCode)
	if err != nil {
		return "", err
	}
	return p.PresignGet(ctx, file.Key, time.Minute)
}

// PublicRead 以当前 scope 配置为准；scope 已从配置删除的历史文件按私有处理
func (b *biz) PublicRead(file *model.File) bool {
	return b.cfg.GetScopes()[file.Scope].GetPublicRead()
}

// FileURL 公有读返回稳定地址（不过期，可长缓存）；私有返回签名地址，
// 过期时间按窗口对齐：同一窗口内同一文件签出的地址相同，客户端缓存能命中（实际有效期 ttl ~ 2×ttl）
func (b *biz) FileURL(file *model.File) string {
	if b.PublicRead(file) {
		return b.fileURL(file.Key)
	}
	ttl := defaultURLTTL
	if sc, ok := b.cfg.GetScopes()[file.Scope]; ok {
		ttl = urlTTL(sc)
	}
	return b.signedURL(http.MethodGet, file.Key, alignedExpire(time.Now(), ttl))
}

// VerifySignature 校验签名 URL：未过期，且能被任一配置的密钥验证（轮换期间新旧密钥都认）
func (b *biz) VerifySignature(method, key string, sig model.Signature) error {
	if sig.Expire < time.Now().Unix() {
		return storagesvc.ErrorSignatureInvalid("signature expired")
	}
	for _, k := range b.cfg.GetSignKeys() {
		if k.GetId() == sig.KeyID && signaturex.Valid(k.GetSecret(), method, key, sig.Expire, sig.Sign) {
			return nil
		}
	}
	return storagesvc.ErrorSignatureInvalid("signature invalid")
}

func (b *biz) fileByKey(ctx context.Context, key string) (*model.File, error) {
	f, err := b.WithContext(ctx).File.Where(b.Q.File.Key.Eq(key)).First()
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, storagesvc.ErrorFileNotFound("file %s not found", key)
	}
	if err != nil {
		return nil, storagesvc.ErrorFileGetFail("get file %s: %v", key, err)
	}
	return f, nil
}

func (b *biz) scope(name string) (*conf.Scope, error) {
	sc, ok := b.cfg.GetScopes()[name]
	if !ok {
		return nil, storagesvc.ErrorScopeNotFound("scope %q not configured", name)
	}
	return sc, nil
}

// fileURL 不带签名的地址；base_url 为空时是相对路径
func (b *biz) fileURL(key string) string {
	return strings.TrimRight(b.cfg.GetBaseUrl(), "/") + constants.FilePathPrefix + key
}

// signedURL 用第一个签名密钥签发（配置校验保证至少一个）
func (b *biz) signedURL(method, key string, expireAt int64) string {
	k := b.cfg.GetSignKeys()[0]
	q := url.Values{
		"expire": {strconv.FormatInt(expireAt, 10)},
		"key_id": {k.GetId()},
		"sign":   {signaturex.Sign(k.GetSecret(), method, key, expireAt)},
	}
	return b.fileURL(key) + "?" + q.Encode()
}

func urlTTL(sc *conf.Scope) time.Duration {
	if d := sc.GetUrlTtl().AsDuration(); d > 0 {
		return d
	}
	return defaultURLTTL
}

// alignedExpire 过期时间对齐到 ttl 窗口的第二个边界：落在 (now+ttl, now+2ttl]
func alignedExpire(now time.Time, ttl time.Duration) int64 {
	window := int64(ttl / time.Second)
	return (now.Unix()/window + 2) * window
}

// newKey {prefix}/{yyyy}/{mm}/{dd}/{32 位随机 hex}{ext}：日期按 UTC，随机部分不可猜
func newKey(prefix, ext string, now time.Time) (string, error) {
	random := make([]byte, 16)
	if _, err := rand.Read(random); err != nil {
		return "", storagesvc.ErrorFileCreateFail("generate key: %v", err)
	}
	return path.Join(prefix, now.UTC().Format("2006/01/02"), hex.EncodeToString(random)) + ext, nil
}
