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
import { useEffect, useRef } from 'react'
import * as z from 'zod'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { Form, FormControl, FormField } from '@/components/ui/form'
import { Switch } from '@/components/ui/switch'
import { Textarea } from '@/components/ui/textarea'
import { SettingsForm } from '../components/settings-form-layout'
import { SettingsGroup, SettingsRow } from '../components/settings-group'
import { SettingsPageFormActions } from '../components/settings-page-context'
import { SettingsSection } from '../components/settings-section'
import { useUpdateOption } from '../hooks/use-update-option'
import {
  formatJsonForTextarea,
  normalizeJsonString,
  validateJsonString,
} from './utils'

const schema = z.object({
  claude: z.object({
    model_headers_settings: z.string().superRefine((value, ctx) => {
      const result = validateJsonString(value)
      if (!result.valid) {
        ctx.addIssue({
          code: z.ZodIssueCode.custom,
          message: result.message || 'common.errors.invalidJson',
        })
      }
    }),
    default_max_tokens: z.string().superRefine((value, ctx) => {
      const result = validateJsonString(value)
      if (!result.valid) {
        ctx.addIssue({
          code: z.ZodIssueCode.custom,
          message: result.message || 'common.errors.invalidJson',
        })
      }
    }),
    remove_claude_code_billing_header_enabled: z.boolean(),
  }),
})

type ClaudeSettingsFormValues = z.output<typeof schema>
type ClaudeSettingsFormInput = z.input<typeof schema>

type FlatClaudeSettings = {
  'claude.model_headers_settings': string
  'claude.default_max_tokens': string
  'claude.remove_claude_code_billing_header_enabled': boolean
}

type ClaudeSettingsCardProps = {
  defaultValues: ClaudeSettingsFormInput
}

export function ClaudeSettingsCard({ defaultValues }: ClaudeSettingsCardProps) {
  const { t } = useTranslation()
  const updateOption = useUpdateOption()
  const normalizedDefaultsRef = useRef<FlatClaudeSettings>({
    'claude.model_headers_settings': normalizeJsonString(
      defaultValues.claude.model_headers_settings
    ),
    'claude.default_max_tokens': normalizeJsonString(
      defaultValues.claude.default_max_tokens
    ),
    'claude.remove_claude_code_billing_header_enabled':
      defaultValues.claude.remove_claude_code_billing_header_enabled ?? true,
  })

  const buildFormDefaults = (
    values: ClaudeSettingsFormInput
  ): ClaudeSettingsFormInput => ({
    claude: {
      model_headers_settings: formatJsonForTextarea(
        values.claude.model_headers_settings
      ),
      default_max_tokens: formatJsonForTextarea(
        values.claude.default_max_tokens
      ),
      remove_claude_code_billing_header_enabled:
        values.claude.remove_claude_code_billing_header_enabled ?? true,
    },
  })

  const form = useForm<
    ClaudeSettingsFormInput,
    unknown,
    ClaudeSettingsFormValues
  >({
    resolver: zodResolver(schema),
    defaultValues: buildFormDefaults(defaultValues),
  })

  useEffect(() => {
    normalizedDefaultsRef.current = {
      'claude.model_headers_settings': normalizeJsonString(
        defaultValues.claude.model_headers_settings
      ),
      'claude.default_max_tokens': normalizeJsonString(
        defaultValues.claude.default_max_tokens
      ),
      'claude.remove_claude_code_billing_header_enabled':
        defaultValues.claude.remove_claude_code_billing_header_enabled ?? true,
    }

    form.reset(buildFormDefaults(defaultValues))
  }, [defaultValues, form])

  const onSubmit = async (values: ClaudeSettingsFormValues) => {
    const normalized: FlatClaudeSettings = {
      'claude.model_headers_settings': normalizeJsonString(
        values.claude.model_headers_settings
      ),
      'claude.default_max_tokens': normalizeJsonString(
        values.claude.default_max_tokens
      ),
      'claude.remove_claude_code_billing_header_enabled':
        values.claude.remove_claude_code_billing_header_enabled,
    }

    const updates = (
      Object.keys(normalized) as Array<keyof FlatClaudeSettings>
    ).filter((key) => normalized[key] !== normalizedDefaultsRef.current[key])

    if (updates.length === 0) {
      toast.info(t('channels.fields.noChangesToSave'))
      return
    }

    for (const key of updates) {
      await updateOption.mutateAsync({ key, value: normalized[key] })
    }
  }

  return (
    <SettingsSection title={t('systemSettings.fields.claude')}>
      <Form {...form}>
        {}
        <SettingsForm onSubmit={form.handleSubmit(onSubmit)}>
          <SettingsPageFormActions
            onSave={form.handleSubmit(onSubmit)}
            isSaving={updateOption.isPending}
          />
          <SettingsGroup>
            <FormField
              control={form.control}
              name='claude.model_headers_settings'
              render={({ field }) => (
                <SettingsRow
                  label={t('channels.fields.requestHeaderOverride')}
                  description={t(
                    'systemSettings.tips.providePerModelHeaderOverridesAsJsonUsefulFor'
                  )}
                >
                  <FormControl>
                    <Textarea rows={8} {...field} />
                  </FormControl>
                </SettingsRow>
              )}
            />

            <FormField
              control={form.control}
              name='claude.default_max_tokens'
              render={({ field }) => (
                <SettingsRow
                  label={t('systemSettings.fields.defaultMaxTokens')}
                  description={
                    <>
                      {t('channels.placeholders.example')}{' '}
                      {`{ "default": 8192, "claude-3-haiku-20240307": 4096, "claude-3-opus-20240229": 4096, "claude-3-7-sonnet-20250219-thinking": 8192 }`}
                    </>
                  }
                >
                  <FormControl>
                    <Textarea rows={8} {...field} />
                  </FormControl>
                </SettingsRow>
              )}
            />

            <FormField
              control={form.control}
              name='claude.remove_claude_code_billing_header_enabled'
              render={({ field }) => (
                <SettingsRow
                  label={t(
                    'systemSettings.actions.removeClaudeCodeBillingHeader'
                  )}
                  description={t(
                    'systemSettings.tips.removeClaudeCodeBillingHeaderForApiCompatibility'
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
