# FuturePPO/new-api 提交历史（按功能模块归类 · 续）

## 项目概况

- **仓库**: https://github.com/futureppo/new-api
- **分析时间段**: 2026年8月21日 - 2026年9月18日
- **总提交数**: 53 个提交（与《futureppo-commits-analysis-new.md》完全一致，一个不少）
- **提交者**: Futureppo 44 个；外部贡献者 beicho 9 个，经 PR #3 / #5 / #6 由 Futureppo 审核合入
- **最近同步**: 2026年9月18日（上游 HEAD `8d7ee9a2`）
- **承接范围**: 承接《futureppo-commits-by-feature.md》（止于 `ef5e329d`），本批覆盖 `ef5e329d..8d7ee9a2`

## 阅读说明

- 本文档将《futureppo-commits-analysis-new.md》中按**时间线**排列的 53 个提交，重新按**功能模块**（二级标题）归类。
- 每个模块内部再按**具体功能**聚合为三级标题：同一功能的实现、后续修复与优化合并在
  同一个三级标题下，按时间先后排列，便于看出该功能的演进过程。
- 三级标题下先给出功能概述，随后逐行列出所属提交，格式为
  `commit hash`-编号：提交标题（编号即《futureppo-commits-analysis-new.md》的时间线序号，便于交叉核对）。
- 编号规则：本批 53 个提交使用 **#1～#53**（越新编号越小，#1 = 2026年9月18日 `8d7ee9a2` 最新）。
- 个别提交涉及多个模块时归入主要模块，并在相关功能概述中注明（如 `267e21f7` 以 VyceAI 为主、
  顺带补齐 GMI Cloud 批量推理任务）。
- 外部贡献者 beicho 的提交按提交时间归入所属功能，merge 提交错后排列；各提交的具体提交日期
  可在文末「附录：提交完整性核对」表中查询。

## 功能模块汇总

| 功能模块 | 功能数 | 提交数 |
|---|---|---|
| 一、风控与封禁体系 | 3 | 3 |
| 二、签到与防刷（外部 PR #3 / #5） | 3 | 8 |
| 三、账号凭证安全 | 1 | 2 |
| 四、注册与邀请体系 | 3 | 4 |
| 五、渠道集成与适配（新增渠道） | 6 | 11 |
| 六、协议兼容与中继改进 | 4 | 5 |
| 七、定价与模型市场 | 2 | 2 |
| 八、模型状态与监控展示 | 3 | 4 |
| 九、管理效率 | 2 | 2 |
| 十、站点背景与液态玻璃主题（外部 PR #6） | 2 | 8 |
| 十一、UI / UX 优化 | 2 | 2 |
| 十二、文档与部署 | 1 | 2 |
| **合计** | **32** | **53** |

---

## 一、风控与封禁体系 — 3 项功能 / 3 个提交

> 在既有风控中心基础上补齐「定时封禁 → 自动解封」闭环，新增全站级用户批量操作，并将错误详情可见性收紧到 root。

### 定时封禁与自动解封

`User` 模型新增禁用时长与截止时间字段（Unix 秒 + 索引，规避三库日期函数差异），后台任务 30 秒 tick 批量到期解封，鉴权路径条件更新惰性解封；ManageUser / 批量用户操作 / 连锁封禁 / GitHub 年龄风控等入口全链路支持 `duration_minutes`，封禁提示统一附「永久 / 时长·解禁时间」后缀。覆盖 49 个文件（+1512/-344 行），含三库迁移测试。

- 632d1b52-#3：feat: support timed user bans and automatic unban

### 全站用户批量操作

新增 `POST /api/user/batch-manage`：启用全部已禁用、禁用全部已启用（必填统一原因）、软删除、永久删除、清理已注销用户五种动作，事务内 500/批分块执行并逐用户失效缓存、写审计；前端扩展为批量操作下拉 + 确认弹窗。覆盖 14 个文件（+711 行）。

- 0c80da7d-#28：feat: add global user batch operations

### 错误详情仅 root 可见

任务失败原因、上游错误码、结果附件对非 root 一律脱敏为占位标识；log / midjourney / task 的可见性判定从「渠道错误详情可见性」改为「查看者角色」，relay 任务链路连带清空私有数据防旁路泄露。覆盖 8 个文件（+195 行）。

- c9ae7f35-#43：fix: restrict error details to root users

---

## 二、签到与防刷（外部 PR #3 / #5） — 3 项功能 / 8 个提交

