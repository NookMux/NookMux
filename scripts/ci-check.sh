#!/usr/bin/env bash
set -euo pipefail

# Local CI check script, aligned with .github/workflows/ci.yml
# Usage:
#   ./scripts/ci-check.sh             # full check (pre-push hook, or manual)
#   ./scripts/ci-check.sh --staged    # staged-only check (pre-commit hook)
#   ./scripts/ci-check.sh --backend   # backend only
#   ./scripts/ci-check.sh --frontend  # frontend only
#
# Full mode vs ci.yml:
#   Go:  fmt / tidy / vet / golangci-lint / test -race / build (go-checks job)
#   Web: typecheck / lint / format / audit(high) / test (web-checks job)
#   Diff: no rsbuild production build locally (web-build job may OOM on
#         low-memory hosts, left to CI)

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
echo "Local CI checks (mode: $MODE)"
echo "=================================================="

# ----------------------------------------------------
# Staged mode (pre-commit hook): staged files only, seconds
# ----------------------------------------------------
if [ "$MODE" = "staged" ]; then
  STAGED_FILES=$(git diff --cached --name-only --diff-filter=ACMR)
  if [ -z "$STAGED_FILES" ]; then
    echo "No staged files, skip"
    exit 0
  fi

  # ---- Go: staged gofmt (batch) + repo-wide go vet ----
  go_files=()
  while IFS= read -r file; do
    if [ -f "$file" ]; then
      go_files+=("$file")
    fi
  done < <(printf '%s\n' "$STAGED_FILES" | grep '\.go$' || true)

  if [ "${#go_files[@]}" -gt 0 ]; then
    echo "[Go] gofmt (${#go_files[@]} files)..."
    UNFORMATTED=$(gofmt -l "${go_files[@]}")
    if [ -n "$UNFORMATTED" ]; then
      echo "FAIL gofmt, run 'gofmt -w <file>':"
      echo "$UNFORMATTED"
      exit 1
    fi
    echo "OK gofmt"

    # Repo-wide vet: catches cross-package compile errors at commit time
    # (minutes cold, seconds warm)
    echo "[Go] go vet ./..."
    go vet ./...
    echo "OK go vet"
  fi

  if printf '%s\n' "$STAGED_FILES" | grep -qE '^go\.(mod|sum)$'; then
    echo "[Go] go mod tidy -diff..."
    go mod tidy -diff
    echo "OK go.mod/go.sum"
  fi

  # ---- Web: staged files only, no repo-wide scan ----
  # Full check as fallback when prettier config or deps change.
  if printf '%s\n' "$STAGED_FILES" | grep -qE '^web/(\.prettierrc[^/]*|prettier\.config\.[^/]*|\.prettierignore|package\.json|bun\.lockb?)$'; then
    echo "[Web] prettier config/deps changed, full format:check..."
    (cd web && bun run format:check)
    echo "OK format (full)"
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
        echo "FAIL: prettier not found in web/node_modules, run 'bun install' in web/"
        exit 1
      fi
      # prettier skips files excluded by .prettierignore (explicit paths too)
      echo "[Web] prettier (${#prettier_files[@]} files)..."
      (cd web && bunx prettier --check -- "${prettier_files[@]}")
      echo "OK prettier"
    fi
  fi

  # typecheck uses tsc incremental cache (sub-second warm), only on
  # TS/build config changes
  if printf '%s\n' "$STAGED_FILES" | grep -qE '^web/.*\.(ts|tsx|js|jsx|mjs|cjs)$|^web/(tsconfig[^/]*\.json|package\.json|bun\.lockb?)$'; then
    echo "[Web] typecheck..."
    (cd web && bun run typecheck)
    echo "OK typecheck"
  fi

  echo "=================================================="
  echo "Staged checks passed"
  echo "=================================================="
  exit 0
fi

# ----------------------------------------------------
# Full / per-side modes
# ----------------------------------------------------
CHECK_BACKEND=true
CHECK_FRONTEND=true
if [ "$MODE" = "backend" ]; then
  CHECK_FRONTEND=false
elif [ "$MODE" = "frontend" ]; then
  CHECK_BACKEND=false
fi

if [ "$CHECK_BACKEND" = true ]; then
  echo "=== Go backend ==="

  echo "[Go 1/6] gofmt..."
  UNFORMATTED=$(gofmt -l $(git ls-files '*.go'))
  if [ -n "$UNFORMATTED" ]; then
    echo "FAIL gofmt:"
    echo "$UNFORMATTED"
    echo "Run: gofmt -w <file>"
    exit 1
  fi
  echo "OK gofmt"

  echo "[Go 2/6] go mod tidy -diff..."
  go mod tidy -diff
  echo "OK go.mod/go.sum"

  echo "[Go 3/6] go vet ./..."
  go vet ./...
  echo "OK go vet"

  echo "[Go 4/6] golangci-lint (.golangci.yml)..."
  if command -v golangci-lint >/dev/null 2>&1; then
    golangci-lint run
    echo "OK golangci-lint"
  else
    echo "WARN: golangci-lint not installed, skip locally (CI enforces it)"
    echo "   Install: https://golangci-lint.run/welcome/install/ (CI uses v2.13.2)"
  fi

  echo "[Go 5/6] go test -race..."
  ./scripts/go-test-backend.sh -race
  echo "OK go test"

  echo "[Go 6/6] go build ./..."
  if ! go build ./...; then
    if [ ! -d web/dist ]; then
      echo "FAIL: web/dist missing, //go:embed in web/embed.go needs it, run 'bun run build' in web/"
    fi
    exit 1
  fi
  echo "OK go build"
fi

if [ "$CHECK_FRONTEND" = true ] && [ -d "web" ]; then
  echo "=== Web frontend ==="
  (
    cd web

    echo "[Web 1/5] typecheck..."
    bun run typecheck

    echo "[Web 2/5] lint..."
    bun run lint

    echo "[Web 3/5] format:check..."
    bun run format:check

    # high+ audit failures block CI (aligned with ci.yml web-checks job)
    echo "[Web 4/5] audit (high+)..."
    bun audit --audit-level=high

    echo "[Web 5/5] test..."
    bun test
  )
  echo "OK web checks"
fi

echo "=================================================="
echo "All checks passed"
echo "=================================================="
