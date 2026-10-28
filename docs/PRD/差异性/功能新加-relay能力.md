# 功能新加 · relay 能力

> **导航**：[主文件 · 合并版总览](合并化文档.md)
> ｜ [安全性](安全性.md) · [修复](修复.md) · [功能新加 · 渠道/运维](功能新加-渠道运维.md) · [功能新加 · 前端体验](功能新加-前端体验.md) · [功能新加 · 基础设施](功能新加-基础设施.md) · [功能新加 · 分组/计费](功能新加-分组计费配置.md) · **功能新加 · relay 能力（本篇）** · [战略级](战略级.md)

> 本篇拆自 [合并化文档](合并化文档.md) 的「功能新加 / relay 能力」小节（v1.0.0-rc.14 ~ v1.0.0-rc.25 差异性分析），条目正文保持源文档原样，标题层级随独立成篇上提一级。

## `85feb7a34` 参数覆盖条件暴露 user/group 上下文

涉及文件：
1. `internal/relay/common/override.go`（`BuildParamOverrideContext` :1244-1271 不放 user/group 键）

原因：
1. fork 的 RelayInfo 已有 `UserId/UserGroup/TokenGroup/UsingGroup` 字段，但 override 条件无法按用户/分组定向。

应当表现为：
1. ctx 增加这四个键，override 条件可用 `user_id`/`user_group`/`token_group`/`using_group` 匹配。

---

## `e99a9bd86` 每渠道 HTTP transport 控制

涉及文件：
1. `internal/dto/channel_settings.go`（:5-18 无任何 HTTP 协议控制字段）
2. `internal/service/http_client.go`（需扩展 transport 策略与分片）
3. `web/src/features/channels/`（表单控件 + i18n）

原因：
1. 部分上游（旧网关、企业代理）对 HTTP/2 多路复用有兼容问题，fork 只能全局 ForceAttemptHTTP2 无法按渠道降级 http1。注意：应先吸收 [修复](修复.md) 篇「性能 / 健壮性」`e13d4033e` 的缓存键重构；且须遵守根 AGENTS.md 待机内存保守约束（shards 默认 1、上限 8、仅显式开启）。

应当表现为：
1. 渠道设置支持 `http_protocol`（auto/http1）与 `http2_connection_shards`（1-8）；`ValidateSettings` 校验组合合法性；transport 缓存键包含策略。

---
