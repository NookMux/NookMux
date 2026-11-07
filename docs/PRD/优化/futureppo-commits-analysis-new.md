# FuturePPO/new-api 项目提交分析（续）

## 项目概况

- **仓库**: https://github.com/futureppo/new-api
- **分析时间段**: 2026年8月21日 - 2026年9月18日
- **总提交数**: 53个提交（Futureppo 本人 44 个 + 外部贡献者 beicho 9 个，经 PR #3/#5/#6 由 Futureppo 审核合入）
- **承接范围**: 承接《futureppo-commits-analysis.md》（首批 #1～#117 与增#1～增#32，止于 `ef5e329d`），本批覆盖 `ef5e329d..8d7ee9a2`

### 本批主题概览

- **渠道集成（6 条线）**: Vercel AI Gateway、GMI Cloud（含音频/音乐任务与批量推理扩展）、VyceAI 生图、Modal（定时保活）、Kilo（匿名免费模型同步）、Mistral 原生推理 API 中继
- **协议兼容（5 个）**: DeepSeek 原生协议、OpenAI 官方渠道、Mistral OpenAI 兼容、Modal 请求透传、OpenRouter Alpha 模型识别
- **风控与安全（8 个）**: 签到防刷评分（PR #3）、错误详情仅 root 可见、邮箱大小写与歧义检查、域名邮箱注册码豁免拆分、16 位邀请码强制、定时封禁与自动解封、密码强度与强制设密
- **管理效率（4 个）**: 全站用户批量操作、按 ID 区间/精确名称批量删渠道、数据看板月/年粒度、渠道测试回显上游响应模型
- **定价与模型市场（2 个）**: 负数定价与异步任务延期结算、模型市场价格/名称排序
- **前端体验与视觉（含 PR #5/#6）**: 站点背景下载与跨域代理化、液态玻璃表面重写与 WebGL 渲染器、模型状态短窗口与服务端定时刷新、推理强度展示

## 编号说明

- 本文件独立编号 **#1～#53**，越新编号越小（#1 = 2026年9月18日 `8d7ee9a2`，#53 = 2026年8月21日 `20aa84fe`）。
- 外部贡献者 beicho 的 9 个提交按**合并日**归组，列于对应合并条目（#41、#44、#49）之后，便于与《futureppo-commits-by-feature.md》风格交叉核对。

## 更新记录

- 2026-09-18：拉取上游至 `8d7ee9a2`，新增 8月21日～9月18日 共 53 个提交（ef5e329d..8d7ee9a2）。

## 详细提交分析

### 2026年9月

#### 9月18日

1. **fix: remove legacy self-service password changes** (8d7ee9a2)
    - **安全加固**：`UpdateSelf`（PUT /api/user/self）不再接受 `password` 字段，登录密码只能走专用接口 `PUT /api/user/self/password`；拦截发生在处理偏好设置之前，且覆盖 JSON 大小写别名等混入写法，旧请求无法再借资料更新"改密成功"
    - 前端 `EditUserModal` 删除密码输入框，改由个人设置的专用改密对话框提交；保留资料修改时的原密码校验
    - 新增 `router/user_password_test.go` 集成测试，穷举 session/access-token × 弱密码/空密码/null/数字/别名键/混合偏好字段等组合，断言密码哈希与偏好均不被写入
    - 覆盖 5 个文件（+166/-29 行）

2. **feat: require login password setup and enforce password strength** (4db618c1)
    - **安全加固**：新增 `common/password_policy.go`，`ValidateLoginPassword` 要求 ≥8 字符（≤72 字节）且同时包含数字/大写/小写/符号；`GenerateLoginPassword` 用 crypto/rand 拒绝采样生成 16 位强密码，邮箱找回密码与初始化向导校验同步升级
    - **强制设置登录密码**：新增 `middleware/password.go` 的 `PasswordSetupForSession`，纯 OAuth 账号（无密码）访问 OAuth 绑定/回调等接口时返回 403 `PASSWORD_SETUP_REQUIRED`，仅放行查询自身与设置密码接口；`HasUserPassword` 直读数据库不信任缓存
    - 新增专用改密接口 `PUT /api/user/self/password`（拒绝 access-token 与跨会话调用），条件更新防并发首次设密互相覆盖、拒绝基于过期密码的修改
    - 前端新增 `PasswordSetupGuard` 挂载于 PageLayout，控制台渲染前先查 `has_password`，监听 403 事件引导设密；RegisterForm/SetupWizard/ChangePasswordModal 接入强度提示与新接口
    - 覆盖 45 个文件（+1236/-239 行），含 4 个新增 Go 测试文件，后端 3 个 yaml 与前端 7 个语言包同步

3. **feat: support timed user bans and automatic unban** (632d1b52)
    - **风控增强**：`User` 新增 `disable_duration_minutes`、`disable_until`（索引，Unix 秒避免三库日期函数差异）；`model/user_disable.go` 一次性计算截止时间保证批量操作同一 deadline
    - **自动解封调度**：新增 `service/user_disable.go` 的 `StartUserDisableExpiryTask`（仅主节点，30 秒 tick 批量到期解封）；鉴权路径按条件更新惰性解封，不会误清更新的封禁，解封后失效用户与令牌缓存并写系统日志
    - **全链路收敛**：ManageUser/批量用户操作/连锁封禁/GitHub 年龄风控等入口均支持 `duration_minutes`（拒绝显式 null 与负数）；封禁提示统一附原因及"永久/时长·解禁时间"后缀
    - 前端新增 `UserDisableInfo` 组件（时长输入校验与解禁时间文案），用户表格列、编辑用户与启用禁用弹窗、用量日志用户信息弹窗均接入
    - 覆盖 49 个文件（+1512/-344 行），新增 controller/middleware/model 四个 Go 测试文件（含三库迁移测试），多语言同步

