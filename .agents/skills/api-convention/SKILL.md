---
name: api-convention
description: 接口命名、proto 组织、HTTP 路由、错误处理、分层、迁移与 gorm、配置等约定。当定义新接口/新 proto/新错误、写迁移、改配置或调整服务结构时参考。
---

# 接口与命名规范

> 本规范覆盖从路由到分层、从 proto 到错误的完整约定。改动前先读这里。示例代码以 `hello-service` / `api-service` / `admin-service` 为准。

## 一、服务命名与目录

| 层 | B 端 | C 端 |
|---|---|---|
| 应用服务 / BFF（HTTP） | `admin-service`（:8000） | `api-service`（:8100） |
| 路由前缀 | `/api/v1/admin` | `/api/v1/consumer` |
| 前端 | `app/web/admin`（运营后台） | 自行接入（App / H5 等） |

- 底层服务（gRPC，碰 DB）两端共用：示例 `hello-service`（:9100）、对象存储 `storage-service`（:9300，文件 HTTP :9301）；新服务 gRPC 端口按百位顺延（:9200、:9400 …），需要 HTTP 端口的用同一百位的下一个号（如 :9301）
- 所有服务统一放 `app/service/`（HTTP 服务也是 service）；网关是独立的 `app/gateway`（APISIX 镜像）
- 应用服务是**单服务多模块**：`internal/service/{domain}/`，一个下游/领域一个包（整个模块纯代理时不建包）
- **数据库隔离**：每个底层服务独立数据库，服务名与库名一一对应（`hello-service` → `hello_service`），DSN 由各服务 `configs/config.yaml` 指定，不共用库；库需预先建好（迁移不建库）

## 二、proto 组织

```
api/service/hello_service/          # 底层服务
├── service.proto                   # service 定义 + Req/Reply
└── error.proto                     # 服务错误
api/service/admin_service/
└── admin_hello/                    # 模块目录（端 + 领域）
    └── admin_hello.proto
api/config/<service_name>/conf.proto  # 服务配置结构
```

- **目录**：底层 `api/service/{service_name}/`；HTTP 模块 `api/service/{service_name}/{module}/`（**目录不带版本**）
- **模块命名**：`{端}_{领域}`，如 `admin_hello` / `api_hello`
- **package**：镜像目录、不带版本 —— 底层 `service.{service_name}`，HTTP 模块 `service.{service_name}.{module}`（RPC 名形如 `/service.admin_service.admin_hello.Hello/SayHello`）。`PACKAGE_DIRECTORY_MATCH` 已开；兼容靠 `buf breaking` + 只增不改
  - 每个模块独立 package 的原因：protoc-gen-typescript-http 按 package 汇总输出单文件，**同 package 的第二个 proto 文件会被丢弃**
- **go_package**：`.../api/gen/go/service/{service_name}/{module};{alias}`（路径与别名均无版本）
- **service 名**：领域名（`Hello` / `Storage`）；同一领域在 B / C 端各有一套时加端前缀区分
- **文件**：HTTP 模块 `{module}.proto`（与目录同名）；底层 `service.proto` + `error.proto`
- **排版**：service 定义在前（rpc），message 在后（Req/Reply），message 按域分组（`// ============ 实体 ============`）
- **时间字段**：时间点用 `google.protobuf.Timestamp`，不要 `string`/`int64`；Go 侧用 `timestamppb.New(t)`
- **TS 只生成对外 HTTP 服务**：`buf.gen.ts.yaml` 只生成 `admin_service` + `api_service` 的请求桩；底层服务是内部 gRPC、前端不直接访问，**不生成 TS**。新增 HTTP 模块后要在 `api/gen/ts/package.json` 的 `exports` 手动补一项，步骤见 `gen-proto` skill
- **HTTP 响应编码**：BFF 挂 `pkg/codecx.ProtoJSON()`，proto 消息按 protojson 映射输出（字段名 proto 原名 snake_case、Timestamp→RFC3339、64 位整数→字符串），请求两种写法都认；TS 生成物由 `scripts/proto/fix-ts-field-names` 改回 snake_case，`fix-ts-int64` 把 64 位字段标成 `string`，前端使用处用 `Number()` 转换
- **公共 message**：跨服务复用的放 `api/service/common/`（package `service.common`；分页 `Page` 等统一进 `params.proto`），列表请求统一嵌套 `common.Page page = N;`（字段名 `current`/`size`），响应总数内联 `int64 total`
- **访问 proto 字段统一用 Get 方法**：读取 proto message 字段（含配置 `conf`）一律用 `GetXxx()`，嵌套 message 用 `GetA().GetB()` 链式调用，**禁止直接访问字段**（如 `req.Name`），避免嵌套 message 为 nil 时 panic。构造 message 用字段名（`&Xxx{Field: v}`）
- **config proto 聚拢 + 公共结构放 common**：相关字段收进一个 message（如 `proxy { downstreams }`），跨服务复用的结构放 `api/config/common/`（如 `conf.proto` 的 `Proxy`）。服务各层统一使用 `*conf.Config` 大结构，**不拆散传参**（`NewBiz(db, cfg)`，不要 `NewBiz(db, a, b)`）

