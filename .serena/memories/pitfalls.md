# Pitfalls — 高频踩坑与易错点

这里只记录**违反后会产生具体 bug 或返工**的陷阱,每条都指向对应 `AGENTS.md` 规则。
规则本身不在 memory 重复,改规则去改 AGENTS.md,这里只存"踩过的雷"。

> 新踩坑后追加到对应分组,格式:`### 陷阱名` + `**后果**:` + `**规则**:` + 简短场景。

---

## 后端 — JSON / 序列化

### 直接用 encoding/json 而非 jsonx 包装
**后果**:与全局序列化行为不一致(数字精度、tag 处理、 RawMessage 类型判定等),relay/计费等敏感路径数据错乱。
**规则**:`internal/common/AGENTS.md` + `pkg/AGENTS.md` — 业务代码禁止直接调 `json.Marshal`/`Unmarshal`/`NewDecoder`,必须用 `jsonx.Marshal`/`jsonx.Unmarshal`/`jsonx.UnmarshalJsonStr`/`jsonx.DecodeJson`(`pkg/jsonx`,原 `common/json.go` 已迁移)。
**场景**:`encoding/json` 的 import 只允许引用类型(如 `json.RawMessage`),不能调函数。

---

## 后端 — 数据库六库兼容

### 只在部分库测试,忽略其余方言
**后果**:布尔值(`true/false` vs `1/0`)、保留字列引用(`"col"` vs `` `col` ``)、JSON 存储、ALTER COLUMN 行为在六库不同,未覆盖库上线即报错。
**规则**:`internal/store/AGENTS.md` — 必须同时兼容 SQLite / MySQL >= 5.7.8 / PostgreSQL >= 9.6 / openGauss 7.0 / 达梦 DM8 / 金仓 KingbaseES。用 `db.UsingPostgreSQL`/`UsingSQLite`/`UsingMySQL`/`UsingKingbase`(`internal/infra/db/database.go`)分支处理差异,优先 GORM,原始 SQL 必须参数化。
**场景**:SQLite 不支持 `ALTER COLUMN`,迁移要走 add-column 兼容模式;自由形态 JSON 文本用 `type:text`,结构化 JSON 对象用 `type:json`,严禁 JSONB。

---

## 后端 — 审计日志

### 管理员资源增删改漏埋 RecordAudit
**后果**:管理员操作无追溯,安全审计失败,出问题无法追责。
**规则**:`internal/httpapi/controller/AGENTS.md` 的"何时埋点" + `internal/domain/audit/AGENTS.md` — 渠道/用户/令牌/兑换码/模型/供应商/动态倍率/预填充分组/系统设置等资源的 create/update/delete 必须调 `audit.RecordAudit`(阶段5.3起,包 `internal/domain/audit`)。多个成功分支(如 enable/disable/delete/add_quota)每个都要埋。
**场景**:只读操作(查询/测试/余额/模型拉取预览)不埋;普通用户自助操作(改自己信息/签到/充值/Passkey)不埋。

### 审计配置变更不用 forceRecord
**后果**:管理员关闭审计总开关或 option 模块后,后续 RecordAudit 被跳过,"关闭审计"这个动作本身也不被记录 —— 安全漏洞。
**规则**:`internal/httpapi/controller/AGENTS.md` 的"审计配置变更的特殊处理" — `UpdateOption` 中对 `audit_setting.*` 的变更必须传 `forceRecord=true`。因为 `model.UpdateOption` 先于 `RecordAudit` 执行,新配置会立即生效。

### 新增审计资源类型不更新检查清单
**后果**:新资源的审计模块未注册,前端审计日志页面看不到该模块,后端 IsAuditModuleEnabled 返回 false 导致 RecordAudit 被跳过。
**规则**:`internal/httpapi/controller/AGENTS.md` 的"新增资源类型时的检查清单"(6 步:model 常量 + AuditModuleList → setting 注册 → 前端 AUDIT_MODULES → static-keys.ts → en/zh.json → controller 埋点)。

---

## 后端 — relay / AI 中继

### 改协议转换破坏流式输出
**后果**:chunk 顺序错乱 → 用户看到乱码/重复;finish_reason/usage 丢失 → 计费不准或无法停止;错误事件丢失 → 上游错误被吞。
**规则**:`internal/relay/AGENTS.md` — 流式必须保护 chunk 顺序、错误事件、finish reason、usage、连接关闭行为。改转换后跑 `go test ./internal/relay/...`。