4. **修复签到不显示ip** (301fb3c2)
    - 前端 `UsageLogsColumnDefs.jsx` 的 IP 列展示条件补充 `record.type === 4`（签到类日志），签到记录的 IP 恢复正常显示
    - 覆盖 1 个文件（+1 行）

5. **移除大量无用组件** (e6f7ea90)
    - **前端瘦身**：彻底移除"邮件日志"侧边栏模块——路由项、开关配置、Mail 图标、个人通知设置入口与 7 个语言包对应词条同步删除
    - `Enhancements` 页移除"IP 日志记录覆盖率"卡片（覆盖率统计与一键开启批量操作）及"AI 封禁/系统工具"分区元数据
    - 覆盖 13 个文件（+12/-155 行），纯前端清理

6. **feat: display upstream response model in channel tests** (5094eac1)
    - **渠道测试增强**：新增 `controller/channel_test_response.go`，通过 `ConversationCapture` 只捕获适配器实际消费的上游响应体，用 gjson 提取上游返回的 `model` 字段
    - `TestChannel` 响应新增 `upstream_model` 字段，前端 `ModelTestModal`/`useChannelsData` 展示上游实际响应模型，便于核对模型重定向/映射是否生效
    - 新增 `controller/channel_test_response_test.go`；覆盖 12 个文件（+370/-16 行），7 个语言包同步

#### 9月17日

7. **feat: bulk delete channels by ID range or exact name** (998f4b5a)
    - **管理效率**：新增管理员接口 `POST /api/channel/batch/range`（按 ID 区间删除）与 `POST /api/channel/batch/name`（按精确名称删除），删除后刷新渠道缓存并重置渠道 RPM 状态
    - **防误删设计**：区间删除先 Pluck 实际存在的 ID，稀疏区间不按区间大小分配；名称删除先用 name 索引 + `FOR UPDATE` 锁候选，再在 Go 内逐个精确比较（防数据库排序规则忽略大小写/重音/尾随空格），事务内按 200 分块删除并级联清理 abilities
    - 前端新增 `DeleteChannelRangeModal`、`DeleteChannelNameModal` 弹窗；`helpers/channelRange.js` 用纯文本正则校验，防止小数/超长 ID 被数字输入框舍入成另一个删除目标
    - 新增 controller/model 层 4 个 Go 测试文件与前端 `channelRange.test.js`；覆盖 24 个文件（+1297 行），7 个语言包同步

8. **fix: refresh model status on a server schedule** (0c4cb469)
    - **服务端定时刷新**：新增 `service/enhancement/model_status_task.go` 后台任务（每进程一个 goroutine，距上次完成超过配置间隔才重算，下限 60s、上限 24h），配置指纹变更即作废旧数据，失败也等待完整间隔避免重试风暴
    - **防访客触发扫描**：公开 embed 状态接口（单模型/批量/全部）改为只读进程内快照（附 `generated_at`/`ready`/`refresh_failed`），访客请求不再触发日志扫描
    - 移除统计槽位最小 5 分钟限制，前端"服务器统计周期"最小值改为 1 分钟
    - 覆盖 17 个文件（+576/-92 行），7 个语言包同步

9. **fix: respect Kilo authentication mode and reset channel form state** (b31cb54d)
    - **尊重显式认证模式**：`AddChannel` 不再"密钥为空即强制匿名"，仅在请求未显式携带 `kilo_anonymous_enabled` 时才回退为匿名；密钥模式下密钥为空直接报错
    - 前端 `EditChannelModal` 匿名开关默认值由 true 改为 false，切换渠道类型不再强制重置为匿名，关闭弹窗时补充重置多密钥模式状态
    - 含 `controller/channel_kilo_test.go` 测试与 `docs/channel/kilo.md` 文档更新；覆盖 4 个文件（+94/-10 行）

#### 9月16日

10. **feat: add short time windows to model status** (4fc34076)
    - **统计粒度增强**：模型状态时间窗口在 today/24h/7d/30d 基础上新增 0.5h/1h/6h/12h 四个短窗口，窗口常量、归一化与分钟换算链路同步扩展
    - 配置校验合法值集合与错误提示更新为全部 8 个窗口；前端 `Enhancements` 页窗口选项与文案同步
    - 覆盖 11 个文件（+133/-6 行），7 个语言包同步

11. **feat: display final upstream reasoning effort in usage logs** (a8ad0591)
    - **推理强度溯源**：新增 `relay/common/reasoning_effort.go`，在协议转换与参数覆盖完成后，从最终上游请求体按 gjson 路径（`reasoning_effort`、`reasoning.effort`、`output_config.effort`、Gemini `thinking_level` 等）提取显式思考强度；只记录不推断、不改写请求体，重试换渠道或覆盖删除字段时自动清空
    - 强度存入 `RelayInfo.ReasoningEffort` 并随日志 `other` 字段落库，用量日志模型列旁以紫色 Tag 展示（tooltip"思考强度"）
    - 新增三个测试文件；覆盖 19 个文件（+319 行），7 个语言包同步

