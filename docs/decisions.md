# 设计决策

模板里每个「为什么这么做」的出处。改相关代码前先看这里。

## 后端（kratos / gRPC）

| # | 决策 | 原因 / 做法 |
|---|---|---|
| 1 | kratos 用 v3 | 运行时库是 `kratos/v3`（与 v2 不兼容，注意不要混用 v2 的 CLI 生成代码）；v3 的 gRPC client 用 `kgrpc.NewClient(ctx, kgrpc.WithEndpoint(addr))`，没有 v2 的 `DialInsecure` |
| 2 | kratos 插件版本 | `protoc-gen-go-http` / `protoc-gen-go-errors` 用伪版本（见 `scripts/proto/build-tools.sh`），`v3.0.0` tag 下没有这两个插件 |
| 3 | 纯 gRPC 进程当 gRPC 客户端会 panic | kratos v3 `transport/grpc` 的 selector 在包 init 时取全局 selector，进程里没链接 `transport/http` 时取到 nil，连接就绪即 panic。底层服务之间要互调时用 gRPC 自带的 `pick_first`（`kgrpc.WithOptions(grpc.WithDefaultServiceConfig(...))`）；BFF 链接了 http，可以直接用 `kgrpc.NewClient` |
| 4 | gRPC server 显式超时 | kratos gRPC server 默认超时 1s，慢请求会被 `DeadlineExceeded`。各服务在 `internal/server` 里显式 `grpc.Timeout(...)`；BFF 的 HTTP 超时要覆盖「串行调几个下游」的总时长 |
| 5 | `http.Middleware` 只能调一次 | kratos HTTP server 的 `http.Middleware` 是赋值语义，多次调用后一次覆盖前一次。中间件必须一次性传入 |
| 6 | kratos 默认 JSON 不是 protojson | kratos v3 的 `application/json` codec 是 `encoding/json`（Timestamp 输出 `{seconds,nanos}`），与生成的 TS 类型不符。BFF 挂 `pkg/codecx.ProtoJSON()`：proto 消息走 protojson（字段 snake_case、Timestamp → RFC3339、64 位整数 → 字符串），错误仍用 kratos 默认错误 JSON |
| 7 | 业务错误不能用 HTTP 200 | gRPC 传输时 200 映射成 OK，会破坏错误还原。业务错误一律 4xx/5xx；错误码按服务分千位段，线上只传 reason 名 + HTTP code |
| 8 | 代理不写透传代码 | BFF 未实现的方法由 `pkg/proxyx` 按配置反射调用下游同名方法；只有需要特殊逻辑（聚合、强制字段、改造响应）的方法才手写 |
| 9 | kratos config 字段映射 | 配置文件按 protojson 反序列化，字段用 snake_case；不认识的枚举名会被静默清零，所以配置尽量不用枚举，必须用时加 buf validate 规则，并由 `pkg/configx` 的测试加载仓库里的全部配置 |

## 数据库（gorm / gen / 迁移）

| # | 决策 | 原因 / 做法 |
|---|---|---|
| 10 | 没有 data 层 | biz 内嵌 gen 生成的 dao，不再抽 Repo 接口；查询条件只用 gen 字段表达式，`scripts/check-gorm-gen.sh` 拦截手写条件 |
| 11 | gen 写不出来的 SQL | 写成 DIY method（`model/query.go`，注释即 SQL，参数 `@name` 化），`make gen` 生成实现；业务只调生成方法 |
| 12 | `make gen` 不连库 | gen 只解析模型，用 `dbx.NewGormOffline`（不 ping），CI 和新机器上不用起数据库 |
| 13 | 迁移结构体独立 | 迁移里用 init 内的快照结构体，**禁止引用 model 包**——model 以后会改，已上线的迁移必须不变 |
| 14 | 迁移只做加法 | 已执行的迁移 ID 不会重跑，改了也不生效还会让环境不一致；`ValidateUnknownMigrations` 打开，库里有代码不认识的迁移 ID 直接报错 |
| 15 | 模型与迁移一致性测试 | `migrate_test.go` 空库跑迁移后，用 `dbxtest.ModelsMatchSchema` 再对模型跑一次 AutoMigrate，gorm 想改任何东西都算失败 |
| 16 | 更新指定列，不整行 `Save` | `Save` 会把零值写回，把 `NULL` 写成 `''`；更新用 `UpdateSimple(q.Col.Value(v))`。`UpdateSimple` 自动写 `updated_at`，再手动写会报 multiple assignments |
| 17 | 软删表的唯一约束 | 用部分唯一索引 `... WHERE deleted_at IS NULL`（迁移里 `db.Exec`），否则删除后同一个值无法再用 |
| 18 | gorm 依赖版本 | gorm v1.31 移除了 `gorm.Stmt`，配套用 postgres driver v1.6.x |

## 契约（proto / buf / TS）

