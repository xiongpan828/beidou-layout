---
name: new-service
description: 创建新的 kratos 服务。当需要新增底层 gRPC 服务或 HTTP 服务时使用。
---

# 新建 kratos 服务

> 命名/路由/错误/分层/迁移/配置规范见 `api-convention` skill，本 skill 只讲脚手架结构和「新服务要登记的地方」。

## 底层服务（gRPC，碰数据库）

**直接复制 `app/service/hello-service`**（带迁移、gen 的最小示例），全局替换 `hello` → 新领域名（同时需要 HTTP server 的——如对外的文件流——参考 `storage-service`：gRPC + HTTP 两个 server，网关单独一条路由，接口自己校验签名）：

1. 在 `api/service/<service_name>/` 下定义 gRPC proto（`service.proto` + `error.proto`）
2. 创建 `app/service/<service-name>/`，结构：
   - `cmd/`：cobra 多命令（`main.go` 根命令 + 子命令目录）：
     - `<domain>/`（如 `hello`）：`<domain>.go` 只放命令定义（RunE 里调装配函数、组装 server、kratos 启动，保持清爽）；`app.go` 放装配函数（config → db → biz → service，返回 service + config + cleanup）
     - `migrate/`、`gen/`
   - 初始化用公共库：`pkg/configx.Load` / `pkg/dbx.NewGorm` / `pkg/loggingx.Init`（无 initializer/ 目录）
   - `internal/service/<domain>/`：实现 gRPC 接口（依赖 `model.I<Svc>Biz`），**按域归类**；`service.go` 放服务结构体 + 构造函数，`rpc.go` 放 RPC 方法，`convert.go` 放 model → pb 转换
   - `internal/biz`：业务逻辑，**内嵌 `*dao.Dao`**，无 data 层
   - `internal/dao`：手写 `dao.go`（Dao + WithContext + Tx）+ gen 生成（勿手改；`make gen` 不需要数据库）。首次生成时 `cmd` 还编译不过（biz 依赖的 dao 代码还没有）：先删掉复制来的 `*.gen.go`，用一个只 import `cmd/gen` 的临时 main 跑一次 `gen.Cmd`，之后 `make gen` 即可
   - `internal/server`：装配 gRPC server，**显式设置 `grpc.Timeout`**（Kratos 默认 1s，示例 10s）
   - `model/`：gorm 模型 + `I<Svc>Biz` 接口 + Param 结构体；`model.go` 是 gen 模型注册表，`query.go` 放 DIY 查询接口
   - `pkg/constants/`：服务自身常量（`ServiceName`，供 cobra/kratos.Name/loggingx 复用）
   - `migrate/`：gormigrate 迁移入口 `migrate.go` + `migrations/`（`migrations.go` 声明 `Migrations`，每个迁移一个 `YYYYMMDDHHMMSS_描述.go`）+ `migrate_test.go`
3. 配置 proto 在 `api/config/<service_name>/conf.proto`（生成结构体，不手写），然后 `make api`
4. `error.proto` 申请新错误码段号（见 `api-convention`「错误码分段」），在 `api/errcode_test.go` 的 `segments` 和 `buf.yaml` 的 `ignore_only` 登记

## HTTP 服务 / BFF（南北流量）

> 现有两个 BFF：`admin-service`（B 端）、`api-service`（C 端）。新增领域一般是在已有 BFF 里**加模块**，而不是新建 HTTP 服务。

在已有 BFF 里加模块：

1. 在 `api/service/<bff>/<module>/` 下定义 HTTP proto（模块目录 + `google.api.http` 注解 + `body: "*"`；纯代理方法成对复用底层 Req/Reply，手写方法成对定义 `{Method}Req/{Method}Reply`）
2. 纯代理：`internal/service/<domain>/service.go` 只嵌 `UnimplementedXxxServer`（参考 `admin-service/internal/service/hello`），配置 `proxy.downstreams.<下游>.paths` 逐条登记
3. 手写：`model/biz_interface.go` 加 `I<Domain>Biz` → `internal/biz/<domain>.go` 实现 → `internal/service/<domain>/` 只做转换（参考 `api-service` 的 hello）
4. 新下游：`initializer/clients.go` 的 `Clients` 加字段 + 循环里加一行；`api/config/<bff>/conf.proto` 加 `<svc>_grpc_addr`
5. `internal/server/server.go` 注册 `RegisterXxxHTTPServer`
6. TS 请求桩：`make api` 后在 `api/gen/ts/package.json` 的 `exports` 补模块（见 `gen-proto` skill）

