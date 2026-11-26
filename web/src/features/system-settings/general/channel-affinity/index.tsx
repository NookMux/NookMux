/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import { useCallback, useEffect, useState } from 'react'
import { Edit, FileText, Plus, RefreshCw, Trash2, X } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { Empty, EmptyDescription, EmptyHeader } from '@/components/ui/empty'
import { Input } from '@/components/ui/input'
import { Switch } from '@/components/ui/switch'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { Textarea } from '@/components/ui/textarea'
import { ConfirmDialog } from '@/components/confirm-dialog'
import { StatusBadge, StatusBadgeList } from '@/components/status-badge'
import { EditorModeButton } from '../../components/editor-mode-button'
import {
  SettingsControl,
  SettingsGroup,
  SettingsGroupSwitch,
  SettingsRow,
} from '../../components/settings-group'
import { SettingsPageFormActions } from '../../components/settings-page-context'
import {
  SettingsPanel,
  SettingsStat,
  SettingsStatGrid,
} from '../../components/settings-panel'
import { SettingsSection } from '../../components/settings-section'
import { useUpdateOption } from '../../hooks/use-update-option'
import { getCacheStats, clearAllCache, clearRuleCache } from './api'
import { RULE_TEMPLATES, cloneTemplate, makeUniqueName } from './constants'
import { RuleEditorDialog } from './rule-editor-dialog'
import type { AffinityRule, CacheStats, ChannelAffinitySettings } from './types'

function parseRules(jsonStr: string): AffinityRule[] {
  try {
    const arr = JSON.parse(jsonStr || '[]')
    if (!Array.isArray(arr)) return []
    return arr.map(
      (r: Record<string, unknown>, i: number) =>
        ({ id: i, ...r }) as AffinityRule
    )
  } catch {
    return []
  }
}

/** 规则列表的归一化文本：去掉本地 id、补齐布尔字段，用来判断规则相对已保存的值是否有改动。 */
function normalizeRulesJson(list: unknown): string | null {
  if (!Array.isArray(list)) return null
  return JSON.stringify(
    list.map((item) => {
      const { id: _id, ...rule } = item as AffinityRule
      return { ...rule, skip_retry_on_failure: !!rule.skip_retry_on_failure }
    })
  )
}

/** 已保存规则的归一化文本；已保存的值无法解析时为 null。 */
function parseSavedRulesJson(raw: string): string | null {
  try {
    return normalizeRulesJson(JSON.parse(raw))
  } catch {
    return null
  }
}

function RuleBadgeList(props: { items: string[] }) {
  return (
    <StatusBadgeList
      items={props.items}
      max={2}
      getKey={(item) => item}
      renderItem={(item) => (
        <StatusBadge
          label={item}
          variant='neutral'
          size='sm'
          copyable={false}
        />
      )}
    />
  )
}

function serializeRules(rules: AffinityRule[]): string {
  return JSON.stringify(
    rules.map(({ id: _, ...rest }) => ({
      ...rest,
      skip_retry_on_failure: !!rest.skip_retry_on_failure,
    }))
  )
}

interface Props {
  defaultValues: ChannelAffinitySettings
}