| # | 决策 | 原因 / 做法 |
|---|---|---|
| 19 | proto 依赖 vendored | googleapis / protovalidate / kratos errors 放在 `api/third_party`，`buf.yaml` 没有远程 deps，生成不依赖 BSR |
| 20 | package = 目录 = Go 路径 = TS 路径 | 开 `PACKAGE_DIRECTORY_MATCH`；包名**不带版本**——版本化只解决「两代不兼容契约长期并存」，代价是方法路径变化；兼容靠 `buf breaking` + 只增不改 |
| 21 | 每个 HTTP 模块独立 package | protoc-gen-typescript-http 按 package 输出单文件，同 package 的第二个 proto 会被丢弃 |
| 22 | 64 位整数在 TS 里是 string | protojson 把 int64/uint64 输出为字符串，插件却生成 `number`；`fix-ts-int64` 读 descriptor 改成 `string`，前端使用处 `Number()` |
| 23 | TS 字段名改回 snake_case | 插件写死驼峰 JSONName；`fix-ts-field-names` 改回 proto 原名，与 HTTP 层一致。TS 管不到的地方要手查：antd `dataIndex`、可选属性的内联类型 |
| 24 | 时间字段用 Timestamp | 时间点用 `google.protobuf.Timestamp`，不用 string / int64；改字段类型是 wire-breaking |
| 25 | buf v2 paths 语义 | `buf.gen.*.yaml` 的 `paths` 相对工作目录，不是相对 input 目录 |
| 26 | `ignore_empty` 已废弃 | 可选字段用 `(buf.validate.field).ignore = IGNORE_IF_ZERO_VALUE` |
| 27 | 生成代码不能文本替换 | `.pb.go` 里嵌着带长度前缀的二进制 descriptor，改 module 路径后必须 `make api` 重新生成，直接替换文本会在 init 时 panic（`new-project.sh` 因此会自动重新生成） |

## 前端

| # | 决策 | 原因 / 做法 |
|---|---|---|
| 28 | workspace 包要被 Umi 编译 | 应用显式依赖 `"@beidou-layout/api-gen": "workspace:*"`，并在 `.umirc.ts` 的 `extraBabelIncludes` 里列出 workspace 包 |
| 29 | Umi request 插件 | `.umirc.ts` 显式写 `request: {}` 才导出 `request` |
| 30 | 关掉 MFSU | monorepo + 生成代码场景下 MFSU 缓存不可靠，`mfsu: false` |
| 31 | antd 静态方法拿不到上下文 | `message.xxx` / `Modal.confirm` 用 `App.useApp()` 的实例；`destroyOnClose` 改 `destroyOnHidden` |
| 32 | pnpm 11 配置位置 | 允许执行安装脚本的依赖写在 `pnpm-workspace.yaml` 的 `allowBuilds` |

## 网关与部署

| # | 决策 | 原因 / 做法 |
|---|---|---|
| 33 | APISIX standalone 末尾必须有 `#END` | `conf/apisix.yaml` 缺 `#END` 时报 `missing valid end flag`，路由全 404 |
| 34 | 网关是自包含镜像 | 插件、配置、路由都打进镜像，运行时只注入环境变量（`${{VAR}}`，缺失即启动失败）；upstream 写短服务名，靠 k8s DNS search 域解析 |
| 35 | 网关 worker 数固定 | `worker_processes` 默认按节点核数，多核节点上容易超出 Pod 内存上限；固定 2 个，扩容靠加副本 |
| 36 | 真实客户端 IP | 外部入口写 `X-Real-IP`，网关只信任内网来源的这个头（`real_ip_from`），日志和上游拿到的才是真实 IP |
| 37 | 镜像 tag 不可变 | `时间戳-commit`，工作区不干净时拒绝构建或带 `-dirty`；chart values 回写 tag，回滚就是回滚 values |
| 38 | 一个 chart 装全部 | 服务之间按固定短名互访，一个命名空间一个 release；服务配置逐项写明、只用 `{{ .变量 }}`，配置变化通过 checksum 只滚动该服务 |

## 对象存储

| # | 决策 | 原因 / 做法 |
|---|---|---|
| 39 | 供应商可插拔、配置存库 | `pkg/storagex` 注册中心 + 各实现 `init` 注册；配置 JSON 的结构是 proto（`MinioConfig` / `S3Config`），protojson + protovalidate 校验；客户端按 `code@updated_at` 缓存，改配置后所有副本自动换新 |
| 40 | 签名 URL + 服务端代理 | 客户端只访问本域名的签名 URL，storage-service 验签后用供应商预签名地址流式代理读写：没有 mixed-content / 跨域，供应商地址不外泄；代价是文件流量经网关（`client_max_body_size` 限大小，`request_buffering: false` 不落盘） |
| 41 | 文件状态不靠定时任务 | 字节经本服务写入，PUT 成功在同一个请求里 PENDING → UPLOADED；改状态失败时由读取路径 `Stat` 一次补上；签名过期的 PENDING 读写都不可能，只是一行惰性记录 |
| 42 | S3 预签名 PUT 关掉默认校验和 | aws-sdk-go-v2 新版本默认给预签名 PUT 带按空 body 算的校验和，代理转发真实内容会被拒；`RequestChecksumCalculation: WhenRequired` |
