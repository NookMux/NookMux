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
import { type TFunction } from 'i18next'
import type { NameRule, ModelStatus, SyncSource } from './types'

// ============================================================================
// Pagination
// ============================================================================

export const DEFAULT_PAGE_SIZE = 20

// ============================================================================
// Name Rule Options
// ============================================================================

export function getNameRuleOptions(t: TFunction) {
  return [
    { label: t('models.fields.exactMatch'), value: 0 as NameRule },
    { label: t('models.fields.prefixMatch'), value: 1 as NameRule },
    { label: t('models.fields.containsMatch'), value: 2 as NameRule },
    { label: t('models.fields.suffixMatch'), value: 3 as NameRule },
  ] as const
}

export function getNameRuleConfig(
  t: TFunction
): Record<NameRule, { label: string; color: string; description: string }> {
  return {
    0: {
      label: t('models.fields.exact'),
      color: 'green',
      description: t('models.fields.matchModelNameExactly'),
    },
    1: {
      label: t('models.fields.prefix'),
      color: 'blue',
      description: t('models.tips.matchModelsStartingWithThisName'),
    },
    2: {
      label: t('models.fields.contains'),
      color: 'orange',
      description: t('models.tips.matchModelsContainingThisName'),
    },
    3: {
      label: t('models.fields.suffix'),
      color: 'purple',
      description: t('models.tips.matchModelsEndingWithThisName'),
    },
  }
}

// ============================================================================
// Model Status
// ============================================================================

export function getModelStatusOptions(t: TFunction) {
  return [
    { label: t('channels.fields.allStatus'), value: 'all' },
    { label: t('channels.status.enabled'), value: 'enabled' },
    { label: t('channels.status.disabled'), value: 'disabled' },
  ] as const
}

export function getModelStatusConfig(
  t: TFunction
): Record<ModelStatus, { label: string; variant: 'success' | 'neutral' }> {
  return {
    1: { label: t('channels.status.enabled'), variant: 'success' },
    0: { label: t('channels.status.disabled'), variant: 'neutral' },
  }
}

// ============================================================================
// Sync Status Options
// ============================================================================

export function getSyncStatusOptions(t: TFunction) {
  return [
    { label: t('models.fields.allSyncStatus'), value: 'all' },
    { label: t('models.fields.officialSync'), value: 'yes' },
    { label: t('models.fields.noSync'), value: 'no' },
  ] as const
}

// ============================================================================
// Quota Type
// ============================================================================

export function getQuotaTypeConfig(
  t: TFunction
): Record<number, { label: string; color: string }> {
  return {
    0: { label: t('models.fields.usageBased'), color: 'violet' },
    1: { label: t('models.fields.perCall'), color: 'teal' },
  }
}

// ============================================================================
// Endpoint Templates
// ============================================================================

export const ENDPOINT_TEMPLATES: Record<
  string,
  { path: string; method: string }
> = {
  openai: { path: '/v1/chat/completions', method: 'POST' },
  'openai-response': { path: '/v1/responses', method: 'POST' },
  anthropic: { path: '/v1/messages', method: 'POST' },
  gemini: { path: '/v1beta/models/{model}:generateContent', method: 'POST' },
  'jina-rerank': { path: '/rerank', method: 'POST' },
  'image-generation': { path: '/v1/images/generations', method: 'POST' },
  embeddings: { path: '/v1/embeddings', method: 'POST' },
}

// ============================================================================
// Sync Locale Options
// ============================================================================

export function getSyncLocaleOptions(t: TFunction) {
  return [
    { label: t('models.fields.chinese'), value: 'zh' },
    { label: t('models.fields.english'), value: 'en' },
    { label: t('models.fields.japanese'), value: 'ja' },
  ] as const
}

export function getSyncSourceOptions(t: TFunction) {
  return [
    {
      label: t('models.fields.officialRepository'),
      value: 'official' as SyncSource,
      description: t('models.tips.syncFromThePublicUpstreamMetadataRepository'),
      disabled: false,
    },
    {
      label: t('models.titles.configurationFile'),
      value: 'config' as SyncSource,
      description: t('models.actions.uploadOrReferenceALocalConfigurationFile'),
      disabled: true,
    },
  ] as const
}
