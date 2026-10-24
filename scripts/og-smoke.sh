#!/usr/bin/env bash
# openGauss 冒烟测试入口（手动可选步骤，不进 CI 门禁）。
#
# 用法:
#   export OG_SMOKE_DSN='opengauss://nookmux:<口令>@127.0.0.1:5433/nookmux_pg?sslmode=disable'
#   bash scripts/og-smoke.sh
#
# 前置条件:
#   - 目标库需以 PG 兼容模式创建（openGauss 7.0）:
#       CREATE DATABASE nookmux_pg DBCOMPATIBILITY 'PG';
#   - DSN 必须是 opengauss:// URL 形式；口令含特殊字符（@ : / ? # 等）时须 percent-encode，
#     可用: python3 -c 'import urllib.parse,sys;print(urllib.parse.quote(sys.argv[1],safe=""))' '<口令>'；
#   - 本机 openGauss 实例的安装与启动方式属环境特定（参考环境为单机安装于
#     /opt/opengauss、端口 5433），请按实际环境自行准备后再运行。
set -euo pipefail
cd "$(dirname "$0")/.."

if [[ -z "${OG_SMOKE_DSN:-}" ]]; then
  echo "FATAL: 缺少环境变量 OG_SMOKE_DSN（必须指向 openGauss 实例，无替代数据库）" >&2
  echo "示例: export OG_SMOKE_DSN='opengauss://nookmux:<口令>@127.0.0.1:5433/nookmux_pg?sslmode=disable'" >&2
  exit 1
fi

# scripts/og-smoke 是独立 go module（不会被主 module 的 ./... 构建卷入），
# 须进入其目录运行
(cd scripts/og-smoke && go run .)
