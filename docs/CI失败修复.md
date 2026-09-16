# CI 失败原因分析与修复方案

> 证据链状态：本地工作区与失败 commit 一致（`git status` 干净，HEAD = `3e8d99598`）。
> 两条根因（新增 audit + lint 门禁、`.golangci.yml` 缺失）已由 `gh run view 34983851014`
> 真实 CI 日志 + 本地复现（CI 同版本 golangci-lint v2.13.2、bun 1.3.14 audit）双重对质，
> 下文具体条目（包版本、依赖路径、违规行号、分类计数）均经逐条核实。

## 一、结论

CI 失败由 `6bca00e6b feat: 优化GitHub工作流` 新增的两个门禁步骤引入，且**当前代码库
从未为这两个门禁做过清理/配套**：

| 失败 job      | 失败步骤                  | 根因层级                  |
|---------------|---------------------------|---------------------------|
| Web checks    | `Dependency audit`（新增）| 依赖层：22 条 high 漏洞通告 |
| Go checks     | `Golangci lint`（新增）   | 配置层 + 代码层：`.golangci.yml` 不存在，按默认全量 linter 跑出违规（CI 日志可见 115 条，全量真实 385 条） |

其余步骤（typecheck/lint/format、gofmt、`go mod tidy -diff`、web build）在 CI 上全部通过；
`go vet`、`go build`、`go test -race` 因 lint 先挂而**从未执行过，状态未知**。

补充观察（不影响结论，但易误判）：

- `cancelled` 不是故障：`concurrency: ci-${{ github.ref }}` + `cancel-in-progress: true`
  的设计行为（dev 分支连续 merge 互相抢占），与失败无关。
- 飞书通知工作正常：`Notify Feishu (failure) 7s ✓`；`trigger-docker` 因 CI 未全绿按设计被跳过。

---

## 二、失败点 1：`bun audit --audit-level=high`

- Run：`34983851014`，job `104430987316`
- 实际：22 条 high 通告，`exit 1`

逐包分布（来自 CI 日志 + 本地 `bun audit --audit-level=high` 复现）：

| 包                 | 版本                | 通告数 | 依赖路径                              |
|--------------------|---------------------|--------|---------------------------------------|
| axios              | 1.16.1              | 1      | **直接依赖**（GHSA-gcfj-64vw-6mp9，需 >=1.18.0） |
| nanoid             | 6.0.1 / 3.3.12      | 2      | **直接依赖** + postcss 传递             |
| fast-uri           | 3.1.2               | 6      | ajv 传递                              |
| brace-expansion    | 5.0.6               | 3      | eslint / typescript-eslint / prettier-plugin 等传递 |
| js-yaml            | 4.1.1               | 3      | cosmiconfig 传递                      |
| browserslist       | 4.28.2              | 2      | babel / shadcn 传递                    |
| undici / hono / form-data / postcss / ip-address | — | 各 1 | shadcn CLI / eslint / babel 等 dev 工具链传递依赖 |

观察：门禁一上线就撞上了近期集中披露的一批 advisory。绝大多数不在直接控制范围内
（dev 工具链传递依赖），仅 axios 和 nanoid 6.0.1 是直接依赖，可直接升级；其余需通过
`package.json` 的 `overrides` 钉到修复版本。

---

## 三、失败点 2：Golangci lint

- Job：`104431479657`
- 工具：golangci-lint v2.13.2

### 3.1 配置层根因（关键）

workflow 注释和步骤声称：

> 门禁 errcheck / ineffassign / unused / staticcheck(SA+S)，配置见 `.golangci.yml`

但仓库里 **`.golangci.yml` / `.golangci.yaml` 根本不存在**（`ls` 与
`git log --all -- .golangci.yml` 均为空），该文件从未被提交过。于是 golangci-lint
v2.13.2 以**默认配置**运行，实际启用的检查范围远超注释声称的范围（staticcheck 的
QF/ST 快修类、govet inline 等全部打开）。

### 3.2 代码层结果

CI 日志 `##[error]` 行精确统计：

| Linter       | CI 日志可见条数 |
|--------------|-----------------|
| errcheck     | 50              |
| ineffassign  | 9               |
| unused       | 3               |
| govet        | 3               |
| staticcheck  | 50              |
| **合计**     | **115**         |

> 注：原始分析中曾写"约 105 条"，实际 CI 日志精确计数为 115 条。

分类特征：

- **errcheck ~50 条**：未检查的 `Close` / `Seek` / `os.Remove` / `fmt.Fprintf` 等。
  重灾区 `internal/infra/cache/body_storage.go`（CI 日志显示 8 条，真实 13 条，见 §3.3）、
  `internal/common`（9 条），含若干 `_test.go`。
- **staticcheck ~50 条**：QF 34 + ST 7 + SA 9。大量 `QF1003`（应改 tagged switch）、
  `QF1008`（可去掉内嵌字段选择器）、`ST1017`（Yoda 条件）、`SA1012`（nil Context）、
  `SA9003` / `SA9004` 等——QF/ST 类占比高，正说明默认配置比预期的 "SA+S" 宽。
- **ineffassign 9 条**、**unused 3 条**（含 `plan_quota_glm.go` 两个未用常量、
  测试里的 `boolPtr`）、**govet 3 条**（`reflect.Ptr` 应为 `reflect.Pointer`，
  `internal/infra/redis/redis.go:141/191/208`）。

### 3.3 CI 日志被截断（重要！）

golangci-lint v2.13.2 默认 `max-issues-per-linter=50`、`max-same-issues=3`，CI 日志
只展示了被压缩后的 115 条。用 `--max-issues-per-linter=0 --max-same-issues=0`
跑无限模式后，**真实违规总量是 385 条**：