#### 9月15日

12. **feat: add model marketplace price and name sorting** (55c095d5)
    - **模型市场排序**：新增 `web/src/helpers/modelPricing.js` 定义五种排序（默认/输入价格/输出价格/单次价格/模型名称），价格叠加当前分组倍率，无固定价格的模型置后，模型名用 `Intl.Collator` 排序且不原地修改源数组
    - 新增 `PricingSort.jsx` 排序字段下拉 + 升/降序按钮；`PricingTable` 在切片分页前先对全量数据排序（Semi 受控分页按远程数据处理）
    - 新增 `modelPricing.test.js`（216 行）覆盖排序取值与方向；覆盖 17 个文件（+676 行），7 个语言文件同步

13. **feat: add Kilo channel with anonymous free model sync** (4c9bcf05)
    - **渠道集成**：新增 Kilo 渠道类型（编号 70，基础地址 `https://api.kilo.ai/api/gateway`），复用 OpenAI API 类型但仅支持 chat completions；`kilo_anonymous_enabled` 开启时 `SetupRequestHeader` 直接删除 `Authorization` 头，实现免密钥匿名调用免费模型
    - **匿名免费模型同步**：新增 `controller/channel_kilo.go`，依据 `isFree` 标记筛选（缺失标记时报错而非静默全量），复用 OpenRouter 托管模型计划（别名冲突处理、名称简化、自动映射生成、手工映射保留），归属判定改用 Kilo 元数据与历史同步记录
    - Kilo 流式错误事件复用 DeepSeek 同款检测（终止流并抑制合成收尾帧）；前端 `EditChannelModal.jsx` 增加匿名与同步设置项（+170 行）
    - 新增 `channel_kilo_test.go`（208 行）等多层测试及 `docs/channel/kilo.md`；覆盖 28 个文件（+1034 行），7 个语言文件同步

14. **feat: support negative model pricing with save confirmation** (d727441b)
    - **定价能力**：后端 `service/quota.go` 新增 `roundSignedQuota` 允许负数费用结算——负费用精确保留小数精度，Realtime 负数（返还）在最终汇总一次性入账，预扣金额钳制为非负
    - **保存确认交互**：前端 `negativePricing.js` 按有效价格（按次价、倍率折算价、补全/缓存/音频价）识别负数项，`confirmNegativePricing.jsx` 弹窗列出负数项并警示净结算规则，保存前强制确认
    - **异步任务延期结算**：新增 task/midjourney 延期结算模型——任务按预估值预扣、终态按实际配额（可为负差额退款）事务结算，状态条件更新防止竞争轮询器重复入账，令牌已删除也不阻断退款
    - 用量日志与上游倍率同步页支持负数价格/退款展示；测试覆盖三库方言；覆盖 39 个文件（+1960 行），7 个语言文件同步

#### 9月14日

15. **feat: relay Mistral native inference APIs** (e19c8cbb)
    - **原生协议中继**：新增 `dto/mistral_native.go` 与 `RelayFormatMistralNative/MistralRealtime` 中继格式，原生请求**保留原始 body（含网关未知字段）**，仅在模型映射时改写路由标识；`IsNativePath` 覆盖 `/v1/ocr`、`/v1/fim/completions`、`/v1/agents/completions`、`/v1/audio/speech` 及实时转写路径
    - **实时转写 WebSocket**：`native_realtime.go` 建立客户端↔上游双向裸帧转发，观测 `transcription.done`/错误事件决定终止或失败，含超时与关闭握手
    - `relay/mistral_native_handler.go` 统一分发，响应头已写出后错误原样透传不再二次包装；`service/text_quota.go` 按原生响应观测到的 usage 计费
    - 新增 `native_test.go`（245 行）等多层测试；覆盖 22 个文件（+1266 行）

16. **fix: complete Mistral OpenAI API compatibility** (6ad174ef)
    - **协议兼容**：重构 `relay/channel/mistral/adaptor.go`——`NormalizeBaseURL` 同时接受根地址与 `/v1` 基址并严格校验，按中继模式映射 chat/embeddings/转写端点，不支持端点显式返回 400 而非 panic
    - **音频转写**：新增 `audio.go` 转换为 Mistral Voxtral multipart（language/temperature/diarize/context_bias/timestamp_granularities），支持 json/text/verbose_json/srt/vtt 且每次仅允许一种粒度
    - **流式细节**：新增 `stream.go` 处理 Mistral 特有行为——usage 与完整 tool call 挂在最后一个 choices 块上，usage 缺失时本地估算；新增 `response.go` 统一解析顶层错误封装（不回显可能含完整 prompt/音频的 `input` 字段）；新增 `embedding.go` 补齐向量接口与 token 用量
    - 渠道连通性测试改用真实 chat 请求；新增 261 行 handler 测试与 213 行 live 测试及 README；覆盖 21 个文件（+2021 行）

#### 9月13日

17. **fix: leave new JSON editor key-value pairs empty** (25068c42)
    - `JSONEditor.jsx` 的 `addKeyValue` 删除自动生成 `field_N` 键名的去重循环，新增键值对时键与值均留空，由用户自行输入，避免占位假键混入 JSON 配置
    - 覆盖 1 个文件（+1 行）

