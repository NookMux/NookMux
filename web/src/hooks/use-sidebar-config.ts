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
import { useMemo } from 'react'
import { useAuthStore } from '@/stores/auth-store'
import { ROLE } from '@/lib/roles'
import { useUserSidebarModules } from '@/hooks/use-user-sidebar-modules'
import type { NavGroup, NavItem } from '@/components/layout/types'

type SidebarSectionConfig = {
  enabled: boolean
  [key: string]: boolean
}

type SidebarModulesAdminConfig = Record<string, SidebarSectionConfig>

/**
 * Default sidebar modules configuration
 */
const DEFAULT_SIDEBAR_MODULES: SidebarModulesAdminConfig = {
  chat: {
    enabled: true,
    playground: true,
    custom_voice: true,
  },
  console: {
    enabled: true,
    detail: true,
    token: true,
    log: true,
    multimodal_files: true,
  },
  personal: {
    enabled: true,
    topup: true,
    order_query: true,
    personal: true,
  },
  support: {
    enabled: true,
    ticket: true,
  },
  admin: {
    enabled: true,
    dynamic_ratio: true,
    channel: true,
    models: true,
    redemption: true,
    user: true,
    setting: true,
    audit_log: true,
    voice_management: true,
  },
}

const mergeWithDefaultSidebarModules = (
  config: SidebarModulesAdminConfig
): SidebarModulesAdminConfig => {
  const merged: SidebarModulesAdminConfig = { ...config }

  Object.entries(DEFAULT_SIDEBAR_MODULES).forEach(
    ([sectionKey, defaultSection]) => {
      const existingSection = merged[sectionKey]
      if (!existingSection) {
        merged[sectionKey] = { ...defaultSection }
        return
      }

      merged[sectionKey] = { ...defaultSection, ...existingSection }
      Object.keys(defaultSection).forEach((moduleKey) => {
        if (existingSection[moduleKey] !== undefined) return
        // Legacy alias: voice_management was previously stored as "minimax"
        // before the module was de-vendorized.
        const alias = moduleKey === 'voice_management' ? 'minimax' : null
        merged[sectionKey][moduleKey] =
          alias && existingSection[alias] !== undefined
            ? existingSection[alias]
            : defaultSection[moduleKey]
      })
    }
  )

  return merged
}

/**
 * Mapping from URL to configuration keys
 */
const URL_TO_CONFIG_MAP: Record<string, { section: string; module: string }> = {
  '/playground': { section: 'chat', module: 'playground' },
  '/multimodal/custom-voice': { section: 'chat', module: 'custom_voice' },
  '/dashboard': { section: 'console', module: 'detail' },
  '/dashboard/overview': { section: 'console', module: 'detail' },
  '/dashboard/models': { section: 'console', module: 'detail' },
  '/dashboard/users': { section: 'console', module: 'detail' },
  '/keys': { section: 'console', module: 'token' },
  '/usage-logs': { section: 'console', module: 'log' },
  '/usage-logs/common': { section: 'console', module: 'log' },
  '/multimodal-files': { section: 'console', module: 'multimodal_files' },
  '/wallet': { section: 'personal', module: 'topup' },
  '/order-query': { section: 'personal', module: 'order_query' },
  '/profile': { section: 'personal', module: 'personal' },
  '/ticket': { section: 'support', module: 'ticket' },
  '/dynamic-ratio': { section: 'admin', module: 'dynamic_ratio' },
  '/channels': { section: 'admin', module: 'channel' },
  '/models': { section: 'admin', module: 'models' },
  '/models/metadata': { section: 'admin', module: 'models' },
  '/users': { section: 'admin', module: 'user' },
  '/redemption-codes': { section: 'admin', module: 'redemption' },
  '/audit-logs': { section: 'admin', module: 'audit_log' },
  '/voice-management': { section: 'admin', module: 'voice_management' },
  '/system-settings': { section: 'admin', module: 'setting' },
  '/system-settings/site': { section: 'admin', module: 'setting' },
}

