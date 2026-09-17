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
import { SettingsPage } from '../components/settings-page'
import type { ModelSettings } from '../types'
import {
  MODELS_DEFAULT_SECTION,
  getModelsSectionContent,
  getModelsSectionMeta,
} from './section-registry.tsx'

const defaultModelSettings: ModelSettings = {
  'general_setting.ping_interval_enabled': false,
  'general_setting.ping_interval_seconds': 60,
  'gemini.safety_settings': '',
  'gemini.version_settings': '',
  'gemini.supported_imagine_models': '',
  'gemini.function_call_thought_signature_enabled': true,
  'gemini.remove_function_response_id_enabled': true,
  'claude.model_headers_settings': '',
  'claude.default_max_tokens': '',
  'claude.remove_claude_code_billing_header_enabled': true,
  'grok.violation_deduction_enabled': true,
  'grok.violation_deduction_amount': 0.05,
  'minimax.enabled': false,
  'minimax.model_redirect': '{}',
  'minimax.voice_whitelist_enabled': false,
  'minimax.custom_voice_enabled': false,
  'minimax.custom_voice_group': '',
  'minimax.custom_voice_billing_model_id': '',
  'minimax.emotion_pattern': '<tts\\s+emotion="([^"]+)">([\\s\\S]*?)</tts>',
  'minimax.emotion_redirect': '{}',
  'minimax.tone_word_pattern': '\\(([^()]+)\\)',
  'minimax.tone_word_redirect': '{}',
  ModelPrice: '',
  ModelRatio: '',
  CacheRatio: '',
  CreateCacheRatio: '',
  CompletionRatio: '',
  AudioRatio: '',
  AudioCompletionRatio: '',
  ContextPricing: '',
  'tool_billing_setting.rules': '[]',
  TopupGroupRatio: '',
  GroupRatio: '',
  UserUsableGroups: '',
  GroupGroupRatio: '',
  AutoGroups: '',
  DefaultUseAutoGroup: false,
  'group_ratio_setting.group_special_usable_group': '{}',
  'channel_affinity_setting.enabled': false,
  'channel_affinity_setting.switch_on_success': true,
  'channel_affinity_setting.keep_on_channel_disabled': false,
  'channel_affinity_setting.max_entries': 100000,
  'channel_affinity_setting.default_ttl_seconds': 3600,
  'channel_affinity_setting.rules': '[]',
}

export function ModelSettings() {
  return (
    <SettingsPage
      routePath='/_authenticated/system-settings/models/$section'
      defaultSettings={defaultModelSettings}
      defaultSection={MODELS_DEFAULT_SECTION}
      getSectionContent={getModelsSectionContent}
      getSectionMeta={getModelsSectionMeta}
    />
  )
}
