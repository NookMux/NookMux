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

// ============================================================================
// Form Schemas
// ============================================================================

export const loginFormSchema = z.object({
  username: z.string().min(1, 'auth.errors.pleaseEnterUsernameOrEmail'),
  password: z
    .string()
    .min(1, 'auth.errors.pleaseEnterPassword')
    .min(8, 'setup.errors.passwordMustBeAtLeast8CharactersLong'),
})

export const registerFormSchema = z
  .object({
    username: z.string().min(1, 'auth.errors.pleaseEnterUsername'),
    email: z.string().optional(),
    password: z
      .string()
      .min(1, 'auth.errors.pleaseEnterPassword')
      .min(8, 'setup.errors.passwordMustBeAtLeast8CharactersLong')
      .max(20, 'users.errors.passwordMustBeBetween8And20Characters'),
    confirmPassword: z.string().min(1, 'auth.errors.pleaseConfirmPassword'),
  })
  .refine((data) => data.password === data.confirmPassword, {
    message: 'profile.fields.passwordsDoNotMatch',
    path: ['confirmPassword'],
  })

export const forgotPasswordFormSchema = z.object({
  email: z.string().email({
    message: 'profile.errors.pleaseEnterAValidEmailAddress',
  }),
})

export const otpFormSchema = z.object({
  otp: z.string().min(1, 'profile.errors.pleaseEnterYourVerificationCode'),
})

// ============================================================================
// Validation Constants
// ============================================================================

export const OTP_LENGTH = 6
export const BACKUP_CODE_LENGTH = 9 // XXXX-XXXX format
export const BACKUP_CODE_REGEX = /^[A-Z0-9]{4}-[A-Z0-9]{4}$/i
export const OTP_REGEX = /^\d{6}$/

// ============================================================================
// Countdown Constants
// ============================================================================

export const EMAIL_VERIFICATION_COUNTDOWN = 30 // seconds
export const PASSWORD_RESET_COUNTDOWN = 30 // seconds

// ============================================================================
// OAuth Constants
// ============================================================================

export const OAUTH_BIND_STORAGE_KEY = 'oauth:binding:result'
