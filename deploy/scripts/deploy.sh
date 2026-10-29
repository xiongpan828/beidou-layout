#!/usr/bin/env bash
# 部署（deploy/charts/beidou-layout：网关 + BFF + gRPC 服务 + 运营后台）。
#
#   deploy/scripts/deploy.sh                       # 生产：命名空间 beidou-layout
#   DEPLOY_ENV=dev deploy/scripts/deploy.sh        # 开发：命名空间 dev-beidou-layout（values.yaml + values-dev.yaml）
#   KUBE_CONTEXT=xxx 指定集群（默认 kubectl 当前 context）
#   deploy/scripts/deploy.sh --dry-run=server      # 其余参数原样传给 helm（预演 / --debug 等）
#   VALUES=... SECRET_VALUES=... deploy/scripts/deploy.sh   # 显式指定 values（可再叠 VALUES_OVERLAY）
#   NAMESPACE=xxx 可临时覆盖 helm -n 与资源 namespace
#
# 部署成功后跑 helm test（经网关验证路由与全链路），加 --skip-test 跳过。
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
CHART_DIR="$ROOT/deploy/charts/beidou-layout"
RELEASE="${RELEASE:-beidou-layout}"
KUBE_CONTEXT="${KUBE_CONTEXT:-$(kubectl config current-context)}"
PROD_NAMESPACE="beidou-layout"
DEPLOY_ENV="${DEPLOY_ENV:-prod}"

case "$DEPLOY_ENV" in
  prod)
    default_values=("$CHART_DIR/values.yaml")
    default_secret="$CHART_DIR/values.secret.local.yaml"
    ;;
  dev)
    default_values=("$CHART_DIR/values.yaml" "$CHART_DIR/values-dev.yaml")
    default_secret="$CHART_DIR/values.secret.dev.local.yaml"
    ;;
  *)
    echo "DEPLOY_ENV 只能是 prod 或 dev（当前：$DEPLOY_ENV）" >&2
    exit 1
    ;;
esac

# VALUES 显式指定时只用该文件（可再叠 VALUES_OVERLAY）；否则按 DEPLOY_ENV
value_files=()
if [ -n "${VALUES:-}" ]; then
  value_files+=("$VALUES")
  [ -n "${VALUES_OVERLAY:-}" ] && value_files+=("$VALUES_OVERLAY")
else
  value_files=("${default_values[@]}")
fi
SECRET_VALUES="${SECRET_VALUES:-$default_secret}"

helm_f_args=()
for f in "${value_files[@]}"; do
  [ -f "$f" ] || { echo "values 文件不存在: $f" >&2; exit 1; }
  helm_f_args+=(-f "$f")
done
helm_f_args+=(-f "$SECRET_VALUES")

# 命名空间：默认取 value_files 里最后一份带 namespace 的；NAMESPACE 可覆盖
if [ -z "${NAMESPACE:-}" ]; then
  NAMESPACE=""
  for f in "${value_files[@]}"; do
    n="$(awk '/^namespace:/ {print $2; exit}' "$f" | tr -d '"')"
    [ -n "$n" ] && NAMESPACE="$n"
  done
fi
[ -n "$NAMESPACE" ] || { echo "values 缺少顶层 namespace（或设 NAMESPACE=）" >&2; exit 1; }

# 生产只能发到生产命名空间；开发绝不能发到生产命名空间（防止 VALUES / NAMESPACE 覆盖后把开发配置和密钥冲到生产）
if [ "$DEPLOY_ENV" = prod ] && [ "$NAMESPACE" != "$PROD_NAMESPACE" ]; then
  echo "DEPLOY_ENV=prod 只能部署到命名空间 $PROD_NAMESPACE（当前：$NAMESPACE）" >&2; exit 1
fi
if [ "$DEPLOY_ENV" = dev ] && [ "$NAMESPACE" = "$PROD_NAMESPACE" ]; then
  echo "DEPLOY_ENV=dev 不能部署到生产命名空间 $PROD_NAMESPACE" >&2; exit 1
fi

run_test=true
args=()
for a in "$@"; do
  case "$a" in
    --skip-test) run_test=false ;;
    --dry-run*) run_test=false; args+=("$a") ;;
    *) args+=("$a") ;;
  esac
done

# 私有 values（不进仓库）：密码 / 密钥等只属于某个环境的值，文件必须存在（没有私密值也建一个空文件）
if [ ! -f "$SECRET_VALUES" ]; then
  echo "缺少私有 values $SECRET_VALUES，见 deploy/charts/beidou-layout/README.md「首次部署」" >&2
  exit 1
fi

# 渲染结果里不能还有 CHANGE_ME（按合并后的 values 检查：密钥可以只写在私有 values 里）
if left="$(helm template "$RELEASE" "$CHART_DIR" "${helm_f_args[@]}" | grep -n "CHANGE_ME")"; then
  echo "渲染结果里还有 CHANGE_ME 没填（values.yaml / values-dev.yaml / 私有 values）：" >&2
  echo "$left" >&2
  exit 1
fi

helm lint "$CHART_DIR" "${helm_f_args[@]}" --quiet

echo "==> helm upgrade $RELEASE → $KUBE_CONTEXT/$NAMESPACE (DEPLOY_ENV=$DEPLOY_ENV)"
# --rollback-on-failure：任一 Pod 未就绪（含 migrate 失败）则回滚到上一个成功版本
helm --kube-context "$KUBE_CONTEXT" upgrade --install "$RELEASE" "$CHART_DIR" \
  -n "$NAMESPACE" --create-namespace \
  "${helm_f_args[@]}" --set-string "namespace=$NAMESPACE" \
  --rollback-on-failure --timeout 10m --history-max 10 \
  ${args[@]+"${args[@]}"}

if $run_test; then
  echo "==> helm test"
  helm --kube-context "$KUBE_CONTEXT" -n "$NAMESPACE" test "$RELEASE" --logs
fi
