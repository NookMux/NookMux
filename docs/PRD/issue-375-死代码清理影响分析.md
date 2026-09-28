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
| 其余 36 个文件 | 36 | 78 | 30 | lib/ 工具库（format 8、time 6、colors 6、utils 3、passkey 3、nav-modules 5 等）、hooks/、stores/、i18n/、context/、单文件 feature 与根组件散布项 |

## 二、处置口径

1. **两层裁决**：knip 的「未使用导出」指无外部模块消费该绑定。符号若在本文件内部仍在使用，处置是**移除 `export` 关键字降级为私有**，而非删除符号；文件内也零使用的才整体删除。典型案例：`components/status-badge.tsx` 的 `statusPresets`/`useStatusPresetLabel`/`dotColorMap`/`textColorMap`——组件内部经动态键消费（`t(statusPresets[preset].label)`），只能 unexport。
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
  use-persistent-column-visibility.ts  exports: loadVisibilityState

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

lib、hooks、stores、i18n、context 与根组件 | 文件 18 | exports 61 | types 8
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
