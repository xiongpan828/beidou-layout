# 对象存储（storage-service）

上传 / 下载统一走 storage-service：客户端只和本服务签发的**签名 URL** 打交道，字节由服务端代理读写对象存储供应商（MinIO / S3 及兼容存储）。
供应商在运营后台配置（存库），业务场景（scope）在服务配置里声明。没有定时任务。

```text
                    ① CreateUploadURL（BFF 纯代理）
运营后台 ───────────────────────────────▶ admin-service ──gRPC :9300──▶ storage-service
   │                                                                                     │ 登记 file（PENDING）
   │ ◀──────────────── { key, upload_url, expire_at } ───────────────────────────────────┘
   │
   │ ② PUT upload_url（文件字节）            ③ GET url（读）
   ▼
APISIX :9080  /api/v1/storage/*（不经过 BFF，request_buffering off）
   ▼
storage-service HTTP :9301  校验签名 → 服务端用供应商预签名地址 PUT / GET（流式转发）
   ▼
MinIO / S3 …（客户端永远拿不到供应商地址）
```

| 组件 | 位置 |
|---|---|
| 服务 | `app/service/storage-service`（gRPC :9300 + 文件 HTTP :9301，库 `storage_service`） |
| 契约 | `api/service/storage_service/{service,error}.proto`（错误码段 3），配置 `api/config/storage_service/conf.proto` |
| 供应商注册中心 | `pkg/storagex`（+ `minio/`、`s3/` 两个实现） |
| URL 签名 | `pkg/signaturex` |
| BFF | `admin-service` 模块 `admin_storage`（全部纯代理） |
| 网关 | `app/gateway/conf/apisix.yaml` 路由 `storage-file` |

## 1. 供应商：注册中心 + 配置存库

- `pkg/storagex` 定义 `Provider`（`PresignPut` / `PresignGet` / `Stat`），各实现在 `init` 里 `storagex.Register("minio", newConfig, New)` 注册；storage-service 的 biz 用空白 import 引入实现。
- 供应商记录在 `provider` 表：`code`（唯一，scope 引用它）、`type`（`minio` / `s3`）、`config`（jsonb）。
- 配置 JSON 的结构就是 proto `MinioConfig` / `S3Config`，校验规则写在 proto 上：`protojson.Unmarshal`（字段拼错直接报错）+ `protovalidate.Validate`，**不手写字段校验**。
- **密钥不回显**：字段名含 `secret` 的（`secret_access_key`）在接口返回时清空；更新时这些字段留空 = 沿用库里的旧值（`storagex.Normalize`），前端改桶名不用重填密钥。
- 客户端缓存：按 `code@updated_at` 缓存 SDK 客户端，每次请求读一行 `provider`（唯一索引）。后台改了配置，所有副本下一次请求就用新客户端，不需要广播失效。
- 删除：被 scope 配置引用、或已有文件写在它上面时拒绝（`PROVIDER_IN_USE`），否则这些文件就读不出来了。
- S3 兼容存储（OSS / COS / R2 …）用 `s3` 类型 + `endpoint` + `use_path_style`；预签名 PUT 关掉了 SDK 默认的校验和参数（按空 body 计算，代理转发真实内容时对不上）。

新增一种供应商：在 `storage_service/service.proto` 加 `XxxConfig`（密钥字段名带 `secret`），`pkg/storagex/xxx/` 实现 `Provider` 并 `Register`，biz 里空白 import，`CreateProviderReq.type` 的 `in` 加上类型名。

## 2. scope（服务配置）

```yaml
sign_keys:                         # 第一个签发，全部可校验
  - id: "k1"
    secret: "…"                    # ≥ 16 位；本地 dev-only-change-me-*，chart 取 config.storage_sign_secret
scopes:
  avatar:
    provider_code: "default"       # 后台「存储供应商」里的 code
    prefix: "avatar"               # key 前缀
    exts: [".jpg", ".jpeg", ".png", ".webp"]   # 允许的扩展名（小写带点）
    public_read: true              # 不带签名可读
    url_ttl: "600s"                # 签名地址有效期（不填 10 分钟）
  attachment:
    provider_code: "default"
    prefix: "attachment"
    exts: [".jpg", ".jpeg", ".png", ".webp", ".pdf", ".txt"]
    public_read: false             # 读需要签名
    url_ttl: "3600s"
```

- key：`{prefix}/{yyyy}/{mm}/{dd}/{32 位随机 hex}{ext}`（日期按 UTC，随机部分不可猜）。
- 读权限以**当前**配置为准：scope 改成私有，历史文件一起变私有；scope 从配置删掉的历史文件按私有处理。
- 文件记录写入时的 `provider_code`：scope 换供应商后，旧文件仍从原供应商读。
- 文件大小由网关 `client_max_body_size`（`app/gateway/conf/config.yaml`，50m）统一限制；外部入口的请求体上限要保持一致。

## 3. 签名 URL + 服务端代理

```text
{base_url}/api/v1/storage/file/{key}?expire={unix 秒}&key_id={sign_keys[].id}&sign={hex}
sign = hex(HMAC-SHA256(secret, METHOD + ":" + key + ":" + expire))
```

- 读写**同一个路径**，method 参与签名：下载签名不能拿来上传，反之亦然。
- `base_url` 为空时签发相对路径（浏览器同域使用；本地运营后台 dev 代理把 `/api/v1/storage` 转到 `:9301`）；部署环境是 `https://{domain}`。
- 密钥轮换：新密钥放 `sign_keys` 第一个，旧的留到它签过的地址都过期再删。
- 为什么代理而不是 302 到供应商：HTTPS 页面跳到 HTTP 的供应商会被当成 mixed-content 拦截；直连还要给供应商配跨域、暴露供应商域名。代理后客户端只访问本域名，换供应商客户端无感。代价是文件流量经过网关和 storage-service（流式转发，不整文件缓存；网关 `request_buffering: false`）。