## 三、HTTP 路由

```
B 端：POST /api/v1/admin/{Service}/{Method}
C 端：POST /api/v1/consumer/{Service}/{Method}
```

- **网关只放行显式前缀**：APISIX（`app/gateway/conf/apisix.yaml`）只为 `/api/v1/{admin,consumer}/*` 配了 API 路由，另有 `/api/v1/storage/*`（文件读写，不经过 BFF，storage-service 校验 URL 签名，见 `docs/storage.md`）
- **POST-only**（所有接口 POST，body 统一 JSON）
- **方法名**：动词 + 资源（`ListGreeting` / `GetFile` / `CreateProvider` / `UpdateProvider` / `DeleteProvider`）
- **必须写完整注解**（省不掉，protoc-gen-go-http 默认路由含 package 前缀且不绑定 body）：

```proto
rpc SayHello(SayHelloReq) returns (SayHelloReply) {
  option (google.api.http) = {
    post: "/api/v1/consumer/Hello/SayHello"
    body: "*"          // 必须，否则 POST JSON 不解析（只 BindQuery）
  };
}
```

### BFF：纯代理 vs 手写

- **纯代理（默认）**：BFF service 不实现该方法（返回 Unimplemented），`pkg/proxyx` 中间件按配置的路径反射调用下游 client 的**同名方法**转发。适用于与底层同名、参数原样透传的方法 —— 示例：`admin-service` 的 `admin_hello`
- **手写**：只有需要特殊处理时才写 service + biz —— 强制字段、多下游聚合、改造响应 —— 示例：`api-service` 的 `api_hello`。**手写的方法必须从 proxy 配置里去掉**，否则改动容易被误以为走代理
- **整个模块都是纯代理**时 service 只嵌 `UnimplementedXxxServer`，不写方法（见 `admin-service/internal/service/hello`）
- **BFF 请求/响应类型（二选一，不混用）**：
  - 纯代理方法：成对复用底层 `Req/Reply`（proto import 底层 service），不重复定义
  - 手写方法：成对定义自己的 `{Method}Req/{Method}Reply`，即使只差一个字段也不套用底层 message；实体 message 可在其中复用
- **身份不信任前端**（接入认证后）：需要「当前用户」的接口，BFF Req 不含用户字段，由 BFF 从自己的身份上下文取出后填进下游请求；底层服务**不从 ctx 取身份**，只用请求参数

代理路径在配置里**逐个方法显式列出**（`filepath.Match` 也支持 `/*` 通配，但统一逐条列，手写方法不在列表里一目了然）：

```yaml
# configs/config.yaml
proxy:
  downstreams:
    hello:
      paths:
        - "/service.admin_service.admin_hello.Hello/SayHello"
        - "/service.admin_service.admin_hello.Hello/ListGreeting"
```

```go
// initializer/clients.go：构造下游 client 后按配置注册
proxyx.AddClient(client, c.GetProxy().GetDownstreams()["hello"].GetPaths())
// internal/server：http.Middleware(recovery → validatex → proxyx.Server()) 一次性传入
```

### 认证与访问控制（模板不包含）

模板不带账号与认证，所有接口直接可用。上线前按自己的方案接入，建议的分工：

- 横切的认证 / 接口级访问控制放在网关（`app/gateway/apisix/plugins/` 加插件，在路由上启用），BFF 只校验请求确实来自网关
- BFF 中间件链在 `validatex` 与 `proxyx` 之间插入身份中间件，手写方法从 ctx 取身份
- 资源内的权限（这条记录是不是你的）留在 BFF / 领域层，底层服务只认请求参数