18. **fix: recognize OpenRouter alpha models from any provider** (d083133e)
    - `isOpenRouterManagedFreeOrAlphaModel` 不再要求 `openrouter/` 前缀，改为取最后一段路径判断 `-alpha` 后缀——任意提供商（含嵌套命名空间）的 Alpha 测试模型均纳入自动同步托管范围
    - 前端与 7 个语言文件同步更新说明文案；扩充嵌套命名空间测试用例；覆盖 11 个文件（+111 行）

19. **fix: complete official OpenAI channel compatibility** (95b0e782)
    - **协议兼容**：DTO 层补齐官方字段保留——`Message` 增加 `audio`/`refusal`，流式响应透传 `service_tier`/`obfuscation`，请求新增 `prompt_cache_options`、`moderation`；重写 Chat↔Responses 双向转换以保全字段
    - **Realtime GA 迁移**：新增 `realtime_ga.go` 全双工事件循环（读 goroutine 只读、事件循环独占会话与用量状态），`response.done` 即时结算、无最终用量时回退本地估算；官方渠道检测到旧 Beta 版本头直接报错引导迁移 GA
    - **流式媒体**：新增 `media_stream.go` 原样转发 SSE 帧（仅观察者解析 usage 记账）；官方图片生成/编辑与语音接口支持流式，媒体请求禁用提前 ping 以免吞掉上游校验错误
    - **计费归一**：`cache_write_tokens` 与旧字段视为别名（显式 0 也生效、绝不累加）；`web_search` 归一为内置工具计费类型；音频支持自定义音色对象
    - 新增多层官方渠道回归测试（HTTP/WebSocket）与 `docs/channel/openai.md`；覆盖 46 个文件（+2296 行）

#### 9月12日

20. **fix: complete DeepSeek native protocol compatibility** (8db3a5de)
    - **协议兼容**：新增 `BoolOrInt` 双类型承载 `logprobs`（chat 为布尔、旧补全为整数）；`StreamOptions` 改指针区分未传与显式 false；请求透传新增 `echo`、`user_id`
    - **端点路由**：重构 `deepseek/adaptor.go`——Claude 格式走原生 `/anthropic/v1/messages`，chat 请求发现消息 `prefix` 或工具 `strict` 时自动升级 `/beta/chat/completions`；V4 思考后缀在 OpenAI 与 Responses 两种格式下统一解析
    - **流式细节**：新增 `response.go` 处理原生 Anthropic 流——错误事件以 SSE 透传、`message_delta` 指针封装区分"缺省 usage"与"显式 0"；`stream_scanner.go` 在终止事件/客户端取消时主动关闭上游 body 解除 `Scan` 阻塞
    - DeepSeek `/responses` 请求允许仅有 `instructions` 而无 `input`；新增 290 行 adaptor 测试等多层测试及 `docs/channel/deepseek.md`；覆盖 39 个文件（+1310 行）

21. **fix: split email domain registration code exemptions** (14a24193)
    - **规则拆分**：单一域名白名单拆分为邀请码豁免与注册码豁免两个独立列表（仍要求不在域名黑名单），注册时仅在邮箱验证开启（即已验证邮箱所有权）后分别判定
    - **配置迁移**：新增 `model/option_email_domain.go`——加载选项前按旧值回填新键（旧开关为 true 时以旧白名单初始化两个新列表），显式保存过的新值始终优先
    - 前端 `RegisterForm.jsx` 在域名可能豁免时延迟客户端必填校验（交由服务端判定），设置界面重构为两组独立域名列表
    - 新增 `option_email_domain_test.go`（112 行）；覆盖 19 个文件（+505 行），7 个语言文件同步

#### 9月10日

22. **feat: complete Agnes text image and video integration** (eccfd97f)
    - **文本链路**：`agnes/adaptor.go` 重构为完整文本 adaptor，透传 OpenAI/Claude/Responses 三种请求格式，`NormalizeBaseURL` 兼容 `/v1` 后缀；AgnesAI 加入 streamSupportedChannels（实测支持 `stream_options.include_usage`）
    - **图片链路**：请求改为 `size`+`ratio`（校验 1:1/16:9 等 8 种）+`return_base64`，图片输入统一收敛进 `extra_body.image` 且仅接受 HTTP(S) URL 或 Data URI，计费固定按次收取；转换失败改为 400+skipRetry
    - **视频任务链路**：新增 `task/agnes/request.go` 统一校验（2.5 系列必选 mode、seconds 限 4–12、720P 限制并拒绝不支持的媒体字段），新增 `ValidateMappedRequest` 钩子在模型映射后、预扣费前校验
    - **任务查询与计费**：导出 `RefreshVideoTask` 复用 CAS 持久化与退款/结算路径（避免绕过计费）；ratio 设置保证老安装 reload 时补齐缺失的 `agnes-*` 默认价
    - 附带修复 logger 计数与轮转标志的数据竞争（atomic）；新增 7 个测试文件与 `docs/channel/agnes.md`；覆盖 26 个文件（+1877/-171 行）

#### 9月3日

23. **fix: preserve Modal OpenAI-compatible requests** (2a2d3859)
    - **协议兼容**：`ConvertOpenAIRequest` 识别到 `ChannelTypeModal` 时直接原样返回请求体，跳过 OpenAI 专用模型名启发式改写——Modal 自部署服务的模型名可能以 "o" 开头但并非 o 系列推理模型
    - 新增 `modal_test.go`（+84 行）验证 Modal 渠道请求不被改写、其它渠道行为不变；覆盖 2 个文件（+91 行）

