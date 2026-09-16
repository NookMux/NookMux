#!/usr/bin/env bash
set -euo pipefail

# CI 本地校验脚本：对齐 .github/workflows/ci.yml 门禁
# 用法:
#   ./scripts/ci-check.sh          # 全量复核（对齐 CI）
#   ./scripts/ci-check.sh --staged # 增量复核（针对 git 暂存区改动，供 pre-commit hook 使用）
#   ./scripts/ci-check.sh --backend # 仅后端检查
#   ./scripts/ci-check.sh --frontend # 仅前端检查

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

MODE="full"
if [ "${1:-}" = "--staged" ]; then
  MODE="staged"
elif [ "${1:-}" = "--backend" ]; then
  MODE="backend"
elif [ "${1:-}" = "--frontend" ]; then
  MODE="frontend"
fi

echo "=================================================="
echo "🚀 运行本地 CI 门禁检查 (模式: $MODE)"
echo "=================================================="

# ----------------------------------------------------
# 增量模式 (pre-commit hook)
# ----------------------------------------------------
if [ "$MODE" = "staged" ]; then
  STAGED_FILES=$(git diff --cached --name-only --diff-filter=ACMR)
  if [ -z "$STAGED_FILES" ]; then
    echo "ℹ️ 没有检测到暂存的文件，跳过检查。"
    exit 0
  fi

  # 1. 检查暂存的 Go 文件格式
  STAGED_GO_FILES=$(echo "$STAGED_FILES" | grep '\.go$' || true)
  if [ -n "$STAGED_GO_FILES" ]; then
    echo "🔍 [1/3] 检查暂存 Go 文件格式 (gofmt)..."
    UNFORMATTED=""
    while IFS= read -r file; do
      if [ -f "$file" ]; then
        diff_out=$(gofmt -l "$file")
        if [ -n "$diff_out" ]; then
          UNFORMATTED="${UNFORMATTED}${file}\n"
        fi
      fi
    done <<< "$STAGED_GO_FILES"

    if [ -n "$UNFORMATTED" ]; then
      echo -e "❌ 以下 Go 文件未通过 gofmt 格式化，请执行 'gofmt -w <file>' 修复：\n$UNFORMATTED"
      exit 1
    fi
    echo "✅ Go 代码格式干净"

    echo "🔍 [2/3] 检查 Go 静态分析 (go vet)..."
    go vet ./...
    echo "✅ go vet 通过"
  fi

  # 2. 检查 go.mod / go.sum
  if echo "$STAGED_FILES" | grep -qE '^go\.(mod|sum)$'; then
    echo "🔍 检查 go mod tidy -diff..."
    go mod tidy -diff
    echo "✅ go.mod / go.sum 干净"
  fi

  # 3. 检查暂存的前端文件
  if echo "$STAGED_FILES" | grep -q '^web/'; then
    echo "🔍 [3/3] 检查前端暂存文件 (format / typecheck)..."
    (
      cd web
      bun run format:check
      bun run typecheck
    )
    echo "✅ 前端检查通过"
  fi

  echo "=================================================="
  echo "🎉 暂存区增量 CI 检查全部通过！"
  echo "=================================================="
  exit 0
fi

# ----------------------------------------------------
# 全量 / 分模块检查模式
# ----------------------------------------------------
CHECK_BACKEND=true
CHECK_FRONTEND=true
if [ "$MODE" = "backend" ]; then
  CHECK_FRONTEND=false
elif [ "$MODE" = "frontend" ]; then
  CHECK_BACKEND=false
fi

if [ "$CHECK_BACKEND" = true ]; then
  echo "📦 === 后端 Go 检查 ==="

  echo "🔍 [Go 1/5] gofmt 格式检查..."
  UNFORMATTED=$(gofmt -l $(git ls-files '*.go'))
  if [ -n "$UNFORMATTED" ]; then
    echo "❌ 以下 Go 文件未通过 gofmt 格式化："
    echo "$UNFORMATTED"
    echo "请运行: gofmt -w <file>"
    exit 1
  fi
  echo "✅ gofmt 检查通过"

  echo "🔍 [Go 2/5] go mod tidy -diff 依赖检查..."
  go mod tidy -diff
  echo "✅ go.mod / go.sum 依赖对齐"

  echo "🔍 [Go 3/5] go vet ./... 静态分析..."
  go vet ./...
  echo "✅ go vet 通过"

  echo "🔍 [Go 4/5] go build ./... 编译检查..."
  go build ./...
  echo "✅ go build 通过"

  echo "🔍 [Go 5/5] go test -short ./... 单元测试..."
  ./scripts/go-test-backend.sh -short
  echo "✅ 后端单元测试通过"
fi

if [ "$CHECK_FRONTEND" = true ] && [ -d "web" ]; then
  echo "📦 === 前端 Web 检查 ==="
  (
    cd web

    echo "🔍 [Web 1/4] bun run format:check 代码格式..."
    bun run format:check

    echo "🔍 [Web 2/4] bun run typecheck TypeScript 类型..."
    bun run typecheck

    echo "🔍 [Web 3/4] bun run lint 代码规范..."
    bun run lint

    echo "🔍 [Web 4/4] bun test 单元测试..."
    bun test
  )
  echo "✅ 前端检查全部通过"
fi

echo "=================================================="
echo "🎉 所有门禁检查全部通过！"
echo "=================================================="