## 四、参数校验（buf validate）

- 依赖：`validate.proto` 已 vendored 在 `api/third_party/`（`buf.yaml` 无远程 deps），proto `import "third_party/buf/validate/validate.proto"`，见 `api/third_party/README.md`
- 服务端加校验中间件：`pkg/validatex`（protovalidate-go + kratos middleware/validate），gRPC/HTTP server 都挂
- 常用规则：

```proto
string name = 1 [(buf.validate.field).string = {min_len: 1, max_len: 50}];       // 必填 + 长度
uint32 id = 1 [(buf.validate.field).uint32 = {gte: 1}];                           // 必填 ID
string email = 4 [                                                                  // 可选 email（空跳过）
  (buf.validate.field).ignore = IGNORE_IF_ZERO_VALUE,
  (buf.validate.field).string.email = true
];
```

- **注意**：`ignore_empty` 已废弃（reserved），可选字段用 `ignore = IGNORE_IF_ZERO_VALUE`
- 校验失败自动返回 400 `VALIDATOR`（kratos 中间件），无需手写参数检查
- **参数校验只在 proto 层**：格式校验统一在 proto 配规则；代码层（service/biz）**禁止手写参数校验**，只保留业务校验（存在性/唯一性/权限/状态机）。发现字段缺规则时补 proto 规则，不要在代码里 `if x == ""` 判空
- **复杂 JSON 配置**：把配置结构定义成 proto message + buf validate 规则；业务层用 `protojson.Unmarshal` + `protovalidate.Validate` 校验，**禁止手写字段校验**（示例：存储供应商配置 `MinioConfig` / `S3Config`，见 `pkg/storagex`）

## 五、错误处理

**所有错误通过 proto 枚举定义 + 生成，禁止 `errors.New` 魔法字符串。**

### 目录

```
api/service/common/error.proto             # 公共错误（跨服务共享）
api/service/{service}/error.proto          # 底层服务错误
api/service/{service}/{module}/error.proto # HTTP 模块错误（可选）
```

### 错误码分段（全局唯一）

**规则：每个服务独占一个千位段（段号 N → `N000–N999`），段内百位划分子域。** 段不重叠，错误码即全局唯一。

| 段号 | 服务 | 子域 |
|---|---|---|
| 1 | common（公共） | 10xx 通用（参数）/ 12xx DB |
| 2 | hello-service | 20xx 问候 |
| 3 | storage-service | 30xx 文件 / 31xx 签名与 scope / 32xx 供应商 |

- 新增服务：段号顺延（下一个为 4），并在 `api/errcode_test.go` 的 `segments` 登记一行；`go test` 会校验每个错误码都落在本服务段内
- 新服务的 `error.proto` 还要在 `buf.yaml` 的 `ignore_only`（`ENUM_VALUE_PREFIX` / `ENUM_ZERO_VALUE_SUFFIX`）登记（kratos errors 惯例：值不带前缀，0 为 `SUCCESS`）
- 线上传输的是错误 **reason（枚举名）+ HTTP code**，枚举数值不上线；因此**枚举名只增不改**（`IsXxx` 依赖名字），数值仅用于分段管理

### HTTP code（不 500 一把梭）

| 类型 | code |
|---|---|
| 参数异常 / 业务校验不通过 | 400 |
| 认证失败（未登录、token 失效） | 401 |
| 无权限/禁用 | 403 |
| 资源不存在 | 404 |
| 系统/DB 失败 | 500 |

**不能用 200**（gRPC 传输时 200 映射 OK 会破坏错误还原）。

- **401 只留给认证失败**（未登录、凭证失效），业务校验失败一律用 400，不要占用 401 的语义（接入认证后前端通常据此跳转登录）
- 资源不存在且涉及他人数据时用 404 而非 403，避免泄露存在性

### 用法

```proto
enum ErrorReason {
  option (errors.default_code) = 500;
  GREETING_GET_FAIL = 2002; // 问候记录查询失败（默认 500）
  // 带 HTTP code 的写法：FILE_NOT_FOUND = 3001 [(errors.code) = 404];（storage-service）
}
```

生成后业务返回 `ErrorXxx`、上层断言 `IsXxx`：

