#!/usr/bin/env bash
# 构建并推送镜像，并把 tag 写进 deploy/charts/beidou-layout/values.yaml（image.tag）。
#
#   deploy/scripts/build-images.sh            # tag = <YYYYMMDDHHMM>-<short-sha>
#   PUSH=0 deploy/scripts/build-images.sh     # 只构建校验，不推送、不改 values
#   ALLOW_DIRTY=1 deploy/scripts/build-images.sh  # 工作区有未提交改动时也构建，tag 追加 -dirty
#   DEPLOY_ENV=dev deploy/scripts/build-images.sh # 开发：tag 写进 values-dev.yaml，不动生产的 values.yaml
#
# 镜像格式：<registry>/<service>:202610201301-e6f27ba（registry 默认取 values.yaml 的 image.registry，可用 REGISTRY 覆盖）
# 私有仓库先 docker login；目标平台由 PLATFORM 指定（默认 linux/amd64），构建机架构不同时自动交叉构建。
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
CHART_DIR="$ROOT/deploy/charts/beidou-layout"
REGISTRY="${REGISTRY:-$(awk '/^image:/ {f=1; next} f && /^  registry:/ {print $2; exit}' "$CHART_DIR/values.yaml" | tr -d '"')}"
PLATFORM="${PLATFORM:-linux/amd64}"
cd "$ROOT"

case "$REGISTRY" in
  ""|*CHANGE_ME*) echo "镜像仓库未配置：改 values.yaml 的 image.registry（或设 REGISTRY=）" >&2; exit 1 ;;
esac

PUSH="${PUSH:-1}"
dirty=""
if [ -n "$(git status --porcelain)" ]; then
  if [ "${ALLOW_DIRTY:-0}" != 1 ]; then
    echo "工作区有未提交的改动：镜像 tag 用 commit 号，请先提交（或 ALLOW_DIRTY=1，tag 追加 -dirty）" >&2
    exit 1
  fi
  dirty="-dirty"
fi
# 不可变 tag：时间戳 + commit，便于按时间排序、回滚时一眼定位；未提交改动构建的镜像带 -dirty，不会被当成干净版本
TAG="${TAG:-$(date +%Y%m%d%H%M)-$(git rev-parse --short HEAD)$dirty}"
if [ "$PUSH" = 1 ]; then
  output=(--push)
else
  # 只校验能否构建：结果留在构建缓存里，之后推送可直接复用
  output=(--output type=cacheonly)
fi

# build <镜像名> <构建上下文> [docker buildx 参数...]
build() {
  local image="$1" context="$2"; shift 2
  echo "==> $REGISTRY/$image:$TAG"
  docker buildx build --platform "$PLATFORM" "${output[@]}" -t "$REGISTRY/$image:$TAG" "$@" "$context"
}

# 后端：通用 Dockerfile-service，按服务传 SERVICE / PACKAGE（新增服务在这里加一行）
for spec in \
  hello-service:./app/service/hello-service/cmd \
  storage-service:./app/service/storage-service/cmd \
  api-service:./app/service/api-service/cmd/server \
  admin-service:./app/service/admin-service/cmd/server; do
  build "${spec%%:*}" . -f deploy/docker/Dockerfile-service \
    --build-arg SERVICE="${spec%%:*}" --build-arg PACKAGE="${spec#*:}"
done

# 前端：运营后台
build web-admin . -f deploy/docker/Dockerfile-web

# 网关：APISIX + 自定义插件 + 路由（app/gateway 自包含）
build gateway app/gateway

if [ "$PUSH" != 1 ]; then
  echo "✔ 构建通过（未推送）：tag = $TAG"
  exit 0
fi

# 业务镜像 tag 写回 values（生产 values.yaml / 开发 values-dev.yaml 的 image.tag）
values_file="$CHART_DIR/values.yaml"
[ "${DEPLOY_ENV:-prod}" = dev ] && values_file="$CHART_DIR/values-dev.yaml"
perl -0 -i -pe "s/(\nimage:\n(?:  .*\n)*?  tag: )\"[^\"]*\"/\$1\"$TAG\"/" "$values_file"
grep -q "tag: \"$TAG\"" "$values_file" || { echo "没能把 tag 写进 $values_file（缺少 image.tag？）" >&2; exit 1; }
echo "✔ 镜像已推送，$values_file image.tag = $TAG"
