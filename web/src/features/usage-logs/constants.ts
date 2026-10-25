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
 * Shared constants for usage logs feature
 */
import type { StatusBadgeProps } from '@/components/status-badge'
import type { LogStatistics } from './types'

// ============================================================================
// Default Values
// ============================================================================

/**
 * Default log statistics when no data is available
 */
export const DEFAULT_LOG_STATS: LogStatistics = {
  quota: 0,
  rpm: 0,
  tpm: 0,
}

/**
 * Default empty logs data
 */
export const DEFAULT_LOGS_DATA = {
  items: [],
  total: 0,
}

// ============================================================================
// Log Type Enum
// ============================================================================

/**
 * Log type enum values
 */
export const LOG_TYPE_ENUM = {
  UNKNOWN: 0,
  TOPUP: 1,
  CONSUME: 2,
  MANAGE: 3,
  SYSTEM: 4,
  ERROR: 5,
  REFUND: 6,
} as const

/**
 * The log list/stat backend uses type=0 as the "all types" sentinel.
 * Row rendering still displays records with type=0 as "Unknown".
 */
export const LOG_TYPE_ALL_VALUE = '0' as const

// ============================================================================
// Common Logs Configuration
// ============================================================================

/**
 * Log types configuration for filtering and display
 */
export const LOG_TYPES = [
  { value: 0, label: 'channels.fields.unknown', color: 'default' },
  { value: 1, label: 'pricing.fields.recharge', color: 'cyan' },
  { value: 2, label: 'common.fields.consume', color: 'green' },
  { value: 3, label: 'common.fields.manage', color: 'orange' },
  { value: 4, label: 'common.titles.system', color: 'purple' },
  { value: 5, label: 'common.errors.error', color: 'red' },
  { value: 6, label: 'common.fields.refund', color: 'blue' },
] as const

/**
 * Log types for DataTableToolbar filters (single select mode)
 * Backend treats type=0 as "all logs" in list/stat endpoints, so the filter
 * must not expose the display-only "Unknown" label for that value.
 */
export const LOG_TYPE_FILTERS = [
  { label: 'pricing.fields.allTypes', value: LOG_TYPE_ALL_VALUE },
  ...LOG_TYPES.filter((type) => type.value !== LOG_TYPE_ENUM.UNKNOWN).map(
    (type) => ({
      label: type.label,
      value: String(type.value),
    })
  ),
] as const

// ============================================================================
// Drawing Logs (Midjourney) Constants
// ============================================================================

/**
 * Midjourney task types
 * Must match backend constants in constant/midjourney.go
 */
export const MJ_TASK_TYPES = {
  IMAGINE: 'IMAGINE', // 绘图
  UPSCALE: 'UPSCALE', // 放大
  VIDEO: 'VIDEO', // 视频
  EDITS: 'EDITS', // 编辑
  VARIATION: 'VARIATION', // 变换
  HIGH_VARIATION: 'HIGH_VARIATION', // 强变换
  LOW_VARIATION: 'LOW_VARIATION', // 弱变换
  PAN: 'PAN', // 平移
  DESCRIBE: 'DESCRIBE', // 图生文
  BLEND: 'BLEND', // 图混合
  UPLOAD: 'UPLOAD', // 上传文件
  SHORTEN: 'SHORTEN', // 缩词
  REROLL: 'REROLL', // 重绘
  INPAINT: 'INPAINT', // 局部重绘
  SWAP_FACE: 'SWAP_FACE', // 换脸
  ZOOM: 'ZOOM', // 缩放
  CUSTOM_ZOOM: 'CUSTOM_ZOOM', // 自定义缩放
  MODAL: 'MODAL', // 窗口
} as const

/**
 * Midjourney task status
 */
const MJ_TASK_STATUS = {
  NOT_START: 'NOT_START', // 未启动
  SUBMITTED: 'SUBMITTED', // 队列中
  IN_PROGRESS: 'IN_PROGRESS', // 执行中
  SUCCESS: 'SUCCESS', // 成功
  FAILURE: 'FAILURE', // 失败
  MODAL: 'MODAL', // 窗口等待
} as const

/**
 * Midjourney submit result codes
 */