```go
return nil, hellosvc.ErrorGreetingCreateFail("create greeting failed: %+v", err) // biz 返回
if storagesvc.IsFileNotFound(err) { ... }                                        // 上层断言
```

- 公共库（pkg/）返回的错误也用 proto 错误（如 `pkg/storagex` 返回 storage-service 的 `ErrorProviderConfigInvalid`）
- biz 层的 DB 错误统一包装成 `XxxFail`（如 `ErrorGreetingGetFail`），不裸返回 gorm err

## 六、分层规范

### 底层服务（gRPC，碰 DB）

```
service → biz（内嵌 dao）→ dao（gen 生成）→ DB
```

- **biz 内嵌 `*dao.Dao`**，数据库访问直接走 gen dao，**没有 data 层**（不搞 Repo 接口抽象）
- **model 接口 `I<Svc>Biz`** 聚合所有子接口，service 只依赖这个接口（hello-service 的 `model.IHelloBiz`）；子接口多了按实体拆文件，接口文件只留聚合
- **参数用 Param 结构体**（`ListGreetingParam` 等，小结构体值传递），不用 model + map 指定字段
- **ID 用 uint32 自增**，不用 uuid string
- **公共 Model**：`pkg/datatypex.Model`（`ID uint32` + `CreatedAt/UpdatedAt` + `DeletedAt` 软删）

#### 数据库访问只用 gorm gen（禁止手写条件）

- **查询 / 更新 / 删除一律走 gen**：`b.WithContext(ctx).Greeting.Where(b.Q.Greeting.ID.Eq(id)).First()`；条件用字段表达式（`Eq`/`In`/`IsNull`/`Gt`…），排序用 `Order(q.ID.Desc())`，更新用 `UpdateSimple(q.A.Value(v), q.B.Add(1))`
- **禁止**在 biz 里写 `.Where("col = ?", v)`、`.Order("id desc")`、`.Raw` / `.Exec`、`.Model(&T{})`、`Updates(map[string]any{...})`、`gorm.Expr`（`make lint` 里的 `scripts/check-gorm-gen.sh` 会检查）
- **事务**：`b.Tx(ctx, fn)`，子调用继续传同一个 `ctx`（`WithContext` 会复用事务连接）
- **`UpdateSimple` / `Update` 会自动写 `updated_at`**：不要再手动 `UpdatedAt.Value(...)`，否则 Postgres 报 multiple assignments
- **gen 写不出来的语句**（advisory lock、`DISTINCT ON`、聚合等）写成 DIY method：在 `model/query.go` 定义接口，注释即 SQL，参数全部 `@name` 化；`model.GetModels()` 里对该模型挂上接口，再 `make gen`；业务只调生成方法
- `make gen` 不需要起数据库（`pkg/dbx.NewGormOffline`，gen 只解析模型）

### 应用服务 / BFF（HTTP，不碰 DB）

```
initializer/clients.go      # 下游 client 构造 + proxyx 注册
model/                      # BFF 根目录：Service↔Biz 契约
  ├── biz_interface.go      # I<Domain>Biz（按域拆，非一个大 IBiz）
  ├── <domain>.go           # 该域 XxxParam / XxxResult（需要时）
  └── convert.go            # (*Result).ToAPI() / To<Domain>()（需要时）
internal/
  ├── biz/<domain>.go       # 持有下游 client，实现 model.I<Domain>Biz
  ├── service/<domain>/     # 每个领域一个包
  └── server/server.go      # HTTP server 装配
```

- **禁止 service 一把梭**：service 只做 pb ↔ model 转换 + 参数解析，**禁止持有/调用下游 client**
- **model 按域拆分**：`I<Domain>Biz` 接口 + `XxxParam`/`XxxResult`；响应转换用 `(*Result).ToAPI()`，禁止 service 手写转换循环
- **biz 按域拆分**：`New<Domain>Biz(client) model.I<Domain>Biz`；参数多时签名用 `(ctx, *Param) (*Result, error)`
- **下游 client 构造收敛到 `initializer/clients.go`**（含 proxyx 注册），`cmd/server/app.go` 只做装配
- **纯代理方法**不经过 service/biz，由 `pkg/proxyx` 直接反射转发

## 七、公共库与常量

> **pkg 目录命名**：项目自有 `pkg/` 目录统一加 `x` 后缀（`configx`/`proxyx`/`dbx`…），防止与标准库/第三方库 import 命名打架；目录名与 package 名一致。