新建 BFF 时复制 `app/service/api-service`，结构：

- `cmd/server/main.go`：入口（flag `-conf` + `newServer()` → kratos.New → Run）
- `cmd/server/app.go`：`newServer()` 装配函数（config → `initializer.NewClients` → biz → service → http server，返回 server + cleanup）
- `initializer/clients.go`：下游 client 构造 + `proxyx.AddClient` 代理注册
- `model/`、`internal/biz/`、`internal/service/<domain>/`、`internal/server/`（中间件**一次性传入** `recovery → validatex → proxyx`，挂 `codecx.ProtoJSON()`）、`pkg/constants/`
- 把服务目录加进 `buf.gen.ts.yaml` 的 `inputs.paths`

> BFF 分层：`service → biz（聚合下游 RPC）→ 下游 client`；**禁止 service 直接持有/调用下游 client**。纯代理方法由 `pkg/proxyx` 直接转发，不经过 service/biz。

## 命令

- 底层服务（cobra）：`<domain>` 启动主服务（如 `hello`，参数 `-c`）、`migrate` 数据库迁移、`gen` 生成 dao
- HTTP 服务：`cmd/server` 单入口，`-conf` 指定配置

## 新服务登记清单（逐项核对）

**本地开发**

- [ ] `Makefile`：`build-go` 加 `go build`；加 `run-<svc>`；底层服务加进 `migrate`、`gen`；`.PHONY` 补上新目标
- [ ] `app/service/<svc>/configs/config.yaml`（本地地址 `127.0.0.1:<port>`；本机密钥放同目录 `config.local.yaml`，已 git 忽略）
- [ ] 调用它的服务配置里加下游地址（如 BFF 的 `<svc>_grpc_addr`，两份配置都要加）
- [ ] `pkg/configx/configs_test.go` 的 `services` 加一行（校验本地配置能加载）
- [ ] PostgreSQL 建库 `<service_name>`（库名 = 服务名下划线形式；迁移不建库）

**镜像**

- [ ] `deploy/scripts/build-images.sh`：后端 `for spec in` 清单加 `<svc>:<main 包路径>`（底层服务 `./app/service/<svc>/cmd`，HTTP 服务 `./app/service/<svc>/cmd/server`）

**k8s（`deploy/charts/beidou-layout/`）**

- [ ] `values.yaml` 的 `services.<svc>`：`config`（`db_name` 等服务专属变量）+ `workloads.server`（`args`、`ports`，底层服务 `migrate: true`）；`values-dev.yaml` 加开发库名
- [ ] `files/config/<svc>/config.yaml`：Helm `tpl` 渲染，逐项写明，密钥 / DSN 写 `{{ .变量 }}`（有这个目录才会生成 ConfigMap 挂到 `/app/configs/`）
- [ ] 调用方的 `files/config/<caller>/config.yaml` 加下游地址

**网关（仅对外暴露路由时）**

- [ ] `app/gateway/conf/apisix.yaml`：加 `upstreams`（短服务名:端口）和 `routes`

## 依赖版本（重要）

- gorm v1.31.x + postgres driver v1.6.x；gormigrate v2、gorm gen v0.3.x、kratos v3
- proto 插件版本写死在 `scripts/proto/build-tools.sh`（与 go.mod 一致）

## 迁移与错误

- 迁移规范（文件名时间戳即执行顺序、只做加法、数据修复幂等）和 gorm 注意事项见 `api-convention`「八」「八之二」
- 错误全部用 proto 枚举定义（`error.proto`），生成 `ErrorXxx`/`IsXxx`；公共错误放 `api/service/common/error.proto`，业务禁止 `errors.New` 魔法字符串
