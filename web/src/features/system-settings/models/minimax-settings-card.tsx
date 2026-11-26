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
import {
  useEffect,
  useRef,
  useState,
  type ComponentProps,
  type ReactNode,
} from 'react'
import * as z from 'zod'
import { useForm, useFormContext } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { createPortal } from 'react-dom'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { Form, FormControl, FormField } from '@/components/ui/form'
import { Input } from '@/components/ui/input'
import { Switch } from '@/components/ui/switch'
import { ModelMappingEditor } from '@/features/channels/components/model-mapping-editor'
import { EditorModeButton } from '../components/editor-mode-button'
import { SettingsForm } from '../components/settings-form-layout'
import {
  SettingsControl,
  SettingsGroup,
  SettingsGroupSwitch,
  SettingsRow,
} from '../components/settings-group'
import { SettingsPageFormActions } from '../components/settings-page-context'
import { SettingsSection } from '../components/settings-section'
import { useUpdateOption } from '../hooks/use-update-option'
import {
  formatJsonForTextarea,
  normalizeJsonString,
  validateJsonString,
} from './utils'

function jsonMapField(value: string, ctx: z.RefinementCtx) {
  const result = validateJsonString(value, { allowEmpty: false })
  if (!result.valid) {
    ctx.addIssue({
      code: z.ZodIssueCode.custom,
      message: result.message || 'common.errors.invalidJson',
    })
  }
}

// 音色白名单/重定向已迁移到数据库音色管理表，这里只保留 TTS 增强（模型/情绪/语气词）。
const schema = z.object({
  minimax: z.object({
    enabled: z.boolean(),
    model_redirect: z.string().superRefine(jsonMapField),
    voice_whitelist_enabled: z.boolean(),
    custom_voice_enabled: z.boolean(),
    custom_voice_group: z.string(),
    custom_voice_billing_model_id: z.string(),
    emotion_pattern: z.string(),
    emotion_redirect: z.string().superRefine(jsonMapField),
    tone_word_pattern: z.string(),
    tone_word_redirect: z.string().superRefine(jsonMapField),
  }),
})

type MiniMaxFormValues = z.output<typeof schema>
type MiniMaxFormInput = z.input<typeof schema>

type FlatMiniMaxSettings = {
  'minimax.enabled': boolean
  'minimax.model_redirect': string
  'minimax.voice_whitelist_enabled': boolean
  'minimax.custom_voice_enabled': boolean
  'minimax.custom_voice_group': string
  'minimax.custom_voice_billing_model_id': string
  'minimax.emotion_pattern': string
  'minimax.emotion_redirect': string
  'minimax.tone_word_pattern': string
  'minimax.tone_word_redirect': string
}

type MiniMaxJsonMapKey =
  | 'minimax.model_redirect'
  | 'minimax.emotion_redirect'
  | 'minimax.tone_word_redirect'

interface Props {
  defaultValues: MiniMaxFormInput
}