24. **fix: invoke all Modal models for keepalive** (e4b9b2d2)
    - **保活机制修正**：保活请求从"GET /v1/models 拉列表"改为对渠道配置的**每个模型**逐一发送真实 `chat/completions` 推理请求（max_tokens=1），逐模型独立 60s 超时，失败用 `errors.Join` 汇总
    - `resolveModalKeepaliveModels` 基于模型列表解析 `model_mapping`（带环检测与去重），无配置模型时保活直接报错；前端与 7 语言同步更新"定时测活"说明（会产生实际推理用量）
    - 覆盖 10 个文件（+127/-28 行）

25. **feat: add Modal channel with scheduled keepalive** (029ab7c1)
    - **渠道集成**：注册第 69 号渠道类型 Modal（API 类型 OpenAI）；`NormalizeBaseURL` 同时接受部署 origin 与 Modal curl 示例中的完整 `/v1/chat/completions` 地址并归一化，无内置模型目录（模型手填或从部署获取）
    - **定时保活**：新增 `controller/modal_keepalive.go`——仅主节点启动、防重入，按渠道 `modal_keepalive_interval_seconds`（默认 30s）判定到期；先记 lastAttempt 再并发发起请求，防止慢部署导致请求堆积
    - 前端 `EditChannelModal.jsx`（+136）新增 Modal 类型（密钥格式提示与"定时测活/测活间隔"设置项），渠道常量与徽标同步
    - 新增 5 组测试；覆盖 25 个文件（+727/-10 行），前端 7 语言文件同步

26. **feat: enforce 16-character invite codes** (6fcfc9b2)
    - **邀请码格式**：新增 `common/invite_code.go`——16 位长度、字母表剔除易混淆的 I/O/0/1，`crypto/rand` 拒绝采样生成，`NormalizeInviteCode`（trim+大写）归一化与 `IsValidInviteCode` 严格校验
    - **生成与唯一性**：`GenerateUniqueInviteCode`（最多 128 次重试查库去重）；注册插入与根账号初始化全部改用 16 位码，查询邀请码时把空码/旧 4 位码升级
    - **存量迁移**：以 option 键为幂等标记，事务化地把所有存量用户 aff_code 重生成为唯一 16 位码；`GetUserIdByAffCode` 对非法格式码按"不存在"处理
    - 前端 `RegisterForm.jsx` 的 `?aff=` 参数、localStorage 与输入框统一 trim+转大写，输入框加 `maxLength=16`；新增 184 行模型测试；覆盖 11 个文件（+493/-25 行）

#### 9月1日

27. **feat: support domain email code-free registration** (85781f43)
    - **注册风控**：新增 `common/email_domain.go`——黑名单优先级高于白名单，支持 `*.edu.cn` 式通配；已验证域名邮箱（命中免码白名单）可同时豁免邀请码与注册码
    - `Register` 在邮箱验证通过后检查黑名单并放行免码；`EmailBind` 绑定邮箱同样拦截黑名单；`/api/status` 暴露开关字段，发码前拦截黑名单域名
    - 前端将"邮箱域名白名单"升级为"配置邮箱域名注册"（注册白名单/免码白名单/黑名单三套 TagInput 共用校验），注册表单跳过必填校验
    - 新增 `email_domain_test.go` 等测试；覆盖 21 个文件（+606/-64 行），后端 3 语言与前端 7 语言同步

### 2026年8月

#### 8月29日

28. **feat: add global user batch operations** (0c80da7d)
    - **管理效率**：新增全站用户批量管理接口 `POST /api/user/batch-manage`（管理员权限），支持 5 种动作——启用全部已禁用普通用户（并清空禁用原因）、禁用全部已启用用户（必填统一原因 ≤255 字符）、软删除、永久删除已禁用、清理已注销用户
    - 后端 `service/enhancement/actions.go` 事务内按角色与状态取 ID 后以 500/批分块执行，完成后逐用户失效用户与令牌缓存并写审计（禁用动作记录原因）
    - 前端 `UsersActions.jsx` 将原单一"清理已注销用户"按钮扩展为批量操作下拉 + 确认弹窗（危险操作红色警示、禁用动作强制填原因），按动作提示影响人数后刷新列表
    - 7 种前端语言各新增 22 条文案；扩充单测（+122 行）；覆盖 14 个文件（+711 行）

#### 8月28日

29. **feat: 数据看板增加月年时间粒度** (e6a06fec)
    - **看板增强**：聚合粒度新增 `month`、`year`；引入日历桶机制——数据点归入自然月/自然年起始，RPM/TPM 换算与补点改用逐点 interval，避免月/年天数不一导致的速率失真
    - 轴标签按粒度输出 `YYYY-MM`/`YYYY`；设置页"聚合粒度"改用共享 `TIME_OPTIONS` 常量
    - 纯前端改动；`zh-TW.json` 补齐"月/年"两条繁体译文；覆盖 6 个文件（+90 行）

#### 8月26日

30. **新增 跨域下载图片 相关汉化** (f561dfb0)
    - 纯 i18n 提交：背景图片下载失败文案统一改写（后端代理化后不再依赖图片源跨域许可），7 个语言文件同步
    - 覆盖 7 个文件（+7 行）

