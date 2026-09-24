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
import { z } from 'zod'
import type { BanIdentifierType } from '../types'

export const banIdentifierTypeSchema = z.enum([
  'github_id',
  'linuxdo_id',
  'email',
  'github_username',
])

export const banUserSchema = z.object({
  type: banIdentifierTypeSchema,
  value: z.string().trim().min(1).max(64),
})

export type BanUserFormValues = z.infer<typeof banUserSchema>

export const BAN_USER_FORM_DEFAULT_VALUES: BanUserFormValues = {
  type: 'github_id',
  value: '',
}

/**
 * Value patterns per identifier type, mirroring the backend validation.
 * Checked in the submit handler so the error message can go through i18n.
 */
export const banValuePattern: Record<BanIdentifierType, RegExp> = {
  github_id: /^[0-9]{1,20}$/,
  linuxdo_id: /^[0-9]{1,20}$/,
  email: /^[^\s@]+@[^\s@]+\.[^\s@]+$/,
  github_username: /^[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,37}[a-zA-Z0-9])?$/,
}