> 本批最集中的外部贡献：beicho 经 PR #3 一次交付「环境分压低奖励 + 行为特征评分 + 额度当日有效」的完整防刷体系，PR #5 修复配套设置键同步缺陷。

### 签到防刷评分与额度当日有效（PR #3）

可选开关下按 9 个请求头信号累加 0-100 环境分，实发奖励按分数**压低而非拦截**（拒绝会给出可被二分测试利用的成败信号）；行为特征评分补齐无头浏览器识别盲区（签到时刻圆周标准差、掐点触发、消费关联，样本不足满分保护）；签到额度可配置当日有效，复用 `checkins` 表作发放台账按天清算回收，超 3 天积压只标记不追溯。合计 11 个文件（+1340 行），提交含 Co-Authored-By: Claude Opus 5 (1M context) 尾注。

- 20aa84fe-#53：feat(checkin): 支持签到额度当日有效，次日清算回收
- 5ff2ec84-#52：test(checkin): 覆盖特殊星期奖励与当日有效的组合场景
- b310c940-#51：feat(checkin): 对非浏览器环境的签到压低奖励而非拦截
- daf365fc-#50：feat(checkin): 增加行为特征评分，覆盖无头浏览器场景
- 1388b9b9-#49：Merge pull request #3（checkin-client-env-scoring）

### 签到设置开关同步修复（PR #5）

三个新签到配置键未同步到父组件 `OperationSetting` 的 inputs 初始状态，字符串 "false" 为 truthy 导致开关显示已开启而功能实际关闭，且严格比较判定无改动、保存无从下手；补齐 3 行默认键修复。

- 1a4492bb-#42：fix(checkin): 补齐设置页父组件缺失的签到配置键，修复开关显示为已开启
- 2e11b1ee-#41：Merge pull request #5（settings-boolean-sync）

### 签到日志 IP 显示修复

用量日志 IP 列展示条件补充签到类日志类型，签到记录的请求 IP 恢复显示。

- 301fb3c2-#4：修复签到不显示ip

---

## 三、账号凭证安全 — 1 项功能 / 2 个提交

### 密码强度策略与强制设密

新增密码策略（≥8 字符且同时含数字/大写/小写/符号，crypto/rand 生成 16 位强密码）；纯 OAuth 账号强制引导设置登录密码（403 `PASSWORD_SETUP_REQUIRED` + 前端守卫）；新增专用改密接口并从资料更新接口移除 legacy 改密通道，杜绝借道改密。两提交合计 50 个文件（+1402/-268 行），含 5 个新增 Go 测试文件。

- 4db618c1-#2：feat: require login password setup and enforce password strength
- 8d7ee9a2-#1：fix: remove legacy self-service password changes

---

## 四、注册与邀请体系 — 3 项功能 / 4 个提交

### 域名邮箱注册与豁免拆分

已验证域名邮箱（白名单命中且不在黑名单，支持 `*.edu.cn` 式通配）可豁免邀请码与注册码；随后将单一白名单拆分为**邀请码豁免**与**注册码豁免**两个独立列表，旧配置在选项加载时自动迁移回填，显式保存过的新值始终优先。

- 85781f43-#27：feat: support domain email code-free registration
- 14a24193-#21：fix: split email domain registration code exemptions

### 邮箱身份大小写与消歧

邮箱比较归一化（trim + 小写，默认开启可配），大小写变体命中多个用户时返回歧义错误而非擅自选定账号；注册验证码、找回密码、OAuth 注册拦截、充值回填全链路接入查重。

- 8807b946-#40：feat: Enhance email identity handling with case insensitivity and ambiguity checks

### 16 位邀请码强制

字母表剔除易混淆的 I/O/0/1，crypto/rand 生成、最多 128 次查库去重；存量用户 aff_code 以幂等标记一次性迁移重生成为 16 位码，非法格式码按不存在处理；前端输入统一归一化并限长。

- 6fcfc9b2-#26：feat: enforce 16-character invite codes

---

## 五、渠道集成与适配（新增渠道） — 6 项功能 / 11 个提交

> 单批新增 5 个渠道（Vercel / GMI Cloud / VyceAI / Modal / Kilo，渠道编号 66~70），并对 AgnesAI 做文本、图片、视频全模态整合。

### Vercel AI Gateway

渠道 66，基址 `https://ai-gateway.vercel.sh`，复用 OpenAI API 类型，端点支持 Chat Completions 与 OpenAI Responses；含 4 组单测。

