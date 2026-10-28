# Issue #375 死代码清理 —— 剩余项清单（二期）

- **Issue**: [NookMux/NookMux#375](https://github.com/NookMux/NookMux/issues/375)（OPEN，标题：fix: 死代码清理）
- **日期**: 2026-09-28
- **现状**: knip 报告的未引用文件、未使用依赖、未声明依赖、重复导出均已归零；本清单为**存活文件内尚未处理的死代码**——328 个未使用导出与 122 个未使用类型，分布 142 个文件。

---

## 一、分布概览

| 域 | 文件数 | exports | types | 备注 |
|---|---|---|---|---|
| features/channels | 10 | 48 | 10 | channel-utils（18）、constants（9）为最大单文件 |
| components/ai-elements | 4 | 38 | 35 | 集中在 prompt-input.tsx（35+33），见处置口径 4 |
| features/pricing | 10 | 37 | 9 | billing-expr（18）、filters（7） |
| assets/brand-icons | 15 | 28 | 0 | 14 个未使用品牌图标 + barrel re-export |
| components/layout | 6 | 23 | 12 | index.ts barrel 占 18+10 |
| features/models | 6 | 23 | 6 | model-utils（10）、model-form（7） |
| features/usage-logs | 10 | 17 | 3 | |
| features/auth | 6 | 9 | 3 | |
| features/system-settings | 15 | 9 | 15 | 以类型为主（各分区 SectionId 等） |
| features/dashboard | 5 | 6 | 2 | |
| features/playground | 4 | 7 | 2 | |
| features/users | 3 | 6 | 2 | |
| components/data-table | 3 | 5 | 2 | |
| features/keys | 3 | 2 | 1 | |
| 其余 42 个文件 | 42 | 70 | 20 | lib/ 工具库（format 8、time 6、colors 6、utils 3、passkey 3、nav-modules 5 等）、hooks/、stores/、i18n/、context/、单文件 feature 与根组件散布项 |

## 二、处置口径

1. **两层裁决**：knip 的「未使用导出」指无外部模块消费该绑定。符号若在本文件内部仍在使用，处置是**移除 `export` 关键字降级为私有**，而非删除符号；文件内也零使用的才整体删除。典型案例：`components/status-badge.tsx` 的 `useStatusPresetLabel` 全仓零消费，其文件内唯一依赖 `statusPresets` 与 `StatusPreset` 随之失去消费者，三者按死链级联删除；`textColorMap` 文件内零使用（组件实际用 `badgeSurfaceMap`），整体删除。反之，`i18n/config.ts` 的 `resources`/`default`、`ai-elements/code-block.tsx` 的 `highlightCode` 属文件内仍有消费的例子，处置是去 `export` 而非删除。
2. **barrel 双层导出甄别**：barrel 的 re-export 与源文件导出需区分「整链死」（源文件符号 + barrel 行同删）与「仅 barrel 层死」（源被深路径直接消费，只删 barrel 行）。涉及 `components/layout/index.ts`（18）、`assets/brand-icons/index.ts`（14）、`components/data-table/index.ts`（4）、`hooks/index.ts`（4）、`pricing/components/index.ts`（3）、`keys/lib/index.ts`（1）。
3. **入口副作用导出**：`i18n/config.ts` 的 `resources`（全仓零消费，可删）与 `default`（i18n 实例；`main.tsx:40` 以 `import './i18n/config'` 副作用导入，实例经模块加载生效）——处置 `default` 前须确认无其他绑定式消费。
4. **组件库风格文件**：`components/ai-elements/prompt-input.tsx`（35 导出 + 33 类型）为 vendored 组件库式 API，其子组件（`PromptInputBody`/`PromptInputCommand` 系等）仅由组件内部组装使用；`docs/PRD/差异性/安全性.md` 以该目录行号为审计证据。建议整体保留导出面，或仅处置经核对零内部使用的项，不盲目 unexport。
5. **同名符号甄别**：`modelFormSchema`（`models/lib/model-form.ts` 与 `models/types.ts` 各一）、`QuotaType`（`pricing/types.ts` 与 `models/types.ts` 各一）——删除前先甄别是否为 re-export 链的两端。
6. **通用核查维度**：模块说明符（`@/` 别名 + 相对路径）、导出符号名（含同名甄别）、动态访问（`obj[fnName]`、注册表/映射表值传递）、字符串引用、测试消费（`bun test` 真实运行，knip 已 ignore 测试文件故不在此清单）。

## 三、批次建议

- 按上表域拆批（channels / pricing+models / layout+data-table+根组件 / brand-icons / lib+hooks+stores / 各 feature 散项），每批 ≤ 4 并行、文件独占，批后统一跑 `bun run typecheck`、`bun run build`、`bun test`、`bun run lint`、`bun run knip` 复核。
- 每批完成后 `bun run i18n:sync` 校验双语文案无漂移（部分导出删除可能伴随删除映射表内的 i18n key）。
- `src/components/ui/**` 在 knip ignore 内，其死导出不在此清单；如需处理先评估是否收紧 ignore。

## 附：完整清单（142 文件 / 328 exports / 122 types）

> 由 `bun run knip --reporter json --no-exit-code` 生成，按域分组。`exports` 为未使用导出、`types` 为未使用类型。

```
features/system-settings | 文件 15 | exports 9 | types 15
  auth/section-registry.tsx            types: AuthSectionId
  billing/section-registry.tsx         types: BillingSectionId
  components/settings-form-layout.tsx  exports: SettingsSwitchRow
  content/section-registry.tsx         types: ContentSectionId
  dashboard/hooks/use-dashboard-config.ts  exports: useResetDashboardConfig
  integrations/utils.ts                exports: isValidJson
  maintenance/config.ts                types: HeaderNavAccessConfig
  models/section-registry.tsx          types: ModelSectionId
  operations/section-registry.tsx      types: OperationsSectionId
  security/section-registry.tsx        types: SecuritySectionId
  site/section-registry.tsx            types: SiteSectionId
  types.ts                             types: SystemOptionKey, OptionJsonMapEntry, DatabaseMigrationTableProgress, DatabaseMigrationJobStatus, DashboardSettings
  utils/json-parser.ts                 exports: tryJsonParse | types: JsonParseResult
  utils/json-validators.ts             exports: isStringArray, isNumberArray, isObjectArray, createObjectValidator, createArrayValidator
  utils/section-registry.ts            types: SectionDefinition

assets/brand-icons | 文件 15 | exports 28
  icon-{docker,facebook,figma,gitlab,gmail,medium,notion,skype,slack,stripe,telegram,trello,whatsapp,zoom}.tsx  各 1 个同名 Icon 导出
  index.ts                            exports: 上述 14 个 Icon 的 re-export

features/usage-logs | 文件 10 | exports 17 | types 3
  components/columns/column-helpers.tsx  exports: CacheTooltip, createTimestampColumn
  constants.ts                        exports: TIME_RANGE_PRESETS, MJ_TASK_STATUS, MJ_SUBMIT_RESULT_CODES, TASK_PLATFORMS
  data/schema.ts                      exports: usageLogSchema
  lib/field-visibility.ts             exports: USAGE_LOG_FIELD_KEYS, parseUsageLogFieldsConfig
  lib/filter.ts                       exports: getLogCategoryLabel
  lib/format.ts                       exports: normalizeTierLabel, resolveMatchedTier
  lib/mappers.ts                      exports: taskPlatformMapper
  lib/utils.ts                        exports: buildBaseParams, buildApiParams
  section-registry.tsx                exports: USAGE_LOGS_SECTION_IDS, getUsageLogsSectionNavItems
  types.ts                            types: CommonFilters, ContextPricingPrices, ContextPricingResult

features/pricing | 文件 10 | exports 37 | types 9
  components/index.ts                 exports: ModelCard, ModelDetails, ModelDetailsContent
  components/model-details-api.tsx    exports: ApiTabIcon
  components/model-details-modalities.tsx  exports: ModalitiesMatrix
  components/model-details.tsx        exports: ModelDetailsContent | types: ModelDetailsContentProps
  constants.ts                        exports: FILTER_SECTIONS, MAX_TAGS_DISPLAY, MAX_FILTER_ITEMS, SIDEBAR_WIDTH
  lib/billing-expr.ts                 exports: BILLING_VARS, BILLING_CONDITION_VARS, BILLING_EXTRA_VARS, BILLING_CACHE_VAR_MAP, SOURCE_PARAM, SOURCE_HEADER, MATCH_GT, MATCH_LTE, TIME_FUNCS, COMMON_TIMEZONES, combineBillingExpr, createEmptyCondition, createEmptyTimeCondition, createEmptyRuleGroup, createEmptyTimeRuleGroup, getRequestRuleMatchOptions, normalizeCondition, buildRequestRuleExpr | types: TimeFunc, ParamHeaderCondition, TimeCondition, MatchOption
  lib/dynamic-price.ts                exports: formatDynamicUnitPrice, hasDynamicRequestRules
  lib/filters.ts                      exports: filterBySearch, filterByVendor, filterByGroup, filterByQuotaType, filterByEndpointType, sortModels, filterByTag
  lib/model-metadata.ts               types: ModelVendor
  types.ts                            types: QuotaType, ContextPricingTier, ContextPricingConfig

features/channels | 文件 10 | exports 48 | types 10
  api.ts                              exports: manageMultiKeys, getOllamaVersion
  components/channels-filter-bar.tsx  types: ChannelFilterOption
  constants.ts                        exports: CHANNEL_STATUS_LABELS, MULTI_KEY_STATUS, MULTI_KEY_STATUS_LABELS, MULTI_KEY_MODES, AUTO_BAN_OPTIONS, DEFAULT_CHANNEL_VALUES, CHANNELS_TABLE_PAGE_SIZE_OPTIONS, SORT_OPTIONS, BALANCE_THRESHOLDS
  lib/channel-actions.ts              exports: handleEnableChannel, handleDisableChannel
  lib/channel-form.ts                 exports: validateJSON, validateModelMapping, parseModels, parseGroups, formatModels, formatGroups
  lib/channel-type-config.ts          exports: CHANNEL_TYPE_CONFIGS, getChannelTypeConfig, requiresOrganization, requiresRegion, getDefaultBaseUrl, getChannelTypeHints, validateKeyFormat | types: ChannelTypeConfig
  lib/channel-utils.ts                exports: getChannelStatusBadge, getMultiKeyStatusBadge, formatChannelKey, formatKeyPreview, countKeys, formatModelsString, formatGroupsString, parseChannelOtherSettings, validateChannelSettings, formatQuota, getPriorityDisplay, getWeightDisplay, validateChannelName, validateApiKey, validateModels, validateGroups, channelNeedsAttention, getAttentionReason
  lib/model-mapping-validation.ts     exports: findExposedTargetModels
  lib/status-code-risk-guard.ts       exports: collectDisallowedStatusCodeRedirects
  types.ts                            exports: channelInfoSchema, channelSchema | types: ChannelInfo, ProxyTestStatus, PlanMcpToolDetail, GlmActivitySummary, GlmResetCardListData, ChannelSortOrder, ChannelTestParams, ChannelFormData

features/auth | 文件 6 | exports 9 | types 3
  api.ts                              exports: githubOAuthStart, bindEmail
  constants.ts                        exports: PASSWORD_MIN_LENGTH, PASSWORD_MAX_LENGTH
  lib/oauth.ts                        exports: getAvailableOAuthProviders, hasOAuthProviders
  lib/storage.ts                      exports: getUserId, removeUserId
  lib/validation.ts                   exports: isValidEmail
  types.ts                            types: PasswordResetPayload, EmailVerificationPayload, BindEmailPayload

components/layout | 文件 6 | exports 23 | types 12
  components/mobile-drawer.tsx        exports: MobileDrawer | types: MobileDrawerProps
  components/nav-link-item.tsx        exports: NavLinkItem, NavLinkList
  components/public-navigation.tsx    exports: PublicNavigation
  index.ts                            exports: AppHeader, AppSidebar, PublicHeader, PublicNavigation, HeaderLogo, NavLinkItem, NavLinkList, NavGroup, SidebarViewHeader, SystemBrand, TopNav, MobileDrawer, SYSTEM_SETTINGS_VIEW, defaultTopNavLinks, MOBILE_DRAWER_ANIMATION, MOBILE_DRAWER_CONFIG, getNavGroupsForPath, resolveSidebarView | types: NavCollapsible, NavGroupType, NavItem, NavLink, ResolvedSidebarView, SidebarData, SidebarView, SidebarViewParent, TopNavLink, SectionPageLayoutProps
  lib/url-utils.ts                    exports: normalizeHref
  types.ts                            types: SidebarViewParent

features/models | 文件 6 | exports 23 | types 6
  api.ts                              exports: searchVendors, getVendor
  lib/model-actions.ts                exports: handleEnableModel, handleDisableModel
  lib/model-form.ts                   exports: modelFormSchema, vendorFormSchema, transformModelToFormDefaults, transformFormDataToModelPayload, formatTagsArray, validateJSON, validateEndpoints | types: ModelFormValues, VendorFormValues
  lib/model-utils.ts                  exports: formatTimestamp, formatRelativeTime, formatTagsString, parseEndpoints, getNameRuleLabelByRule, getNameRuleConfigByRule, formatQuotaTypes, validateModelName, validateEndpointsJSON, isModelSyncOfficial
  section-registry.tsx                exports: getModelsSectionNavItems | types: ModelsSectionId
  types.ts                            exports: modelFormSchema | types: BoundChannel, ModelFormValues, QuotaType

features/dashboard | 文件 5 | exports 6 | types 2
  components/ui/stat-card.tsx         types: StatCardDetail
  hooks/use-status-data.ts            exports: useStatusData
  lib/index.ts                        exports: getLatencyColorClass, testUrlLatency, openExternalSpeedTest, getDefaultPingStatus
  section-registry.tsx                exports: getDashboardSectionNavItems
  types.ts                            types: LatencyErrorReason

features/playground | 文件 4 | exports 7 | types 2
  constants.ts                        exports: DEFAULT_GROUP
  lib/message-utils.ts                exports: createMessageVersion, getCurrentVersion, updateCurrentVersionContent, buildMessageContent, getTextContent
  lib/storage.ts                      exports: clearPlaygroundData
  types.ts                            types: MessageRole, MessageStatus

components/ai-elements | 文件 4 | exports 38 | types 35
  code-block.tsx                      exports: highlightCode
  conversation.tsx                    exports: ConversationEmptyState | types: ConversationEmptyStateProps
  message.tsx                         exports: MessageAvatar | types: MessageAvatarProps
  prompt-input.tsx                    exports: usePromptInputController, useProviderAttachments, PromptInputProvider, usePromptInputAttachments, PromptInputAttachment, PromptInputAttachments, PromptInputActionAddAttachments, PromptInputBody, PromptInputHeader, PromptInputActionMenu, PromptInputActionMenuTrigger, PromptInputActionMenuContent, PromptInputActionMenuItem, PromptInputSubmit, PromptInputSpeechButton, PromptInputModelSelect, PromptInputModelSelectTrigger, PromptInputModelSelectContent, PromptInputModelSelectItem, PromptInputModelSelectValue, PromptInputHoverCard, PromptInputHoverCardTrigger, PromptInputHoverCardContent, PromptInputTabsList, PromptInputTab, PromptInputTabLabel, PromptInputTabBody, PromptInputTabItem, PromptInputCommand, PromptInputCommandInput, PromptInputCommandList, PromptInputCommandEmpty, PromptInputCommandGroup, PromptInputCommandItem, PromptInputCommandSeparator | types: TextInputContext, PromptInputProviderProps, PromptInputAttachmentProps, PromptInputAttachmentsProps, PromptInputActionAddAttachmentsProps, PromptInputBodyProps, PromptInputHeaderProps, PromptInputActionMenuProps, PromptInputActionMenuTriggerProps, PromptInputActionMenuContentProps, PromptInputActionMenuItemProps, PromptInputSubmitProps, PromptInputSpeechButtonProps, PromptInputModelSelectProps, PromptInputModelSelectTriggerProps, PromptInputModelSelectContentProps, PromptInputModelSelectItemProps, PromptInputModelSelectValueProps, PromptInputHoverCardProps, PromptInputHoverCardTriggerProps, PromptInputHoverCardContentProps, PromptInputTabsListProps, PromptInputTabProps, PromptInputTabLabelProps, PromptInputTabBodyProps, PromptInputTabItemProps, PromptInputCommandProps, PromptInputCommandInputProps, PromptInputCommandListProps, PromptInputCommandEmptyProps, PromptInputCommandGroupProps, PromptInputCommandItemProps, PromptInputCommandSeparatorProps

components/data-table | 文件 3 | exports 5 | types 2
  data-table-page.tsx                 types: DataTablePageToolbarProps
  index.ts                            exports: DataTablePagination, DataTableFacetedFilter, DataTableToolbar, MobileCardList | types: DataTablePageProps
  use-persistent-column-visibility.ts  exports: loadVisibilityState（豁免：被 use-persistent-column-visibility.test.ts 真实消费，保留导出）

features/users | 文件 3 | exports 6 | types 2
  constants.ts                        exports: USER_STATUS_DELETED
  lib/user-ban-schema.ts              exports: banIdentifierTypeSchema
  types.ts                            exports: userStatusSchema, userRoleSchema, userSchema, userListSchema | types: UserStatus, UserRole

features/keys | 文件 3 | exports 2 | types 1
  components/api-key-quota-type-combobox.tsx  types: ApiKeyQuotaTypeOption
  lib/api-key-form.ts                 exports: API_KEY_FORM_DEFAULT_VALUES
  lib/index.ts                        exports: API_KEY_FORM_DEFAULT_VALUES（re-export）

features/key-query | 文件 2 | exports 2 | types 1
  api.ts                              exports: fetchKeyLogsLegacy, isValidKeyFormat
  types.ts                            types: KeyQueryLogsPaginatedData

features/wallet | 文件 2 | exports 3
  lib/billing.ts                      exports: STATUS_CONFIG, PAYMENT_METHOD_NAMES
  lib/format.ts                       exports: formatQuotaShort

features/profile | 文件 2 | exports 1 | types 1
  api.ts                              exports: bindWeChat
  types.ts                            types: CheckinStats

features/rankings | 文件 2 | exports 1 | types 3
  lib/format.ts                       exports: formatReleaseDate
  types.ts                            types: RankingCategoryId, ModelHistoryPoint, VendorSharePoint

features/redemption-codes | 文件 1 | exports 1
  constants.ts                        exports: ERROR_MESSAGES

features/audit-logs | 文件 1 | exports 1 | types 2
  api.ts                              exports: getAuditModules | types: AuditModule, AuditModulesResponse

features/performance-metrics | 文件 1 | exports 1
  lib/format.ts                       exports: formatUptimePct

features/dynamic-ratio | 文件 1 | types 1
  types.ts                            types: DynamicRatioSummary

features/voice-management | 文件 1 | types 1
  components/voice-filter-bar.tsx     types: VoiceTypeFilter

features/multimodal | 文件 1 | types 1
  custom-voice/api.ts                 types: CustomVoiceTagsResponse

lib、hooks、stores、i18n、context 与根组件 | 文件 28 | exports 60 | types 10
  lib/build-metadata.ts               exports: getBuildRevision
  context/font-provider.tsx           exports: useFont
  i18n/config.ts                      exports: resources, default
  i18n/languages.ts                   types: InterfaceLanguageCode
  lib/roles.ts                        exports: getRoleLabelKey | types: RoleValue
  lib/nav-modules.ts                  exports: parseHeaderNavBoolean, getModuleAccessFromStatus, getModuleAccess, getCachedUserSidebarModules, cacheUserSidebarModules
  stores/system-config-store.ts       exports: getSystemName, getLogo, getFooterHtml
  stores/auth-store.ts                types: UserPermissions
  lib/constants.ts                    exports: STORAGE_KEYS
  lib/utils.ts                        exports: sleep, sanitizeCssVariableName, tryPrettyJson
  lib/format.ts                       exports: formatPercent, formatCurrencyUSD, formatDateTimeStr, formatDateStr, formatTimeStr, formatTimestampForInput, parseTimestampFromInput, stringToColor
  lib/currency.ts                     exports: isCurrencyDisplayType, parseCurrencyDisplayType
  lib/motion.ts                       exports: SIDEBAR_STAGGER_VARIANTS, SIDEBAR_ITEM_VARIANTS
  lib/passkey.ts                      exports: base64UrlToArrayBuffer, arrayBufferToBase64Url, getCredential
  lib/time.ts                         exports: dateToUnixTimestamp, toStartOfDay, getStartOfDay, getEndOfDay, getNormalizedDateRange, formatDate
  lib/theme-radius.ts                 exports: resolveThemeRadiusPx
  lib/colors.ts                       exports: colorToBgClass, avatarColorMap, getAvatarColorClass, CHART_COLORS, getChartColor, ANNOUNCEMENT_TYPE_COLORS | types: AnnouncementType
  lib/oauth.ts                        exports: getOAuthState
  lib/http-status-code-rules.ts       types: StatusCodeRange
  lib/tanstack-table.ts               types: AppTableFeatures
  hooks/index.ts                      exports: useSystemConfig, useTopNavLinks, useNotifications, useDebounce
  hooks/use-dialog.ts                 exports: useDialog | types: DialogHandlers
  hooks/use-debounce.ts               exports: useDebounce
  hooks/use-table-url-state.ts        types: NavigateFn
  components/page-transition.tsx      exports: TableStaggerContainer, TableStaggerRow
  components/status-badge.tsx         exports: dotColorMap, textColorMap, statusPresets, useStatusPresetLabel | types: StatusPreset
  components/model-group-selector.tsx  exports: ModelSelector, GroupSelector
  components/multi-select.tsx         types: Option
```

另有存量失效登记 `systemSettings.fields.uptimeKumo`（`src/i18n/static-keys.ts`，字典中不存在对应 key，d22217c26 引入）。

---

## 修复规划（2026-09-28 复核后拟定，备用）

> 本节基于同日复核结论拟定。复核方式：重跑 `bun run knip --reporter json --no-exit-code` 与本文档附录逐符号双向比对；对全部 450 个符号做全仓 `rg -w` 扫描（含测试文件）排查动态/字符串/测试消费；人工核验第二部分处置口径各项声明。
> **执行状态（2026-09-28）**：批次 0-5 已全部执行完毕，终态 typecheck / lint / bun test（34 用例）/ 生产构建 / i18n:sync 全绿，knip 未使用导出与类型仅剩显式豁免项（prompt-input.tsx 保留面 68 项 + loadVisibilityState 测试豁免 1 项），未用文件/依赖/重复导出类目归零。改动未提交，留在工作区待审。

### 0. 复核结论与前置修正（批次 0，先于任何代码清理）

复核确认：总量（142 文件 / 328 exports / 122 types）、未引用文件与依赖类目归零、附录逐符号清单、6 个 barrel 行数、`i18n/config.ts` 与 `main.tsx:40` 描述、同名符号（`modelFormSchema`/`QuotaType` 均为两处独立定义而非 re-export 链）、`uptimeKumo` 失效登记均属实。以下 4 处需先修正本文档：

1. **「一、分布概览」"其余 36 个文件 | 78 | 30" 行数字错误**：实际为 **42 文件 / 70 exports / 20 types**（10 个小 feature 域合计 14 文件 / 10 / 10，加 lib 组 28 文件 / 60 / 10）。
2. **附录 lib 组组头「文件 18 | exports 61 | types 8」错误**：正文实际列 **28 个文件**（与 knip 一致），exports **60**、types **10**。
3. **口径 1 status-badge 案例修正**：`useStatusPresetLabel` 全仓零消费（文件内亦无人调用），`statusPresets`/`StatusPreset` 仅被其消费，三者按死链级联删除；`dotColorMap` 仅被类型层 `keyof` 消费、键集与存活的 `badgeSurfaceMap` 完全一致，整体删除并将 `StatusVariant` 改由 `badgeSurfaceMap` 派生；`textColorMap` **零使用**，整体删除。
4. **附录 `use-persistent-column-visibility.ts` 的 `loadVisibilityState` 加豁免标注**：被 `use-persistent-column-visibility.test.ts:21` 真实导入（knip 忽略测试文件故仍报死），**直接删除会挂 `bun test`**。处置二选一：保留导出并在清单标注豁免，或连同测试一起删除（需确认该测试的回归价值）。

另记录两处口径补充：口径 4 所指的审计行号引用实际落在 `code-block.tsx:129` 与 `response.tsx:22/46`（`docs/PRD/差异性/安全性.md`），不涉及 `prompt-input.tsx` 本身；同名符号除 `modelFormSchema`/`QuotaType` 外，`vendorFormSchema` 亦为双份独立定义（`models/lib/model-form.ts` 份已死、`models/types.ts` 份存活）。

### 1. 清理顺序（风险从低到高，文件独占、每批 ≤ 4 并行）

| 批次 | 范围 | 操作要点 |
|---|---|---|
| 0 | 本文档 | 完成上述 4 处前置修正 |
| 1 | 6 个 barrel：`components/layout/index.ts`（18+10）、`hooks/index.ts`（4）、`components/data-table/index.ts`（4+1）、`features/pricing/components/index.ts`（3）、`features/keys/lib/index.ts`（1）、`features/dashboard/lib/index.ts`（4） | 仅删 barrel 死行；删除前逐行确认源符号经深路径存活（复核已抽查：`ModelDetails`、`useSystemConfig`、`DataTablePagination`、`resolveSidebarView`、`getLatencyColorClass` 等均深路径消费）。注意 `data-table/index.ts` 的 `export { DataTablePage, type DataTablePageProps }` 同行混合生死，只去掉 `type DataTablePageProps`；`keys/lib/index.ts` 为块状 re-export，只删 `API_KEY_FORM_DEFAULT_VALUES` 标识符 |
| 2 | `assets/brand-icons`：14 个 `icon-*.tsx` + `index.ts` 对应 14 行 re-export | 复核确认整链死（无深路径消费），文件与 barrel 行同删 |
| 3 | lib 工具层：`lib/format`（8）、`lib/time`（6）、`lib/colors`（6+1 type）、`lib/utils`（3）、`lib/passkey`（3）、`lib/nav-modules`（5）、`lib/roles`、`lib/currency`（2）、`lib/motion`（2）、`lib/theme-radius`、`lib/oauth`、`lib/http-status-code-rules`、`lib/tanstack-table`、`lib/constants`、`lib/build-metadata`、`i18n/config.ts`（`resources` 去 export——它被本文件 `i18n.init` 内部消费；`default` 去 export，保留实例初始化） | 先判定"unexport 还是删除"（见 3.1），再动手 |
| 4 | feature 散项（按域拆小批）：channels(10) → pricing+models → usage-logs → system-settings → auth/users/keys/dashboard/playground → 其余小域（wallet/profile/rankings/redemption-codes/audit-logs/performance-metrics/dynamic-ratio/voice-management/multimodal/key-query） | 同名活份高发区，编辑时锚定精确文件路径（见 3.2） |
| 5 | 根组件 + ai-elements：`components/status-badge.tsx`、`page-transition.tsx`、`model-group-selector.tsx`、`multi-select.tsx`、`hooks/use-dialog.ts`、`use-debounce.ts`、`use-table-url-state.ts`、`stores/system-config-store.ts`、`stores/auth-store.ts`、`context/font-provider.tsx`、`i18n/languages.ts`；ai-elements 的 `code-block.tsx`（`highlightCode`）、`conversation.tsx`、`message.tsx` | `prompt-input.tsx` 按口径 4 **整体保留导出面**不动 |

### 2. 单符号处置判定规则

- **文件内仍被使用（含类型层/派生消费）→ 去 `export` 降级私有**。实际例子：`i18n/config.ts` 的 `resources`/`default`、`ai-elements/code-block.tsx` 的 `highlightCode`、`keys/lib/api-key-form.ts` 的 `API_KEY_FORM_DEFAULT_VALUES`、`playground/types.ts` 的 `ContentPart`、`layout/types.ts` 的 `SidebarViewParent` 与 `data-table-page.tsx` 的 `DataTablePageToolbarProps`（后三者被同文件存活类型引用）。
- **文件内零使用 → 连符号一起删除**；删除后级联失去消费者的私有辅助与常量一并删除（如 `channel-form.ts` 的 `validateJSON`+`validateModelMapping` 链、`status-badge.tsx` 的 `statusPresets`/`useStatusPresetLabel`/`StatusPreset` 链、`usage-logs/constants.ts` 的 `TASK_PLATFORMS`）。
- **删除符号后同步清理该文件内因此孤儿化的 import**，由 lint 门禁兜底。
- **豁免项**：`loadVisibilityState`（测试消费，见批次 0 第 4 条）。

### 3. 风险点

1. **同名活份误删**：以下符号在别的文件存在同名存活定义，删除时必须锚定精确路径——`vendorFormSchema`（types.ts 份活）、`ERROR_MESSAGES`（19+ 文件各有独立定义）、`STORAGE_KEYS`/`DEFAULT_GROUP`/`TIME_RANGE_PRESETS`/`SORT_OPTIONS`/`formatQuota`/`formatTimestamp`/`formatRelativeTime`/`normalizeTierLabel`/`normalizeCondition`/`validateJSON`/`getOAuthState`/`getUserId`/`bindEmail`/`stringToColor`（`lib/colors.ts:178` 有独立实现，删的是 `lib/format.ts:231` 份）/`SIDEBAR_WIDTH`/`formatDate`。
2. **i18n 文案漂移**：删除常量映射（`MJ_TASK_STATUS`、`LOG_TYPES`、`ANNOUNCEMENT_TYPE_COLORS`、`STATUS_CONFIG`、`PAYMENT_METHOD_NAMES`、section-registry `titleKey` 等）可能连带删除映射表内 i18n key，批后跑 `bun run i18n:sync` 并确认无意外 diff；`static-keys.ts` 中失效登记（含 `uptimeKumo`）一并清理。
3. **安全审计行号失效**：`安全性.md` 以行号引用 `code-block.tsx:129`、`response.tsx:22/46`；`code-block.tsx` 的 `highlightCode` 若删除（其实现含该 `dangerouslySetInnerHTML` 站点），必须同步更新 `安全性.md`。
4. **prompt-input.tsx**：vendored 组件库式 API，子组件由内部组装，整体保留导出面，不 unexport。
5. **barrel 行误删**：源符号可能同时存在"深路径存活"与"整链死"两种状态（brand-icons 为整链死、其余 barrel 抽查均为仅 barrel 层死），删行前逐行 grep 确认。

### 4. 验证方式（每批完成后全绿才进下一批）

```
cd web
bun run typecheck   # tsc -b
bun run lint        # eslint
bun test            # 真实运行测试（loadVisibilityState 豁免项在此暴露）
bun run build       # rsbuild 生产构建
bun run knip --no-exit-code   # 对照清单核减量；终态应仅剩豁免项与 prompt-input 保留项
bun run i18n:sync   # 确认双语文案无漂移
```

- 终态目标：`bun run knip` 报告中未使用导出/类型归零或仅剩显式豁免项；`git diff` 不含任何与本清单无关的改动。
- 全部批次完成后由 `.githooks/pre-push`（`./scripts/ci-check.sh` 全量门禁）做最终把关。
