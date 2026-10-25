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
 * 手机端 User-Agent 判定，用于在渲染前决定是否放行控制台。
 *
 * 只认 UA、不看屏幕宽度：
 * - 手机默认模式的 UA 含 "Mobi"（iPhone / Android 手机）
 * - 手机浏览器开启「桌面模式」后 UA 已替换为桌面 UA，天然放行
 * - 老 iPad 的 UA 含 "Mobile/<build>"，显式排除；iPadOS 13+ 桌面化 UA
 *   与 Android 平板 UA 本就不含 "Mobi"
 */
export function isPhoneUA(userAgent: string = navigator.userAgent): boolean {
  return /Mobi/i.test(userAgent) && !/iPad|Tablet/i.test(userAgent)
}
