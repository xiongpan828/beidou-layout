---
name: gen-proto
description: 重新生成 proto 代码（Go gRPC stub 和 TS 请求桩）。当修改 api/ 下的 .proto 文件后需要生成代码时使用。
---

# 生成 proto 代码

在仓库根目录执行：

```bash
make api          # 生成全部代码
make proto-check  # buf lint + buf breaking（对比 main 分支），CI 同款
```

## `make api` 实际做了什么

1. `make tools` → `scripts/proto/build-tools.sh`：`go install` 固定版本的 proto 插件到 `tools/`（git 忽略）
   - `protoc-gen-go`、`protoc-gen-go-grpc`、kratos 的 `protoc-gen-go-errors` / `protoc-gen-go-http`、kratos fork 的 `protoc-gen-typescript-http`
2. `buf generate --template buf.gen.go.yaml`：`api/service` → `api/gen/go/`（message + gRPC stub + `ErrorXxx/IsXxx` + kratos HTTP 路由）
3. `buf generate --template buf.gen.ts.yaml`：只生成 `api/service/admin_service` 和 `api/service/api_service` → `api/gen/ts/`（前端请求桩包 `@beidou-layout/api-gen`）；底层 gRPC 服务不生成 TS
4. `buf generate --template buf.gen.config.yaml`：`api/config` → `api/gen/go/config/`（服务配置结构体，只生成 Go message）
5. `go run ./scripts/proto/fix-ts-int64 -dir api/gen/ts`：protojson 把 `int64/uint64` 输出为字符串，该工具用 `buf build` 拿 FileDescriptorSet，把 TS 生成物里这些字段的类型改成 `string`（生成器默认给 `number`）。因此前端使用 64 位字段（如 `total`、`File.size`）时要 `Number()` 转换
6. `go run ./scripts/proto/fix-ts-transport -dir api/gen/ts`：kratos 版 TS 插件在每个文件里都导出一份相同的传输层类型（`ClientTransport` / `SSETransport` / `createDefaultTransport` 等），该工具去掉这些 `export`，避免多模块 `export *` 时 TS2308 名称歧义；前端统一由 `app/web/admin/src/lib/http.ts` 注入 transport
7. `go run ./scripts/proto/fix-ts-field-names -dir api/gen/ts`：HTTP 层输出 proto 原名（snake_case），而插件写死驼峰 `field.JSONName()`；该工具读 FileDescriptorSet，把消息类型字段、客户端里的 `request.xxx` 与 query 键改回 proto 名。必须在第 5 步之后（fix-ts-int64 按驼峰名匹配）

`buf.yaml` 的模块根是 `api/`（排除 `api/gen`），proto 之间按 `api/` 相对路径 import；第三方 proto vendored 在 `api/third_party/`，不参与生成（见 `api/third_party/README.md`）。

## 新增 / 改动时的额外步骤

- **新增 HTTP 模块**（`api/service/{admin,api}_service/<module>/`）：buf 不生成 `api/gen/ts/package.json`，要手动在 `exports` 里补 `./service/<service>/<module>` 一项（照抄已有条目），前端才能 `import '@beidou-layout/api-gen/service/<service>/<module>'`
- **新增对外 HTTP 服务**：把目录加进 `buf.gen.ts.yaml` 的 `inputs.paths`
- **新增服务的 error.proto**：在 `api/errcode_test.go` 的 `segments` 登记错误码段（`go test ./api/...` 校验），并在 `buf.yaml` 的 `ignore_only` 登记
- 删除/重命名 proto 后，`api/gen/` 下旧的生成文件不会被自动删除，需要手动删掉对应文件再 `make api`

## 提交与 CI

- 生成后提交生成代码（`api/gen/`）。CI（`.github/workflows/ci.yml`）会跑 `buf lint`、`buf breaking`（对比 base 分支），再执行 `make api`，之后 `git status --porcelain` 非空（含新增文件）即失败
- 兼容性靠 `buf breaking` + 只增不改：字段/枚举名/RPC 不删不改名，新增用新编号

注意：`api/gen/` 下内容为生成物，**禁止手改**（`api/gen/ts/package.json` 除外），只改 `api/` 下的 `.proto` 源文件。