31. **修复跨域图片背景无法下载问题** (255cab29)
    - **架构修复**：新增公开接口 `GET /api/site-background/image?source=<index>`，由服务端按配置源代拉图片后流式返回（带 `no-store`/`nosniff` 头），前端不再直连第三方图片，彻底绕开浏览器 CORS 限制
    - 新增 `service/site_background.go`（183 行）：URL 校验（无凭据 HTTP(S)、相对路径基于 ServerAddress 解析）、支持直连图片源与 JSON API 源（按 `json_path` 逐段解析图片地址）、按 `MaxFileDownloadMB` 限额读取，具备基础 SSRF 防护
    - 前端为每个源标注 `source_index`：同源图片仍直连，跨域源改走代理接口并转 blob 预加载
    - 含后端单测（121 行）；覆盖 10 个文件（+507 行）

32. **新部署推薦優先使用 PostgreSQL** (ecd5dff7)
    - 纯文档提交：5 份 README（en/zh_CN/zh_TW/ja/fr）部署要求后新增"新部署推荐优先使用 PostgreSQL"提示
    - 覆盖 5 个文件（+10 行）

33. **修改镜像下载地址为当前项目** (0eaef5b8)
    - **元数据调整**：5 份 README、`docker-compose.yml` 与 `docs/installation/BT.md` 的 Docker 镜像地址从上游统一替换为 `ghcr.io/futureppo/new-api:latest`
    - 覆盖 7 个文件（+33 行）

34. **支持 下载背景** (ca22fc71)
    - **功能增强**：顶栏用户下拉菜单新增「下载背景」入口——前端 fetch 背景 URL 转 blob 经 `<a download>` 触发保存（按 content-type 推断扩展名），含禁用态提示与下载中 loading
    - 7 种语言各新增 4 条文案；该直连实现依赖图片源允许跨域，即后续后端代理化（#31）的动因
    - 覆盖 13 个文件（+156 行）

35. **修复渠道 VyceAI 502问题** (473f6300)
    - **错误映射修复**：消费上游 SSE error 事件时识别安全拦截类文案（safety filter/content policy/prompt blocked），包装为 `promptBlockedError` 并映射为 HTTP 403（带 skip-retry，不再触发无意义重试）；其余上游错误保持 502
    - 新增两个用例分别验证安全拦截→403/skip-retry 与普通错误→502；覆盖 2 个文件（+60 行）

36. **新增 VyceAI OpenAI 生图渠道** (267e21f7)
    - **渠道集成**：新增渠道类型 VyceAI（68，OpenAI 生图端点），`adaptor.go` 将生图请求转换为上游 `/v1/images/stream` 的 SSE 请求（按模型名映射 `aspectRatio`，上游模型/尺寸与 `enableNsfw` 为硬编码常量，计费固定按次收取），`response.go` 消费事件流聚合为 OpenAI 同步图片响应（含 base64 解码、64MB 单事件上限）
    - 注册链路完整：api_type/endpoint_type/channel/task 常量、adaptor 工厂、图片处理器与前端渠道选项同步
    - **顺带扩展**：补齐 gmicloud 批量推理任务——新增 `/v1/batch/generations` 提交与查询路由、新端点类型；渠道测试按渠道/模型自动选择生图或批量任务端点
    - 新增 9 个测试文件；7 种语言 i18n 同步；覆盖 47 个文件（+1138 行）

#### 8月25日

37. **适配 gmicloud 渠道的音频音乐模型** (732ed59e)
    - **渠道集成**：新建 `relay/channel/task/gmicloud/adaptor.go`（312 行）——提交 `/v1/audio/generations`、`/v1/music/generations` 任务，按 payload 是否含 `lyrics`/`source_audio` 与模型名单自动判定动作（音频生成/音乐生成/voice_clone），查询聚合音频地址
    - 新增 `router/audio-task-router.go`，relay_mode 增加音频任务提交/查询；`distributor.go` 识别新路径且 GET 查询不选渠道
    - 模型拉取合并 OpenAI 兼容模型与任务端点返回的 `model_ids`（经 `IsAudioModel` 过滤），支持控制台一键拉取；前端任务日志扩展音频试听弹窗
    - 含 adaptor（+204 行）/relay_mode/渠道测试；覆盖 16 个文件（+799 行）

38. **新增 gmicloud 渠道** (f429081b)
    - **渠道集成**：新增渠道类型 GMICloud（67）"GMI Cloud"，基址 `https://api.gmi-serving.com`，复用 OpenAI API 类型与端点；维护 MiniMax 系列 ModelList 并在 `openai/adaptor.go` 按渠道类型切换
    - 前端注册选项并纳入支持模型拉取的渠道集合；含 4 组单测；覆盖 11 个文件（+246 行）

39. **新增vercel渠道** (242aae56)
    - **渠道集成**：新增渠道类型 Vercel（66）"Vercel AI Gateway"，基址 `https://ai-gateway.vercel.sh`，复用 OpenAI API 类型，端点支持 Chat Completions 与 OpenAI Responses
    - 新建 `relay/channel/vercel` 包（ModelList 含 minimax 免费模型等）并在 `openai/adaptor.go` 中分发；含 4 组单测（覆盖请求 URL 构造与请求头）；覆盖 11 个文件（+291 行）