### relay DTO 用非指针可选标量字段
**后果**:客户端显式传 `0`/`0.0`/`false` 被 `omitempty` 丢弃,上游收到错误默认值。
**规则**:`internal/relay/AGENTS.md` + 根 `AGENTS.md` — 需要转发给上游的可选标量字段用 `*int`/`*uint`/`*float64`/`*bool` + `omitempty`,保留显式零值。

### 吞掉上游错误
**后果**:错误类型/HTTP 状态/用户可见信息丢失,无法诊断。
**规则**:`internal/relay/AGENTS.md` — 错误类型、HTTP 状态、用户可见信息保持可诊断,不吞错。
**场景**:请求身份、渠道 key、用户 token、签名、敏感 header 不得写入日志(relay + middleware 都有此约束)。

---

## 后端 — 分层 / 边界

### 路由层承载业务逻辑
**后果**:层次混乱,controller/service 难复用,重复代码。
**规则**:`internal/httpapi/router/AGENTS.md` + 根 `AGENTS.md` — 路由只挂载,controller 只做边界(校验/权限/响应组织),domain(internal/domain)承载业务,store(internal/store)承载持久化。

### middleware 读 body 后不恢复
**后果**:后续 handler 读不到 body,relay / 文件上传 / 签名校验全炸。
**规则**:`internal/httpapi/middleware/AGENTS.md` — 读取请求 body 后必须恢复给后续处理器。

### 全局 gzip 破坏 SSE / streaming
**后果**:AI 回复被缓冲,用户看到卡住或一次性吐出。
**规则**:`internal/httpapi/router/AGENTS.md` + `internal/httpapi/middleware/AGENTS.md` — 不要添加会破坏 SSE/streaming/websocket 的全局 gzip;web 静态资源 gzip 只在 web router 中处理。

---

## 后端 — 配置 / 性能

### 待机内存参数随意调大
**后果**:低流量/待机场景内存浪费,容器 OOM 或成本飙升。
**规则**:根 `AGENTS.md` — 连接池 idle 上限、prepared statement 缓存、后台 worker/goroutine 池、ticker 唤醒频率等常驻资源默认值必须保守;确需调大必须保留环境变量覆盖 + 同步 `.env.example` + 中英文环境变量文档。

### 倍率/价格/限流配置解析失败静默回落默认值
**后果**:看似成功但用了错误配置,计费/限流失准。
**规则**:`internal/config/AGENTS.md` — 解析失败必须显式返回错误,不能回落到看似成功的默认值。

---

## 后端 — i18n(后端响应消息)

### 后端响应硬编码中英文字符串
**后果**:英文用户看到中文,或反之;无法国际化。
**规则**:`internal/i18n/AGENTS.md` — 用户可见 API 错误/成功提示走 `i18n.Msg*` 常量 + `common.ApiErrorI18n`/`ApiSuccessI18n`/`i18n.T`,不硬编码。内部日志(SysError/SysLog)不翻译。
**场景**:key 必须定义在 `internal/i18n/keys.go` 常量中按模块分组,不要在调用处写字面量 key;`locales/en.yaml` 和 `locales/zh.yaml` 必须同步加同一 key(缺失会原样返回 key 字符串)。

---

## 前端 — 文案 / i18n

### 只改 en 不改 zh(或反之)
**后果**:`bun run i18n:sync` 报错,另一语言用户看到原始 key 字符串(如 `channels.fields.xxx`)。
**规则**:`web/AGENTS.md` + `internal/i18n/AGENTS.md` — 新增用户可见文案在 `web/src/i18n/locales/en/<section>.json` 和 `zh/<section>.json` 各加一行;新增 section 文件必须在 `config.ts` 导入展开。
**场景**:语义 key 格式 `<section>.<group>.<name>`,section 名含连字符时前缀转 camelCase(`usage-logs` → `usageLogs`);`keySeparator:false`/`nsSeparator:false`,不要写成嵌套对象。

