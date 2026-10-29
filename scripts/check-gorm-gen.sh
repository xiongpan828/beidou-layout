#!/usr/bin/env bash
# 底层服务 biz 禁止手写 gorm 条件：必须走 gen 字段表达式（见 api-convention「数据库访问只用 gorm gen」）。
set -euo pipefail
root=$(cd "$(dirname "$0")/.." && pwd)
# 字符串或反引号开头的条件都算手写；变量参数无法静态区分（gen 表达式也是变量），靠评审
pattern='\.(Where|Or|Not|Order|Group|Select|Having|Joins|Pluck|Update|UpdateColumn)\(["`]|\.(Raw|Exec|Model|Table)\(|gorm\.Expr|\.gdb\(|func \(b \*biz\) (gdb|DB)\('
fail=0
# 有 internal/dao 的都是底层服务（BFF 不连库）
for dao in "$root"/app/service/*/internal/dao; do
  [[ -d "$dao" ]] || continue
  dir=$(dirname "$(dirname "$dao")")
  svc=$(basename "$dir")
  # 用系统自带 grep（不依赖 rg）：0 = 有命中，1 = 没有命中，其它 = 出错（不能当作通过）
  rc=0
  hits=$(grep -rEn --include='*.go' --exclude='*_test.go' --exclude-dir=migrations --exclude-dir=dao --exclude-dir=cmd "$pattern" "$dir") || rc=$?
  if [[ $rc -gt 1 ]]; then
    echo "gorm gen: grep failed on $svc (exit $rc)" >&2
    exit 2
  fi
  if [[ -n "$hits" ]]; then
    echo "gorm gen: hand-written DB conditions in $svc:"
    echo "$hits"
    fail=1
  fi
done
if [[ $fail -ne 0 ]]; then
  echo
  echo "Use gen field expressions (b.WithContext(ctx).X.Where(b.Q.X.Col.Eq(...)))."
  echo "Complex SQL → model/query.go DIY method + make gen. See api-convention."
  exit 1
fi
echo "gorm gen: ok"