| Linter       | 真实违规条数 |
|--------------|--------------|
| errcheck     | 154          |
| staticcheck  | 199          |
| ineffassign  | 19           |
| govet        | 10           |
| unused       | 3            |
| **合计**     | **385**      |

例如 `body_storage.go` 实际有 13 条 errcheck，CI 日志只显示 8 条；`gemini_handler.go`
的 SA4010 因 `max-same-issues=3` 在本地默认输出被隐藏但 CI 日志里有。

**修复时若只按日志上的 115 条逐条去修，清零后会再暴露约 270 条，lint 门禁仍会挂。**
正确路径是先补 `.golangci.yml` 收窄检查范围（与 workflow 注释声称的
errcheck / ineffassign / unused / staticcheck(SA+S) 对齐，禁用 QF/ST 类），而不是逐条修
115 条表面违规。

### 3.4 需要人工确认的真实 bug 味道

`internal/relay/handler/gemini_handler.go:169/182` 报 `SA4010`（append 结果未使用）。
经核实：`inputTexts`（157 行声明）只在 169/182 被 append，之后再无读取——确实是死代码
或遗漏逻辑。**这条建议优先人工确认**。

---

## 四、其他 CI 步骤状态

| 步骤                       | 状态 |
|----------------------------|------|
| Web typecheck / lint / format | ✓ 通过 |
| Web build                  | ✓ 通过 |
| Go `gofmt -l`             | ✓ 通过 |
| Go `go mod tidy -diff`    | ✓ 通过 |
| Go `go vet`               | ✗ 从未执行（lint 先挂） |
| Go `go build`             | ✗ 从未执行（lint 先挂） |
| Go `go test -race`        | ✗ 从未执行（lint 先挂） |

修复 lint 后，这三步将首次真实运行，可能还有未知问题需要处理。

---

## 五、修复方案（按依赖顺序）

### 5.1 优先级 1：补交 `.golangci.yml`（lint 门禁生效的前提）

将检查范围收窄到 workflow 注释声称的：

```yaml
# 仅示意，提交前需与团队确认具体 linters 与 severity
linters:
  disable-all: true
  enable:
    - errcheck
    - ineffassign
    - unused
    - staticcheck

linters-settings:
  staticcheck:
    checks:
      - "SA"   # 仅 SA 系列，关闭 QF/ST
```

要点：禁用 QF/ST 类快修（`QF1003`/`QF1008`/`ST1017` 等），仅保留 SA 系列。提交后
CI 违规总量应从 385 条大幅下降到接近 115 条（CI 日志原始可见量），其中绝大多数是
errcheck + SA 系列。

### 5.2 优先级 2：清理 lint 违规（按 linter 分批）

推荐顺序：

1. **unused（3 条）** —— 最小、最干净，先修。
   - `plan_quota_glm.go` 两个未用常量：删除或加 `_ =`。
   - 测试里的 `boolPtr`：删除或实际使用。
2. **govet（10 条真实）** —— `reflect.Ptr` → `reflect.Pointer`（`redis.go:141/191/208`
   等共 3 处），其余 inline 检查逐条确认。
3. **errcheck（154 条真实）** —— 重灾区，按目录分批：
   - `internal/infra/cache/body_storage.go`（13 条）：`_ = ...Close()` 或显式检查
     返回值。
   - `internal/common`（9 条）：同上模式。
   - 统一原则：能用 `defer func() { _ = f.Close() }()` 或统一错误处理 helper 的，
     不要逐处加 `_ =`。
4. **ineffassign（19 条真实）** —— 逐条确认是否真的有副作用被丢弃。
5. **staticcheck SA 系列（9 条）** —— 重点人工确认 `gemini_handler.go` 的 `SA4010`：
   - 若 `inputTexts` 真的就是死代码：删除。
   - 若是遗漏逻辑（例如应合并到上游请求体）：补回读取。
   - 其余 `SA1012`（nil Context）逐处加 `context.Background()` 或调用方 context 透传。

### 5.3 优先级 3：审计依赖（audit 门禁）

- 直接依赖：升级 `axios >= 1.18.0`、`nanoid` 修复版。
- 传递依赖：在 `web/package.json` 加 `overrides`，钉到修复版本：
  - `fast-uri`、`brace-expansion`、`js-yaml`、`browserslist`、`undici`、`hono`、
    `form-data`、`postcss`、`ip-address`。
- 评估 audit 策略：是否仅盯生产依赖（`--production`），避免 dev 工具链波动反复挂门禁。
  该决定需团队确认后改 workflow。

### 5.4 优先级 4：lint 通过后，盯紧首次执行的真实结果

`go vet` / `go build` / `go test -race` 此前从未在 CI 上跑过，修复 lint 后这三步将
首次真实运行，可能暴露新的未知问题（race、build tag、平台差异等）。

---

## 六、本报告与原分析之间的修正记录

| 项目                  | 原分析           | 本报告（核验后） | 依据 |
|-----------------------|------------------|------------------|------|
| 静态检查违规条数       | "约 40"（报告中写 ~40）| 50             | CI 日志精确统计 |
| 违规总数              | "约 105"          | 115（CI 日志）/ 385（真实全量）| 默认 `max-issues-per-linter=50` 截断，`--max-issues-per-linter=0` 复现 |
| 修复路径              | "分批清理 ~50 条 errcheck" | 先补 `.golangci.yml` 收窄，再分批修剩余 ~115 条 | 否则清零后再暴露 ~270 条 |

其余逐条核验（包版本、依赖路径、违规行号、cancelled 设计、飞书通知正常、trigger-docker
跳过等）全部 ✅ 属实，未做修正。