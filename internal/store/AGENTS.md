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
| 其他单资源目录 | `<资源>store` | redemption/ticket/topup/checkin/usedata/audit/twofa/passkey/voice/missing_models/prefill_group/stored_media/vendor_meta |

`vendormetastore`（`vendor_meta/`）持有 `Model`/`Vendor` 元数据，是跨资源包的公共基础依赖；`dbstore` 位于持久层最底端，不依赖任何资源包。

## 分层与依赖方向

- **单向无环依赖**：`dbstore` 严禁 import 任何业务资源子包；资源子包 → `dbstore` 获取数据库句柄与方言列变量。
- **迁移集中调度**：`InitDB`/`InitLogDB` 与自动迁移集中在 `dbmigrate` 编排，由启动装配层统一调用。
- **批量更新机制**：批量聚合机制（stores/locks/定时 flush）统一收口在 `dbstore`，各资源包在 `init()` 中通过 `dbstore.RegisterBatchFlushers` 注册落盘函数（注册与实体同包，保证 flusher 存在；nil 时显式报错，严禁静默丢弃）。
- **解耦动作日志**：用户管理操作日志（`RecordLog`/`RecordLogWithAdminInfo`）收口在 `userstore`，`logstore` 专注承载请求消费与错误日志查询，消除双向循环依赖。

## 数据库多库兼容性规范

系统必须同时严谨支持 SQLite、MySQL >= 5.7.8、PostgreSQL >= 9.6、openGauss 7.0（PG 兼容内核）、达梦 DM8（Oracle 系）与金仓 KingbaseES（Oracle 兼容模式，PG 兼容内核）：

- 优先使用 GORM 链式 API 执行查询、更新与迁移。
- 原始 SQL 必须严格参数化，禁止通过字符串拼接外部输入。
- 保留字列名（如 `group`、`key`）必须调用 `dbstore.CommonGroupCol` / `CommonKeyCol` / `LogGroupCol` 变量进行方言拼接，严禁手写反引号或双引号。
- 布尔值、JSON 存取、类型转换与 ALTER 行为必须兼容各库方言差异。
- JSON 列持久化按数据形态二选一并显式声明：自由形态 JSON 文本（配置、运行时拼装键值等）统一 `gorm:"type:text"`；结构化 JSON 对象（Go 结构体或自定义类型整体序列化，如 `channels.channel_info`、`prefill_groups.items`）用 `gorm:"type:json"`——`JSON` 类型六库可用（MySQL / PostgreSQL / openGauss 原生支持，DM8 支持 JSON 列，金仓底层按 jsonb 处理，SQLite 宽松类型直接接收）。严禁 `JSONB` 等 PG 专有变体及其他缺乏跨库等价物的类型。
- 可超长大文本列（JSON 配置、多 Key 拼接、模型列表、日志正文等）必须显式声明
  `gorm:"type:text"`，不得省略类型标签依赖各库方言默认值：PG 系默认建 TEXT，
  DM8 却建 VARCHAR(8188 CHAR)（8188 字符上限），超长写入即失败。