- `pkg/configx`：配置加载 `configx.Load(path, cfg)`（kratos config + protovalidate 校验，规则在 conf.proto）；同目录有 `config.local.yaml` 时叠加覆盖（放本机密钥，git 忽略）
- `pkg/dbx`：gorm 初始化 `dbx.NewGorm(dsn)`（postgres + 单数表名，返回 db + cleanup）；`NewGormOffline` 不连库（gorm gen 用）；`dbx/dbxtest` 迁移测试辅助
- `pkg/loggingx`：kratos logger 初始化（JSON 结构化 + `FilterKey` 脱敏 password / secret / token / authorization + `service.name` 属性），服务启动时 `loggingx.Init(serviceName)`
- `pkg/proxyx`：HTTP→gRPC 代理中间件（`AddClient` 注册路径，`Server()` 对未实现方法反射转发）
- `pkg/validatex`：buf validate 校验中间件
- `pkg/codecx`：`codecx.ProtoJSON()` —— HTTP 层 proto 消息按 protojson 编码（snake_case/RFC3339，对齐 TS 类型），BFF 均挂载
- `pkg/storagex`：对象存储供应商注册中心（`Register` / `New` / `Normalize` / `Redact`），实现在 `storagex/minio`、`storagex/s3`（storage-service 空白 import）
- `pkg/signaturex`：URL 签名 `Sign` / `Valid`（HMAC-SHA256，method + key + expire）
- `pkg/datatypex`：公共基础模型 `Model`（ID/时间/软删）
- `pkg/enumx`：库内字符串 ↔ proto 枚举互转（`From` / `To`），各枚举的具体转换函数按需加在这里
- 魔法字符串都提取为常量；服务自身常量放 `app/service/<svc>/pkg/constants`
- **枚举（`buf.yaml` 的 ENUM_* 规则 + Google AIP-126）**：
  - **什么用枚举**：封闭、稳定、前后端会按值分支的集合（状态、种类、结果）。定义在 `api/service/common/*_enums.proto`：大驼峰类型名；取值带类型前缀 `FILE_STATUS_PENDING`；`0 = XXX_UNSPECIFIED`，只表示「没填 / 不限」
  - **什么不用枚举（用 string + 校验）**：运营可扩展或数据驱动的值、外部系统定义的值、只在服务内部用的实现细节。没人用的枚举不要预先定义
  - **发布后不改名、不改编号**：JSON 传的是枚举名，改名就是不兼容；删掉的值用 `reserved`
  - **接口字段**：必填 `(buf.validate.field).enum = {defined_only: true, not_in: [0]}`；列表筛选用 `UNSPECIFIED` 表示「不限」，前端不筛选时**不传**
  - **落库存枚举名**（与 JSON 一致），转换只在 `pkg/enumx`；biz 里**禁止**写枚举值字面量
  - **配置文件尽量不用枚举**（kratos 加载时不认识的枚举名会被静默清零）；必须用时加 buf validate 规则，`pkg/configx` 的 `TestRepoConfigsLoad` 会加载仓库里的配置并校验
  - TS 用 `@beidou-layout/api-gen` 生成的字符串联合类型，不手写
- 业务日志用 `klog.InfoContext(ctx, "msg", "k", v)`，敏感字段靠 FilterKey 脱敏

## 八、迁移规范（gormigrate）

