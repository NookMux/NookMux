# Changelog

本文件记录项目的所有重要变更。

格式基于 [Keep a Changelog](https://keepachangelog.com/zh-CN/1.1.0/)，
版本号遵循 [语义化版本](https://semver.org/lang/zh-CN/)。

## [Unreleased]

### Added

- **管理接口 API 参考文档全量补全**（Mintlify 中英）：管理接口下全部 137 个
  端点页面由「仅 URL 占位」补全为完整参考——含鉴权要求（以路由中间件为准）、
  cURL 请求示例、路径/查询/请求体参数（`ParamField`）与响应示例及字段
  （`ResponseField`）逐项解析；字段名、类型、必填与默认值均对照后端源码
  （绑定结构体、控制器响应组装）核实。回调类端点（Stripe Webhook、易支付
  notify）按实际返回纯状态码/纯文本如实记录；个别偏离「HTTP 200 + success
  包裹」约定的端点（`/api/usage/token/`、`/api/status/test` 等）亦逐一注明。
  同时新增 4 个此前缺失的密钥类端点页（`POST /api/token/batch/keys`、
  `POST|PUT /api/token/{id}/key`、`GET /api/redemption/{id}/key`），删除一页
  陈旧文档：`GET /api/user/groups` 路由已不存在于后端与前端（真实端点
  `GET /api/user/self/groups` 已有专属页面）。
- **管理接口 API 参考文档第二批扩容**（Mintlify 中英）：把此前未进入文档的
  15 个路由家族共 90 个端点补建为完整参考页（中英 180 份文件）——新增审计
  日志、消费日志（含 `TokenAuthReadOnly` 令牌直查日志）、仪表盘配置、动态
  倍率、定制音色、工单系统、媒体存储、性能运维、数据库迁移九个分组，并向
  渠道管理（Ollama 模型管理、GLM 套餐查询、上游供应商拉取、代理连通性
  测试）、系统设置（`json_map`/`json_array`/配置键值细粒度读写）、数据统计、
  系统、用户管理（签到、封禁、Passkey 更新）、充值六个现有分组追加页面。
  全部字段对照后端源码核验；工单端点的错误表按服务端实际统一映射
  （`MsgDatabaseError`）修正，SSE（`/api/channel/ollama/pull/stream`）、
  固定中文错误文案（`/api/log/self` 过滤拒绝）等特殊行为均如实注明。docs.json
  中英导航同步登记。
- **Stripe Webhook 密钥守卫回归测试**：为「未配置 `StripeWebhookSecret` 时
  webhook 直接 403 拒绝验签」的安全守卫补充 HTTP 层回归测试（对应
  928b28065 修复的空密钥伪造充值漏洞），同时钉住相邻边界——已配置密钥但
  无有效签名的请求返回 400，防止守卫在后续重构中被意外移除。
- **金仓 KingbaseES 数据库支持**：新增人大金仓 KingbaseES（Oracle 兼容模式）作为
  原生后端数据库（PG 兼容内核）。DSN 以 `kingbase://` 前缀识别（金仓官方 Gokb
  驱动原生支持的 URL scheme，口令按标准 URL 规则 percent-encode），连接层使用
  `godoes/gorm-kingbase` v1.11.0（内置金仓官方 Gokb 驱动，Migrator 基于
  pg_catalog / information_schema 探测表结构），SQL 方言行为归入 PostgreSQL 家族
  （双引号标识符、小写折叠）。
  - Oracle 模式的空字符串即 NULL 语义（GUC `ora_input_emptystr_isnull`）在连接层
    自动归一：连接建立时下发 `ora_input_emptystr_isnull=off` 启动参数（未携带或
    仅携带空值时补为 off，判定仅基于 DSN query 段），与
    SQLite/MySQL/PostgreSQL/openGauss/DM 的空串与 NULL 严格区分语义对齐，
    避免空串写入 NOT NULL 列失败与 `WHERE col = ''` 永远失配；DSN 显式携带
    on 类取值时连接期直接拒绝（GUC 名与 Gokb 参数解析均大小写不敏感、同名
    参数后值覆盖，重复携带与大小写变体同样命中拒绝或收敛为单值；DSN URL
    解析失败立即报错且不回显原文，不放行未归一化的连接）。
  - 实例要求：不得开启 `enable_ci`（实例级 initdb 选项，字符串比较大小写不敏感会使
    令牌密钥、用户名等等值比较跨大小写误匹配，连接时校验失败即拒绝启动）；未开启
    SSL 的实例 DSN 须显式 `sslmode=disable`（Gokb 默认 require 且无明文回退）。
  - 高并发部署建议：金仓默认 `max_connections=100`（实测 V9R2C016），主库与日志库
    两套连接池默认各 `SQL_MAX_OPEN_CONNS=1000`，须按实例会话上限调低，避免高峰
    FATAL 53300 耗尽会话（未显式设置时启动会对照实例会话上限输出告警）；Gokb
    默认路径每条带参 SQL 需两次往返且无语句缓存，生产部署建议开启
    `SQL_PREPARE_STMT=true` 并同步调大 `SQL_MAX_LIFETIME`（默认 60 秒的连接
    轮换会周期性清空语句缓存）。
  - 方言分支适配：用量排行时间分桶纳入 FLOOR 截断分支（金仓整数除法不截断）；
    跨库预迁移支持金仓与 MySQL/PostgreSQL/openGauss/DM/SQLite 互迁，同类型迁移
    支持 KingbaseES → KingbaseES（实例身份校验复用 pg_control_system 实例指纹，
    不含连接路径字段；迁移完成后自动重置目标自增序列）。
  - 安装向导数据库类型展示新增 `kingbase`（前端含数据库步骤部署前提提示与中英文文案）。

- **达梦 DM8 数据库支持**：新增达梦 DM8 作为原生后端数据库（Oracle 系内核，不依赖
  MySQL/Oracle 兼容模式）。DSN 以 `dm://` 前缀识别（达梦官方 Go 驱动原生支持的 URL
  scheme，口令按字面值写入且不得含 '?'，':'、'@' 可用，连接前对 DSN 口令做
  非法字符前置校验），连接层使用 `godoes/gorm-dameng`
  v0.7.2（GORM 的 OnConflict 由其 Create 回调翻译为 MERGE INTO，自增主键经 IDENTITY
  列自动回填，显式插值后计数器自动跟随、迁移后无需重置）。
  - 实例要求：以 CASE_SENSITIVE=N 初始化且运行于 COMPATIBLE_MODE=0（DM 原生
    模式，dm.ini 服务器参数；适配与实机验证均基于原生模式完成，两项均连接时校验，
    不满足即启动失败），业务账号授予 SELECT ANY DICTIONARY 权限（AutoMigrate
    依赖 SYS 系统视图探测表结构）；高并发部署建议按实例 dm.ini 的 MAX_SESSIONS
    调低 SQL_MAX_OPEN_CONNS（默认 1000，未显式设置时启动会对照实例会话上限输出
    告警），避免高峰耗尽会话。
  - 字符串列按 VARCHAR(n CHAR) 字符口径建列，与其他数据库的长度语义对齐，避免中文
    在字节口径下意外截断。
  - 方言分支适配：保留字列与渠道搜索/兑换码/充值单的列名引用在 DM 下走双引号；DM
    整数除法向零截断（原生模式实测），用量排行时间分桶沿用整除分支；跨库预迁移支持
    DM 与 MySQL/PostgreSQL/openGauss/SQLite 互迁，同类型迁移支持 DM → DM（含实例
    身份校验，基于 V$INSTANCE 实例指纹）；目标库写入按批包裹事务，钉定达梦
    IDENTITY_INSERT 的会话粘性并获得批级原子回滚。
  - 安装向导数据库类型展示新增 `dameng`（前端含 DM 检测提示与中英文文案）。

- **openGauss 数据库支持**：新增 openGauss 7.0 作为原生后端数据库。DSN 以
  `opengauss://` 前缀识别（openGauss 官方 Go connector 原生支持的 URL scheme），
  连接层使用 `gitcode.com/opengauss/openGauss-connector-go-pq`（openGauss 私有
  sha256 认证协议与 pgx/libpq 不兼容，无法复用 PostgreSQL 驱动），SQL 方言复用
  GORM postgres 方言（PG 兼容内核）。目标库需以 `DBCOMPATIBILITY='PG'` 创建。
  - 迁移链路同步适配：预迁移支持 MySQL/PostgreSQL/SQLite ↔ openGauss 方向，
    同类型迁移支持 openGauss → openGauss（含实例身份校验与自增序列修复，
    openGauss 序列 `pg_class.relkind` 为 `'z'`）。
  - openGauss 整数除法返回 double precision（不截断），用量排行的时间分桶表达式按
    数据库类型自动切换 FLOOR 截断。
  - 安装向导数据库类型展示新增 `opengauss`。

- **密钥前缀自定义**：新增系统配置「密钥前缀」（站点与品牌 → 系统信息，配置键
  `general_setting.token_key_prefix`，默认 `sk-`），支持设置为 `自定义-`、`sk-自定义-`
  等以 `-` 结尾的前缀，仅影响之后新建的令牌密钥。（[#294](https://github.com/NookMux/NookMux/issues/294)）
  - 令牌密钥统一以「完整密钥」（密钥前缀 + 48 位随机串）形态存入数据库，`tokens.key`
    列宽由 `char(48)` 扩展为 `varchar(64)`。
  - 一次性数据迁移自动为历史无前缀密钥补默认前缀 `sk-`（幂等，基于 options 表迁移标记）。
  - 鉴权、搜索、展示与复制全部直接使用完整密钥，前端不再拼接 `sk-` 前缀。
  - 密钥查询页的密钥格式校验放宽为自定义前缀形态。

### Fixed

- **react / react-dom 版本错配导致前端整体白屏修复**：dependabot 升级
  （c8bc34c24）将 `react` 升至 19.3.1 但未联动 `react-dom`（仍 19.2.7），
  React 19 要求两者严格同版本，运行时抛 Minified React error #527、SPA 无法
  挂载（任意页面空白）。`react-dom` 联动升级至 19.3.1（`@types/react-dom`
  至 19.3.0），浏览器实测登录、数据看板、系统设置全部恢复正常渲染。
- **仪表台配置「刷新间隔 / 数据限制」分区渲染崩溃修复**：两个分区的表单控件
  渲染在 `<Form>` Provider 之外，`FormItem` 调用 `useFormContext()` 得到 null
  抛出 `Cannot destructure property 'getFieldState'` TypeError，点击标签即被
  路由错误边界带入 `/500` 错误页。现按设置页标准结构补上 `<Form {...form}>`
  包裹层，两分区恢复正常渲染与保存。

### Changed

- **仪表台配置全面接管数据看板（旧「数据看板」分区移除）**：控制台内容下
  「仪表台配置」成为数据看板行为的唯一配置入口——
  - 4 个刷新间隔接线生效：`quota_data_refresh_interval`（秒）控制 quota_data
    落库循环周期；`rankings_refresh_interval` 控制排行榜快照缓存 TTL；
    `user_analytics_refresh_interval` 控制用户分析聚合查询缓存（按查询时间
    窗口缓存，上限 32 窗口）；`uptime_kuma_refresh_interval` 控制运行状态
    监控拉取的快照缓存。间隔或监控分组配置变更后对应缓存立即失效。
  - 数据管道写入开关统一：quota_data / user_analytics / rankings 任一指标
    启用即维持 quota_data 写入管道（原 `DataExportEnabled` 语义），消费日志
    与用量统计的 quota_data 旁路写入、日志页统计捷径同步切换。
  - `default_time_range_days` 接线：数据看板未保存本地偏好的用户，默认查询
    窗口取服务端配置（经 `/api/status` 下发）。
  - 旧选项 `DataExportEnabled` / `DataExportInterval` / `DataExportDefaultTime`
    移除：启动时一次性迁移到 `dashboard_config`（已在新面板保存过的字段不被
    覆盖；`DataExportInterval` 分钟值映射为两个刷新间隔的秒值；图表时间粒度
    本就是浏览器本地偏好，无后继配置），迁移后删除旧行防止复活。
  - `/api/status` 字段 `enable_data_export` / `data_export_interval` /
    `data_export_default_time` 替换为 `dashboard_refresh_interval`（秒，
    数据看板页自动刷新依据）与 `dashboard_default_time_range_days`。
- **Stripe 支付合规升级与实付金额核对**：Stripe SDK 由 `stripe-go v81` 升级到
  `v87.0.0`（API 版本随之升至 `2026-09-30.endive`），Stripe 客户端从已弃用的
  全局 `stripe.Key` 模式迁移到 `stripe.NewClient` 实例（v2 调用风格，Checkout
  Session 创建带 `integration_identifier` 追踪标签）；Webhook 验签执行严格
  API 版本校验——Stripe 控制台创建 webhook 端点时必须选择 `2026-09-30.endive`，
  版本不匹配的事件直接拒绝（400），暴露端点配置漂移。Webhook 入账前新增实付
  金额核对——`实付金额 + 促销折扣 == 数量 × StripeUnitPrice × 币种倍率`
  （零小数币种按主单位核对），Stripe Price 单价与 `StripeUnitPrice` 配置漂移时
  订单拒绝入账并返回 5xx 触发 Stripe 重投告警，杜绝"实收≠入账"的静默资损窗口。
- **JSON 列持久化规范按数据形态二分**：`internal/store/AGENTS.md` 的多库兼容规范
  现规定——自由形态 JSON 文本（配置、运行时拼装键值等）统一 `gorm:"type:text"`；
  结构化 JSON 对象（Go 结构体或自定义类型整体序列化，如 `channels.channel_info`、
  `prefill_groups.items`）用 `gorm:"type:json"`。`JSON` 类型六库可用（MySQL /
  PostgreSQL / openGauss 原生支持，DM8 实测支持 JSON 列建表与读写，金仓底层按
  jsonb 处理，SQLite 宽松类型直接接收）；仍严禁 `JSONB` 等 PG 专有变体及其他
  缺乏跨库等价物的类型。
- **计费扣减改为同步条件 UPDATE（防并发透支）**：用户余额（`users.quota`）与
  令牌剩余额度（`tokens.remain_quota`）的扣减不再进入批量更新队列，改为
  `WHERE ... >= ?` 条件 UPDATE 原子扣减，余额不足时返回
  `ErrUserQuotaNotEnough` / `ErrTokenQuotaNotEnough` 且不落库，数据库层面杜绝
  高并发窗口内额度扣成负数（实测修复前限额 20000 的令牌 8 秒内可被扣穿至
  -20689）。批量更新机制（`BATCH_UPDATE_ENABLED`）此后仅承载统计型字段
  （用户/令牌/渠道已用额度、请求计数），开启批量不再依赖 Redis 部署形态；
  `IncreaseUserQuota` 移除已无意义的 `db` 强制同步参数，配额缓存（Redis）改为
  在落库成功后异步调整，失败路径不污染缓存。
- **结算失败升级为系统级告警**：计费结算（`SettleBilling`）失败意味着实际消费
  未能足额扣减、平台承担差额损失，由请求级日志升级为 `SysError` 结构化告警
  （含 userId/tokenId/channelId/quota），不再静默。

### Added

- **进程优雅停机**：HTTP Server 接管为 `http.Server` 生命周期，SIGINT/SIGTERM
  触发 15 秒优雅停机，停止接收新流量后调用 `FlushBatchUpdates` 把批量暂存的
  统计增量落库，消除退出时最多一个批量周期的数据丢失；启动时执行
  `CheckNegativeQuotaBalances` 扫描，暴露历史遗留或异常写入造成的负值余额
  存量（用户余额、令牌剩余额度、窗口/周期超用）。

- **智谱套餐模型消耗查询升级**：模型用量改接 token-usage 明细接口
  （`/api/monitor/token-usage/usage-detail`，`type=1&usageType=MODEL`，海内外
  上游域名沿用套餐区域划分），按时间范围自动返回小时/天粒度。渠道套餐弹窗的
  模型消耗图由多折线趋势图改为按模型堆叠的柱形图（缓存/未缓存/输出 Tokens），
  并新增缓存命中率、低峰期用量占比、Tokens 总数、日均 Tokens 四项汇总卡片
  （含环比趋势展示）。

- **依赖批量升级**：合并 dependabot 全量依赖更新。Go：`shopspring/decimal`
  1.6.0、`aws-sdk-go-v2/credentials` 1.20.9、`gin-contrib/cors` 1.7.9、
  `Calcium-Ion/go-epay` 0.0.5、`grafana/pyroscope-go` 1.4.3；web：`react`
  19.3.1 与 `@types/react` 19.3.0、`ai` 7.0.148、`i18next` 26.5.4、
  `react-day-picker` 10.0.2、`@tanstack/react-query-devtools` 5.104.5，并
  联动升级 `@tanstack/react-query` 至 5.104.5 以满足 devtools 的 peer 版本
  要求（TanStack 官方要求两者同版本）。

- **系统设置分区调整**：「默认折叠侧边栏」由 运维 → 系统行为 迁移至
  站点与品牌 → 侧边栏模块 顶部，作为切换即时保存的独立开关；运维下的
  「系统行为」分区更名为「自动重试」。

### Fixed

- **Stripe 无折扣事件 webhook panic 修复**：真实无折扣 Checkout 事件的
  `total_details` 缺失或为 null，stripe-go `GetObjectValue` 双层路径首键
  即 panic（Cannot descend into non-map non-slice object），webhook 返回
  500 触发 Stripe 无限重投、订单永远无法入账；改为 `amountDiscountOf`
  安全提取（缺失/null/非数字按零折扣参与金额核对，数字值正常核对）。
  已用本地 HMAC 签名请求对运行实例完成全链路联调验证：正常入账（订单
  pending→success、额度精确增加、stripe_customer 回写）、错误签名/篡改
  payload/过期时间戳 400 拒绝、重放幂等（额度不重复增加）、实付金额
  错配 500 拒绝入账、未知单号 200 终态跳过。
- **Stripe 支付链路错误处理与弹窗拦截**：创建充值订单前获取用户失败不再吞错
  （原 `user, _ :=` 在异常时直接空指针 panic），改为返回本地化错误提示
  （新增 `topup.get_user_failed` 文案）；前端 Stripe 支付改为在异步请求前同步
  预开支付窗口，规避 Safari 等浏览器对异步 `window.open` 的弹窗拦截，弹窗被
  拦截时降级为当前页跳转。
- **DM8 大文本列类型（跨库迁移矩阵）**：`options.value`、`channels.key/other/
  models/other_info/settings`、`logs.content/other` 此前未声明列类型，PG 系
  （PostgreSQL/openGauss/金仓）按方言默认建成无上限 TEXT，DM8 却按
  VARCHAR(8188 CHAR) 建列（8188 字符上限），向 DM8 预迁移时超过 8188 字符的
  option（如 ModelRatio 大型 JSON 配置）即报 `-6169 列长度超出定义` 失败。
  现统一显式声明 `gorm:"type:text"`，六库均为无长度上限的大文本语义。
- **同类型迁移的同库防护（PostgreSQL）**：连接标识此前把 `inet_server_addr()`/
  `inet_server_port()` 连接路径字段计入身份比较，同一实例经 127.0.0.1/::1/
  主机 IP 等不同地址连接会被误判为不同库，勾选覆盖目标库时可能清空正在服务的
  主库；unix socket 连接下身份探测还会因 NULL 扫描失败而阻断迁移。现身份仅由
  实例指纹（`pg_control_system().system_identifier`）+ 库名构成。
- **同类型迁移的同库防护（openGauss）**：实例指纹取自 postmaster 启动时刻，
  实例在校验窗口内重启/主备切换时前后指纹不一致，可能把同一库判为不同库而
  绕过覆盖拦截；同批编排同秒拉起的不同实例也可能指纹相同被误拦。现指纹
  附加 `inet_server_port()` 监听端口区分同秒实例，并在校验期间复检源库指纹，
  变化即中止迁移要求重试。
- **迁移目标库的遗留唯一约束清理**：预迁移/同类型迁移的目标库建表路径此前以
  当前源库的全局旗标门控清理逻辑，源为 PG 系而目标为 MySQL 时会对目标库执行
  PG 专有清理 SQL，源为 SQLite/MySQL 而目标为 PG 系时则漏清理；现按目标库
  方言（Dialector.Name）门控。
- **达梦 DSN 口令校验的切分规则**：校验器此前在含 query 的整串上定位 userinfo，
  query 值携带 '@' 的合法 DSN 会被误报"口令包含 '?'"拒绝；现与驱动 parseDSN
  对齐（先以最后一个 '?' 剥离 query，再按最后一个 '@' 切分 userinfo）。
- **金仓 DSN 空串语义守卫**：`ora_input_emptystr_isnull=on` 硬拒绝此前只匹配
  小写参数名的首个取值，GUC 名大小写不敏感与 Gokb 同名参数后值覆盖的语义可
  被大写键或重复携带绕过；URL 解析失败也会被静默跳过归一化与拒绝。现按
  大小写不敏感逐值检查全部取值，重复/大小写变体收敛为单一 off，解析失败
  立即报错且不回显 DSN 原文。
- **迁移链路连接资源**：迁移目标库连接此前不设连接池上限，可能叠加实例会话
  上限（金仓默认 max_connections=100、DM 受 MAX_SESSIONS 约束）挤占线上会话，
  现设固定上限；主库/日志库未显式设置 `SQL_MAX_OPEN_CONNS` 时，启动对照
  金仓/DM 实例会话上限输出告警。openGauss/金仓未显式设置 `SQL_PREPARE_STMT`
  时，启动输出语句缓存与 `SQL_MAX_LIFETIME` 的成对配置提示。
- **存储层去重**：预迁移目标库的 AutoMigrate 模型清单直接复用 `mainDBModels`
  单一清单（此前为逐项复制的第二份清单，漏同步会导致目标库静默少表丢数据）；
  预迁移与同类型迁移共用的 PostgreSQL 系序列重置实现合并为单一函数。
- **og-smoke 冒烟脚本**：DSN 解析失败的报错不再回显含口令的 URL 原文。
- **文档与文案同步**：docker-compose.yml 补齐 DM/金仓示例 DSN 与建库前提
  （含外部库主机名说明）；系统设置迁移能力文案（中英）补齐 DM/金仓的全部
  方向；安装向导达梦提示补充 COMPATIBLE_MODE=0 前提；Mintlify 中英文档的
  DM 口令规则与建库前提同步为终态口径。
- 恢复误删的 .github 工作流（CI、Docker 镜像发布、dependabot、飞书通知）。

### Security

- **og-smoke 独立模块依赖安全升级**：GitHub Dependabot 在默认分支报出的 20 条
  漏洞告警（8 critical / 5 high / 7 moderate）全部源于 scripts/og-smoke 的间接依赖
  golang.org/x/crypto 被 openGauss connector（openGauss-connector-go-pq v1.0.9）
  锁定在 2021-07 的 v0.0.0-20210711020723-a769d52b0f97，聚合了 2021–2026 年间
  x/crypto 的全部已知 CVE（CVE-2024-45337 ssh 未授权访问、CVE-2023-48795
  Terrapin、CVE-2021-43565、CVE-2022-27191 等）。显式升至与主模块一致的
  v0.58.0（连带 x/sync v0.24.0、x/text v0.43.0），合入 main 后 20 条告警全部关闭；
  官方 connector 的 SCRAM 认证路径与全部 17 项冒烟用例实测通过。同步补充
  og-smoke.sh 前置条件：openGauss 新库 public schema 归初始用户所有，须为
  连接用户执行 GRANT CREATE, USAGE ON SCHEMA public。
- **web 依赖安全升级**：修复 CI 依赖审计（`bun audit --audit-level=high`）报出的
  3 个 high 级漏洞：
  - `zod` < 4.6.6 校验超大受限集合可耗尽内存
    （GHSA-mrhx-c7xc-m3w5）：直接依赖升至 4.6.7 并经 overrides 全树锁定；
    `shadcn` CLI 升至 4.22.1，消除其依赖树中携带的 zod 3.x（3.x 无补丁版本）。
  - `undici` 7.18.0 ~ 7.30.0 因过多 content-encodings 触发未处理错误导致 DoS
    （GHSA-xjj6-f33q-929c）：overrides 由 7.29.1 升至 7.30.1。
  - `body-parser` 2.0.0 ~ 2.3.x 压缩请求体后尾随字节导致 DoS
    （GHSA-27r3-jxmh-p828）：overrides 锁定 2.4.0（express 传递依赖）。
