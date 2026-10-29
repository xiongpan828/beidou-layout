#!/usr/bin/env bash
# 基于本 layout 新建项目：复制目录 → 全局替换项目名与 Go module → 重新生成 proto 代码 → git init。
#
#   scripts/new-project.sh <目标目录> <项目名> <Go module>
#
#   scripts/new-project.sh ../order order github.com/acme/order
#
# 依赖：go、buf（替换 module 后要 make api 重新生成 proto 代码）。
#
# 替换规则（layout 里只用这几种占位，别的地方不会误伤）：
#   github.com/xiongpan828/beidou-layout → <Go module>
#   beidou-layout                       → <项目名>   npm scope @<项目名>/、chart 名与目录、镜像名、命名空间
#   Beidou Layout                       → <项目名>   页面标题
#
# 不复制：.git、LICENSE（新项目自己选许可证）、本脚本、依赖与构建产物、本机文件（*.local.yaml）。
# 新项目 README 里关于「新建项目」的说明会一起删掉。
set -euo pipefail

usage() {
  sed -n '2,15p' "$0" | sed 's/^# \{0,1\}//' >&2
  exit 1
}

[ $# -eq 3 ] || usage
DEST="$1"
NAME="$2"
MODULE="$3"
OLD_MODULE="github.com/xiongpan828/beidou-layout"
SRC="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

if ! [[ "$NAME" =~ ^[a-z][a-z0-9-]*[a-z0-9]$ ]]; then
  echo "项目名只能是小写字母、数字、中划线，且以字母开头（如 order、shop-admin）：$NAME" >&2
  exit 1
fi
for cmd in go buf; do
  command -v "$cmd" >/dev/null 2>&1 || { echo "缺少 $cmd：替换 module 后要用它重新生成 proto 代码" >&2; exit 1; }
done
if [ -e "$DEST" ] && [ -n "$(ls -A "$DEST" 2>/dev/null)" ]; then
  echo "目标目录已存在且不为空：$DEST" >&2
  exit 1
fi
# 先判断目标是否在 layout 里面，再建目录（失败时不留空目录）
case "$(cd "$(dirname "$DEST")" 2>/dev/null && pwd)/$(basename "$DEST")/" in
  "$SRC"/*) echo "目标目录不能在 layout 目录里面：$DEST" >&2; exit 1 ;;
esac
mkdir -p "$DEST"
DEST="$(cd "$DEST" && pwd)"

echo "==> 复制 $SRC → $DEST"
(cd "$SRC" && tar -cf - \
  --exclude='./.git' \
  --exclude='./LICENSE' \
  --exclude='./scripts/new-project.sh' \
  --exclude='node_modules' \
  --exclude='./tools' \
  --exclude='./build' \
  --exclude='dist' \
  --exclude='.umi' \
  --exclude='.umi-production' \
  --exclude='.test-dist' \
  --exclude='test-results' \
  --exclude='playwright-report' \
  --exclude='.idea' \
  --exclude='.vscode' \
  --exclude='.DS_Store' \
  --exclude='*.local.yaml' \
  .) | (cd "$DEST" && tar -xf -)

echo "==> 替换项目名：beidou-layout → $NAME，module → $MODULE"
cd "$DEST"
# 只处理文本文件（grep -I 跳过二进制）；module 先替换，避免被项目名规则拆开
grep -rlI -e 'beidou-layout' -e 'Beidou Layout' . | while IFS= read -r f; do
  perl -pi -e "s#\Q$OLD_MODULE\E#$MODULE#g; s#beidou-layout#$NAME#g; s#Beidou Layout#$NAME#g;" "$f"
done
mv deploy/charts/beidou-layout "deploy/charts/$NAME"
# 新项目不再是模板：删掉 README 里「用 new-project.sh 生成项目」的说明
perl -0pi -e 's/用 `scripts\/new-project\.sh` 批量生成新项目。//; s/new-project\.sh、//; s/## 新建项目\n.*?(?=\n## )\n//s' README.md
if left="$(grep -rlI 'beidou-layout' .)"; then
  echo "还有文件残留 beidou-layout，请检查：" >&2
  echo "$left" >&2
  exit 1
fi

# .pb.go 里的 descriptor 是带长度前缀的二进制串，文本替换 module 后长度对不上（init 时 panic），必须重新生成
echo "==> 重新生成 proto 代码（make api）"
make api >/dev/null

git init -q
echo "✔ 已创建 $DEST"
cat <<EOF

下一步（详见 README「快速开始」）：
  cd $DEST
  git add -A && git commit -m init        # make proto-check 的 buf breaking 要和 main 分支比较
  go build ./... && pnpm install          # Go 编译 + 运营后台依赖
  make migrate                            # 先在本地 PostgreSQL 建好各服务的库
  make run-hello / run-storage / run-api / run-admin，再 pnpm dev:admin
  部署前把 deploy/charts/$NAME/values*.yaml 里的 CHANGE_ME 换成真实值（密钥放私有 values）
EOF