/**
 * Parse backend SidebarModulesAdmin configuration
 */
function parseSidebarConfig(
  value: string | null | undefined
): SidebarModulesAdminConfig {
  // If empty string, null, or undefined, use default config
  if (!value || value.trim() === '') {
    return DEFAULT_SIDEBAR_MODULES
  }

  try {
    const parsed = JSON.parse(value) as SidebarModulesAdminConfig
    return mergeWithDefaultSidebarModules(parsed)
  } catch {
    // eslint-disable-next-line no-console
    console.error('Failed to parse sidebar modules configuration')
    return DEFAULT_SIDEBAR_MODULES
  }
}

/**
 * Check if a module is enabled based on admin config only.
 */
function isModuleEnabled(
  url: string,
  adminConfig: SidebarModulesAdminConfig
): boolean {
  const mapping = URL_TO_CONFIG_MAP[url]
  if (!mapping) {
    // No mapping config, default to visible (e.g. system settings and new features)
    return true
  }

  const { section, module } = mapping
  const adminSection = adminConfig[section]
  return Boolean(
    adminSection && adminSection.enabled && adminSection[module] === true
  )
}

/**
 * Check if a navigation item should be visible
 */
function isNavItemVisible(
  item: NavItem,
  adminConfig: SidebarModulesAdminConfig
): boolean {
  // Handle direct link type
  if ('url' in item && item.url) {
    const configUrls = item.configUrls ?? [item.url]
    return configUrls.some((url) => isModuleEnabled(url as string, adminConfig))
  }

  // Handle collapsible type (with sub-items)
  if ('items' in item && item.items) {
    // If has sub-items, show this collapsible item if at least one sub-item is visible
    return item.items.some((subItem) =>
      isModuleEnabled(subItem.url as string, adminConfig)
    )
  }

  return true
}

/**
 * Filter navigation items
 */
function filterNavItems(
  items: NavItem[],
  adminConfig: SidebarModulesAdminConfig
): NavItem[] {
  return items
    .map((item) => {
      // If collapsible item, also filter its sub-items
      if ('items' in item && item.items) {
        const filteredSubItems = item.items.filter((subItem) =>
          isModuleEnabled(subItem.url as string, adminConfig)
        )

        return {
          ...item,
          items: filteredSubItems,
        }
      }
      return item
    })
    .filter((item) => isNavItemVisible(item, adminConfig))
}

/**
 * Filter sidebar navigation groups by sidebar_modules config and user role.
 *
 * 配置统一由 `/api/status/user_modules`（UserAuth）下发：服务端对非管理员
 * 剥离 admin 段，管理员返回全量。query key 编入 isAdmin 档位，防止管理员
 * 与普通用户在同页面会话切换时复用彼此的缓存。非 root 管理员始终隐藏
 * 系统设置。
 */
export function useSidebarConfig(navGroups: NavGroup[]): NavGroup[] {
  const userRole = useAuthStore((s) => s.auth.user?.role)
  const isLoggedIn = userRole !== undefined
  const isAdmin = isLoggedIn && userRole >= ROLE.ADMIN
  const { data: sidebarModules } = useUserSidebarModules(isLoggedIn, isAdmin)

  const effectiveConfig = useMemo(() => {
    const config = parseSidebarConfig(sidebarModules)

    // Role-based overrides: non-root admins should not see system settings
    if (userRole === ROLE.SUPER_ADMIN) return config
    const merged = { ...config }
    if (merged.admin) {
      merged.admin = { ...merged.admin, setting: false }
    }
    return merged
  }, [sidebarModules, userRole])

  const filteredNavGroups = useMemo(
    () =>
      navGroups
        .map((group) => ({
          ...group,
          items: filterNavItems(group.items, effectiveConfig),
        }))
        .filter((group) => group.items.length > 0), // Only show navigation groups with visible items
    [navGroups, effectiveConfig]
  )

  return filteredNavGroups
}