### 用 mock 数据替代真实后端
**后果**:看似能跑但联调炸,接不上真实 API。
**规则**:根 `AGENTS.md` + `web/AGENTS.md` — 禁止 mock 数据、假分页、假成功状态、静默吞错。服务端响应以本项目后端为准,参考项目 API 不存在时隐藏入口或改前端适配,不新增后端业务 API。

### 改 TS/TSX 不跑 typecheck
**后果**:类型错误漏到运行时,白屏或运行时崩溃。
**规则**:`web/AGENTS.md` — 改 TS/TSX 后至少 `bun run typecheck`;改路由/API/核心页面/构建配置后 `bun run build` 或 `bun run build:check`。

### 凭记忆调用 package.json 未定义的 script
**后果**:命令不存在,直接报错(如 `bun run lint:fix` / `bun run eslint:fix` / `bun run i18n:extract` 都不在当前 `web/package.json` 中)。
**规则**:见 `web/AGENTS.md` 的"命令"章节 — 实际可用的是 `bun run dev`/`typecheck`/`lint`/`format:check`/`build`/`build:check`/`i18n:sync`。调用前先查 `package.json` 的 scripts,不要凭印象用命令。

---

## 工具链 — Serena 配置

### 手改 project.yml 用旧字段名或不全字段
**后果**:Serena 加载时检测到旧字段名(如 `languages`)或缺字段,标记 incomplete,自动重写整个文件 —— 改名字段被追加到文件末尾、自定义中文注释被官方模板注释强制覆盖(`save()` 的 `transfer_yaml_comments(force_update_all=True)`),git diff 全是噪音。
**规则**:改 `.serena/project.yml` 必须用最新 schema 字段名(参考 `参考项目/serena/src/serena/config/serena_config.py` 的 `RENAMED_FIELDS` 与 dataclass 字段)且字段齐全;注释保持与官方模板 `src/serena/resources/project.template.yml` 完全一致(逐字节,包括 `e.g.` 与 `e.g.,` 的差异);项目的中文维护说明写到 memories,不写进 project.yml 注释。
**场景**:`languages` → `language_servers` 是 1.7 前后的改名;round-trip 验证方式:用 serena 真实源码 `ProjectConfig.load()` + `save()`,文件应字节级不变。

### 全局配置丢失导致索引爆炸
**后果**:全局 `~/.serena/serena_config.yml` 的 `ignored_paths` 含 `node_modules`/`dist`/`参考项目` 等;若机器换环境全局配置缺失,只靠 gitignore 的忽略不够快,Serena 会尝试索引 web/node_modules 和参考项目。
**规则**:project.yml 的 `ignored_paths` 显式包含 `web/node_modules`、`web/dist`、`.serena/cache`、`参考项目` 等重目录(显式比依赖全局更稳定),以 `*` 开头的模式必须加引号(YAML alias 限制)。

---

## 跨层 — 流程

### 不读子目录 AGENTS.md 就改代码
**后果**:违反该包特有约定(遗漏审计埋点、数据库兼容性、前端 i18n 缺失等),返工。
**规则**:根 `AGENTS.md` 的"必读:分层规则" — 改某个包/目录下的代码前,必须先读该目录的 `AGENTS.md`。跨包改动读所有受影响包的 AGENTS.md。

### 删除/改名项目元数据
**后果**:AGPL/版权头丢失有法律风险;Go module path 改了导致 import 全断;Docker/CI 镜像名改了构建失败。
**规则**:根 `AGENTS.md` — 不要顺手删除、替换或改名项目标识、AGPL/版权头、Go module path、Docker/CI 镜像名等元数据。

### 复制参考项目代码不适配
**后果**:接口/配置对不上,编译或运行报错。
**规则**:根 `AGENTS.md` + `docs/AGENTS.md` — `参考项目/` 仅用于比对上游实现,复制代码前必须适配本项目 API 和配置。参考项目已被 .gitignore 忽略,不要让 Serena 索引。

### 使用日志字段新增/删除不同步
**后果**:详情弹窗字段错乱,管理员/普通用户可见性配置失效。
**规则**:`internal/httpapi/controller/AGENTS.md` 的"使用日志字段可见性"(4 步同步:后端 `UsageLogFieldsDefaults()` + `UsageLogField*` 常量 → 前端 `field-visibility.ts` → `details-dialog.tsx` 条件渲染改 `isVisible('<fieldKey>')`)。