export function ChannelAffinitySection(props: Props) {
  const { t } = useTranslation()
  const updateOption = useUpdateOption()

  const [enabled, setEnabled] = useState(
    props.defaultValues['channel_affinity_setting.enabled']
  )
  const [switchOnSuccess, setSwitchOnSuccess] = useState(
    props.defaultValues['channel_affinity_setting.switch_on_success']
  )
  const [keepOnChannelDisabled, setKeepOnChannelDisabled] = useState(
    props.defaultValues['channel_affinity_setting.keep_on_channel_disabled']
  )
  const [maxEntries, setMaxEntries] = useState(
    props.defaultValues['channel_affinity_setting.max_entries']
  )
  const [defaultTtl, setDefaultTtl] = useState(
    props.defaultValues['channel_affinity_setting.default_ttl_seconds']
  )
  const [rules, setRules] = useState<AffinityRule[]>(() =>
    parseRules(props.defaultValues['channel_affinity_setting.rules'])
  )

  const [editMode, setEditMode] = useState<'visual' | 'json'>('visual')
  const [jsonText, setJsonText] = useState(() =>
    JSON.stringify(
      parseRules(props.defaultValues['channel_affinity_setting.rules']).map(
        ({ id: _, ...r }) => r
      ),
      null,
      2
    )
  )
  const [cacheStats, setCacheStats] = useState<CacheStats | null>(null)
  const [cacheLoading, setCacheLoading] = useState(false)
  const [saving, setSaving] = useState(false)

  const [ruleEditorOpen, setRuleEditorOpen] = useState(false)
  const [editingRule, setEditingRule] = useState<AffinityRule | null>(null)
  const [ruleTemplateKey, setRuleTemplateKey] = useState<string | null>(null)
  const [clearAllDialogOpen, setClearAllDialogOpen] = useState(false)
  const [clearRuleName, setClearRuleName] = useState<string | null>(null)
  const [fillTemplateDialogOpen, setFillTemplateDialogOpen] = useState(false)

  const loadDefaults = useCallback(() => {
    setEnabled(props.defaultValues['channel_affinity_setting.enabled'])
    setSwitchOnSuccess(
      props.defaultValues['channel_affinity_setting.switch_on_success']
    )
    setKeepOnChannelDisabled(
      props.defaultValues['channel_affinity_setting.keep_on_channel_disabled']
    )
    setMaxEntries(props.defaultValues['channel_affinity_setting.max_entries'])
    setDefaultTtl(
      props.defaultValues['channel_affinity_setting.default_ttl_seconds']
    )
    const parsed = parseRules(
      props.defaultValues['channel_affinity_setting.rules']
    )
    setRules(parsed)
    setJsonText(
      JSON.stringify(
        parsed.map(({ id: _, ...r }) => r),
        null,
        2
      )
    )
  }, [props.defaultValues])

  useEffect(() => {
    loadDefaults()
  }, [loadDefaults])

  const defaults = props.defaultValues
  const savedRulesJson = parseSavedRulesJson(
    defaults['channel_affinity_setting.rules'] || '[]'
  )
  const currentRulesJson =
    editMode === 'json'
      ? (() => {
          try {
            return normalizeRulesJson(JSON.parse(jsonText))
          } catch {
            return null
          }
        })()
      : normalizeRulesJson(rules)
  const isDirty =
    enabled !== defaults['channel_affinity_setting.enabled'] ||
    switchOnSuccess !==
      defaults['channel_affinity_setting.switch_on_success'] ||
    keepOnChannelDisabled !==
      defaults['channel_affinity_setting.keep_on_channel_disabled'] ||
    maxEntries !== defaults['channel_affinity_setting.max_entries'] ||
    defaultTtl !== defaults['channel_affinity_setting.default_ttl_seconds'] ||
    currentRulesJson === null ||
    currentRulesJson !== savedRulesJson

  const refreshCache = useCallback(async () => {
    setCacheLoading(true)
    try {
      const res = await getCacheStats()
      if (res.success) setCacheStats(res.data || null)
    } catch {
      toast.error(t('systemSettings.errors.failedToRefreshCacheStats'))
    } finally {
      setCacheLoading(false)
    }
  }, [t])

  useEffect(() => {
    refreshCache()
  }, [refreshCache])

  const appendCliTemplates = () => {
    const existingNames = new Set(
      rules.map((r) => (r.name || '').trim()).filter((x) => x.length > 0)
    )

    const templates = Object.values(RULE_TEMPLATES).map((tpl) => {
      const base = cloneTemplate(tpl)
      const name = makeUniqueName(existingNames, tpl.name)
      existingNames.add(name)
      return { ...base, name }
    })

    setRules((prev) =>
      [...prev, ...templates].map((r, idx) => ({ ...r, id: idx }))
    )
    toast.success(t('systemSettings.fields.templatesAppended'))
    setFillTemplateDialogOpen(false)
  }

  const handleFillTemplates = () => {
    if (rules.length === 0) {
      appendCliTemplates()
    } else {
      setFillTemplateDialogOpen(true)
    }
  }

  const handleSave = async () => {
    let rulesJson: string
    if (editMode === 'json') {
      try {
        const parsed = JSON.parse(jsonText)
        if (!Array.isArray(parsed)) {
          toast.error(t('systemSettings.errors.rulesJsonMustBeAnArray'))
          return
        }
        rulesJson = JSON.stringify(parsed)
      } catch {
        toast.error(t('systemSettings.errors.invalidRulesJsonFormat'))
        return
      }
    } else {
      rulesJson = serializeRules(rules)
    }

    setSaving(true)
    try {
      const updates: { key: string; value: string }[] = []

      if (enabled !== props.defaultValues['channel_affinity_setting.enabled'])
        updates.push({
          key: 'channel_affinity_setting.enabled',
          value: String(enabled),
        })
      if (
        switchOnSuccess !==
        props.defaultValues['channel_affinity_setting.switch_on_success']
      )
        updates.push({
          key: 'channel_affinity_setting.switch_on_success',
          value: String(switchOnSuccess),
        })
      if (
        keepOnChannelDisabled !==
        props.defaultValues['channel_affinity_setting.keep_on_channel_disabled']
      )
        updates.push({
          key: 'channel_affinity_setting.keep_on_channel_disabled',
          value: String(keepOnChannelDisabled),
        })
      if (
        maxEntries !==
        props.defaultValues['channel_affinity_setting.max_entries']
      )
        updates.push({
          key: 'channel_affinity_setting.max_entries',
          value: String(maxEntries),
        })
      if (
        defaultTtl !==
        props.defaultValues['channel_affinity_setting.default_ttl_seconds']
      )
        updates.push({
          key: 'channel_affinity_setting.default_ttl_seconds',
          value: String(defaultTtl),
        })

      const origRules = props.defaultValues['channel_affinity_setting.rules']
      const origSerialized = (() => {
        try {
          return JSON.stringify(JSON.parse(origRules || '[]'))
        } catch {
          return '[]'
        }
      })()
      if (rulesJson !== origSerialized) {
        updates.push({
          key: 'channel_affinity_setting.rules',
          value: rulesJson,
        })
      }

      if (updates.length === 0) {
        toast.info(t('systemSettings.fields.noChanges'))
        return
      }

      for (const u of updates) {
        await updateOption.mutateAsync(u)
      }
      toast.success(t('systemSettings.status.savedSuccessfully'))
    } catch {
      toast.error(t('systemSettings.errors.failedToSave'))
    } finally {
      setSaving(false)
    }
  }

  const handleReset = () => {
    loadDefaults()
    toast.success(t('systemSettings.status.formResetToSavedValues'))
  }

  const handleRuleSave = (rule: AffinityRule) => {
    setRules((prev) => {
      const existIdx = prev.findIndex(
        (r) => r.id === rule.id || (rule.name && r.name === editingRule?.name)
      )
      if (existIdx >= 0) {
        const next = [...prev]
        next[existIdx] = { ...rule, id: existIdx }
        return next
      }
      return [...prev, { ...rule, id: prev.length }]
    })
    setEditingRule(null)
  }

  const handleDeleteRule = (idx: number) => {
    setRules((prev) =>
      prev.filter((_, i) => i !== idx).map((r, i) => ({ ...r, id: i }))
    )
    toast.success(t('dynamicRatio.status.deletedSuccessfully'))
  }

  const handleClearAll = async () => {
    const res = await clearAllCache()
    if (res.success) {
      toast.success(t('systemSettings.fields.cleared'))
      refreshCache()
    }
    setClearAllDialogOpen(false)
  }

  const handleClearRule = async () => {
    if (!clearRuleName) return
    const res = await clearRuleCache(clearRuleName)
    if (res.success) {
      toast.success(t('systemSettings.fields.cleared'))
      refreshCache()
    }
    setClearRuleName(null)
  }

  const switchToJsonMode = () => {
    setJsonText(
      JSON.stringify(
        rules.map(({ id: _, ...r }) => r),
        null,
        2
      )
    )
    setEditMode('json')
  }

  const switchToVisualMode = () => {
    try {
      const parsed = JSON.parse(jsonText)
      if (!Array.isArray(parsed)) {
        toast.error(t('systemSettings.errors.rulesJsonMustBeAnArray'))
        return
      }
      setRules(
        parsed.map(
          (r: Record<string, unknown>, i: number) =>
            ({ id: i, ...r }) as AffinityRule
        )
      )
      setEditMode('visual')
    } catch {
      toast.error(t('systemSettings.errors.invalidRulesJsonFormat'))
    }
  }

  return (
    <>
      <SettingsSection title={t('systemSettings.fields.channelAffinity')}>
        <SettingsPageFormActions
          onSave={() => void handleSave()}
          onReset={handleReset}
          isSaving={saving || updateOption.isPending}
          isDirty={isDirty}
        />

        <div className='flex w-full min-w-0 flex-col gap-8'>
          <SettingsPanel
            title={t('pricing.fields.cache')}
            actions={
              <>
                <Button
                  type='button'
                  variant='outline'
                  size='sm'
                  onClick={refreshCache}
                  disabled={cacheLoading}
                >
                  <RefreshCw
                    data-icon='inline-start'
                    className={cacheLoading ? 'animate-spin' : undefined}
                  />
                  {t('systemSettings.actions.refreshCache')}
                </Button>
                <Button
                  type='button'
                  variant='destructive'
                  size='sm'
                  onClick={() => setClearAllDialogOpen(true)}
                >
                  {t('systemSettings.actions.clearAllCache')}
                </Button>
              </>
            }
          >
            <div className='py-2'>
              <SettingsStatGrid columns={3}>
                <SettingsStat
                  label={t('systemSettings.fields.cacheEntries')}
                  value={
                    cacheStats
                      ? `${cacheStats.total} / ${cacheStats.cache_capacity}`
                      : '-'
                  }
                />
              </SettingsStatGrid>
            </div>
          </SettingsPanel>

          <SettingsGroup
            title={t('systemSettings.actions.enableChannelAffinity')}
            description={t(
              'systemSettings.status.channelAffinityReusesTheLastSuccessfulChannelBasedOn'
            )}
            control={
              <SettingsGroupSwitch
                checked={enabled}
                onCheckedChange={setEnabled}
              />
            }
            enabled={enabled}
          >
            <SettingsRow
              label={t('systemSettings.fields.maxEntries')}
              htmlFor='channel-affinity-max-entries'
            >
              <SettingsControl>
                <Input
                  id='channel-affinity-max-entries'
                  type='number'
                  min={0}
                  value={maxEntries}
                  onChange={(event) =>
                    setMaxEntries(Number(event.target.value))
                  }
                />
              </SettingsControl>
            </SettingsRow>
            <SettingsRow
              label={t('systemSettings.fields.defaultTtlSeconds')}
              htmlFor='channel-affinity-default-ttl'
            >
              <SettingsControl>
                <Input
                  id='channel-affinity-default-ttl'
                  type='number'
                  min={0}
                  value={defaultTtl}
                  onChange={(event) =>
                    setDefaultTtl(Number(event.target.value))
                  }
                />
              </SettingsControl>
            </SettingsRow>
            <SettingsRow
              label={t('systemSettings.actions.switchAffinityOnSuccess')}
              description={t(
                'systemSettings.status.ifTheAffinityChannelFailsAndRetrySucceedsOn'
              )}
            >
              <Switch
                checked={switchOnSuccess}
                onCheckedChange={setSwitchOnSuccess}
              />
            </SettingsRow>
            <SettingsRow
              label={t('systemSettings.actions.keepAffinityOnChannelDisabled')}
              description={t(
                'systemSettings.status.whenEnabledKeepTheAffinityEntryEvenIfThe'
              )}
            >
              <Switch
                checked={keepOnChannelDisabled}
                onCheckedChange={setKeepOnChannelDisabled}
              />
            </SettingsRow>
            <SettingsRow
              label={t('systemSettings.fields.rule')}
              stacked
              actions={
                <EditorModeButton
                  visualMode={editMode === 'visual'}
                  onToggle={
                    editMode === 'visual'
                      ? switchToJsonMode
                      : switchToVisualMode
                  }
                />
              }
            >
              {editMode === 'visual' ? (
                <div className='flex flex-col gap-4'>
                  <div className='flex flex-wrap items-center gap-2 sm:justify-end'>
                    <DropdownMenu>
                      <DropdownMenuTrigger
                        render={
                          <Button type='button' variant='outline' size='sm' />
                        }
                      >
                        <Plus data-icon='inline-start' />
                        {t('systemSettings.actions.addRule')}
                      </DropdownMenuTrigger>
                      <DropdownMenuContent>
                        <DropdownMenuItem
                          onClick={() => {
                            setEditingRule(null)
                            setRuleTemplateKey(null)
                            setRuleEditorOpen(true)
                          }}
                        >
                          {t('systemSettings.fields.blankRule')}
                        </DropdownMenuItem>
                        <DropdownMenuItem
                          onClick={() => {
                            setEditingRule(null)
                            setRuleTemplateKey('codexCli')
                            setRuleEditorOpen(true)
                          }}
                        >
                          Codex CLI
                        </DropdownMenuItem>
                        <DropdownMenuItem
                          onClick={() => {
                            setEditingRule(null)
                            setRuleTemplateKey('claudeCli')
                            setRuleEditorOpen(true)
                          }}
                        >
                          Claude CLI
                        </DropdownMenuItem>
                      </DropdownMenuContent>
                    </DropdownMenu>
                    <Button
                      type='button'
                      variant='outline'
                      size='sm'
                      onClick={handleFillTemplates}
                    >
                      <FileText data-icon='inline-start' />
                      {t('systemSettings.actions.fillTemplates')}
                    </Button>
                  </div>
                  {rules.length === 0 ? (
                    <Empty>
                      <EmptyHeader>
                        <EmptyDescription>
                          {t('systemSettings.fields.noRulesYet')}
                        </EmptyDescription>
                      </EmptyHeader>
                    </Empty>
                  ) : (
                    <div>
                      <Table className='[&_td:first-child]:pl-0 [&_td:last-child]:pr-0 [&_th:first-child]:pl-0 [&_th:last-child]:pr-0'>
                        <TableHeader>
                          <TableRow>
                            <TableHead>{t('channels.fields.name')}</TableHead>
                            <TableHead>
                              {t('systemSettings.fields.modelRegex')}
                            </TableHead>
                            <TableHead>
                              {t('systemSettings.fields.keySources')}
                            </TableHead>
                            <TableHead>
                              {t('systemSettings.fields.ttl')}
                            </TableHead>
                            <TableHead>{t('common.actions.retry')}</TableHead>
                            <TableHead>
                              {t('systemSettings.fields.scope')}
                            </TableHead>
                            <TableHead>{t('pricing.fields.cache')}</TableHead>
                            <TableHead className='text-right'>
                              {t('channels.fields.actions')}
                            </TableHead>
                          </TableRow>
                        </TableHeader>
                        <TableBody>
                          {rules.map((rule, idx) => (
                            <TableRow key={idx}>
                              <TableCell className='font-medium'>
                                {rule.name || '-'}
                              </TableCell>
                              <TableCell>
                                <RuleBadgeList items={rule.model_regex || []} />
                              </TableCell>
                              <TableCell>
                                <RuleBadgeList
                                  items={(rule.key_sources || []).map(
                                    (src) =>
                                      `${src.type}:${src.type === 'gjson' ? src.path : src.key}`
                                  )}
                                />
                              </TableCell>
                              <TableCell>{rule.ttl_seconds || '-'}</TableCell>
                              <TableCell>
                                <StatusBadge
                                  label={
                                    rule.skip_retry_on_failure
                                      ? t('systemSettings.fields.noRetry')
                                      : t('common.actions.retry')
                                  }
                                  variant={
                                    rule.skip_retry_on_failure
                                      ? 'danger'
                                      : 'neutral'
                                  }
                                  copyable={false}
                                />
                              </TableCell>
                              <TableCell>
                                {(() => {
                                  const scopeItems = [
                                    rule.include_using_group &&
                                      t('common.fields.group'),
                                    rule.include_model_name &&
                                      t('common.fields.model'),
                                    rule.include_rule_name &&
                                      t('systemSettings.fields.rule'),
                                  ].filter(Boolean) as string[]
                                  if (scopeItems.length === 0) return '-'
                                  return <RuleBadgeList items={scopeItems} />
                                })()}
                              </TableCell>
                              <TableCell>
                                {rule.include_rule_name &&
                                cacheStats?.by_rule_name
                                  ? cacheStats.by_rule_name[rule.name] || 0
                                  : 'N/A'}
                              </TableCell>
                              <TableCell className='text-right'>
                                <div className='flex justify-end gap-2'>
                                  {rule.include_rule_name && (
                                    <Button
                                      type='button'
                                      variant='ghost'
                                      size='sm'
                                      onClick={() =>
                                        setClearRuleName(rule.name)
                                      }
                                      title={t(
                                        'systemSettings.actions.clearCacheForThisRule'
                                      )}
                                      aria-label={t(
                                        'systemSettings.actions.clearCacheForThisRule'
                                      )}
                                    >
                                      <X className='h-4 w-4' />
                                    </Button>
                                  )}
                                  <Button
                                    type='button'
                                    variant='ghost'
                                    size='sm'
                                    onClick={() => {
                                      setEditingRule(rule)
                                      setRuleTemplateKey(null)
                                      setRuleEditorOpen(true)
                                    }}
                                    aria-label={t('channels.actions.edit')}
                                  >
                                    <Edit className='h-4 w-4' />
                                  </Button>
                                  <Button
                                    type='button'
                                    variant='ghost'
                                    size='sm'
                                    onClick={() => handleDeleteRule(idx)}
                                    aria-label={t('common.actions.delete')}
                                  >
                                    <Trash2 className='h-4 w-4' />
                                  </Button>
                                </div>
                              </TableCell>
                            </TableRow>
                          ))}
                        </TableBody>
                      </Table>
                    </div>
                  )}
                </div>
              ) : (
                <Textarea
                  className='min-h-[300px] font-mono text-xs'
                  value={jsonText}
                  onChange={(event) => setJsonText(event.target.value)}
                />
              )}
            </SettingsRow>
          </SettingsGroup>
        </div>
      </SettingsSection>

      <RuleEditorDialog
        open={ruleEditorOpen}
        onOpenChange={setRuleEditorOpen}
        rule={editingRule}
        onSave={handleRuleSave}
        templateKey={ruleTemplateKey}
      />

      <ConfirmDialog
        open={clearAllDialogOpen}
        onOpenChange={setClearAllDialogOpen}
        title={t(
          'systemSettings.actions.confirmClearingAllChannelAffinityCache'
        )}
        desc={t(
          'systemSettings.tips.deleteAllChannelAffinityCacheEntriesStillInMemory'
        )}
        handleConfirm={handleClearAll}
        destructive
      />

      {clearRuleName !== null && (
        <ConfirmDialog
          open
          onOpenChange={(value) => !value && setClearRuleName(null)}
          title={t('systemSettings.actions.confirmClearingCacheForThisRule')}
          desc={`${t('systemSettings.fields.rule')}: ${clearRuleName}`}
          handleConfirm={handleClearRule}
          destructive
        />
      )}

      <ConfirmDialog
        open={fillTemplateDialogOpen}
        onOpenChange={setFillTemplateDialogOpen}
        title={t('systemSettings.actions.fillCodexCliClaudeCliTemplates')}
        desc={t('systemSettings.tips.append2TemplateRulesCodexCliAndClaudeCli')}
        handleConfirm={appendCliTemplates}
      />
    </>
  )
}