| 请求 | 规则 |
|---|---|
| `PUT` | 必须带上传签名；必须有 `Content-Length`（否则 411）；开始写入前把 `PENDING` 条件更新成 `UPLOADING` 占位，并发 / 重复的 PUT 只有一个能占到（一个 key 只写一次，其余 409）；写入失败退回 `PENDING`，签名没过期时可重试。`Content-Type` 一律按扩展名定（忽略客户端传的值）。成功返回 `File` JSON（snake_case，含访问地址 `url`） |
| `GET` 公有读 | 不校验签名；`Cache-Control: public, max-age=31536000, immutable`（key 只写一次，内容不会变） |
| `GET` 私有 | 必须带下载签名；**先验签再查文件**，没签名探测不出私有文件在不在；`Cache-Control: private, max-age=<到过期的秒数>` |
| 共同 | 透传 `Range`（视频可拖动）；文件与运营后台同域，响应一律带 `X-Content-Type-Options: nosniff` 和 `Content-Security-Policy: sandbox`，图片以外加 `Content-Disposition: attachment`（不在浏览器里打开）；`exts` 仍不要放 `.html` 这类可执行内容 |

私有读地址的过期时间按 `url_ttl` 窗口对齐（落在 `(ttl, 2×ttl]`）：同一窗口内同一文件签出的地址相同，列表刷新时客户端缓存能命中。

## 4. 文件状态：没有定时任务

```text
CreateUploadURL ──▶ PENDING ──PUT 开始──▶ UPLOADING ──写入成功（同一个请求里）──▶ UPLOADED
                                          └──写入失败──▶ PENDING（可重试）
                       │
                       └─ 没人上传：签名过期后无法再写，记录惰性留存（读不到、不占对象存储）
```

- **完成即确认**：字节经 storage-service 代理写入，供应商返回 2xx 后在同一个 PUT 请求里把状态改成 `UPLOADED`（回填 size / etag / content_type），不需要客户端再调「确认上传」，也不需要轮询供应商。
- **惰性补偿**：写入成功但改状态失败（进程恰好退出 / 数据库抖动）时，文件停在 `PENDING` 或 `UPLOADING`。之后任何读取（`GetFile`、HTTP GET）遇到未完成的状态会 `Stat` 一次供应商：对象在就补成 `UPLOADED`，不在就返回 `FILE_NOT_FOUND`。
- **悬空的 PENDING**（签了地址没上传）：签名过期后写不进去，也读不出来，只是一行记录。需要清理时手动执行（按需调整时间）：

  ```sql
  DELETE FROM file WHERE status = 'FILE_STATUS_PENDING' AND created_at < now() - interval '7 days';
  ```

- 文件不删除（业务表引用 key；删除策略因业务而异，需要时再加接口）。

## 5. 接口

storage-service gRPC（`service.storage_service.StorageService`）：`CreateUploadURL`、`GetFile`、`ListProvider`、`CreateProvider`、`UpdateProvider`、`DeleteProvider`。

| HTTP | 实现 |
|---|---|
| `POST /api/v1/admin/Storage/{CreateUploadURL,GetFile}` | admin-service 纯代理 |
| `POST /api/v1/admin/Storage/{List,Create,Update,Delete}Provider` | admin-service 纯代理 |
| `PUT / GET /api/v1/storage/file/{key}` | storage-service HTTP（URL 签名） |

模板不带身份认证：上线前要在网关或 BFF 接入自己的认证与访问控制，至少保护供应商管理接口（含密钥）和签发上传地址。

业务接口引用文件：业务表只存 `key`；返回给前端时调 storage-service `GetFile` 拿 `url`（私有文件是限时地址，前端加载失败时重新拉接口即可）。需要记录上传者时，在 `CreateUploadURLReq` / `File` 上加字段，由 BFF 从自己的身份上下文填入（不信任前端）。

### 前端上传示例

```ts
const { upload_url, key } = await client.CreateUploadURL({ scope: 'attachment', filename: file.name });
const res = await fetch(upload_url!, { method: 'PUT', body: file }); // 类型由服务端按扩展名定
if (!res.ok) throw await res.json();           // 错误体与 BFF 相同：{ code, reason, message }
const uploaded = await res.json();              // File：{ key, size, status, url, ... }
// 业务接口提交 key；展示用 uploaded.url
```

PUT 走的是网关的 `/api/v1/storage/*`，签名就是凭证，不需要其他请求头。

## 6. 本地与部署

- 本地：`make run-storage`（:9300 / :9301），`make migrate` 建表；运营后台 dev 代理把 `/api/v1/storage` 直接转到 `:9301`。
- 首次使用：运营后台「存储供应商」创建 code 为 `default` 的供应商（scope 默认都指向它），例如本地 MinIO：

  ```bash
  docker run -p 9000:9000 -p 9001:9001 -e MINIO_ROOT_USER=minio -e MINIO_ROOT_PASSWORD=minio123 \
    minio/minio server /data --console-address :9001
  ```

  在控制台 http://127.0.0.1:9001 建一个桶（如 `files`），然后创建供应商：`type = minio`，`config = {"endpoint":"127.0.0.1:9000","access_key_id":"minio","secret_access_key":"minio123","bucket":"files"}`。
- k8s：chart `services.storage-service`（`migrate: true`），`config.storage_sign_secret` 放私有 values；网关路由已打进网关镜像。
- helm test 会请求 `/api/v1/storage/file/helm-test/none.txt`，期望 403 `SIGNATURE_INVALID`（验证网关 → storage-service 链路，且不带签名探测不出文件）。