- 迁移文件放 `migrate/migrations/`，命名 `YYYYMMDDHHMMSS_描述.go`，`Migration.ID` 与文件名时间戳一致。每个文件在 `init()` 里 `Migrations = append(...)`，Go 按**文件名字典序**执行 init，所以**文件名时间戳就是执行顺序**；新迁移的时间戳必须大于本服务已有的最大值，**不能用未来日期**
- `migrate.go` 开了 `ValidateUnknownMigrations`：库里有代码不认识的迁移 ID（删过迁移没清库 / 镜像回滚到更早版本）会直接报错
- **列定义约定**：`varchar` 最小 `varchar(100)`（短名字段可按业务收紧，如示例 `name varchar(50)`）；存枚举名的列用 `varchar(100)`；外键列写明指向哪张表（`category_id` 而不是 `type_id`）
- **模型与迁移必须一致**：`migrate/migrate_test.go` 在空库跑迁移后，用 `pkg/dbx/dbxtest.ModelsMatchSchema` 对 gorm 模型再跑一次 AutoMigrate，gorm 想加列 / 改类型 / 建索引都会报错（`TEST_*_DSN` 指向专用测试库，测试会清空它；不设则跳过）
- **只做加法**：已上线的迁移**不改不删**；要改表结构/数据就新增一个迁移
- 表结构快照**定义在 init 内**（局部类型，大写开头），**禁止引用 model 包**（迁移不可变）
- 表名靠 gorm `SingularTable: true`（`pkg/dbx`）自动推导（Greeting → greeting），无需 TableName 方法；AutoMigrate 表达不了的（部分索引、数据修复）用 `db.Exec` 原生 SQL
- 默认数据不自设 ID（自增回填后再关联）
- **数据修复迁移必须幂等、只动该动的行**（`WHERE` 条件、`IF EXISTS` / `IF NOT EXISTS`）
- 部署时迁移在服务启动前执行：k8s 由 values `workloads.server.migrate: true` 生成 initContainer，本地 `make migrate`

## 八之二、gorm 注意事项

- **更新指定列，不要整行 `Save`**：`Save` 会把结构体所有字段写回，`string` 零值会把库里的 `NULL` 写成 `''`。更新用 gen 的 `UpdateSimple(q.Col.Value(v), ...)` 只写需要的列
- **可空的唯一列**：模型字段加 `default:null`，唯一约束用**部分唯一索引**（迁移里 `db.Exec`，如 `... WHERE deleted_at IS NULL`），不要用 gorm tag 的 `uniqueIndex`
- 软删表的唯一约束都要考虑 `deleted_at IS NULL`，否则删除后无法再用同一个值

## 九、配置

### 配置两份副本

每个服务的配置有两份，**改配置项（尤其新增字段）要两处同步**，否则某个环境启动时 `configx` 校验失败或缺功能：

| 位置 | 用途 | 下游地址 / 密钥 |
|---|---|---|
| `app/service/<svc>/configs/config.yaml` | 本地 `make run-*` | `127.0.0.1:<port>`、开发密钥（`dev-only-change-me-*`） |
| `deploy/charts/beidou-layout/files/config/<svc>/config.yaml` | k8s（Helm `tpl` 渲染成 ConfigMap） | 服务名；密钥/库名等写成 `{{ .变量 }}`，值在 `values.yaml` 的 `config` / `services.<svc>.config`（密钥默认 `CHANGE_ME`，放私有 values） |

- chart 配置**逐项写明**，只用 `{{ .xxx }}` 取值，不用 range / dict / if 生成配置
- 本机密钥写同目录 `config.local.yaml`（`pkg/configx` 自动叠加覆盖，`*.local.yaml` 已 git 忽略），不要提交进 `configs/config.yaml`
- 配置结构以 `api/config/<svc>/conf.proto` 为准（改完 `make api`），必填项用 buf validate 规则约束；`go test ./pkg/configx` 会加载本地配置校验
- 网关路由/插件配置只有一份：`app/gateway/conf/apisix.yaml`（打进网关镜像，密钥用 `${{ENV}}` 注入，chart 在 `services.gateway` 的 env 里给值）

## 十、一键检查清单

新增接口/服务前对照：

1. [ ] 服务在 `app/service/` 下，BFF 模块在 `internal/service/{domain}/`
2. [ ] proto 在 `api/service/{service}/{module}/{module}.proto`（底层服务 `api/service/{service}/service.proto`）
3. [ ] 路由 `/api/v1/{admin|consumer}/{Service}/{Method}`，rpc 写了 `google.api.http` 注解 + `body: "*"`
4. [ ] 纯代理方法加进 proxy 配置；手写方法不在 proxy 配置里
5. [ ] 错误用 proto 枚举（`error.proto`），公共错误放 `common`，code 用 4xx/5xx 不用 200；新服务登记错误码段
6. [ ] biz 内嵌 dao，无 data 层，参数用 Param 结构体，ID 用 uint32；更新用 `UpdateSimple` 指定列；不手写 gorm 条件
7. [ ] 迁移只加不改，数据修复幂等
8. [ ] 配置改动两份副本同步（本地 + chart）
9. [ ] 无魔法字符串（`errors.New` 等）
10. [ ] 新 HTTP 模块：`api/gen/ts/package.json` 的 `exports` 已登记
