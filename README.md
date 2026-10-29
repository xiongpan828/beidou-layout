# Beidou Layout

Go 微服务 + 管理后台的 monorepo 项目模板：kratos v3（gRPC + HTTP）、proto 契约驱动、APISIX 网关、React 管理后台、Helm 部署。
业务只有 hello world 和一个对象存储服务；模板的价值在工程化约定和可以直接复用的设计。用 `scripts/new-project.sh` 批量生成新项目。

> 模板不包含账号、登录与访问控制，所有接口直接可用。上线前请按自己的方案接入（建议放在网关层，见 `.agents/skills/api-convention`「认证与访问控制」）。

## 设计要点

| 设计 | 说明 | 文档 |
|---|---|---|
| **proto 是唯一契约** | Go stub、HTTP 路由、错误码、配置结构体、前端 TS client 全部由 proto 生成；`buf lint` + `buf breaking` 守兼容；package = 目录 = Go 路径 = TS 路径 | `.agents/skills/gen-proto` |
| **BFF 两种写法** | 与底层同名同参的方法不写代码，`pkg/proxyx` 按配置反射转发；需要聚合、强制字段、改造响应的才手写 service → biz → client | `.agents/skills/api-convention` |
| **对象存储** | 供应商可插拔（MinIO / S3，配置存库、proto 校验）；签名 URL + 服务端代理读写；文件状态在上传请求里完成，不靠定时任务 | `docs/storage.md` |
| **底层服务没有 data 层** | biz 内嵌 gorm gen 生成的 dao，禁止手写 SQL 条件（`make lint` 检查）；`make gen` 不连库 | `.agents/skills/api-convention` |
| **迁移不可变** | gormigrate，迁移里用快照结构体；测试校验模型与迁移建出的表结构一致 | `.agents/skills/api-convention` |
| **错误码全局唯一** | proto 枚举定义错误，每个服务独占千位段，测试校验越段；HTTP code 按语义（400/404/409/500…），前端按 reason 本地化 | `.agents/skills/api-convention` |
| **配置即校验** | 配置结构来自 proto + protovalidate，启动即校验；`config.local.yaml` 叠加本机密钥（不进 git） | `.agents/skills/api-convention` |
| **网关是自包含镜像** | APISIX standalone：路由、插件打进镜像，运行时只注入环境变量；k8s 用短服务名路由 | `app/gateway/README.md` |
| **一个 chart 装全部** | 服务配置逐项写明、只用 `{{ .变量 }}`；配置变化只滚动该服务；迁移是 initContainer；`CHANGE_ME` 未填拒绝部署 | `deploy/charts/beidou-layout/README.md` |

每条设计决策的原因见 [`docs/decisions.md`](docs/decisions.md)。

## 架构

```
外部入口（TLS）
  │
  ▼
网关 APISIX :9080（app/gateway）
  ├─ /api/v1/admin/*     → admin-service :8000（B 端 BFF）
  ├─ /api/v1/consumer/*  → api-service   :8100（C 端 BFF）
  ├─ /api/v1/storage/*   → storage-service :9301（签名 URL，服务端代理读写对象存储）
  └─ /*                  → web-admin :80（运营后台，/admin/）

BFF ─gRPC→ hello-service :9100 / storage-service :9300
底层服务 ─→ PostgreSQL（一服务一库）；storage-service ─→ MinIO / S3
```

| 服务 | 端口 | 职责 |
|---|---|---|
| `app/gateway` | 9080 | APISIX：统一入口与路由 |
| `admin-service` | 8000 | B 端 BFF：Hello、存储全部纯代理 |
| `api-service` | 8100 | C 端 BFF：Hello 手写示例 |
| `hello-service` | 9100 | 最小示例：gRPC + PostgreSQL + 迁移 + gen |
| `storage-service` | 9300 / 9301 | 文件与存储供应商；文件读写 HTTP |
| `app/web/admin` | 3000（dev） | 运营后台：Hello、存储供应商、上传示例 |

## 目录

