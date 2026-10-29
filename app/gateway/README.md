# 网关（APISIX）

南北向统一入口：按路径分发到运营后台、BFF 和文件读写服务。
网关是一个**自包含镜像**：官方 APISIX + 本目录的配置、路由（和可选的 Lua 插件），运行时只注入环境变量，不挂载任何文件。

## 目录

```text
app/gateway/
├── Dockerfile               FROM apache/apisix（可用 build-arg APISIX_IMAGE 替换基础镜像）
├── conf/
│   ├── config.yaml          数据面配置：standalone、启用的插件、extra_lua_path、真实 IP
│   └── apisix.yaml          路由与上游（standalone YAML，末尾必须保留 #END）
└── apisix/plugins/          自定义 Lua 插件（目录与 require 路径一致；模板里为空）
```

自定义插件按社区推荐放在 `extra_lua_path`（镜像内 `/usr/local/apisix/custom/apisix/plugins/`），不覆盖 APISIX 内置插件，升级 APISIX 只改 `FROM`。
新增插件后在 `conf/config.yaml` 的 `plugins` 里登记，再在路由上启用。认证、限流等横切能力适合放在这一层。

## 路由表

`conf/apisix.yaml` 的 `routes`（priority 高的先匹配）：

| id | 路径 | 上游 | 插件 | priority |
|---|---|---|---|---|
| `consumer-api` | `/api/v1/consumer/*` | api-service:8100 | — | 90 |
| `admin-api` | `/api/v1/admin/*` | admin-service:8000 | — | 90 |
| `storage-file` | `/api/v1/storage/*`（GET / PUT） | storage-service:9301 | `proxy-control`（不缓冲请求体）；靠 URL 签名，由 storage-service 校验（见 `docs/storage.md`） | 90 |
| `admin-web` | `/*` | web-admin:80（其 nginx 把 `/` 跳到 `/admin/`） | — | 1 |

upstream 写短服务名，由容器 DNS 的 search 域解析（k8s 同命名空间 CoreDNS）。

## 环境变量

路由里需要密钥 / 环境相关的值时写成 `${{VAR}}`（`${{VAR:=默认值}}` 带默认值），APISIX 启动时替换，**缺失直接启动失败**。
k8s 在 chart `services.gateway.workloads.server.env` 注入，值可引用 `config.*`（与服务配置同一份）。模板目前没有用到。

## 改路由

1. 改 `conf/apisix.yaml`（末尾保留 `#END`）
2. 重新构建网关镜像（`deploy/scripts/build-images.sh`），再部署
