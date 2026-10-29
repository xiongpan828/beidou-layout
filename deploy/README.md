# 部署

只有一种部署方式：**Helm**，一个 chart（`charts/beidou-layout`）装下网关、BFF、底层服务和运营后台。

```text
deploy/
├── docker/
│   ├── Dockerfile-service   通用 Go 服务镜像（--build-arg SERVICE / PACKAGE 选服务）
│   ├── Dockerfile-web       运营后台镜像（pnpm 构建 app/web/admin → nginx）
│   └── nginx.conf           运营后台 nginx：托管 /admin/，其余路径 302 到 /admin/
├── charts/beidou-layout/    Helm chart，详见其 README
└── scripts/
    ├── build-images.sh      构建全部镜像（含 app/gateway）推送到仓库，回写 values 的 image.tag
    └── deploy.sh            helm lint → upgrade --install（失败自动回滚）→ helm test
```

```bash
# 开发环境：tag（时间戳-commit）回写 values-dev.yaml，部署时叠加 values.yaml + values-dev.yaml
DEPLOY_ENV=dev deploy/scripts/build-images.sh && DEPLOY_ENV=dev deploy/scripts/deploy.sh
# 生产环境：tag 回写 values.yaml
deploy/scripts/build-images.sh && deploy/scripts/deploy.sh
```

外部依赖由使用方提供：Helm 4、kubectl、Docker buildx、Kubernetes ≥ 1.30、PostgreSQL（每个底层服务一个库）、镜像仓库、对外入口（LoadBalancer 或 Ingress + TLS）。
首次部署步骤、values 写法、私有 values、发布与回滚见 [`charts/beidou-layout/README.md`](charts/beidou-layout/README.md)。
