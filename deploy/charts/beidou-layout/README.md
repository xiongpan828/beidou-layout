# beidou-layout chart

一个 chart 装下全部业务：网关（`app/gateway`，APISIX）+ 2 个 BFF（api-service / admin-service）+ gRPC 服务（storage-service / 示例 hello-service）+ 运营后台（web-admin）。
镜像构建与部署脚本见 [`deploy/README.md`](../../README.md)。

```text
deploy/charts/beidou-layout/
├── Chart.yaml
├── values.yaml            生产基线：命名空间、镜像、配置变量、工作负载（CHANGE_ME 必须填）
├── values-dev.yaml        开发 overlay（DEPLOY_ENV=dev 时叠在 values.yaml 后）
├── values.schema.json     values 校验：字段拼错 / 类型不对 / 必填缺失在 lint / install 时直接报错
├── files/config/<服务>/   服务配置文件模板（tpl 渲染）→ ConfigMap <服务>-config → /app/configs/
└── templates/
    ├── _helpers.tpl       命名、标签、workload 合并、Pod 模板（含 migrate initContainer）
    ├── configmap.yaml / deployment.yaml / service.yaml / pdb.yaml
    ├── tests/gateway.yaml helm test：经网关调接口、文件路由、运营后台
    └── NOTES.txt
deploy/scripts/
├── build-images.sh        构建 amd64 镜像推仓库（含 gateway），回写 values 的 image.tag
└── deploy.sh              lint → upgrade --install（失败自动回滚）→ helm test
```

## 首次部署

1. **填 values**：`values.yaml` 里的 `CHANGE_ME`（`image.registry`、`config.domain`、`config.db_host`、`config.db_password`，以及对象存储签名密钥 `config.storage_sign_secret`——密码与密钥放私有 values，每个环境各自用 `openssl rand -hex 32` 生成），`values-dev.yaml` 的 `config.domain`；`deploy.sh` 发现 `CHANGE_ME` 会拒绝部署
2. **私有 values**（不进仓库，`*.local.yaml` 已 git 忽略）：生产 `values.secret.local.yaml`、开发 `values.secret.dev.local.yaml`，放只属于某个环境的密码 / 密钥（覆盖 `config.*`）。没有私密值也要建一个空文件
3. **建库**：每个底层服务一个库，生产 `storage_service` / `hello_service`，开发加 `dev_` 前缀（迁移不建库）
4. **镜像仓库凭据**：私有仓库在命名空间里建拉取 Secret，并写进 `imagePullSecrets`
5. 开发：`DEPLOY_ENV=dev deploy/scripts/build-images.sh` → `DEPLOY_ENV=dev deploy/scripts/deploy.sh`；生产：`deploy/scripts/build-images.sh` → `deploy/scripts/deploy.sh`（两边的 image.tag 分别回写 values-dev.yaml / values.yaml）
6. **部署后**：在运营后台「存储供应商」创建 code 为 `default` 的供应商（storage-service 配置的 scope 都指向它），上传才可用

## values 怎么写

```yaml
namespace: beidou-layout     # 部署命名空间：写进全部资源的 metadata.namespace，deploy.sh 也按它 helm -n
image:
  registry: registry.example.com/beidou-layout
  tag: "202610201603-010a6e9"  # build-images.sh 回写；为空时用 Chart.appVersion
config:                      # 全局配置变量（配置文件、env 里用 {{ .变量 }} 引用）
  db_host: ...
services:
  hello-service:
    image: ...               # 可选，默认 = 服务名
    config:                  # 本服务变量，覆盖全局
      db_name: hello_service
    workloads:
      server:                # server → 资源名 hello-service；其他 → hello-service-<名字>
        args: [...]
        ports: { grpc: 9100 }    # 有端口才建 Service；第一个端口（按名字排序）做 TCP 探针
        migrate: true            # 启动前 initContainer 跑 ./hello-service migrate
```

- 服务配置 `files/config/<服务>/config.yaml` **逐项写明**，只用 `{{ .变量 }}` 取值，不用 range / dict / if 生成；改完 `helm template t deploy/charts/beidou-layout` 渲染核对
- 配置变化 → Pod 模板的 `checksum/config` 变化 → 只有该服务滚动重启
- `replicas > 1` 自动生成 PodDisruptionBudget，并尽量分散到不同节点

## 对外入口

网关 Service 默认 `ClusterIP`。对外暴露二选一：

- **LoadBalancer**：`services.gateway.workloads.server.serviceType: LoadBalancer`（固定 IP 等 LB 实现相关的设置写在 `serviceAnnotations`），外部入口 TLS 终止后转发到 `<LB>:9080`
- **Ingress**：自行加一个 Ingress 指向 `gateway:9080`

外部入口需保留 Host，并写 `X-Real-IP`（网关只信任内网来源的这个头，见 `app/gateway/conf/config.yaml` 的 `real_ip_from`）。

## 日常发布 / 回滚

```bash
deploy/scripts/build-images.sh                     # 新镜像 + 回写 tag（工作区必须干净，或 ALLOW_DIRTY=1；开发环境加 DEPLOY_ENV=dev）
deploy/scripts/deploy.sh                           # 失败自动回滚到上一个成功版本
deploy/scripts/deploy.sh --dry-run=server          # 只预演
helm -n beidou-layout history beidou-layout        # 历史版本
helm -n beidou-layout rollback beidou-layout <REV> # 手动回滚
```