```text
.
├── api/
│   ├── service/                 接口 proto：common/、<底层服务>/、admin_service/<模块>/、api_service/<模块>/
│   ├── config/                  服务配置 proto
│   ├── third_party/             vendored proto（validate / errors / google api），生成不依赖远程 buf 模块
│   ├── gen/{go,ts}/             生成代码（提交；ts 是 pnpm 包 @beidou-layout/api-gen）
│   └── errcode_test.go          错误码分段校验
├── app/
│   ├── service/<name>-service/  后端服务（底层 gRPC 服务与 BFF 同一套结构）
│   ├── gateway/                 APISIX 镜像：conf/ + apisix/plugins/
│   └── web/admin/               运营后台（Umi Max + antd）
├── pkg/                         Go 公共库（x 后缀，只放被引用的）
├── deploy/                      Dockerfile + Helm chart + 构建 / 部署脚本
├── scripts/                     new-project.sh、proto 插件与 TS 修正工具、check-gorm-gen.sh
├── tests/                       api（Go 集成测试，经网关）/ e2e（Playwright）
├── docs/                        设计文档与决策记录
└── .agents/skills/              开发规范：api-convention / new-service / gen-proto
```

底层服务结构（以 `hello-service` 为例）：

```text
cmd/{main.go,hello,migrate,gen}   cobra 多命令：启动 / 迁移 / 生成 dao
configs/config.yaml               本地配置（密钥放同目录 config.local.yaml）
internal/server                   gRPC server 装配（显式超时 + 校验中间件）
internal/service/<domain>         实现 gRPC 接口：pb ↔ model 转换
internal/biz                      业务逻辑，内嵌 dao
internal/dao                      dao.go（事务）+ gorm gen 生成物
model/                            gorm 模型 + biz 接口 + Param
migrate/                          gormigrate 迁移 + 一致性测试
pkg/constants                     服务常量
```

BFF：`cmd/server` + `initializer/clients.go`（下游 client + 代理注册）+ `internal/{service,server}`；有手写方法时再加 `model/`（biz 接口）和 `internal/biz`（见 `api-service`）。

## 快速开始

