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
/**
 * Home page constants
 * All hardcoded data for home page sections
 */
import { type TFunction } from 'i18next'

// Layout - Main base classes
export const MAIN_BASE_CLASSES = 'bg-background text-foreground w-full'

// Hero section - AI Applications (Left side)
export const AI_APPLICATIONS = [
  'LobeHub.Color',
  'Dify.Color',
  'OpenWebUI',
  'Cline',
] as const

// Hero section - AI Models (Right side)
export const AI_MODELS = [
  'Qwen.Color',
  'DeepSeek.Color',
  'Doubao.Color',
  'OpenAI',
  'Claude.Color',
  'Gemini.Color',
] as const

// Hero section - Gateway Features
export const GATEWAY_FEATURES = [
  'home.fields.costTracking',
  'common.fields.modelAccess',
  'common.fields.guardrails',
  'common.fields.observability',
  'common.fields.budgets',
  'home.fields.loadBalancing',
  'home.fields.rateLimiting',
  'systemSettings.titles.tokenManagement',
  'common.fields.promptCaching',
  'common.fields.passThrough',
] as const

// Features section - Default features
export const DEFAULT_FEATURES = [
  {
    title: 'home.fields.lightningFast',
    description:
      'home.tips.optimizedNetworkArchitectureEnsuresMillisecondResponseTimes',
    iconName: 'Zap',
  },
  {
    title: 'home.fields.secureReliable',
    description:
      'home.placeholders.enterpriseGradeSecurityWithComprehensivePermissionManagement',
    iconName: 'Shield',
  },
  {
    title: 'home.fields.globalCoverage',
    description: 'home.tips.multiRegionDeploymentForStableGlobalAccess',
    iconName: 'Globe',
  },
  {
    title: 'home.fields.developerFriendly',
    description: 'home.tips.compatibleApiRoutesForCommonAiApplicationWorkflows',
    iconName: 'Code',
  },
  {
    title: 'home.fields.highPerformance',
    description:
      'home.tips.supportForHighConcurrencyWithAutomaticLoadBalancing',
    iconName: 'Gauge',
  },
  {
    title: 'home.fields.transparentBilling',
    description: 'home.tips.payAsYouGoWithRealTimeUsageMonitoring',
    iconName: 'DollarSign',
  },
  {
    title: 'home.fields.teamCollaboration',
    description:
      'home.tips.multiUserManagementWithFlexiblePermissionAllocation',
    iconName: 'Users',
  },
  {
    title: 'home.actions.openSource',
    description: 'home.tips.communityDrivenSelfHostedAndExtensible',
    iconName: 'HeartHandshake',
  },
] as const

export function getGatewayFeatures(t: TFunction) {
  return GATEWAY_FEATURES.map((feature) => t(feature))
}

export function getDefaultFeatures(t: TFunction) {
  return DEFAULT_FEATURES.map((feature) => ({
    ...feature,
    title: t(feature.title),
    description: t(feature.description),
  }))
}
