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
 * 客户端平台（Agent）元数据：后端仅下发受控 platform ID（见
 * internal/store/log/platform.go），显示名与 lobe 图标名在此统一映射，
 * 供使用日志平台列与排行榜热门 Agent 卡片共用。
 * icon 名对齐 @lobehub/icons 的导出；未知 ID 回退显示原始 ID，
 * getLobeIcon 未命中时自动渲染首字母圆形。
 */
export interface PlatformMeta {
  name: string
  icon: string
}

export const PLATFORM_META: Record<string, PlatformMeta> = {
  'claude-code': { name: 'Claude Code', icon: 'ClaudeCode' },
  codex: { name: 'Codex', icon: 'Codex' },
  opencode: { name: 'OpenCode', icon: 'OpenCode' },
  'hermes-agent': { name: 'Hermes Agent', icon: 'HermesAgent' },
  'deepseek-harness': { name: 'DeepSeek Harness', icon: 'DeepSeek' },
  openclaw: { name: 'OpenClaw', icon: 'OpenClaw' },
  pi: { name: 'pi', icon: 'Pi' },
  'new-api': { name: 'New API', icon: 'NewAPI' },
  'cherry-studio': { name: 'Cherry Studio', icon: 'CherryStudio' },
}

export function getPlatformMeta(platform: string): PlatformMeta {
  return PLATFORM_META[platform] ?? { name: platform, icon: '' }
}
