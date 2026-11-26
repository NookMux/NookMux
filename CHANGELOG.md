# Changelog

本文件记录项目的所有重要变更。

格式基于 [Keep a Changelog](https://keepachangelog.com/zh-CN/1.1.0/)，
版本号遵循 [语义化版本](https://semver.org/lang/zh-CN/)。

## [Unreleased]

### Changed

- **系统设置全部页面统一为「分区 + 行」结构**：7 个分组下的设置页按同一套页面结构重排——随「保存」生效的设置写成「分区 + 行」，行与行之间只用细横线分隔，带边框的面板只用于不随「保存」逐项生效的内容（立即执行的运维操作、只读监控数据、危险操作、自带列表或编辑区的编辑器）；行的标题与控件同处一条横带，是否分成标题、控件两列由所在容器宽度决定，对话框等窄容器自动单列，页面宽度跟随主题的内容布局设置；只放一个开关的行两端对齐，文字在左、开关贴行的右边缘，分区标题上的总开关同样贴右端。
  - **新增设置页基础组件**（`web/src/features/system-settings/components/`）：`SettingsGroup`（分区，支持分组级开关与从属行：开关关闭时从属行整块禁用；表单内绑定布尔字段，页面自管状态时传 `checked` / `onCheckedChange`）、`SettingsRow` / `SettingsControl`（行与控件宽度档，含通栏行）、`SettingsPanel` 与 `SettingsStatGrid` / `SettingsStat` / `SettingsMeter`（面板与只读指标块）、`SettingsListPanel`（列表面板）、`SettingsNotice`（无框提示）、`EditorModeButton`（可视化 / JSON 切换）。
  - **编辑器与列表页**：定价、倍率、限额、支付等「可视化 / JSON」编辑器的切换按钮与工具栏统一（搜索靠左、添加靠右），编辑器内表格贴边；公告、API 地址、常见问题、Uptime Kuma 分组等列表编辑页统一为页头「重置 / 保存」+ 列表面板（标题行「添加」，勾选后底部出现「已选 n 项 / 删除所选」）；定价页左右两栏按容器宽度切换，列表的名称列占满剩余宽度，编辑器表格的输入框列在窄容器下不再被挤压，只读指标网格的列数按容器宽度切换。
  - **页头、页签与提示**：页头操作栏统一为 `SettingsPageFormActions`（无未保存改动时「重置」「保存」禁用，「恢复默认」式页面可随时重置，有改动时页面标题旁显示「未保存的更改」标记；标题与操作栏放不下同一行时操作栏换到标题下一行，标题不再被操作栏挤压截断）；页签统一为 `TabsList variant='line'`；设置页提示统一为无框的 `SettingsNotice`。
  - 设计规范同步：`DESIGN.md` 新增设置页组成规则，并更新字号与卡片规范中的设置页条目。
  - **抽检调整**：设置行里的输入框（文本、数字、带单位的数值）占满控件列，下拉保持紧凑；性能页的「性能监控」与渠道亲和页的「缓存」两块只读数据面板移到设置之前；监控页归为「自动检查」与「额度提醒」两个分区；只有说明和一个操作的危险面板并为一行；模型价格的设置面板只让档位列表滚动；定价页左右两栏分两栏、占满剩余高度与列表占满用同一个宽度条件，编辑器与下方的危险面板之间留 2rem。
  - **工具价格页签改为左右分栏**：与模型价格页签一致，左侧列表显示规则名称、工具类型与价格，右侧编辑选中规则的 ID、名称、工具类型、价格、启用与条件；右侧改名后左侧同步，新增规则后自动选中并把光标放进 ID 输入框，删除规则后选中相邻规则。
- **「模型定价」更名为「价格设置」，分组价格并入同一页面**：页面包含「模型价格」「分组价格」「工具价格」三个页签，侧边栏不再有「分组定价」，旧地址 `/system-settings/billing/group-pricing` 自动转到该页面；页签面板保持挂载，切换页签不再丢失未保存的改动；页头操作栏在窄屏下放不下时自动折行；工具价格右侧的设置面板改为标签在左、控件在右，启用开关移到面板标题行右侧。
- **工具价格右侧面板只滚动条件列表**：ID、名称、工具类型、价格与「添加条件」按钮保持不动，条件多时只有条件列表自己滚动，与模型价格的阶梯档位一致；新增条件后自动滚到新条件。
- **内置模型价格按厂商官方价补齐**：按 OpenAI、Claude、智谱 GLM、DeepSeek、Kimi、豆包、Gemini、阿里云百炼 Qwen、MiniMax 九家厂商的第一方官方标准美元价更新内置倍率表，并用 OpenRouter 第一方端点价交叉核对；Batch、Fast、Flex 等服务档价与缓存存储费等平台不保存的价格维度不导入，厂商不收缓存写入费的型号不设缓存写入倍率（`internal/config/ratio/model_ratio.go`、`internal/config/ratio/cache_ratio.go`）。
  - **新增在售型号**：Claude Fable / Mythos / Opus / Sonnet / Haiku 5 系列，智谱 GLM 5 至 5.3 系列（含 `glm-5v-turbo`、`glm-ocr`），DeepSeek `deepseek-flash`、`deepseek-v4-flash`、`deepseek-v4-pro`，Gemini 3.7 / 3.8 Flash、Gemini 4 系列、Embedding 2 与 Robotics ER 2，Kimi K3、K3.1-pro、K2.7-code，MiniMax M3 与 M3.1-Flash，豆包 Seed 2.0 / 2.1（ModelArk 国际站型号 ID），OpenAI GPT-5.4 快照、GPT-5.5、GPT-5.6、GPT-6 全系、GPT-Image 2.x、RealTime、Audio 1.5、文本向量与审核模型，以及 Qwen 的百炼型号（`qwen4-max`、`qwen3.8-max`、`qwen3.7-plus`、`qwen3.7-flash`、`qwen3-coder-flash` 等，取百炼国际站新加坡地域牌价）与 `qwen/` 前缀的 OpenRouter 型号。
  - **同步厂商现行价**：`gpt-5.6-terra`、`gpt-5.6-terra-pro` 输入价 $2 / 1M tokens，`gpt-5.6-luna`、`gpt-5.6-luna-pro` 输入价 $0.20；`glm-4.7-flash`、`glm-4.6v-flash`、`glm-4.5-flash` 为免费模型；`gpt-audio-mini` 系列音频输入 / 输出价 $10 / $20；豆包 `seed-2.0-lite`、`seed-2.0-mini` 缓存命中按输入价的 20% 计价；GLM、DeepSeek、Kimi、Gemini、Qwen 其余在售型号一并按现行价更新。
  - **长上下文阶梯计费**：补充 Claude Sonnet 4.5（超过 200K tokens）与 Haiku 5.5（超过 100K tokens）、OpenAI GPT-5.4 及之后的模型（超过 272K tokens）、MiniMax M3（超过 512K tokens）、豆包 Seed 2.0 / 2.1（超过 128K tokens）、Qwen Plus / Flash / Coder / VL（超过 32K、128K 或 256K tokens）的分段价格，单个型号最多四档（`internal/config/ratio/context_pricing.go`）。
  - 新增按厂商官方价核对内置表、按档位边界核对阶梯价格的单元测试（`internal/config/ratio/builtin_pricing_test.go`）。
  - Mintlify 中英文档同步：内置定价接口参考页与定价接口页注明档位数量不固定（内置规则两到四档）、内置档位的命名顺序与档位上下界的开闭区间。
- **渠道默认模型列表同步厂商在售型号**：按 OpenAI、Claude、Gemini、智谱 GLM、DeepSeek、Kimi、豆包、MiniMax 八家厂商官方文档的在售型号，把内置价格表已配价的型号补进各渠道适配器的默认模型列表；该列表是渠道「获取模型列表」不可用时控制台的型号来源（`internal/relay/channel/*/constant*.go`）。
  - **新增型号**：OpenAI 25 个（`chat-latest`、GPT-5.4 mini 与 GPT-5.5 系列快照、GPT-6 全系、GPT-Image 2.5 / 2.6、RealTime 1.5 / 2 / 2.1、Audio 1.5、审核与文本向量模型），Claude 9 个（Fable 5.1 / 5.2、Mythos 5 系列、Opus 5 / 5.5、Sonnet 5.5、Haiku 5.5），Gemini 6 个（3.7 Flash、4 carbon / hydrogen / neon、Robotics ER 2 两个预览版），智谱 GLM 10 个（GLM 5.3 系列、`glm-ocr`、`glm-4.7-flashx`、`glm-4.6v-flash`、`glm-4.6v-flashx`、`glm-4.5-airx`、`glm-4.5-flash`），DeepSeek `deepseek-flash`，Kimi `kimi-k3.1-pro`、`kimi-k3.1-pro-highspeed`，MiniMax `MiniMax-M3`、`MiniMax-M3.1-Flash`、`minimax-m3.1-flash`。
  - **豆包渠道默认列表**：由空列表改为 BytePlus ModelArk（国际站）在售且已配价的 Seed 2.0 / 2.1 型号 ID（5 个）；按账号创建的推理端点 ID、国内站 Model ID 与其他厂商模型仍由管理员在渠道上手动填写。
  - Vertex AI 渠道的默认列表由 Vertex、Claude、Gemini 三份默认列表合并而成，随以上新增同步扩大。
  - 新增单元测试，逐个渠道核对厂商在售型号同时存在于默认模型列表与内置价格表。
- **全量国际化（i18n）深度校对、术语对齐与双语润色**：
  - **产品术语口径收敛**：统一前后端中英文核心产品术语与命名，包括模型广场（`Model Hub`）、预设组合、厂商（模型研发方）与供应商（上游渠道方）区分、排行榜 `Agent` 与 `This Week / This Month / This Year`、英文充值统一 `Top-up`、兑换码体系替代 CDK、钱包推广体系、账户剩余余额、阶梯计费与档位、上下文分段计费、缓存写入倍率（`5m / 1h`）、2FA 恢复码、退出登录、系统公告、完整布局、常见问题、SSRF 防护、`Fix Channel Abilities`、`OpenAI 协议格式`、Claude beta 查询参数、渠道代理协议列表（HTTP/HTTPS/SOCKS5/SOCKS5H）、系统初始化向导、定制音色已创建状态、`Vocal expression tags`、支持的图像生成模型及硅基流动中文品牌名。
  - **后端多语言字典（`internal/i18n/locales/{zh,en}.yaml`）**：逐条校对全部 713 项中英文 API 响应与任务日志消息，修复语义偏差与歧义（邮箱域名白名单提示、Responses API 请求类型、工具调用测试、分发器无可用渠道、渠道亲和性规则缓存提示、数据库迁移强制覆盖开关名称对齐前端实际标签等），修正中式英语与中英文标点排版（47 处中文半角冒号、括号、空格及未译标签），统一术语（「额度」「封禁」「分组」「密钥」「告警」「身份验证器」等）。
  - **前端多语言字典（`web/src/i18n/locales/{en,zh}/*.json`）**：逐条校对全部 28 个功能分区共 4,494 项中英文文案并结合组件调用点验证，修复多处误译、逻辑/边界偏差与拼接错误（如动态倍率创建提示误译为「创建时间」、7 天星期误译为 `Weekdays`、分页/日期筛选共用键「本月获得」、两步验证步骤拼接「共 3 个」、模型详情代码提示重复「替换」、自动分组与预消耗额度描述与代码逻辑不符、`less than N` 与 Zod `.max(N)` 闭区间边界偏差、普通文本中裸露 Markdown 反引号等），统一多密钥轮询（`Round-robin`）、重试次数（`Retry Count`）、定制音色、数据看板、许可协议等中英文领域术语，并将 `Chat Completions`、`Secret Key` 纳入 `sync-i18n.mjs` 专有名词白名单。
- **全量国际化（i18n）翻译核查、术语统一与前后端本地化完善**：
  - **后端接口本地化（`internal/i18n` 与控制层/领域层）**：补全审计模块名称、用户分组（`auto` 倍率及内置 `default` / `vip` 描述）、自助日志过滤拒签与详情开关提示、每日签到及 Turnstile 校验、工单管理全流程、`db_migrate` 数据库迁移任务日志与错误、预置模型信息及模块禁用提示等随 `Accept-Language` 本地化；规范 `*i18n.Error` 传递与 `ResolveContextError` 映射，统一 `zh.yaml` / `en.yaml` 中英文文案及标点排版。
  - **前端多语言文案与术语规范（`web/src/i18n`）**：全面校对并润色 14 个命名空间的 `en/*.json` 与 `zh/*.json` 译文，统一中文界面术语（「渠道」「邮箱」「数据看板」「额度」「划转」「封禁」「兑换码」「定制音色」「请求头」「告警」「倍率」「分组」「密钥」「撤销」）及全角标点/盘古之白，保留技术专有名词与渠道原名（`API Key`、`Playground`、`Passkey`、`Access Token`、`Base URL`、`Site Key`、`Webhook` 等），并清理 `static-keys.ts` 中的重复及失效键引用。

### Fixed

- **gpt-5.4 及之后型号的补全倍率表条目不生效**：补全倍率对 `gpt-5` 前缀的型号恒定按输入价的 8 倍推导并跳过倍率表，表中 `gpt-5.4-mini`、`gpt-5.4-nano`、`gpt-5.5-pro` 等型号写明的输出价不参与计费。现在表里有该型号的精确条目时以条目为准，没有条目时才按输入价的 8 倍推导（`internal/config/ratio/model_ratio.go`）。
- **发版后旧标签页进入新页面直接显示 500**：内嵌 `dist` 中不存在的构建产物路径（`/static/*`）不再回退为首页 HTML，改为返回 JSON 404；旧标签页请求已删除的分块时由前端据此在同一标签页内自动刷新拉取新版本，刷新生效前不显示错误内容，也不再把首页 HTML 当脚本执行后报错。不回退为前端页面的路径（`/v1`、`/api`、`/assets`、`/static`）的 JSON 404 统一声明 `Cache-Control: no-store`，避免发版切换期间的 404 被浏览器或中间缓存留存（`internal/httpapi/router/web_router.go`、`web/src/features/errors/general-error.tsx`）。
- **接口返回 500 后页面一直停留在 500 错误页**：接口 500 不再把地址栏替换为 `/500` 路由，改为停留在当前页面并提示；刷新即重新加载当前地址，后端恢复后可以直接回来（`web/src/main.tsx`）。
- **密钥用量查询与登录共用关键限流额度**：凭 API Key 匿名查询用量与日志的接口（`/api/usage/token/`、`/api/log/token*`）改由独立的限流器计数，默认每 IP 180 秒 60 次，新增 `API_KEY_QUERY_RATE_LIMIT_ENABLE`、`API_KEY_QUERY_RATE_LIMIT`、`API_KEY_QUERY_RATE_LIMIT_DURATION`；查询密钥不再占用登录、注册等关键操作的额度，也不会连带把登录拒掉（`internal/httpapi/middleware/rate_limit.go`、`internal/httpapi/router/api_router.go`、`internal/app/env.go`、`internal/common/constants.go`）。
- **接口 429 被当作普通失败处理**：查询遇到 429 不再自动重试（401、403、429 均直接失败）；会话验证遇到 429、5xx 或网络错误时保留登录态并按状态码进入错误边界，只有服务端明确返回 401 才清除登录态并跳转登录页；接口 429 的提示统一为「请求过于频繁，请稍候再试。」，密钥用量查询页遇到 429 不再误报为网络错误（`web/src/lib/request-error.ts`、`web/src/routes/_authenticated/route.tsx`、`web/src/lib/api.ts`、`web/src/lib/handle-server-error.ts`、`web/src/features/key-query/api.ts`）。
- **模型广场模型详情页签顺序（#458）**：模型详情页与模型列表右侧详情抽屉的页签统一按「概览、性能、API」排列（`web/src/features/pricing/components/model-details.tsx`）。
- **控制台页面刷新返回 HTTP 429**：全局 Web 限流按 IP 只计页面与未匹配路径的请求，前端构建产物（入口脚本、懒加载分块等）不再占用页面额度，避免连续刷新或浏览较多页面后静态资源被限流、分块加载失败。
  - 前端构建产物改由独立的静态限流器计数：`GLOBAL_STATIC_RATE_LIMIT_ENABLE`（默认关闭，关闭时构建产物不限流）、`GLOBAL_STATIC_RATE_LIMIT`（默认 1000）、`GLOBAL_STATIC_RATE_LIMIT_DURATION`（默认 180）；Web 限流与静态资源服务显式挂在 SPA 回退链上，不涉及 API 与 Relay 路由（`internal/httpapi/middleware/rate_limit.go`、`internal/httpapi/router/web_router.go`、`internal/app/env.go`）。
  - IP 级与用户级限流器（全局 API / Web / 静态 / 关键操作 / 搜索等）拒绝响应附带 `Retry-After` 响应头，取完整窗口时长；模型级限流与邮箱验证频控不受影响（`internal/httpapi/middleware/rate_limit.go`）。
  - 错误页遇到路由分块加载失败时在同一标签页内自动刷新一次，再次失败停留在错误页；作为错误边界渲染时，状态码标题仅在错误携带真实 HTTP 状态码时展示，直接访问的 `/500` 页面仍固定显示 500（`web/src/features/errors/general-error.tsx`、`web/src/features/errors/lib/general-error.ts`）。
- **统一遗留品牌标识为 NookMux**：
  - 将 Bark 与 Gotify 通知请求的 `User-Agent` 统一为 `NookMux-Bark-Notify/1.0` 与 `NookMux-Gotify-Notify/1.0`（`internal/infra/notify/user_notify.go`），收口请求发送逻辑并补充单元测试。
  - 将日志文件落盘命名格式调整为 `nookmux-%s.log`（`internal/infra/log/logger.go`）并补充单元测试。
  - 将页面统计脚本注入注释更新为 `<!--Umami NookMux-->` 与 `<!--Google Analytics NookMux-->`（`internal/app/analytics.go`），同步更新单元测试。

- **前端运行时 i18n 键值透传、重复翻译与硬编码问题**：
  - 修复多模态文件列、数据库迁移状态、使用日志平台与任务状态、工单类型/错误提示、渠道类型与套餐额度弹窗（含补充 `xiaomi-coding-plan*` 分区及状态/MCP 工具映射）、货币标签、支付方式、模型元数据（分词器/协议/厂商显示名）、Passkey 与二步验证错误提示等将原始值或接口原文直接传入 `t(...)` 导致展示裸键值的问题。
  - 修复公开页眉导航、页脚备用列及 MiniMax 语音设置的 `t(t(...))` 重复翻译问题，重构渠道参数覆盖预设、系统设置校验（含 13 处 Zod 表单规则）、账号注销及兑换码批量删除弹窗中的文案拼接与半括号键，统一使用完整翻译键或 `<Trans />`。

### Security

- **对照上游 47 项安全修复的全量安全加固**（审计结论：29 项存在同类缺
  陷已修复，12 项本项目已等效修复，3 项不适用）：
  - **会话与凭证**：用户表新增 `auth_version` 凭证版本列，改密、关闭
    2FA、管理员封禁/重置等凭证变更自增该值并使全部旧会话即时失效
    （等价分布式强制下线）；会话认证每请求按数据库最新值重校验角色/
    状态/分组并比对凭证版本；长期访问令牌不再授予管理面 API。
  - **敏感操作二次验证（scope 化窗口期证明）**：访问令牌生成、邮箱绑
    定、注销账号、开启 2FA、OAuth 绑定与管理员用户管理（创建/更新/
    删除/封禁/升降权/重置 Passkey/重置 2FA）均须先通过安全验证
    （2FA / Passkey / 原密码三选一），验证证明绑定会话与操作范围，作
    用域内 5 分钟窗口有效（cookie 会话无法服务端强制单次消费，窗口期
    语义与上游一致）；无二因子账户以原密码再认证为退路；前端敏感操作
    统一接入验证对话框。
  - **账户与邮箱**：邮箱统一规范化（去首尾空白 + 小写）贯穿注册、绑
    定、找回密码与 OAuth 合并；密码重置仅作用于唯一匹配账户（歧义或
    不存在一律拒绝）；自助改密强制校验原密码，无密码账户必须先经邮
    箱重置建立密码；注册与用户名更新拒绝空白与仿冒用户名；创建与更
    新用户校验角色白名单。
  - **删除与凭据生命周期**：硬删除用户改为事务内级联清除其令牌、
    Passkey、2FA 与备用码，杜绝 SQLite rowid 复用下的凭据继承；2FA
    状态查询失败按失败关闭；2FA 失败计数与备用码核销改为条件原子更
    新，并发下不可绕过锁定或重复核销。
  - **并发与数据完整性**：Redis 用户/令牌鉴权缓存新增 HMAC-SHA256 完
    整性签名（绑定键名、凭证版本与时效界；配额计数器除外），篡改条
    目读取即拒绝并回源修复（保留仍为整数的计数）；用户更新改用定向
    列更新与原子自增，杜绝陈旧快照覆盖配额/令牌/邀请统计；渠道状态
    保存改为多字段原子写。
  - **计费与额度**：额度换算全面饱和化（float/decimal → int32 钳制，
    NaN → 0），令牌计数非负钳制；充值上限改为按系统配额换算上限预校
    验，无法入账的订单在下单前拒绝。
  - **传输与解析防护**：登录密码支持 RSA-OAEP 传输加密（纵深防御，
    opt-in 环境变量 `PASSWORD_LOGIN_ENCRYPTION_ENABLED`，公钥经
    `GET /api/user/login/encryption-key` 下发，前端 WebCrypto 加密、
    HTTP 内网回退 node-forge）；zstd 解压施加窗口/内存/并发上限；各
    HTTP transport 的 TLS 配置相互隔离。
  - **信息泄露**：访问日志遮蔽查询串中的 `key` 等凭据参数与 OAuth 敏
    感路径；错误信息统一遮蔽上游地址/代理/IP 与 URL 内嵌凭据；用户日
    志详情按可见级别投影渠道字段；富文本与自定义 HTML 渲染经
    DOMPurify / rehype-sanitize 消毒。
  - **限流**：访问令牌生成、邀请划转、改密与管理员用户管理等敏感路由
    补齐关键级限流。
  - **依赖审计门禁回归无豁免基线**：braces 漏洞通告（GHSA-vfj7-8cjw-p6xm /
    CVE-2026-93687）经上游复核于 2026-10-23 撤销（非实际可利用漏洞），
    `bun audit` 不再报告该通告；据此移除 CI 与本地门禁中的通告级豁免及
    `braces@3.0.3` 深度防护补丁，审计步骤恢复为无豁免的
    `bun audit --audit-level=high`。

### Added

- **模型性能指标（API 模型性能）**：新增按「模型 × 分组 × 时间桶」聚合的
  中继性能统计全链路。采样挂在中继请求边界（`Relay()` defer），每请求恰好
  一次、天然覆盖文本 / Claude / Gemini / 音频 / 实时 / 图像 / 嵌入 / 重排全
  部模态，多渠道重试不重复计数；成功侧输出 token 由各结算出口回填
  `RelayInfo` 快照。指标含端到端延迟、首字延迟（仅流式已发出首帧）、成功
  率与输出吞吐（tokens/生成阶段秒数）；成败口径排除业务拒绝（敏感词、额
  度不足、预扣失败、违规费）与本地校验失败，未知错误码按失败计入。数据
  走内存热桶 → 定时（默认 5 分钟）增量落库新表 `perf_metrics`（唯一键
  model+group+bucket_start，等值点查条件累加的六库兼容 upsert），支持保留
  期清理（默认 30 天，0=永久）与停机兜底落库。新增两个公开查询接口
  `GET /api/perf-metrics`（单模型：汇总 + 逐桶序列 + 各分组明细）与
  `GET /api/perf-metrics/summary`（全站汇总 + 各模型摘要，按请求量降序），
  公开性跟随头部分组模块开关（`perf_metrics`），仅暴露组倍率表内分组的
  数据。模型广场卡片底部性能徽章（延迟 / 吞吐 / 成功率三色信号）通电，
  模型详情新增「性能」页签（KPI 卡 + 各分组性能表 + 首字延迟与成功率趋
  势图）。配置项 `perf_metrics_setting.*`（启用开关、落库间隔、聚合桶粒
  度 minute/5min/hour、保留天数）进入运维设置分区，保存链路带范围校验。
- **模型广场分段计费的多分组展示**：上下文分段计费（`context_pricing`）
  模型在详情页完整展开——概览区新增基准分档价格表（不含分组倍率），「按
  分组定价」区每个可用分组一张分档卡（分组徽章 + 倍率，卡内 tier × 价格
  字段表按该组倍率计价），价格口径与列表一致（倍率 ×2 基价、按 token 单
  位与充值价换算）。列表与卡片视图新增分组价格联动：侧栏选中分组后，价
  格列 / 缓存价列 / 卡片价格（含分段与动态计费摘要）切换为该组倍率计价，
  未选中时维持可用分组最低倍率口径。分散在价格列与卡片中的三份
  格式化」重复实现收敛为共享工具
  （`lib/context-price.ts`）。卡片与表格行点击弹出右侧详情抽屉快速预
  览；卡片「详情」按钮与表格模型名列的全屏图标直达 `/pricing/<模型名>`
  全屏详情页（携带当前筛选参数，返回列表时自动恢复筛选状态）。
- **模型性能指标与分段计费的文档同步**（Mintlify 中英）：新增
  `GET /api/perf-metrics` 与 `GET /api/perf-metrics/summary` 端点参考页
  （查询参数、嵌套响应字段到逐桶序列与分组明细级、模块门控与错误口径；
  summary 无首字延迟字段已注明），docs.json 中英导航按字母序插入系统组；
  运维设置指南补「模型性能指标」条目（采样开关、落库间隔、聚合桶粒度
  与保留天数）；模型计费指南补性能徽章、「性能」页签与分组分档价格
  展示说明。
- **日志自动清理与 `quota_data` 清理范围**：日志维护设置新增「自动清理」，
  可配置数据最长保存时长与清理间隔（正整数数值 + 年/月/周/天单位，数值不做上限限制）。
  开启后由后台任务在服务器本地时区的自然边界时刻自动删除超期数据：每天为 00:00，
  每周对齐周一 00:00，每月对齐 1 日 00:00，每年对齐 1 月 1 日 00:00；等待期间修改
  间隔配置至多 10 分钟内切换到新时刻，清理按 `created_at` 截止删除，天然幂等。
  清理范围支持使用日志（`logs`）、数据看板用量聚合（`quota_data`）、存储图片、
  存储视频与审计日志五类目标，复用手动清理的分批删除实现；手动清理同步新增
  「数据看板统计」目标，删除后自动失效用户分析与排行榜缓存。配置持久化为
  `log_cleanup_setting.*` 系统设置（默认关闭，保留近两年、每周 00:00 清理
  `logs` 与 `quota_data`），写入路径校验正整数、单位枚举与目标白名单。
- **使用平台识别（Agent 客户端归因）**：通过 `User-Agent`、`HTTP-Referer`、
  `X-Title` 三个客户端请求头识别发起请求的 Agent 平台，首批支持
  Claude Code、Codex、OpenCode、Hermes Agent、DeepSeek Harness、OpenClaw、
  pi、New API（网关自报 UA 弱信号）与 Cherry Studio 九个平台，规则均经
  源码级核实并带防误报边界（如 `pi` 精确匹配不受 `pinecone/1.0` 干扰）。
  识别结果落库为 `logs.platform` 受控短枚举列，历史日志由一次性迁移
  （`data_migration.log_platform.v1.done` marker，id 游标分批）基于已回填的
  三头列自动识别回填。使用日志新增「平台」列（logo+文本、居中、溢出省略，
  列宽对齐渠道列）；`quota_data` 聚合维度加入平台（内存缓存、定时落盘与
  `POST /api/data/recalculate` 重算链路同步），排行榜页在市场份额上方新增
  「热门 Agent」卡片（平台堆叠柱状图 + 双列排行榜），`/api/rankings` 响应
  新增 `platforms` 与 `platforms_history` 字段；仪表台配置新增
  `rankings_platform_limit`（默认 10，范围 1-50）。已部署实例升级后，可对
  历史时间范围执行一次 `POST /api/data/recalculate` 让热门 Agent 图表覆盖
  存量流量。平台显示名与图标由前端统一映射（`@lobehub/icons`），后端仅
  下发平台 ID。
- **使用平台识别接口文档同步**（Mintlify 中英）：`GET /api/log` 与
  `GET /api/log/self` 响应示例与字段表新增 `platform`；`GET /api/rankings`
  响应新增 `platforms` 榜单与 `platforms_history` 时序结构的完整解析
  （含 `Others` 归并、环比省略语义）；`GET/PUT /api/dashboard/config` 与
  `POST /api/dashboard/config/reset` 补 `rankings_platform_limit`（默认 10、
  范围 1-50）；`POST /api/data/recalculate` 的聚合粒度说明补平台维度。
  全部字段对照后端源码核实。
- **使用平台识别新增 RikkaHub**：基于源码级核实的两条识别规则——UA 默认
  `RikkaHub-Android/{version}`（客户端 OkHttp 拦截器对全部出站请求注入，
  `rikkahub` 前缀锚定，兼容 `RikkaHub/{version}` 变体且不误报
  `rikkahubite/1.0` 之类近似前缀）与 Referer host `rikka-ai.com`（官方
  站点，OpenRouter 场景固定上报，用户亦可在提供商自定义头中显式配置）；
  `X-Title: RikkaHub` 按既有设计仅作弱佐证不参与判定。历史日志回填迁移
  marker 由 `data_migration.log_platform.v1.done` 升级为
  `data_migration.log_platform.v2.done`，已部署实例升级后启动时自动对
  `platform` 为空的存量行增量重跑识别（已识别行不受影响），无需人工干预。
- **使用平台识别扩展 Claude Desktop**：Claude Desktop（Electron）桌面端进程
  发起的请求，UA 形如
  `Mozilla/5.0 (<os>) AppleWebKit/537.36 (KHTML, like Gecko) Claude/{version} Chrome/{version} Electron/{version} Safari/537.36`，
  应用自报的 `Claude/{version}` 位于串中而非串首，现按空白分隔 token 对 '/'
  之前的 product 做精确匹配并归入 Claude Code（`claude-code`）；系统段与各段
  版本号随运行环境与发版变化，均不参与判定，`Claude-User` / `Claude-Web` 等
  product 不同的 UA 不受影响。历史日志回填迁移 marker 由
  `data_migration.log_platform.v2.done` 升级为
  `data_migration.log_platform.v3.done`，已部署实例升级后启动时自动对
  `platform` 为空的存量行增量重跑识别（已识别行不受影响）；「热门 Agent」
  图表如需覆盖存量 Claude Desktop 流量，对历史时间范围执行一次
  `POST /api/data/recalculate`。
- **使用平台识别新增 WorkBuddy**：WorkBuddy（腾讯云桌面 AI Agent）发起请求
  的 UA 形如 `WorkBuddy/{version} WorkBuddy/{version} CLI/{version}`（如
  `WorkBuddy/5.11.3 WorkBuddy/5.11.3 CLI/2.176.0`），串首 `workbuddy` 前缀
  锚定命中，大小写不敏感，且不误报 `workbuddyite/1.0` 之类近似前缀；
  `X-Title: WorkBuddy` 按既有设计仅作弱佐证不参与判定。历史日志回填迁移
  marker 由 `data_migration.log_platform.v3.done` 升级为
  `data_migration.log_platform.v4.done`，已部署实例升级后启动时自动对
  `platform` 为空的存量行增量重跑识别（已识别行不受影响），无需人工干预。
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

- **模型定价可视化编辑器支持导入内置价格**：「系统设置 → 计费与支付 → 模型定价」右侧
  价格设置卡片的模型名右侧新增「导入内置价格」按钮，一键把当前模型在随版本发布的
  内置定价表中的原始条目（按次价格、输入/补全/缓存读写/音频倍率、上下文阶梯定价）
  填入表单并替换该模型的现有条目，计费类型（按请求 / 按 Token / 阶梯计费）随之
  自动选中，保存后生效；模型在内置定价表中没有可计费条目时明确提示且表单保持不变。
  模型名匹配规则与运行时计费取价一致（思考预算与 gizmo 通配，缓存倍率按原始名称
  匹配）。新增仅超级管理员可访问的只读接口
  `GET /api/option/builtin_model_pricing?model=<模型名>`。Mintlify 中英文档同步
  新增该接口的参考页并登记导航，API 契约页与「价格倍率」指南补充「导入内置
  价格」说明。

### Fixed

- **列表页表格统一化（#444）**：`DataTablePage` 升级为整页固定高度容器——
  根节点 `h-full` 填满 `SectionPageLayout.Content`，页面高度不再随数据条数
  伸缩；表格区 `flex-1` 顶到底部固定分页栏，数据超出时在表格容器内部滚动；
  表头内置 `sticky top-0`（`Table` 原语新增 `containerClassName` 以打通滚动
  容器，原实现 sticky 相对不滚动的包装层导致固定失效），筛选区与统计区固定
  不随滚动移动，移动端卡片列表同步在固定高度内滚动。同步迁移工单页
  （`features/tickets/`）与密钥用量查询页（`features/key-query/`）到手拼
  Card+Table 之外的统一 `DataTablePage` 容器，并清理各列表页冗余的
  `tableClassName` / `tableHeaderClassName` sticky 传参。
- **使用日志「Token」列标签显示为裸 i18n 键修复**：`prompt_tokens` 列的
  `meta.label` 未包 `t()`，而移动端卡片视图与列显隐菜单直接渲染该字符串，
  界面显示 `rankings.fields.tokens` 而非「Token」/「Tokens」；现按列定义
  约定传入 `t()` 翻译值，与同文件其余列一致。
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
- **模型定价编辑器清空价格不再丢失计费类型**：可视化编辑器中清空「按次
  收费」或「按量计费」的价格输入框时，此前会从 JSON 配置中删除该模型条目，
  已选择的计费类型随之回退为「未配置」；现改为写入 0（语义等价未配置价格），
  计费类型不再随输入清空丢失。定价输入框中 0 统一显示为空白（由占位提示
  建议值），价格摘要等只读展示仍显示 `$0 / request`。
- **前端高危传递依赖漏洞治理**：`overrides` 将 `query-string` 钉至 9.5.1
  （依赖方 `@lobehub/ui` 约束 `^9.3.1` 内的修复版），其依赖
  `decode-uri-component` 随之解析至修复版 0.5.1，消除百分号编码二次复杂度
  替换 DoS 漏洞（GHSA-qr28-fpjm-wq7f）；`follow-redirects` 钉至 1.16.2，
  消除跨域敏感头丢弃与凭据跟随重定向漏洞（GHSA-mrwp-f3q8-3g9p）。
  `bun audit --audit-level=high` 高危告警清零，CI 与本地 pre-push 门禁
  恢复全绿。
- **钱包与个人资料页面宽度与其他页面不一致修复**：钱包页正文容器与个人资料
  页整页内容均额外套 `mx-auto max-w-7xl` 居中限宽，宽屏下明显窄于其他页面
  （`SectionPageLayout.Content` 等页面为全宽）；现移除两处限宽，页面内容
  与控制台其余页面宽度对齐。

### Changed

- **日志渠道名改为随日志持久化的快照**：消费/错误日志（含定制音色试听
  计费日志）写入时记录请求时刻的渠道名快照，管理端日志查询直接读取快照
  列，不再按渠道 ID 实时联查渠道表；渠道后续改名不影响历史日志展示，
  历史日志由启动迁移按现存渠道回填，已删除渠道的历史行为空串。

- **首页路由图交互精简与视觉更新**：移除首页 Hero 右侧悬浮翻转卡片与 API
  请求示例终端面板（`HubFlipCard`、`HeroTerminalDemo` 组件删除），路由拓扑
  图常驻展示并转为纯装饰（`HubDiagram` 交互 props 移除）；左侧模型图标列
  改为从下到上匀速无缝循环（20s/圈 linear，豁免系统「减少动态效果」设置
  常驻运行，单向滚动动画工具类收敛为 `animate-scroll-vertical`）；右侧开
  发者工具图标换为 ClaudeCode、Codex、Cursor、OpenCode、CodeBuddy 五席并
  等距重排（中间图标正对 hub 中线，扇出曲线补第五条紫罗兰色）；同步清理
  翻转交互相关 i18n 文案。
- **使用平台识别数据管道性能优化**：历史日志平台回填迁移由逐行 UPDATE 改为
  同批按平台分组批量更新（每组一条 `UPDATE ... WHERE id IN (...)`，单条
  上限 500 个 id，兼容 SQLite 绑定参数与 Oracle 系 IN 列表限制），百万级
  存量回填的数据库往返从每行一次降为每组一次；`quota_data` 新增
  `(user_id, model_name, platform, created_at)` 复合索引（`idx_qdt_dim_time`），
  把定时落盘 upsert 的全表扫描降为索引点查，落盘改为锁内换出快照、锁外
  执行数据库 I/O，不再长时间阻塞请求路径的缓存写入；
  `POST /api/data/recalculate` 重算的聚合以 SQL `GROUP BY` 下推日志库
  （小时分桶表达式按日志库方言自动选择 FLOOR/整除截断，日志库可与主库
  方言不同），内存占用从时间范围内的日志原始行数降为聚合结果行数，
  消除大时间范围重算的内存峰值风险。
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

- **依赖批量升级**：合并 dependabot 全量依赖更新。Go：`tidwall/gjson`
  1.20.0、`gin-contrib/gzip` 1.2.9、`gin-contrib/static` 1.1.9、
  `golang.org/x/net` 0.62.0、`aws-sdk-go-v2/credentials` 1.20.13；web：
  `@rsbuild/core` 2.3.1、`@visactor/vchart` 2.2.1、`streamdown` 2.7.1、
  `zustand` 5.0.16、`@types/node` 26.6.6；CI：`docker/login-action` 4.8.0、
  `sigstore/cosign-installer` 4.1.3。

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
