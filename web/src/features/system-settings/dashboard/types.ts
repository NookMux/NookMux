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
 * 仪表板配置类型定义
 */
export interface DashboardConfig {
  // 数据指标启用开关
  quota_data_enabled: boolean
  user_analytics_enabled: boolean
  rankings_enabled: boolean
  media_convert_stats_enabled: boolean

  // 写入层聚合粒度控制
  quota_data_track_tokens: boolean
  quota_data_track_by_model: boolean
  quota_data_track_by_user: boolean

  // 面板启用开关
  api_info_enabled: boolean
  uptime_kuma_enabled: boolean
  announcements_enabled: boolean
  faq_enabled: boolean

  // 刷新间隔配置（秒）
  quota_data_refresh_interval: number
  user_analytics_refresh_interval: number
  rankings_refresh_interval: number
  uptime_kuma_refresh_interval: number

  // 时间范围限制（天）
  default_time_range_days: number
  max_time_range_days: number

  // 数据上限配置
  rankings_model_limit: number
  rankings_vendor_limit: number
  rankings_platform_limit: number
  user_analytics_top_n: number
}

/**
 * 仪表板配置更新请求
 */
export type DashboardConfigUpdate = Partial<DashboardConfig>

/**
 * 仪表板配置响应
 */
export interface DashboardConfigResponse {
  success: boolean
  message: string
  data?: DashboardConfig
}