- SQLite 原生不支持 `ALTER COLUMN`，表结构演进必须采用追加新列或兼容转换模式。
- openGauss 连接必须走官方 connector（`gitcode.com/opengauss/openGauss-connector-go-pq`，私有 sha256 认证与 pgx/libpq 不兼容），DSN 以 `opengauss://` 前缀识别，目标库需以 `DBCOMPATIBILITY='PG'` 创建；其序列 `pg_class.relkind` 为 `'z'`（PostgreSQL 为 `'S'`），整数除法返回 double precision 不截断（时间分桶等截断语义需 `FLOOR`）。openGauss 连接无法使用 postgres 方言的简单协议优化，点查吞吐对语句缓存更敏感，生产部署建议开启 `SQL_PREPARE_STMT=true`（实测默认关闭时点查约慢 2.7 倍）并同步调大 `SQL_MAX_LIFETIME`（默认 60 秒的连接轮换会周期性清空语句缓存，openGauss 重连还携带私有握手）。
- 达梦 DM8 连接使用 `github.com/godoes/gorm-dameng` 方言（内置达梦官方 Go 驱动；GORM 的 OnConflict 由其 Create 回调翻译为 MERGE INTO，仅当插入列包含全部主键时生效，否则降级普通 INSERT），DSN 以 `dm://` 前缀识别（驱动不做 percent-decode，口令按字面值写入且不得含 '?'，':'、'@' 可用）。目标实例必须以 CASE_SENSITIVE=N 初始化且运行于 COMPATIBLE_MODE=0（DM 原生模式，dm.ini 服务器参数；适配与实机验证均基于原生模式，兼容模式改变内核语义属未验证路径；两项均在连接时校验，不满足即拒绝启动），业务账号需 SELECT ANY DICTIONARY 权限（Migrator 依赖 SYS 系统视图探测表结构）。字符串列按 VARCHAR(n CHAR) 字符长度建列；IDENTITY 自增列显式插值后计数器自动跟随，迁移后无需重置；整数除法向零截断（原生模式实测 7/2=3），与 PostgreSQL 同走整除分桶分支；默认连接池上限 1000 需结合实例 dm.ini 的 MAX_SESSIONS 评估，未显式设置 SQL_MAX_OPEN_CONNS 时启动会对照实例会话上限输出告警，高并发场景建议显式调低，避免高峰耗尽会话。
- 金仓 KingbaseES（Oracle 兼容模式）连接使用 `github.com/godoes/gorm-kingbase` 方言（内置金仓官方 Gokb 驱动，DSN 以 `kingbase://` 前缀识别，userinfo 按标准 URL 规则 percent-decode）。金仓为 PG 兼容内核：标识符折叠为小写、双引号引用、pg_catalog/information_schema 齐备，方言行为归入 PostgreSQL 家族（`UsingPostgreSQL` 与 `UsingKingbase` 同为 true）。Oracle 模式默认把输入空字符串当 NULL（GUC `ora_input_emptystr_isnull`），连接层把该 GUC 统一归一为 off 对齐其余五库空串语义（`EnsureKingbaseOraEmptyStrIsNotNull`；GUC 名与 Gokb 的同名参数解析均大小写不敏感、后值覆盖，DSN 显式携带 =on 类取值或多个大小写变体时连接期直接拒绝或收敛为单值）；实例开启 `enable_ci`（实例级 initdb 选项，字符串比较大小写不敏感）时连接校验直接拒绝启动，避免密钥等值比较跨大小写误匹配；整数除法不截断，与 openGauss 同走 FLOOR 分桶分支（截断能力集中声明于 `internal/infra/db` 的 `IntegerDivisionTruncates`）。金仓默认 `max_connections=100`（低于其余五库常见默认），主库与日志库两套连接池默认各 `SQL_MAX_OPEN_CONNS=1000`，未显式设置时启动会对照实例会话上限输出告警，高并发部署必须按实例会话上限调低，避免高峰 FATAL 53300 耗尽会话；Gokb（lib/pq 系）默认路径每条带参 SQL 需两次往返（匿名 Parse+Describe 与 Bind+Execute）且无语句缓存，点查吞吐对语句缓存敏感，生产部署建议开启 `SQL_PREPARE_STMT=true` 并同步调大 `SQL_MAX_LIFETIME`（默认 60 秒的连接轮换会周期性清空语句缓存）；未开启 SSL 的实例 DSN 须显式 `sslmode=disable`（Gokb 默认 require 且无明文回退）。

## 缓存与数据一致性

- 全局内存缓存（`OptionMap`、渠道缓存、动态倍率缓存）必须严格保障读写锁安全与节点间同步机制。
- 数据迁移与清理脚本必须具备严格幂等性，确保可安全重复运行。

## 验证

- `go test ./internal/store/...`
- 涉及 SQL 或模型迁移变动时，必须验证 SQLite 路径；有条件时运行 MySQL/PostgreSQL/openGauss/DM/KingbaseES 验证。
- 跨层联动测试：
  `go test ./internal/store/... ./internal/domain/... ./internal/httpapi/controller/...`