40. **feat: Enhance email identity handling with case insensitivity and ambiguity checks** (8807b946)
    - **风控增强**：新增 `EmailCaseInsensitiveEnabled` 开关（默认开启、后台可配），`NormalizeEmailIdentity`（trim + lowercase）作为比较与验证码缓存键，数据库存储与展示保留原始大小写
    - **消歧机制**：大小写变体命中多个用户时返回 `ErrEmailIdentityAmbiguous` 而非擅自选定账号；登录改为先精确匹配用户名、未命中才按邮箱身份回退
    - **全链路接入**：注册验证码 key 归一化、找回密码按唯一用户 ID 更新、OAuth 注册拦截已占用邮箱、Creem 充值回填邮箱前查重；前端系统设置页增加开关
    - 新增 3 个测试文件（约 300 行）；覆盖 24 个文件（+553 行），后端 i18n 与前端 7 语言同步

#### 8月24日

41. **Merge pull request #5 from Beicho/fix/settings-boolean-sync** (2e11b1ee)
    - **合并外部 PR #5**：修复签到设置开关"显示已开启、实际未生效"的父子组件配置键同步缺陷，含 1 个提交，作者 beicho（外部贡献者）

42. **fix(checkin): 补齐设置页父组件缺失的签到配置键，修复开关显示为已开启** (1a4492bb)
    - **前端修复**：三个新签到配置键（当日有效开关/回收方式/客户端环境检查）只加入 `SettingsCheckin` 的 defaultInputs，未同步到父组件 `OperationSetting` 的 inputs 初始状态——键缺失时字符串 "false" 原样存入，非空字符串为 truthy 导致开关渲染成已开启而功能实际关闭，且严格比较判定无改动、保存无从下手
    - 仅补 3 行默认键（已核对其余设置页仅此三键父子不同步）；覆盖 1 个文件（+3 行）；外部贡献者 beicho 提交，经 PR #5 合入

43. **fix: restrict error details to root users** (c9ae7f35)
    - **安全收紧**：错误详情仅对 root 用户可见——任务失败原因、上游错误码、结果附件等对非 root 一律脱敏为占位标识
    - `model/log.go` 新增 `SanitizeLogsForNonRoot`（从 other 字段归一提取错误信息）并在用户日志格式化中生效；midjourney/task 控制器的可见性判定从"渠道错误详情可见性"改为"查看者角色"，失败任务连带清空状态与资源地址
    - `relay/relay_task.go` 在隐藏详情时连带清空 Properties 与失败任务 ResultURL，防止旁路泄露
    - 更新 3 个测试；覆盖 8 个文件（+195 行）

44. **Merge pull request #6 from Beicho/feat/glass-surface** (fc424100)
    - **合并外部 PR #6**：液态玻璃表面重写主题——去磨砂塑料观感、提升文字可读性、折射改可选增强、新增 WebGL 渲染器，含 4 个提交，作者 beicho（外部贡献者）
    - 覆盖后端 `setting/system_setting/site_background.go` 与前端玻璃渲染相关组件，合计 10 个文件（+1692 行）

45. **feat(site-background): 新增 WebGL 玻璃渲染器，折射、色散与边缘光走着色器** (bdf18112)
    - **前端视觉**：新增 WebGL 渲染路径，绕开 CSS 折射滤镜整层掉出 GPU 合成路径的性能问题——背景静态图预烘焙模糊（逐级降采样至 1/8 + 分离高斯），玻璃矩形合批，每帧 2 个 draw call，静止时不画
    - 着色器光学配方：位移沿圆角矩形 SDF 内法线向心采样、色散按折射偏移反向错开 R/B 通道、边缘光用 SDF 梯度做法线做 Blinn-Phong 并叠内描边，折射带内按位移强度还原背景清晰度
    - 新增 `siteBackgroundGlassRenderer.js`（830 行）与 `SiteBackgroundGlassCanvas.jsx`，接入双路径切换；WebGL 不支持、上下文丢失或图源跨域受限时自动退回 CSS 路径
    - 后端新增 `glass_renderer`（css/webgl，默认 css，存量配置不受影响）与边缘清晰度/色散/边缘光三个参数（含兼容与测试）；覆盖 8 个文件（+1201 行）；外部贡献者 beicho 提交，经 PR #6 合入

46. **feat(site-background): 折射作为可选增强项加回，默认关闭** (af436d06)
    - **前端视觉**：以 SVG `feDisplacementMap` 滤镜重新实现边缘折射与色散，但默认强度为 0——backdrop-filter 引用 url() 滤镜会让玻璃层脱离 GPU 加速改由 CPU 逐帧计算（代价与强度无关），需管理员显式开启
    - 光学映射整体调柔（满档位移 0.6→0.22、色散 0.26→0.09）；位移贴图 R/G 通道分别编码横/纵位移并以 screen 叠加合并为一张图；折射只作用于 `.semi-card`（避免顶栏极端宽高比糊掉内容）；不支持时经 -webkit- 前缀退回无折射版本
    - 新增 `SiteBackgroundGlassFilter.jsx`（217 行，站点本体与预览使用不同滤镜 id 避免冲突）；覆盖 7 个文件（+361 行）；外部贡献者 beicho 提交，经 PR #6 合入

47. **fix(site-background): 提高玻璃上的文字可读性** (9a1a6155)
    - **可读性修复**：实测各档参数下文字对比度均已超 11:1（远超 WCAG AAA），真正干扰是背景高频细节与笔画竞争，调整方向改为销毁背景细节：blur 6px→26px、saturate 165%→115%、新增 contrast 88%、移动端模糊 12px→18px
    - 以模型广场正文区域度量：背景亮度标准差降低 65%，文字最差对比度维持 12.5；仅改 `index.css`；覆盖 1 个文件（+20 行）；外部贡献者 beicho 提交，经 PR #6 合入