依赖：Go 1.26+、[buf](https://buf.build)、Node 22 + pnpm 11、PostgreSQL；`make lint` 需要 golangci-lint，`make dep-check` 需要 staticcheck；部署需要 Docker（buildx）、Helm 4 与 kubectl。

```bash
# 1. 建库（每个底层服务一个，库名 = 服务名下划线形式）
createdb hello_service && createdb storage_service

# 2. 编译与迁移（连接串在各服务 configs/config.yaml，默认 postgres:postgres@127.0.0.1:5432）
make build
make migrate

# 3. 起服务（各开一个终端）
make run-hello
make run-storage
make run-admin
make run-api

# 4. 运营后台
pnpm install
pnpm dev:admin               # http://localhost:3000（dev 代理：/api/v1/admin → :8000，/api/v1/storage → :9301）
```

C 端 BFF 直接调：

```bash
curl -X POST http://127.0.0.1:8100/api/v1/consumer/Hello/SayHello -d '{"name":"world"}'
```

上传示例需要先在「存储供应商」页创建 code 为 `default` 的供应商（例如本地 MinIO，见 `docs/storage.md`「本地与部署」）。

## 常用命令

| 命令 | 说明 |
|---|---|
| `make tools` | 安装 proto 插件到 `tools/`（`make api` 会先跑它） |
| `make api` | 编译 proto 插件到 `tools/` → 生成 Go / TS / 配置结构体 → 修正 TS（int64、transport、字段名） |
| `make proto-check` | `buf lint` + `buf breaking`（对比 main） |
| `make gen` | gorm gen 生成 dao（不需要数据库） |
| `make build` | 编译全部服务到 `build/bin/` |
| `make migrate` | 全部底层服务跑迁移 |
| `make run-<svc>` | 本地运行：`hello` / `storage` / `api` / `admin` |
| `make test` | `go test ./...`（迁移一致性测试设置 `TEST_HELLO_DSN` / `TEST_STORAGE_DSN` 才跑，见 `tests/README.md`） |
| `make lint` | golangci-lint + 禁止手写 gorm 条件 |
| `make dep-check` | staticcheck SA1019：禁止使用 Deprecated API |
| `make check-gorm-gen` | 只跑「禁止手写 gorm 条件」检查（`make lint` 已包含） |
| `make test-api` | 经网关跑 `tests/api` 集成测试，一般对部署好的开发环境：`TEST_BASE_URL=https://<域名> make test-api` |
| `make test-e2e` | Playwright 跑运营后台（`ADMIN_WEB_URL`） |
| `pnpm dev:admin` / `pnpm --filter @beidou-layout/web-admin {typecheck,test,build}` | 运营后台 |

## 部署（Helm）

```bash
# 开发环境：tag 回写 values-dev.yaml，再叠加它部署
DEPLOY_ENV=dev deploy/scripts/build-images.sh && DEPLOY_ENV=dev deploy/scripts/deploy.sh
# 生产环境：tag 回写 values.yaml
deploy/scripts/build-images.sh && deploy/scripts/deploy.sh
```

首次部署：

1. 把 `deploy/charts/beidou-layout/values*.yaml` 里的 `CHANGE_ME` 换成真实值；数据库密码、存储签名密钥写在不进 git 的私有 values（`values.secret.local.yaml` / `values.secret.dev.local.yaml`）里覆盖，每个环境各自生成（`openssl rand -hex 32`）。`deploy.sh` 检查合并渲染后的结果，还有 `CHANGE_ME` 就拒绝部署
2. 建库：`hello_service` / `storage_service`（开发环境加 `dev_` 前缀）
3. 部署后在运营后台「存储供应商」创建 code 为 `default` 的供应商

详见 [`deploy/README.md`](deploy/README.md) 与 [`deploy/charts/beidou-layout/README.md`](deploy/charts/beidou-layout/README.md)。

## 新建项目

```bash
scripts/new-project.sh <目标目录> <项目名> <Go module>
# 例：scripts/new-project.sh ../order order github.com/acme/order
```

脚本复制本目录（不含 `.git`、`LICENSE`、依赖与构建产物、`*.local.yaml`），把占位 `beidou-layout` / `Beidou Layout` 和 Go module 全局替换成新项目名，`deploy/charts/beidou-layout` 改名，
再 `make api` 重新生成 proto 代码（`.pb.go` 里的 descriptor 不能文本替换），最后 `git init`。需要本机有 go 和 buf。
模板里写新代码时，项目名只用这两种写法，批量生成才不会漏替换。

## 开发规范

规范放在 `.agents/skills/`（Markdown，人和编码助手都按它来），改代码前先读：

- [`api-convention`](.agents/skills/api-convention/SKILL.md)：命名、proto 组织、路由、错误、分层、gorm、迁移、配置
- [`new-service`](.agents/skills/new-service/SKILL.md)：新增底层服务 / BFF 模块的脚手架与登记清单
- [`gen-proto`](.agents/skills/gen-proto/SKILL.md)：proto 生成链路与额外步骤
- [`app/web/admin/README.md`](app/web/admin/README.md)：运营后台结构与新增页面

## 技术栈

| 层 | 选型 |
|---|---|
| 后端 | Go 1.26、kratos v3、gRPC、gorm v1.31 + gen、gormigrate v2、cobra、protovalidate |
| 存储 | minio-go v7、aws-sdk-go-v2（S3） |
| 契约 | buf v2、protoc-gen-go / go-grpc、kratos errors / http 插件、protoc-gen-typescript-http（kratos fork） |
| 网关 | APISIX 3.x standalone |
| 前端 | React 19、Umi Max 4、antd 6、pnpm workspace、Playwright |
| 部署 | Docker 镜像 + Helm 4（Kubernetes ≥ 1.30） |

## 贡献

欢迎通过 GitHub Issue 反馈问题、PR 提交改进。提交前请确认：

- `make lint test proto-check` 与 `pnpm --filter @beidou-layout/web-admin typecheck test` 通过
- 改了 proto 要 `make api` 并提交生成物（CI 会检查生成物与 proto 一致）
- 有意的 proto 破坏性变更在 PR 上打 `buf-breaking-ok` 标签并说明原因

## License

[MIT](LICENSE)