export function MiniMaxSettingsCard({ defaultValues }: Props) {
  const { t } = useTranslation()
  const updateOption = useUpdateOption()

  const buildFormDefaults = (values: MiniMaxFormInput): MiniMaxFormInput => ({
    minimax: {
      enabled: values.minimax.enabled ?? false,
      model_redirect: formatJsonForTextarea(values.minimax.model_redirect),
      voice_whitelist_enabled: values.minimax.voice_whitelist_enabled ?? false,
      custom_voice_enabled: values.minimax.custom_voice_enabled ?? false,
      custom_voice_group: values.minimax.custom_voice_group ?? '',
      custom_voice_billing_model_id:
        values.minimax.custom_voice_billing_model_id ?? '',
      emotion_pattern: values.minimax.emotion_pattern ?? '',
      emotion_redirect: formatJsonForTextarea(values.minimax.emotion_redirect),
      tone_word_pattern: values.minimax.tone_word_pattern ?? '',
      tone_word_redirect: formatJsonForTextarea(
        values.minimax.tone_word_redirect
      ),
    },
  })

  const buildNormalizedDefaults = (
    values: MiniMaxFormInput
  ): FlatMiniMaxSettings => ({
    'minimax.enabled': values.minimax.enabled ?? false,
    'minimax.model_redirect': normalizeJsonString(
      values.minimax.model_redirect
    ),
    'minimax.voice_whitelist_enabled':
      values.minimax.voice_whitelist_enabled ?? false,
    'minimax.custom_voice_enabled':
      values.minimax.custom_voice_enabled ?? false,
    'minimax.custom_voice_group': values.minimax.custom_voice_group ?? '',
    'minimax.custom_voice_billing_model_id':
      values.minimax.custom_voice_billing_model_id ?? '',
    'minimax.emotion_pattern': values.minimax.emotion_pattern ?? '',
    'minimax.emotion_redirect': normalizeJsonString(
      values.minimax.emotion_redirect
    ),
    'minimax.tone_word_pattern': values.minimax.tone_word_pattern ?? '',
    'minimax.tone_word_redirect': normalizeJsonString(
      values.minimax.tone_word_redirect
    ),
  })

  const normalizedDefaultsRef = useRef<FlatMiniMaxSettings>(
    buildNormalizedDefaults(defaultValues)
  )

  const form = useForm<MiniMaxFormInput, unknown, MiniMaxFormValues>({
    resolver: zodResolver(schema),
    defaultValues: buildFormDefaults(defaultValues),
  })
  const enabled = form.watch('minimax.enabled')
  const customVoiceEnabled = form.watch('minimax.custom_voice_enabled')

  const syncJsonMapBaseline =
    (key: MiniMaxJsonMapKey, fieldName: MiniMaxJsonMapKey) =>
    (loadedValue: string) => {
      const safeValue = loadedValue.trim() ? loadedValue : '{}'
      normalizedDefaultsRef.current = {
        ...normalizedDefaultsRef.current,
        [key]: normalizeJsonString(safeValue),
      }
      form.setValue(fieldName, formatJsonForTextarea(safeValue), {
        shouldDirty: false,
        shouldTouch: false,
        shouldValidate: false,
      })
    }

  useEffect(() => {
    normalizedDefaultsRef.current = buildNormalizedDefaults(defaultValues)
    form.reset(buildFormDefaults(defaultValues))
  }, [defaultValues, form])

  const onSubmit = async (values: MiniMaxFormValues) => {
    const normalized: FlatMiniMaxSettings = {
      'minimax.enabled': values.minimax.enabled,
      'minimax.model_redirect': normalizeJsonString(
        values.minimax.model_redirect
      ),
      'minimax.voice_whitelist_enabled': values.minimax.voice_whitelist_enabled,
      'minimax.custom_voice_enabled': values.minimax.custom_voice_enabled,
      'minimax.custom_voice_group': values.minimax.custom_voice_group,
      'minimax.custom_voice_billing_model_id':
        values.minimax.custom_voice_billing_model_id,
      'minimax.emotion_pattern': values.minimax.emotion_pattern,
      'minimax.emotion_redirect': normalizeJsonString(
        values.minimax.emotion_redirect
      ),
      'minimax.tone_word_pattern': values.minimax.tone_word_pattern,
      'minimax.tone_word_redirect': normalizeJsonString(
        values.minimax.tone_word_redirect
      ),
    }

    const updates = (
      Object.keys(normalized) as Array<keyof FlatMiniMaxSettings>
    ).filter((key) => normalized[key] !== normalizedDefaultsRef.current[key])

    if (updates.length === 0) {
      toast.info(t('channels.fields.noChangesToSave'))
      return
    }

    for (const key of updates) {
      const value = normalized[key]
      await updateOption.mutateAsync({
        key,
        value,
      })
    }
  }

  return (
    <SettingsSection title={t('systemSettings.fields.miniMaxTtsEnhancement')}>
      <Form {...form}>
        <SettingsForm onSubmit={form.handleSubmit(onSubmit)}>
          <SettingsPageFormActions
            onSave={form.handleSubmit(onSubmit)}
            isSaving={updateOption.isPending}
          />

          <SettingsGroup
            title={t('systemSettings.actions.enableTtsEnhancement')}
            description={t(
              'systemSettings.status.enabledModelRedirectEmotionTagAndToneWordTag'
            )}
            control={<SettingsGroupSwitch name='minimax.enabled' />}
            enabled={enabled}
          >
            <MappingRow
              name='minimax.model_redirect'
              label={t('systemSettings.fields.modelRedirect')}
              description={
                <>
                  {t('systemSettings.tips.jsonMapOfClientModelNameToMiniMax')}{' '}
                  {`{ "tts-1": "speech-01-turbo", "tts-1-hd": "speech-01-240228" }`}
                </>
              }
              editor={{
                optionKey: 'minimax.model_redirect',
                fromLabel: 'systemSettings.fields.clientModel',
                toLabel: 'systemSettings.fields.minimaxModel',
                fromPlaceholder: 'tts-1',
                toPlaceholder: 'speech-01-turbo',
                jsonPlaceholder: t('common.tips.clientModelMinimaxModel'),
                template: JSON.stringify(
                  {
                    'tts-1': 'speech-01-turbo',
                    'tts-1-hd': 'speech-01-240228',
                  },
                  null,
                  2
                ),
                emptyText:
                  'common.tips.noRedirectsConfiguredClickAddMappingToGetStarted',
                onFullValueLoaded: syncJsonMapBaseline(
                  'minimax.model_redirect',
                  'minimax.model_redirect'
                ),
              }}
            />

            <FormField
              control={form.control}
              name='minimax.emotion_pattern'
              render={({ field }) => (
                <SettingsRow
                  label={t('systemSettings.fields.emotionPattern')}
                  description={
                    <>
                      {t(
                        'systemSettings.status.regexToExtractMiniMaxVoiceSettingEmotionFrom'
                      )}{' '}
                      {`<tts\\s+emotion="happy">text</tts> -> emotion="happy", text kept; <tts>text</tts> -> tags stripped, text kept`}
                    </>
                  }
                >
                  <SettingsControl>
                    <FormControl>
                      <Input
                        {...field}
                        placeholder={`<tts(?:\\s+emotion="([^"]+)")?>([\\s\\S]*?)</tts>`}
                      />
                    </FormControl>
                  </SettingsControl>
                </SettingsRow>
              )}
            />

            <MappingRow
              name='minimax.emotion_redirect'
              label={t('systemSettings.fields.emotionRedirect')}
              description={
                <>
                  {t('systemSettings.tips.jsonMapOfEmotionTagValueToMiniMax')}{' '}
                  {`{ "happy": "happy", "sad": "sad" }`}
                </>
              }
              editor={{
                optionKey: 'minimax.emotion_redirect',
                fromLabel: 'systemSettings.fields.emotionTag',
                toLabel: 'systemSettings.fields.minimaxEmotion',
                fromPlaceholder: 'happy',
                toPlaceholder: 'happy',
                jsonPlaceholder: t('common.tips.tagValueMinimaxEmotion'),
                template: JSON.stringify(
                  { happy: 'happy', sad: 'sad' },
                  null,
                  2
                ),
                emptyText:
                  'common.tips.noRedirectsConfiguredClickAddMappingToGetStarted',
                onFullValueLoaded: syncJsonMapBaseline(
                  'minimax.emotion_redirect',
                  'minimax.emotion_redirect'
                ),
              }}
            />

            <FormField
              control={form.control}
              name='minimax.tone_word_pattern'
              render={({ field }) => (
                <SettingsRow
                  label={t('systemSettings.fields.toneWordPattern')}
                  description={
                    <>
                      {t(
                        'systemSettings.tips.regexToIdentifyToneWordTagsInTextOnly'
                      )}{' '}
                      {`\\(([^()]+)\\) -> (laugh) (crying)`}
                    </>
                  }
                >
                  <SettingsControl>
                    <FormControl>
                      <Input {...field} placeholder={`\\(([^()]+)\\)`} />
                    </FormControl>
                  </SettingsControl>
                </SettingsRow>
              )}
            />

            <MappingRow
              name='minimax.tone_word_redirect'
              label={t('systemSettings.fields.toneWordRedirect')}
              description={
                <>
                  {t(
                    'systemSettings.status.jsonMapOfToneWordTagValueToReplacement'
                  )}{' '}
                  {JSON.stringify({
                    laughs: t('systemSettings.tips.toneWordSampleLaugh'),
                    crying: t('systemSettings.tips.toneWordSampleCry'),
                  })}
                </>
              }
              editor={{
                optionKey: 'minimax.tone_word_redirect',
                fromLabel: 'systemSettings.fields.toneWordTag',
                toLabel: 'systemSettings.fields.replacementText',
                fromPlaceholder: 'laughs',
                toPlaceholder: t('systemSettings.tips.toneWordSampleLaugh'),
                jsonPlaceholder: t('common.fields.tagValueReplacement'),
                template: JSON.stringify(
                  {
                    laughs: t('systemSettings.tips.toneWordSampleLaugh'),
                    crying: t('systemSettings.tips.toneWordSampleCry'),
                  },
                  null,
                  2
                ),
                emptyText:
                  'common.tips.noRedirectsConfiguredClickAddMappingToGetStarted',
                onFullValueLoaded: syncJsonMapBaseline(
                  'minimax.tone_word_redirect',
                  'minimax.tone_word_redirect'
                ),
              }}
            />
          </SettingsGroup>

          <SettingsGroup
            title={t('multimodal.fields.customVoice')}
            description={t(
              'systemSettings.status.enabledUsersCanUseTheCustomVoicePageTo'
            )}
            control={
              <SettingsGroupSwitch name='minimax.custom_voice_enabled' />
            }
            enabled={customVoiceEnabled}
          >
            <FormField
              control={form.control}
              name='minimax.custom_voice_group'
              render={({ field }) => (
                <SettingsRow
                  label={t('systemSettings.fields.customVoiceGroup')}
                  description={t(
                    'systemSettings.status.userGroupUsedByTheCustomVoicePageIt'
                  )}
                >
                  <SettingsControl>
                    <FormControl>
                      <Input {...field} placeholder='default' />
                    </FormControl>
                  </SettingsControl>
                </SettingsRow>
              )}
            />

            <FormField
              control={form.control}
              name='minimax.custom_voice_billing_model_id'
              render={({ field }) => (
                <SettingsRow
                  label={t('systemSettings.fields.customVoiceBillingModelId')}
                  description={t(
                    'systemSettings.errors.modelIdChargedOnceWhenAUserConfirmsVoice'
                  )}
                >
                  <SettingsControl>
                    <FormControl>
                      <Input
                        {...field}
                        placeholder={t(
                          'systemSettings.placeholders.voiceCustomization'
                        )}
                      />
                    </FormControl>
                  </SettingsControl>
                </SettingsRow>
              )}
            />
          </SettingsGroup>

          <SettingsGroup>
            <FormField
              control={form.control}
              name='minimax.voice_whitelist_enabled'
              render={({ field }) => (
                <SettingsRow
                  label={t('systemSettings.actions.enableVoiceWhitelist')}
                  description={t(
                    'systemSettings.status.enabledOnlyVoicesMarkedAsAllowedInVoiceManagement'
                  )}
                >
                  <FormControl>
                    <Switch
                      checked={field.value}
                      onCheckedChange={field.onChange}
                    />
                  </FormControl>
                </SettingsRow>
              )}
            />
          </SettingsGroup>
        </SettingsForm>
      </Form>
    </SettingsSection>
  )
}