- 242aae56-#39：新增vercel渠道

### GMI Cloud（含音频/音乐任务与批量推理）

渠道 67，复用 OpenAI 类型维护 MiniMax 系列模型；新增音频/音乐异步任务 adaptor（按 payload 与模型名自动判定音频生成 / 音乐生成 / voice_clone）、音频任务路由与模型拉取合并；VyceAI 提交（`267e21f7`）中顺带补齐批量推理任务端点 `/v1/batch/generations`。

- f429081b-#38：新增 gmicloud 渠道
- 732ed59e-#37：适配 gmicloud 渠道的音频音乐模型

### VyceAI 生图

渠道 68，将 OpenAI 生图请求转换为上游 `/v1/images/stream` 的 SSE 请求并聚合为同步响应（计费固定按次）；随后修复安全拦截类上游错误误报 502 的问题（识别后映射 403 + skip-retry）。

- 267e21f7-#36：新增 VyceAI OpenAI 生图渠道
- 473f6300-#35：修复渠道 VyceAI 502问题

### Modal（定时保活）

渠道 69，无内置模型目录，`NormalizeBaseURL` 同时接受部署 origin 与完整 `/v1/chat/completions` 地址；按渠道配置间隔对**每个模型**发真实推理请求保活（默认 30s，先记 lastAttempt 防请求堆积）；Modal 请求跳过 OpenAI 模型名启发式改写，防止 "o" 开头的自部署模型被误改。

- 029ab7c1-#25：feat: add Modal channel with scheduled keepalive
- e4b9b2d2-#24：fix: invoke all Modal models for keepalive
- 2a2d3859-#23：fix: preserve Modal OpenAI-compatible requests

### Kilo（匿名免费模型同步）

渠道 70，可免密钥匿名调用免费模型；模型列表按 `isFree` 筛选并复用 OpenRouter 托管模型计划（别名冲突、名称简化、自动映射）；随后修正认证模式尊重显式配置、渠道表单状态重置与匿名开关默认值。

- 4c9bcf05-#13：feat: add Kilo channel with anonymous free model sync
- b31cb54d-#9：fix: respect Kilo authentication mode and reset channel form state

### AgnesAI 全模态整合

文本 adaptor 重构（OpenAI / Claude / Responses 三格式透传，加入流式支持渠道名单）；图片参数归一（`size`+`ratio`+`return_base64`，输入收敛进 `extra_body.image`）；视频任务统一校验并新增「模型映射后、预扣费前」的校验钩子，任务查询导出 `RefreshVideoTask` 复用 CAS 持久化与退款结算路径。覆盖 26 个文件（+1877/-171 行），附渠道文档与 7 个测试文件。

- eccfd97f-#22：feat: complete Agnes text image and video integration

---

## 六、协议兼容与中继改进 — 4 项功能 / 5 个提交

> 本批密集补齐 DeepSeek / OpenAI 官方 / Mistral 的原生与兼容协议细节，Mistral 由此获得原生推理 API 中继能力。

### Mistral 原生推理 API 与 OpenAI 兼容

新增原生协议中继：保留原始 body（含网关未知字段），覆盖 `/v1/ocr`、`/v1/fim/completions`、`/v1/agents/completions`、`/v1/audio/speech` 与实时转写 WebSocket 双向裸帧转发；兼容侧重构 adaptor（BaseURL 归一化、端点映射）、补齐 Voxtral 音频转写 multipart、流式 usage 挂最后一个 choices 块的特有行为与顶层错误解析（不回显敏感 `input`）。两提交合计 43 个文件（+3287 行），附 README 与多层测试。

- e19c8cbb-#15：feat: relay Mistral native inference APIs
- 6ad174ef-#16：fix: complete Mistral OpenAI API compatibility

### DeepSeek 原生协议

新增 `BoolOrInt` 双类型承载 `logprobs`、`StreamOptions` 指针化区分未传与显式 false；端点按格式路由（Claude 走原生 `/anthropic/v1/messages`，发现 `prefix`/`strict` 自动升级 beta 端点）；Anthropic 流式错误 SSE 透传，scanner 在终止事件时主动关流解除阻塞。覆盖 39 个文件（+1310 行），附渠道文档。

- 8db3a5de-#20：fix: complete DeepSeek native protocol compatibility

### OpenAI 官方渠道兼容

