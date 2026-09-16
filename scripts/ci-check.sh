#!/usr/bin/env bash
set -euo pipefail

# CI 本地校验脚本：对齐 .github/workflows/ci.yml 门禁
# 用法:
#   ./scripts/ci-check.sh            # 全量复核（对齐 CI，由 .githooks/pre-push 触发，也可手动执行）
#   ./scripts/ci-check.sh --staged   # 增量复核（秒级，针对 git 暂存区改动，由 .githooks/pre-commit 触发）
#   ./scripts/ci-check.sh --backend # 仅后端检查
#   ./scripts/ci-check.sh --frontend # 仅前端检查
#
# 全量模式与 ci.yml 的对齐关系：
#   Go:  fmt / tidy / vet / golangci-lint / test -race / build（go-checks job）
#   Web: typecheck / lint / format / audit(high) / test（web-checks job）
#   差异：本地不跑 rsbuild 生产构建（web-build job 在低内存环境易 OOM，交给 CI）。

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
# 增量模式 (pre-commit hook)：只看暂存区，目标秒级完成
# ----------------------------------------------------
if [ "$MODE" = "staged" ]; then
  STAGED_FILES=$(git diff --cached --name-only --diff-filter=ACMR)
  if [ -z "$STAGED_FILES" ]; then
    echo "ℹ️ 没有检测到暂存的文件，跳过检查。"
    exit 0
  fi

  # ---- Go：暂存文件 gofmt（批量）+ 全仓 go vet ----
  go_files=()
  while IFS= read -r file; do
    if [ -f "$file" ]; then
      go_files+=("$file")
    fi
  done < <(printf '%s\n' "$STAGED_FILES" | grep '\.go$' || true)

  if [ "${#go_files[@]}" -gt 0 ]; then
    echo "🔍 [Go] 检查暂存 Go 文件格式 (gofmt, ${#go_files[@]} 个文件)..."
    UNFORMATTED=$(gofmt -l "${go_files[@]}")
    if [ -n "$UNFORMATTED" ]; then
      echo "❌ 以下 Go 文件未通过 gofmt 格式化，请执行 'gofmt -w <file>' 修复："
      echo "$UNFORMATTED"
      exit 1
    fi
    echo "✅ gofmt 通过"

    # 全仓 vet：跨包编译错误在提交时就暴露，冷缓存首次约分钟级，热缓存秒级
    echo "🔍 [Go] go vet ./...（全仓静态分析）..."
    go vet ./...
    echo "✅ go vet 通过"
  fi

  if printf '%s\n' "$STAGED_FILES" | grep -qE '^go\.(mod|sum)$'; then
    echo "🔍 [Go] go mod tidy -diff..."
    go mod tidy -diff
    echo "✅ go.mod / go.sum 干净"
  fi

  # ---- 前端：只检查暂存文件，不再全仓扫描 ----
  # prettier 配置或依赖变化会让逐文件检查失效，此时回退全量检查。
  if printf '%s\n' "$STAGED_FILES" | grep -qE '^web/(\.prettierrc[^/]*|prettier\.config\.[^/]*|\.prettierignore|package\.json|bun\.lockb?)$'; then
    echo "🔍 [Web] 检测到 prettier 配置/依赖变更，执行全量 format:check..."
    (cd web && bun run format:check)
    echo "✅ 前端全量格式检查通过"
  else
    prettier_files=()
    while IFS= read -r file; do
      case "$file" in
        web/*)
          rel="${file#web/}"
          case "$rel" in
            *.ts|*.tsx|*.js|*.jsx|*.mjs|*.cjs|*.json|*.css|*.scss|*.html|*.htm|*.md|*.mdx|*.yaml|*.yml)
              if [ -f "$file" ]; then
                prettier_files+=("$rel")
              fi
              ;;
          esac
          ;;
      esac
    done < <(printf '%s\n' "$STAGED_FILES")

    if [ "${#prettier_files[@]}" -gt 0 ]; then
      if [ ! -e web/node_modules/.bin/prettier ]; then
        echo "❌ web/node_modules 中未找到 prettier，请先在 web/ 目录执行 'bun install'。"
        exit 1
      fi
      # .prettierignore 白名单之外的文件会被 prettier 自动跳过（对显式路径同样生效）
      echo "🔍 [Web] 检查暂存前端文件格式 (prettier, ${#prettier_files[@]} 个文件)..."
      (cd web && bunx prettier --check -- "${prettier_files[@]}")
      echo "✅ prettier 通过"
    fi
  fi

  # typecheck 走 tsc 增量缓存（热缓存亚秒级），仅在 TS/构建配置相关文件变更时触发
  if printf '%s\n' "$STAGED_FILES" | grep -qE '^web/.*\.(ts|tsx|js|jsx|mjs|cjs)$|^web/(tsconfig[^/]*\.json|package\.json|bun\.lockb?)$'; then
    echo "🔍 [Web] bun run typecheck..."
    (cd web && bun run typecheck)
    echo "✅ typecheck 通过"
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

  echo "🔍 [Go 1/6] gofmt 格式检查..."
  UNFORMATTED=$(gofmt -l $(git ls-files '*.go'))
  if [ -n "$UNFORMATTED" ]; then
    echo "❌ 以下 Go 文件未通过 gofmt 格式化："
    echo "$UNFORMATTED"
    echo "请运行: gofmt -w <file>"
    exit 1
  fi
  echo "✅ gofmt 检查通过"

  echo "🔍 [Go 2/6] go mod tidy -diff 依赖检查..."
  go mod tidy -diff
  echo "✅ go.mod / go.sum 依赖对齐"

  echo "🔍 [Go 3/6] go vet ./... 静态分析..."
  go vet ./...
  echo "✅ go vet 通过"

  echo "🔍 [Go 4/6] golangci-lint 静态检查（配置见 .golangci.yml）..."
  if command -v golangci-lint >/dev/null 2>&1; then
    golangci-lint run
    echo "✅ golangci-lint 通过"
  else
    echo "⚠️ 未安装 golangci-lint，本地跳过此项（推送后 CI 仍会强制执行）。"
    echo "   安装方式见 https://golangci-lint.run/welcome/install/（CI 使用 v2.13.2）。"
  fi

  echo "🔍 [Go 5/6] go test -race ./... 单元测试..."
  ./scripts/go-test-backend.sh -race
  echo "✅ 后端单元测试通过"

  echo "🔍 [Go 6/6] go build ./... 编译检查..."
  if ! go build ./...; then
    if [ ! -d web/dist ]; then
      echo "❌ 提示：web/dist 不存在，web/embed.go 的 //go:embed 需要真实产物，请先在 web/ 执行 'bun run build'。"
    fi
    exit 1
  fi
  echo "✅ go build 通过"
fi

if [ "$CHECK_FRONTEND" = true ] && [ -d "web" ]; then
  echo "📦 === 前端 Web 检查 ==="
  (
    cd web

    echo "🔍 [Web 1/5] bun run typecheck TypeScript 类型..."
    bun run typecheck

    echo "🔍 [Web 2/5] bun run lint 代码规范..."
    bun run lint

    echo "🔍 [Web 3/5] bun run format:check 代码格式..."
    bun run format:check

    # high 及以上漏洞直接卡 CI（对齐 ci.yml web-checks job）
    echo "🔍 [Web 4/5] bun audit 依赖漏洞 (high+)..."
    bun audit --audit-level=high

    echo "🔍 [Web 5/5] bun test 单元测试..."
    bun test
  )
  echo "✅ 前端检查全部通过"
fi

echo "=================================================="
echo "🎉 所有门禁检查全部通过！"
echo "=================================================="
