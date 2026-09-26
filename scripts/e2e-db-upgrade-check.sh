#!/usr/bin/env bash
set -euo pipefail

# 多库端到端迁移验证：用 Docker 起临时 MySQL / PostgreSQL，运行
# internal/store/db/migrate 的 e2edb 标签测试（旧库升级 + 全新安装），
# 结束后自动清理容器。
#
# 用法：
#   ./scripts/e2e-db-upgrade-check.sh            # MySQL 8.0 + PostgreSQL
#   ./scripts/e2e-db-upgrade-check.sh --mysql57  # 追加 MySQL 5.7（最低支持版本）
#   ./scripts/e2e-db-upgrade-check.sh --no-cleanup  # 保留容器便于排查
#
# 无 Docker 时可手工起库，只要导出对应 DSN 后直接运行：
#   go test -tags e2edb ./internal/store/db/migrate/ -run TestE2E -v -count=1
#
# 注意：测试会清空 DSN 指向 schema 中的全部数据表，切勿指向生产库。

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

WITH_MYSQL57=0
WITH_MYSQL8=1
WITH_PG=1
CLEANUP=1
for arg in "$@"; do
  case "$arg" in
    --mysql57) WITH_MYSQL57=1 ;;
    --mysql8) WITH_MYSQL8=1 ;;
    --no-mysql8) WITH_MYSQL8=0 ;;
    --pg) WITH_PG=1 ;;
    --no-pg) WITH_PG=0 ;;
    --no-cleanup) CLEANUP=0 ;;
    *) echo "unknown option: $arg" >&2; exit 2 ;;
  esac
done

if ! docker version >/dev/null 2>&1; then
  echo "Docker 不可用；请手工准备数据库并导出 E2E_MYSQL_DSN / E2E_PG_DSN 后运行：" >&2
  echo "  go test -tags e2edb ./internal/store/db/migrate/ -run TestE2E -v -count=1" >&2
  exit 1
fi

NETWORK="e2e-upgrade-check"
MYSQL8_CONTAINER="e2e-upgrade-mysql8"
MYSQL57_CONTAINER="e2e-upgrade-mysql57"
PG_CONTAINER="e2e-upgrade-pg"
declare -a STARTED=()

cleanup() {
  if [ "$CLEANUP" -eq 1 ] && [ "${#STARTED[@]}" -gt 0 ]; then
    docker rm -f "${STARTED[@]}" >/dev/null 2>&1 || true
    docker network rm "$NETWORK" >/dev/null 2>&1 || true
  fi
}
trap cleanup EXIT

docker network create "$NETWORK" >/dev/null 2>&1 || true

start_and_wait_mysql() {
  local container="$1" image="$2" port="$3"
  docker run -d --name "$container" --network "$NETWORK" \
    -e MYSQL_ALLOW_EMPTY_PASSWORD=yes -e MYSQL_DATABASE=e2e_upgrade \
    -p "$port:3306" "$image" >/dev/null
  STARTED+=("$container")
  echo "等待 $container ($image) 就绪..."
  for _ in $(seq 1 60); do
    if docker exec "$container" mysqladmin ping -h 127.0.0.1 --silent >/dev/null 2>&1; then
      return 0
    fi
    sleep 2
  done
  echo "$container 启动超时" >&2
  exit 1
}

start_and_wait_pg() {
  local container="$1" image="$2" port="$3"
  docker run -d --name "$container" --network "$NETWORK" \
    -e POSTGRES_PASSWORD=postgres -e POSTGRES_DB=e2e_upgrade \
    -p "$port:5432" "$image" >/dev/null
  STARTED+=("$container")
  echo "等待 $container ($image) 就绪..."
  for _ in $(seq 1 60); do
    if docker exec "$container" pg_isready -U postgres --silent >/dev/null 2>&1; then
      return 0
    fi
    sleep 2
  done
  echo "$container 启动超时" >&2
  exit 1
}

if [ "$WITH_MYSQL8" -eq 1 ]; then
  start_and_wait_mysql "$MYSQL8_CONTAINER" "mysql:8.0" 33061
  E2E_MYSQL_DSN="root@tcp(127.0.0.1:33061)/e2e_upgrade?charset=utf8mb4&parseTime=True"
fi
if [ "$WITH_MYSQL57" -eq 1 ]; then
  start_and_wait_mysql "$MYSQL57_CONTAINER" "mysql:5.7" 33062
  E2E_MYSQL_DSN="root@tcp(127.0.0.1:33062)/e2e_upgrade?charset=utf8mb4&parseTime=True"
fi
if [ "$WITH_PG" -eq 1 ]; then
  start_and_wait_pg "$PG_CONTAINER" "postgres:16" 54321
  E2E_PG_DSN="postgres://postgres:postgres@127.0.0.1:54321/e2e_upgrade?sslmode=disable"
fi

export E2E_MYSQL_DSN="${E2E_MYSQL_DSN:-}" E2E_PG_DSN="${E2E_PG_DSN:-}"
echo "E2E_MYSQL_DSN=$E2E_MYSQL_DSN"
echo "E2E_PG_DSN=$E2E_PG_DSN"

go test -tags e2edb ./internal/store/db/migrate/ -run TestE2E -v -count=1
