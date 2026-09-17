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
import { useCallback, useState } from 'react'
import { type VisibilityState } from '@/lib/tanstack-table'

const STORAGE_PREFIX = 'nookmux:table:column-visibility:'

export type OnColumnVisibilityChange = (
  updater: VisibilityState | ((prev: VisibilityState) => VisibilityState)
) => void

export function loadVisibilityState(
  storageKey: string,
  defaultVisibility: VisibilityState
): VisibilityState {
  if (typeof window === 'undefined') {
    return defaultVisibility
  }
  try {
    const raw = window.localStorage.getItem(storageKey)
    if (!raw) {
      return defaultVisibility
    }
    const parsed = JSON.parse(raw) as unknown
    if (
      typeof parsed !== 'object' ||
      parsed === null ||
      Array.isArray(parsed)
    ) {
      return defaultVisibility
    }
    // Defaults apply to any column not present in storage. This keeps the
    // behaviour of newly added columns predictable while preserving user
    // overrides for columns they have explicitly toggled.
    return { ...defaultVisibility, ...parsed }
  } catch {
    return defaultVisibility
  }
}

function saveVisibilityState(storageKey: string, state: VisibilityState) {
  if (typeof window === 'undefined') {
    return
  }
  try {
    window.localStorage.setItem(storageKey, JSON.stringify(state))
  } catch {
    // Ignore storage errors (e.g. private mode, quota exceeded) so that a
    // broken localStorage does not break the table UI.
  }
}

/**
 * usePersistentColumnVisibility persists the user-defined column visibility
 * of a TanStack Table to localStorage. It returns a state tuple that is a
 * drop-in replacement for `useState<VisibilityState>()`.
 *
 * @param tableId A stable identifier for the table (e.g. "channels-table").
 * @param defaultVisibility Default visibility used on first load and for any
 * columns that the user has not explicitly toggled yet.
 */
export function usePersistentColumnVisibility(
  tableId: string,
  defaultVisibility: VisibilityState = {}
): [VisibilityState, OnColumnVisibilityChange] {
  const storageKey = STORAGE_PREFIX + tableId

  const [columnVisibility, setColumnVisibility] = useState<VisibilityState>(
    () => loadVisibilityState(storageKey, defaultVisibility)
  )

  const onColumnVisibilityChange = useCallback<OnColumnVisibilityChange>(
    (updater) => {
      setColumnVisibility((previous) => {
        const next = typeof updater === 'function' ? updater(previous) : updater
        saveVisibilityState(storageKey, next)
        return next
      })
    },
    [storageKey]
  )

  return [columnVisibility, onColumnVisibilityChange]
}