DTO 补齐官方字段保留（`audio`/`refusal`/`service_tier`/`obfuscation`/`prompt_cache_options`/`moderation` 等），重写 Chat↔Responses 双向转换；Realtime GA 迁移（旧 Beta 头直接报错引导）；SSE 媒体流原样转发（仅观察者解析 usage 记账）；缓存写 token 计费别名归一。覆盖 46 个文件（+2296 行），附渠道文档与 HTTP/WebSocket 回归测试。

- 95b0e782-#19：fix: complete official OpenAI channel compatibility

### OpenRouter Alpha 模型识别

托管模型判定不再要求 `openrouter/` 前缀，改为取最后一段路径判断 `-alpha` 后缀，任意提供商（含嵌套命名空间）的 Alpha 测试模型均纳入自动同步范围。

- d083133e-#18：fix: recognize OpenRouter alpha models from any provider

---

## 七、定价与模型市场 — 2 项功能 / 2 个提交

### 负数定价与异步任务延期结算

后端支持负数费用结算（返还型差额退款，预扣钳制非负、负数在最终汇总入账）；任务 / Midjourney 按预估预扣、终态按实际配额事务结算，状态条件更新防竞争轮询器重复入账，令牌已删除不阻断退款；前端保存前强制确认负数项并警示净结算规则。覆盖 39 个文件（+1960 行），测试含三库方言。

- d727441b-#14：feat: support negative model pricing with save confirmation

### 模型市场价格与名称排序

新增五种排序维度（默认/输入价/输出价/单次价/模型名），价格叠加当前分组倍率、无固定价格的模型置后；修正 Semi 受控分页下「先分页后排序」的失真。新增 216 行排序测试；覆盖 17 个文件（+676 行）。

- 55c095d5-#12：feat: add model marketplace price and name sorting

---

## 八、模型状态与监控展示 — 3 项功能 / 4 个提交

### 模型状态服务端定时刷新与短时间窗口

公开 embed 状态接口改为只读进程内快照（访客请求不再触发日志扫描），后台任务按配置间隔（下限 60s、上限 24h）重算，配置指纹变更即作废旧数据；时间窗口新增 0.5h/1h/6h/12h 四个短窗口，统计槽位最小限制放宽至 1 分钟。

- 4fc34076-#10：feat: add short time windows to model status
- 0c4cb469-#8：fix: refresh model status on a server schedule

### 渠道测试上游响应模型回显

通过 `ConversationCapture` 捕获适配器实际消费的上游响应体并提取 `model` 字段，渠道测试响应新增 `upstream_model`，便于核对模型重定向/映射是否生效。

- 5094eac1-#6：feat: display upstream response model in channel tests

### 用量日志推理强度展示

在协议转换与参数覆盖完成后，从最终上游请求体按多协议 gjson 路径提取显式思考强度，随日志 `other` 字段落库并在用量日志以 Tag 展示；只记录不推断，重试换渠道或覆盖删除字段时自动清空。

- a8ad0591-#11：feat: display final upstream reasoning effort in usage logs

---

## 九、管理效率 — 2 项功能 / 2 个提交

### 批量删除渠道（ID 区间 / 精确名称）

新增两个管理员接口；区间删除先 Pluck 实际存在 ID（稀疏区间不按区间大小分配），名称删除先 name 索引 + `FOR UPDATE` 锁候选、再在 Go 内逐个精确比较（防数据库排序规则忽略大小写/重音/尾随空格），事务内分块删除并级联清理 abilities；前端纯文本正则校验防数字输入框舍入 ID。覆盖 24 个文件（+1297 行）。

- 998f4b5a-#7：feat: bulk delete channels by ID range or exact name

### 数据看板月 / 年时间粒度

聚合粒度新增 `month`/`year`，引入日历桶机制按自然月/自然年起始归点，RPM/TPM 换算与补点改用逐点 interval，避免月/年天数不一导致的速率失真。

- e6a06fec-#29：feat: 数据看板增加月年时间粒度

---

## 十、站点背景与液态玻璃主题（外部 PR #6） — 2 项功能 / 8 个提交

> 8 月增量批次的站点背景主题在本批继续演进：先补齐「下载背景 + 跨域代理化」，再由外部贡献者 beicho 经 PR #6 完成玻璃表面视觉重写与 WebGL 渲染器。

### 背景下载与跨域代理化