type MappingRowProps = {
  name: MiniMaxJsonMapKey
  label: ReactNode
  description: ReactNode
  /** 映射编辑器除 value、onChange 以外的属性。 */
  editor: Omit<
    ComponentProps<typeof ModelMappingEditor>,
    'value' | 'onChange' | 'renderModeToggle'
  >
}

/**
 * 带映射编辑器的通栏设置行：「JSON / 可视」切换按钮挂在标题行右侧，
 * 编辑器的其余工具留在内容上方。
 */
function MappingRow({ name, label, description, editor }: MappingRowProps) {
  const form = useFormContext<MiniMaxFormInput, unknown, MiniMaxFormValues>()
  const [actionsHost, setActionsHost] = useState<HTMLDivElement | null>(null)

  if (form === null) {
    throw new Error('MappingRow 必须位于表单内。')
  }

  return (
    <FormField
      control={form.control}
      name={name}
      render={({ field }) => (
        <SettingsRow
          label={label}
          description={description}
          stacked
          actions={
            <div ref={setActionsHost} className='flex items-center gap-2' />
          }
        >
          <FormControl>
            <ModelMappingEditor
              value={field.value}
              onChange={field.onChange}
              renderModeToggle={({ visualMode, toggle }) =>
                actionsHost === null
                  ? null
                  : createPortal(
                      <EditorModeButton
                        visualMode={visualMode}
                        onToggle={toggle}
                      />,
                      actionsHost
                    )
              }
              {...editor}
            />
          </FormControl>
        </SettingsRow>
      )}
    />
  )
}
