# 运营后台（B 端）

Umi Max + React 19 + antd 6，接口走 `admin-service`（`/api/v1/admin/*`，:8000），类型来自 proto 生成的 `@beidou-layout/api-gen/service/admin_service/*`。
生产路径 `/admin/`（网关 → `web-admin` 镜像的 nginx）。

```bash
pnpm install                                   # 仓库根目录
pnpm dev:admin                                 # http://localhost:3000，/api/v1/admin → 127.0.0.1:8000（make run-admin），/api/v1/storage → 127.0.0.1:9301（storage-service 文件读写）
pnpm --filter @beidou-layout/web-admin typecheck
pnpm --filter @beidou-layout/web-admin test    # tests/*.test.ts，node:test，只测 src/lib 纯函数
pnpm --filter @beidou-layout/web-admin build   # 产物 dist/（base /admin/）
```

模板不带登录与权限：所有页面直接可用，上线前请按自己的方案接入身份认证与访问控制。

## 目录

```
.umirc.ts                  路由 / dev 代理 / base
public/icons/              页签图标
src/app.tsx                根容器（主题、空状态）+ 全局请求错误处理
src/theme.ts               antd 主题
src/global.css             全局样式（admin- 前缀）
src/feedback.ts            非组件代码弹提示（配合 components/FeedbackBridge）
src/layouts/AdminLayout.tsx 外壳：顶栏导航 + 内容区
src/components/            EmptyState、FeedbackBridge
src/services/index.ts      proto client 组装 + 错误文案 + 类型出口
src/lib/                   纯函数：http.ts（错误解析 / 传输层）、storage.ts（供应商配置 JSON）、hello.ts
src/pages/hello/           示例页
src/pages/storage/         providers.tsx 存储供应商、upload.tsx 上传示例
tests/                     node:test 单测（tsconfig.test.json）
```

## 存储页面

- **存储供应商**（`/storage/providers`）：MinIO / S3 兼容存储的增删改。`config` 是 JSON 字符串，字段见 `api/service/storage_service/service.proto` 的 `MinioConfig` / `S3Config`（没生成 TS 类型，表单类型在 `src/lib/storage.ts`，只提交该类型的字段——后端拒绝未知字段）。
  密钥不回显，编辑时留空 = 沿用原值；类型、编码创建后不能改；被 scope 或文件引用的供应商删不掉（`PROVIDER_IN_USE`）。服务配置 `scopes.*.provider_code` 引用这里的编码（本地默认 `default`）。
- **上传示例**（`/storage/upload`）：`CreateUploadURL` 签发地址 → 浏览器 `fetch` PUT 文件到签名地址（不走 umi request）→ 返回 File JSON（含 `url`，头像预览）；`GetFile` 按 key 重新换访问地址（附件是限时签名地址）。
  直传的错误体同样是 `{code, reason, message}`，用 `resolveErrorMessage` 提示（`SIGNATURE_INVALID` / `FILE_ALREADY_UPLOADED` / `FILE_EXT_NOT_ALLOWED`）。

## 新增页面

1. proto 加接口 → `make api` 生成 TS client，新模块在 `api/gen/ts/package.json` 的 `exports` 登记
2. `src/services/index.ts` 加 client 与类型
3. `src/pages/<name>/index.tsx` 写页面，`.umirc.ts` 加路由，`AdminLayout` 的 `NAV` 加导航

## 错误文案

后端 `message` 是英文开发文案，前端按 `reason` 本地化：映射表在 `src/services/index.ts` 的 `ADMIN_ERROR_TEXT`，
`src/app.tsx` 的 `errorConfig` 统一弹提示（reason 文案 > 后端 message > 兜底）。新增要给用户看的错误码时补进映射表。
