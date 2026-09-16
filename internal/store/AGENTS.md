# internal/store/AGENTS.md

`internal/store/` 是数据持久层，承载系统 GORM 实体定义、数据持久化访问（CRUD）、底层缓存、数据迁移与存储清理。

## 目录结构与包命名

子包按资源垂直拆分；包名统一使用 `store` 后缀（避免与调用方局部变量 `user`/`token`/`channel`/`log`/`db` 冲突），`db/` 系列以 `db` 前缀命名：

| 目录 | 包名 | 核心职责 |
|---|---|---|
| `db/` | `dbstore` | `DB`/`LOG_DB` 句柄、方言列名变量、连接初始化、批量聚合更新器 |
| `db/migrate/` | `dbmigrate` | `InitDB`/`InitLogDB` 迁移编排、AutoMigrate、同类型迁移、日志头回填 |
| `db/cleanup/` | `dbcleanup` | 历史过期数据清理 |
| `channel/` | `channelstore` | 渠道配置、ability 模型映射、动态倍率配置持久化 |
| `user/` | `userstore` | 用户实体、用户缓存、操作动作日志 |
| `token/` | `tokenstore` | 令牌实体、令牌缓存、时间窗口/周期配额 |
| `log/` | `logstore` | 消费日志、错误日志检索与用量统计聚合 |
| `pricing/` | `pricingstore` | 模型定价缓存与刷新 |
| `option/` | `optionstore` | 系统 Option 键值对、系统 Setup 记录、迁移状态 Marker |
| 其他单资源目录 | `<资源>store` | redemption/ticket/topup/checkin/usedata/audit/twofa/passkey/minimax_voice/missing_models/prefill_group/stored_media/vendor_meta |

`vendormetastore`（`vendor_meta/`）持有 `Model`/`Vendor` 元数据，是跨资源包的公共基础依赖；`dbstore` 位于持久层最底端，不依赖任何资源包。

## 分层与依赖方向

- **单向无环依赖**：`dbstore` 严禁 import 任何业务资源子包；资源子包 → `dbstore` 获取数据库句柄与方言列变量。
- **迁移集中调度**：`InitDB`/`InitLogDB` 与自动迁移集中在 `dbmigrate` 编排，由启动装配层统一调用。
- **批量更新机制**：批量聚合机制（stores/locks/定时 flush）统一收口在 `dbstore`，各资源包在 `init()` 中通过 `dbstore.RegisterBatchFlushers` 注册落盘函数（注册与实体同包，保证 flusher 存在；nil 时显式报错，严禁静默丢弃）。
- **解耦动作日志**：用户管理操作日志（`RecordLog`/`RecordLogWithAdminInfo`）收口在 `userstore`，`logstore` 专注承载请求消费与错误日志查询，消除双向循环依赖。

## 数据库三库兼容性规范

系统必须同时严谨支持 SQLite、MySQL >= 5.7.8、PostgreSQL >= 9.6：

- 优先使用 GORM 链式 API 执行查询、更新与迁移。
- 原始 SQL 必须严格参数化，禁止通过字符串拼接外部输入。
- 保留字列名（如 `group`、`key`）必须调用 `dbstore.CommonGroupCol` / `CommonKeyCol` / `LogGroupCol` 变量进行方言拼接，严禁手写反引号或双引号。
- 布尔值、JSON 存取、类型转换与 ALTER 行为必须兼容三库方言差异。
- JSON 数据统一以 `TEXT` 类型持久化，严禁引入缺乏跨库回退方案的专有类型（如 JSONB）。
- SQLite 原生不支持 `ALTER COLUMN`，表结构演进必须采用追加新列或兼容转换模式。

## 缓存与数据一致性

- 全局内存缓存（`OptionMap`、渠道缓存、动态倍率缓存）必须严格保障读写锁安全与节点间同步机制。
- 数据迁移与清理脚本必须具备严格幂等性，确保可安全重复运行。

## 验证

- `go test ./internal/store/...`
- 涉及 SQL 或模型迁移变动时，必须验证 SQLite 路径；有条件时运行 MySQL/PostgreSQL 验证。
- 跨层联动测试：
  `go test ./internal/store/... ./internal/domain/... ./internal/httpapi/controller/...`