const MJ_SUBMIT_RESULT_CODES = {
  NOT_SUBMITTED: 0, // 未提交
  SUBMITTED: 1, // 已提交
  WAITING: 21, // 等待中
  DUPLICATE: 22, // 重复任务
} as const

// ============================================================================
// Task Logs Constants
// ============================================================================

/**
 * Task action types
 * Must match backend constants in constant/task.go
 */
export const TASK_ACTIONS = {
  // Suno (uppercase)
  MUSIC: 'MUSIC', // 生成音乐
  LYRICS: 'LYRICS', // 生成歌词

  // Video generation (camelCase)
  GENERATE: 'generate', // 图生视频
  TEXT_GENERATE: 'textGenerate', // 文生视频
  FIRST_TAIL_GENERATE: 'firstTailGenerate', // 首尾生视频
  REFERENCE_GENERATE: 'referenceGenerate', // 参照生视频
  REMIX_GENERATE: 'remixGenerate', // 视频 Remix
} as const

/**
 * Task status
 */
export const TASK_STATUS = {
  NOT_START: 'NOT_START', // 未启动
  SUBMITTED: 'SUBMITTED', // 队列中
  IN_PROGRESS: 'IN_PROGRESS', // 执行中
  SUCCESS: 'SUCCESS', // 成功
  FAILURE: 'FAILURE', // 失败
  QUEUED: 'QUEUED', // 排队中
  UNKNOWN: 'UNKNOWN', // 未知
} as const

// ============================================================================
// Status Mappings
// ============================================================================

/**
 * Status mapping configuration type
 */
export interface StatusMapping {
  label: string
  variant: StatusBadgeProps['variant']
}

/**
 * Midjourney task type mappings
 */
export const MJ_TASK_TYPE_MAPPINGS: Record<string, StatusMapping> = {
  [MJ_TASK_TYPES.IMAGINE]: { label: 'common.fields.draw', variant: 'blue' },
  [MJ_TASK_TYPES.UPSCALE]: {
    label: 'common.fields.upscale',
    variant: 'orange',
  },
  [MJ_TASK_TYPES.VIDEO]: { label: 'pricing.fields.video', variant: 'orange' },
  [MJ_TASK_TYPES.EDITS]: { label: 'channels.actions.edit', variant: 'orange' },
  [MJ_TASK_TYPES.VARIATION]: { label: 'common.fields.vary', variant: 'violet' },
  [MJ_TASK_TYPES.HIGH_VARIATION]: {
    label: 'common.fields.varyStrong',
    variant: 'violet',
  },
  [MJ_TASK_TYPES.LOW_VARIATION]: {
    label: 'common.fields.varySubtle',
    variant: 'violet',
  },
  [MJ_TASK_TYPES.PAN]: { label: 'common.fields.pan', variant: 'cyan' },
  [MJ_TASK_TYPES.DESCRIBE]: {
    label: 'common.fields.describe',
    variant: 'yellow',
  },
  [MJ_TASK_TYPES.BLEND]: { label: 'common.fields.blend', variant: 'lime' },
  [MJ_TASK_TYPES.UPLOAD]: { label: 'common.actions.upload', variant: 'blue' },
  [MJ_TASK_TYPES.SHORTEN]: { label: 'common.fields.shorten', variant: 'pink' },
  [MJ_TASK_TYPES.REROLL]: { label: 'common.fields.reroll', variant: 'indigo' },
  [MJ_TASK_TYPES.INPAINT]: { label: 'common.fields.inpaint', variant: 'teal' },
  [MJ_TASK_TYPES.SWAP_FACE]: {
    label: 'common.fields.swapFace',
    variant: 'purple',
  },
  [MJ_TASK_TYPES.ZOOM]: { label: 'common.fields.zoom', variant: 'green' },
  [MJ_TASK_TYPES.CUSTOM_ZOOM]: {
    label: 'common.fields.customZoom',
    variant: 'green',
  },
}

/**
 * Midjourney task status mappings
 */