顶栏用户下拉新增「下载背景」入口（fetch 转 blob 触发保存）；直连下载受浏览器 CORS 限制后，新增服务端代理接口按配置源代拉图片流式返回（无凭据 HTTP(S) 校验、SSRF 防护、体积限额、JSON API 源按 `json_path` 解析），前端跨域源切换走代理。三提交合计 30 个文件（+670 行）。

- ca22fc71-#34：支持 下载背景
- 255cab29-#31：修复跨域图片背景无法下载问题
- f561dfb0-#30：新增 跨域下载图片 相关汉化

### 液态玻璃表面重写与 WebGL 渲染器（PR #6）

CSS 表面重写：定位磨砂塑料感五个根因，改为厚度梯度底色、方向性亮边、折射暗带、内缘聚光、镜面反射层与双层阴影；折射以 SVG `feDisplacementMap` 重新实现但**默认关闭**（backdrop-filter 引用滤镜会脱离 GPU 加速，代价与强度无关）；可读性以销毁背景高频细节为主（blur 提至 26px，实测最差对比度维持 12.5，远超 WCAG AAA）；新增 WebGL 渲染器：背景预烘焙模糊（降采样 + 分离高斯）、玻璃合批每帧 2 draw call、折射/色散/边缘光着色器，WebGL 不可用或跨域受限时自动回退 CSS。PR #6 合计 10 个文件（+1692 行），后端新增渲染器与三个光学参数配置。

- 69f947f6-#48：feat(site-background): 重写液态玻璃表面，去掉磨砂塑料观感
- 9a1a6155-#47：fix(site-background): 提高玻璃上的文字可读性
- af436d06-#46：feat(site-background): 折射作为可选增强项加回，默认关闭
- bdf18112-#45：feat(site-background): 新增 WebGL 玻璃渲染器，折射、色散与边缘光走着色器
- fc424100-#44：Merge pull request #6（glass-surface）

---

## 十一、UI / UX 优化 — 2 项功能 / 2 个提交

### 前端瘦身（移除无用组件）

彻底移除「邮件日志」侧边栏模块（路由项、开关配置、图标、通知入口）与增强页「IP 日志记录覆盖率」卡片，7 个语言包词条同步删除。覆盖 13 个文件（+12/-155 行）。

- e6f7ea90-#5：移除大量无用组件

### JSON 编辑器交互修复

新增键值对不再自动生成 `field_N` 占位键，键与值均留空由用户输入，避免假键混入 JSON 配置。

- 25068c42-#17：fix: leave new JSON editor key-value pairs empty

---

## 十二、文档与部署 — 1 项功能 / 2 个提交

### 部署文档与镜像元数据

5 份语言 README 新增「新部署推荐优先使用 PostgreSQL」提示；Docker 镜像地址从上游统一替换为 `ghcr.io/futureppo/new-api:latest`。

- ecd5dff7-#32：新部署推薦優先使用 PostgreSQL
- 0eaef5b8-#33：修改镜像下载地址为当前项目

---

## 附录：提交完整性核对

53 个提交按提交时间从旧到新排列（beicho 的 9 个提交为实际提交日期，分别经 8月21日 PR #3、8月24日 PR #5/#6 合入；merge 提交错后于其分支提交）。

