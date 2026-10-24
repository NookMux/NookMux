# Changelog

本文件记录项目的所有重要变更。

格式基于 [Keep a Changelog](https://keepachangelog.com/zh-CN/1.1.0/)，
版本号遵循 [语义化版本](https://semver.org/lang/zh-CN/)。

## [Unreleased]

### Added

- **达梦 DM8 数据库支持**：新增达梦 DM8 作为原生后端数据库（Oracle 系内核，不依赖
  MySQL/Oracle 兼容模式）。DSN 以 `dm://` 前缀识别（达梦官方 Go 驱动原生支持的 URL
  scheme，口令按字面值写入且不得含 '?'，':'、'@' 可用，连接前对 DSN 口令做
  非法字符前置校验），连接层使用 `godoes/gorm-dameng`
  v0.7.2（GORM 的 OnConflict 由其 Create 回调翻译为 MERGE INTO，自增主键经 IDENTITY
  列自动回填，显式插值后计数器自动跟随、迁移后无需重置）。
  - 实例要求：以 CASE_SENSITIVE=N 初始化（连接时校验，不满足即启动失败），业务账号
    授予 SELECT ANY DICTIONARY 权限（AutoMigrate 依赖 SYS 系统视图探测表结构）；
    高并发部署建议按实例 dm.ini 的 MAX_SESSIONS 调低 SQL_MAX_OPEN_CONNS（默认
    1000），避免高峰耗尽会话。
  - 字符串列按 VARCHAR(n CHAR) 字符口径建列，与其他数据库的长度语义对齐，避免中文
    在字节口径下意外截断。
  - 方言分支适配：保留字列与渠道搜索/兑换码/充值单的列名引用在 DM 下走双引号；用量
    排行时间分桶纳入 FLOOR 截断分支；跨库预迁移支持 DM 与 MySQL/PostgreSQL/openGauss/
    SQLite 互迁，同类型迁移支持 DM → DM（含实例身份校验，基于 V$INSTANCE 实例指纹）；
    目标库写入按批包裹事务，钉定达梦 IDENTITY_INSERT 的会话粘性并获得批级原子回滚。
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

### Changed

- **智谱套餐模型消耗查询升级**：模型用量改接 token-usage 明细接口
  （`/api/monitor/token-usage/usage-detail`，`type=1&usageType=MODEL`，海内外
  上游域名沿用套餐区域划分），按时间范围自动返回小时/天粒度。渠道套餐弹窗的
  模型消耗图由多折线趋势图改为按模型堆叠的柱形图（缓存/未缓存/输出 Tokens），
  并新增缓存命中率、低峰期用量占比、Tokens 总数、日均 Tokens 四项汇总卡片
  （含环比趋势展示）。

### Fixed

- 恢复误删的 .github 工作流（CI、Docker 镜像发布、dependabot、飞书通知）。

### Security

- **web 依赖安全升级**：修复 CI 依赖审计（`bun audit --audit-level=high`）报出的
  3 个 high 级漏洞：
  - `zod` < 4.6.6 校验超大受限集合可耗尽内存
    （GHSA-mrhx-c7xc-m3w5）：直接依赖升至 4.6.7 并经 overrides 全树锁定；
    `shadcn` CLI 升至 4.22.1，消除其依赖树中携带的 zod 3.x（3.x 无补丁版本）。
  - `undici` 7.18.0 ~ 7.30.0 因过多 content-encodings 触发未处理错误导致 DoS
    （GHSA-xjj6-f33q-929c）：overrides 由 7.29.1 升至 7.30.1。
  - `body-parser` 2.0.0 ~ 2.3.x 压缩请求体后尾随字节导致 DoS
    （GHSA-27r3-jxmh-p828）：overrides 锁定 2.4.0（express 传递依赖）。