export const MJ_STATUS_MAPPINGS: Record<string, StatusMapping> = {
  [MJ_TASK_STATUS.SUCCESS]: {
    label: 'channels.status.success',
    variant: 'green',
  },
  [MJ_TASK_STATUS.NOT_START]: {
    label: 'common.fields.notStarted',
    variant: 'neutral',
  },
  [MJ_TASK_STATUS.SUBMITTED]: {
    label: 'common.fields.queued',
    variant: 'yellow',
  },
  [MJ_TASK_STATUS.IN_PROGRESS]: {
    label: 'common.fields.progress',
    variant: 'blue',
  },
  [MJ_TASK_STATUS.FAILURE]: { label: 'channels.errors.failed', variant: 'red' },
  [MJ_TASK_STATUS.MODAL]: { label: 'common.status.waiting', variant: 'amber' },
}

/**
 * Midjourney submit result mappings
 */
export const MJ_SUBMIT_RESULT_MAPPINGS: Record<string, StatusMapping> = {
  [String(MJ_SUBMIT_RESULT_CODES.SUBMITTED)]: {
    label: 'common.fields.submitted',
    variant: 'green',
  },
  [String(MJ_SUBMIT_RESULT_CODES.WAITING)]: {
    label: 'common.status.waiting',
    variant: 'lime',
  },
  [String(MJ_SUBMIT_RESULT_CODES.DUPLICATE)]: {
    label: 'channels.errors.duplicate',
    variant: 'orange',
  },
  [String(MJ_SUBMIT_RESULT_CODES.NOT_SUBMITTED)]: {
    label: 'common.fields.notSubmitted',
    variant: 'yellow',
  },
}

/**
 * Task action type mappings
 */
export const TASK_ACTION_MAPPINGS: Record<string, StatusMapping> = {
  [TASK_ACTIONS.MUSIC]: {
    label: 'common.fields.generateMusic',
    variant: 'neutral',
  },
  [TASK_ACTIONS.LYRICS]: {
    label: 'common.fields.generateLyrics',
    variant: 'pink',
  },
  [TASK_ACTIONS.GENERATE]: {
    label: 'common.fields.imageToVideo',
    variant: 'blue',
  },
  [TASK_ACTIONS.TEXT_GENERATE]: {
    label: 'common.fields.textToVideo',
    variant: 'blue',
  },
  [TASK_ACTIONS.FIRST_TAIL_GENERATE]: {
    label: 'common.fields.firstLastFrameToVideo',
    variant: 'blue',
  },
  [TASK_ACTIONS.REFERENCE_GENERATE]: {
    label: 'common.fields.referenceVideo',
    variant: 'blue',
  },
  [TASK_ACTIONS.REMIX_GENERATE]: {
    label: 'common.fields.videoRemix',
    variant: 'blue',
  },
}

/**
 * Task status mappings
 */
export const TASK_STATUS_MAPPINGS: Record<string, StatusMapping> = {
  [TASK_STATUS.SUCCESS]: { label: 'channels.status.success', variant: 'green' },
  [TASK_STATUS.NOT_START]: {
    label: 'common.fields.notStarted',
    variant: 'neutral',
  },
  [TASK_STATUS.SUBMITTED]: { label: 'common.fields.queued', variant: 'yellow' },
  [TASK_STATUS.IN_PROGRESS]: {
    label: 'common.fields.progress',
    variant: 'blue',
  },
  [TASK_STATUS.FAILURE]: { label: 'channels.errors.failed', variant: 'red' },
  [TASK_STATUS.QUEUED]: { label: 'common.fields.queued', variant: 'orange' },
  [TASK_STATUS.UNKNOWN]: {
    label: 'channels.fields.unknown',
    variant: 'neutral',
  },
}

// ============================================================================
// Log Type Checkers (Constants)
// ============================================================================

/**
 * Log types that are displayable (have detailed info)
 */
export const DISPLAYABLE_LOG_TYPES = [0, 2, 5, 6] as const

/**
 * Log types that show timing info
 */
export const TIMING_LOG_TYPES = [2, 5] as const

/**
 * Log types whose details dialog shows client request headers
 * (ua / x_title / http_referer). Backend writes these headers to dedicated
 * Log columns via extractClientHeaders for both consume and error logs.
 */
export const CLIENT_HEADERS_LOG_TYPES = [2, 5] as const