| 提交 | 编号 | 提交日期 | 提交标题 |
|---|---|---|---|
| 20aa84fe | #53 | 2026-08-21 | feat(checkin): 支持签到额度当日有效，次日清算回收 |
| 5ff2ec84 | #52 | 2026-08-21 | test(checkin): 覆盖特殊星期奖励与当日有效的组合场景 |
| b310c940 | #51 | 2026-08-21 | feat(checkin): 对非浏览器环境的签到压低奖励而非拦截 |
| daf365fc | #50 | 2026-08-21 | feat(checkin): 增加行为特征评分，覆盖无头浏览器场景 |
| 1388b9b9 | #49 | 2026-08-21 | Merge pull request #3 from Beicho/feat/checkin-client-env-scoring |
| 1a4492bb | #42 | 2026-08-22 | fix(checkin): 补齐设置页父组件缺失的签到配置键，修复开关显示为已开启 |
| 69f947f6 | #48 | 2026-08-22 | feat(site-background): 重写液态玻璃表面，去掉磨砂塑料观感 |
| 9a1a6155 | #47 | 2026-08-22 | fix(site-background): 提高玻璃上的文字可读性 |
| af436d06 | #46 | 2026-08-22 | feat(site-background): 折射作为可选增强项加回，默认关闭 |
| bdf18112 | #45 | 2026-08-22 | feat(site-background): 新增 WebGL 玻璃渲染器，折射、色散与边缘光走着色器 |
| fc424100 | #44 | 2026-08-24 | Merge pull request #6 from Beicho/feat/glass-surface |
| c9ae7f35 | #43 | 2026-08-24 | fix: restrict error details to root users |
| 2e11b1ee | #41 | 2026-08-24 | Merge pull request #5 from Beicho/fix/settings-boolean-sync |
| 8807b946 | #40 | 2026-08-25 | feat: Enhance email identity handling with case insensitivity and ambiguity checks |
| 242aae56 | #39 | 2026-08-25 | 新增vercel渠道 |
| f429081b | #38 | 2026-08-25 | 新增 gmicloud 渠道 |
| 732ed59e | #37 | 2026-08-25 | 适配 gmicloud 渠道的音频音乐模型 |
| 267e21f7 | #36 | 2026-08-26 | 新增 VyceAI OpenAI 生图渠道 |
| 473f6300 | #35 | 2026-08-26 | 修复渠道 VyceAI 502问题 |
| ca22fc71 | #34 | 2026-08-26 | 支持 下载背景 |
| 0eaef5b8 | #33 | 2026-08-26 | 修改镜像下载地址为当前项目 |
| ecd5dff7 | #32 | 2026-08-26 | 新部署推薦優先使用 PostgreSQL |
| 255cab29 | #31 | 2026-08-26 | 修复跨域图片背景无法下载问题 |
| f561dfb0 | #30 | 2026-08-26 | 新增 跨域下载图片 相关汉化 |
| e6a06fec | #29 | 2026-08-28 | feat: 数据看板增加月年时间粒度 |
| 0c80da7d | #28 | 2026-08-29 | feat: add global user batch operations |
| 85781f43 | #27 | 2026-09-01 | feat: support domain email code-free registration |
| 6fcfc9b2 | #26 | 2026-09-03 | feat: enforce 16-character invite codes |
| 029ab7c1 | #25 | 2026-09-03 | feat: add Modal channel with scheduled keepalive |
| e4b9b2d2 | #24 | 2026-09-03 | fix: invoke all Modal models for keepalive |
| 2a2d3859 | #23 | 2026-09-03 | fix: preserve Modal OpenAI-compatible requests |
| eccfd97f | #22 | 2026-09-10 | feat: complete Agnes text image and video integration |
| 14a24193 | #21 | 2026-09-12 | fix: split email domain registration code exemptions |
| 8db3a5de | #20 | 2026-09-12 | fix: complete DeepSeek native protocol compatibility |
| 95b0e782 | #19 | 2026-09-13 | fix: complete official OpenAI channel compatibility |
| d083133e | #18 | 2026-09-13 | fix: recognize OpenRouter alpha models from any provider |
| 25068c42 | #17 | 2026-09-13 | fix: leave new JSON editor key-value pairs empty |
| 6ad174ef | #16 | 2026-09-14 | fix: complete Mistral OpenAI API compatibility |
| e19c8cbb | #15 | 2026-09-14 | feat: relay Mistral native inference APIs |
| d727441b | #14 | 2026-09-15 | feat: support negative model pricing with save confirmation |
| 4c9bcf05 | #13 | 2026-09-15 | feat: add Kilo channel with anonymous free model sync |
| 55c095d5 | #12 | 2026-09-15 | feat: add model marketplace price and name sorting |
| a8ad0591 | #11 | 2026-09-16 | feat: display final upstream reasoning effort in usage logs |
| 4fc34076 | #10 | 2026-09-16 | feat: add short time windows to model status |
| b31cb54d | #9 | 2026-09-17 | fix: respect Kilo authentication mode and reset channel form state |
| 0c4cb469 | #8 | 2026-09-17 | fix: refresh model status on a server schedule |
| 998f4b5a | #7 | 2026-09-17 | feat: bulk delete channels by ID range or exact name |
| 5094eac1 | #6 | 2026-09-18 | feat: display upstream response model in channel tests |
| e6f7ea90 | #5 | 2026-09-18 | 移除大量无用组件 |
| 301fb3c2 | #4 | 2026-09-18 | 修复签到不显示ip |
| 632d1b52 | #3 | 2026-09-18 | feat: support timed user bans and automatic unban |
| 4db618c1 | #2 | 2026-09-18 | feat: require login password setup and enforce password strength |
| 8d7ee9a2 | #1 | 2026-09-18 | fix: remove legacy self-service password changes |
