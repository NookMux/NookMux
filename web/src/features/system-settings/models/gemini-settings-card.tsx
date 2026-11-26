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
import { useEffect, useMemo, useRef } from 'react'
import * as z from 'zod'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { Form, FormControl, FormField } from '@/components/ui/form'
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
  gemini: z.object({
    safety_settings: z.string().superRefine((value, ctx) => {
      const result = validateJsonString(value)
      if (!result.valid) {
        ctx.addIssue({
          code: z.ZodIssueCode.custom,
          message: result.message || 'common.errors.invalidJson',
        })
      }
    }),
    version_settings: z.string().superRefine((value, ctx) => {
      const result = validateJsonString(value)
      if (!result.valid) {
        ctx.addIssue({
          code: z.ZodIssueCode.custom,
          message: result.message || 'common.errors.invalidJson',
        })
      }
    }),
    supported_imagine_models: z.string().superRefine((value, ctx) => {
      const result = validateJsonString(value, {
        predicate: (parsed) =>
          Array.isArray(parsed) &&
          parsed.every((item) => typeof item === 'string'),
        predicateMessage: 'systemSettings.errors.expectedJsonArrayOfModels',
      })
      if (!result.valid) {
        ctx.addIssue({
          code: z.ZodIssueCode.custom,
          message: result.message || 'common.errors.invalidJsonArray',
        })
      }
    }),
  }),
})

type GeminiSettingsFormValues = z.output<typeof schema>
type GeminiSettingsFormInput = z.input<typeof schema>

type FlatGeminiSettings = {
  'gemini.safety_settings': string
  'gemini.version_settings': string
  'gemini.supported_imagine_models': string
}

type GeminiSettingsCardProps = {
  defaultValues: GeminiSettingsFormInput
}

export function GeminiSettingsCard({ defaultValues }: GeminiSettingsCardProps) {
  const { t } = useTranslation()
  const updateOption = useUpdateOption()
  const normalizedDefaultsRef = useRef<FlatGeminiSettings>({
    'gemini.safety_settings': normalizeJsonString(
      defaultValues.gemini.safety_settings
    ),
    'gemini.version_settings': normalizeJsonString(
      defaultValues.gemini.version_settings
    ),
    'gemini.supported_imagine_models': normalizeJsonString(
      defaultValues.gemini.supported_imagine_models
    ),
  })

  const buildFormDefaults = (
    values: GeminiSettingsFormInput
  ): GeminiSettingsFormInput => ({
    gemini: {
      safety_settings: formatJsonForTextarea(values.gemini.safety_settings),
      version_settings: formatJsonForTextarea(values.gemini.version_settings),
      supported_imagine_models: formatJsonForTextarea(
        values.gemini.supported_imagine_models
      ),
    },
  })

  const form = useForm<
    GeminiSettingsFormInput,
    unknown,
    GeminiSettingsFormValues
  >({
    resolver: zodResolver(schema),
    defaultValues: buildFormDefaults(defaultValues),
  })

  useEffect(() => {
    normalizedDefaultsRef.current = {
      'gemini.safety_settings': normalizeJsonString(
        defaultValues.gemini.safety_settings
      ),
      'gemini.version_settings': normalizeJsonString(
        defaultValues.gemini.version_settings
      ),
      'gemini.supported_imagine_models': normalizeJsonString(
        defaultValues.gemini.supported_imagine_models
      ),
    }

    form.reset(buildFormDefaults(defaultValues))
  }, [defaultValues, form])

  const onSubmit = async (values: GeminiSettingsFormValues) => {
    const normalized: FlatGeminiSettings = {
      'gemini.safety_settings': normalizeJsonString(
        values.gemini.safety_settings
      ),
      'gemini.version_settings': normalizeJsonString(
        values.gemini.version_settings
      ),
      'gemini.supported_imagine_models': normalizeJsonString(
        values.gemini.supported_imagine_models
      ),
    }

    const updates = (
      Object.keys(normalized) as Array<keyof FlatGeminiSettings>
    ).filter((key) => normalized[key] !== normalizedDefaultsRef.current[key])

    if (updates.length === 0) {
      toast.info(t('channels.fields.noChangesToSave'))
      return
    }

    for (const key of updates) {
      await updateOption.mutateAsync({
        key,
        value: normalized[key],
      })
    }
  }

  const imaginePlaceholder = useMemo(
    () => JSON.stringify(['gemini-nano-banana-2.1'], null, 2),
    []
  )

  return (
    <SettingsSection title={t('systemSettings.fields.gemini')}>
      <Form {...form}>
        <SettingsForm onSubmit={form.handleSubmit(onSubmit)}>
          <SettingsPageFormActions
            onSave={form.handleSubmit(onSubmit)}
            isSaving={updateOption.isPending}
          />
          <SettingsGroup>
            <FormField
              control={form.control}
              name='gemini.safety_settings'
              render={({ field }) => (
                <SettingsRow
                  label={t('systemSettings.titles.safetySettings')}
                  description={t(
                    'common.tips.providePerCategorySafetyOverridesAsJsonUseDefault'
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
              name='gemini.version_settings'
              render={({ field }) => (
                <SettingsRow
                  label={t('systemSettings.fields.versionOverrides')}
                  description={t(
                    'common.tips.mapModelIdentifiersToGeminiApiVersionsADefault'
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
              name='gemini.supported_imagine_models'
              render={({ field }) => (
                <SettingsRow
                  label={t('systemSettings.titles.supportedImagineModels')}
                  description={t(
                    'systemSettings.tips.acceptsAJsonArrayOfModelIdentifiersThatSupport'
                  )}
                >
                  <FormControl>
                    <Textarea
                      rows={6}
                      placeholder={imaginePlaceholder}
                      {...field}
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