48. **feat(site-background): 重写液态玻璃表面，去掉磨砂塑料观感** (69f947f6)
    - **前端视觉**：定位磨砂塑料感五个根因（单一平铺色底、四边等亮描边、仅顶部一条硬线、backdrop-filter 只有 blur+saturate、单层极淡阴影），改为厚度梯度底色、方向性 inset 亮边、折射暗带、内缘聚光、镜面反射层与"接触阴影 + 环境投影"双层阴影
    - 边缘各层光学量刻意收细（亮边 1.5px、内缘聚光 14px/-8px），防止加宽后沿四周糊出白雾、填平圆角曲线；暗色主题单独处理（镜面反射保持亮边强度、折射暗带用真暗色）
    - 仅改 `index.css`，不新增配置项、glass_opacity 语义不变；覆盖 1 个文件（+138 行）；外部贡献者 beicho 提交，经 PR #6 合入

#### 8月21日

49. **Merge pull request #3 from Beicho/feat/checkin-client-env-scoring** (1388b9b9)
    - **合并外部 PR #3**：签到防刷风控主题——对脚本与无头浏览器签到"压低奖励而非拦截"（请求头环境分 + 历史行为评分），并附带签到额度当日有效机制，含 4 个提交，作者 beicho（外部贡献者）
    - 后端改动集中在 `model/checkin.go`（发放台账与清算）、`service/checkin_client_score.go`、`service/checkin_behavior_score.go`、`service/checkin_expiry_task.go` 与签到设置；三份新增测试文件随分支合入，合计 11 个文件（+1340 行）

50. **feat(checkin): 增加行为特征评分，覆盖无头浏览器场景** (daf365fc)
    - **风控增强**：客户端可信度改为"请求头特征（上限 55）+ 历史行为特征（上限 45）"，补齐 Playwright/Puppeteer 等无头浏览器请求头与真人完全一致的识别盲区
    - 行为特征三项：签到时刻离散度 25 分（用圆周标准差，避免 23:59 与 00:01 跨零点样本被线性方差误判为人类作息）、掐点触发 10 分（距本地零点秒数）、消费关联 10 分（只签到从不消费正是要抑制的模式）
    - 误伤保护：历史样本少于 3 次一律满分、账号不足 7 天不参与消费关联、查询出错按满分；全部线性无阈值映射，无可被二分定位的跳变点
    - 测试大幅扩充（含平滑无跳变用例），覆盖 5 个文件（+351 行）；提交信息含 Co-Authored-By: Claude Opus 5 (1M context) 尾注；外部贡献者 beicho 提交，经 PR #3 合入

51. **feat(checkin): 对非浏览器环境的签到压低奖励而非拦截** (b310c940)
    - **风控增强**：新增 `client_check_enabled` 开关（默认关闭），按 9 个小权重请求头信号累加出 0-100 环境分，实发 = MinQuota + (应得 − MinQuota) × score/100，特殊星期固定大额同样被压制
    - 取"压低而非拒绝"的设计：拒绝会给出可被二分测试利用的成败信号，压低后响应与一次运气差的正常签到完全一致、公示区间自洽；评分细节只写服务端日志供管理员排查
    - 信号覆盖 Sec-Fetch 三件套、Accept-Language、br/zstd 压缩协商、Sec-Ch-Ua、Origin/Referer、形似浏览器的 UA（对照 20+ 脚本 UA 特征表）与非 `*/*` 的 Accept
    - 新增 `checkin_client_score_test.go`（137 行）与设置测试；覆盖 7 个文件（+359 行）；提交信息含 Co-Authored-By: Claude Opus 5 (1M context) 尾注；外部贡献者 beicho 提交，经 PR #3 合入

52. **test(checkin): 覆盖特殊星期奖励与当日有效的组合场景** (5ff2ec84)
    - **测试补强**：端到端用例走真实签到路径获得特殊星期固定大额（单日发放可达平日 6-7 倍），跨天后仅回收未消耗部分，覆盖本仓库特有功能与当日有效的组合场景
    - 仅扩充 `checkin_expiry_test.go`；覆盖 1 个文件（+43 行）；提交信息含 Co-Authored-By: Claude Opus 5 (1M context) 尾注；外部贡献者 beicho 提交，经 PR #3 合入

53. **feat(checkin): 支持签到额度当日有效，次日清算回收** (20aa84fe)
    - **核心功能**：可选"签到额度当日有效"机制——`expire_enabled`（默认关闭，存量站点行为不变）+ `expire_mode`（unused=仅回收当日未消耗部分 / all=次日全额回收）
    - 因 `users.quota` 是单一标量无法区分额度来源，复用 `checkins` 表作发放台账，新增 `expired_quota`/`settled_at` 两列按天结算、天然防重复回收；回收走相对更新 + 余额守卫，永不扣成负数
    - 新增 `service/checkin_expiry_task.go` 清算任务（仅主节点、sync.Once 防重复）；超 3 天的历史积压只标记不追溯回收，避免用户余额无预警缩水
    - 前端新增"签到额度有效期"开关与回收方式下拉；新增 222 行模型测试；覆盖 7 个文件（+629 行）；提交信息含 Co-Authored-By: Claude Opus 5 (1M context) 尾注；外部贡献者 beicho 提交，经 PR #3 合入
