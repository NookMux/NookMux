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
export function formatThroughput(tps: number): string {
  if (!Number.isFinite(tps) || tps <= 0) return '—'
  if (tps >= 1_000) return `${(tps / 1_000).toFixed(1)}K t/s`
  return `${tps.toFixed(tps < 10 ? 2 : 1)} t/s`
}

export function formatLatency(ms: number): string {
  if (!Number.isFinite(ms) || ms <= 0) return '—'
  if (ms >= 1_000) return `${(ms / 1_000).toFixed(2)}s`
  return `${Math.round(ms)}ms`
}

export function formatSuccessRate(rate: number): string {
  if (!Number.isFinite(rate)) return '—'
  return `${rate.toFixed(1)}%`
}

export type SuccessRateTone = {
  textClass: string
  dotClass: string
}

// Upstream-aligned success rate tiers:
// 100% green · >=90% light green · >=70% amber · <70% red.
export function getSuccessRateTone(rate: number): SuccessRateTone {
  if (!Number.isFinite(rate) || rate < 0) {
    return {
      textClass: 'text-muted-foreground',
      dotClass: 'bg-muted-foreground/40',
    }
  }
  if (rate >= 100) {
    return { textClass: 'text-success', dotClass: 'bg-success' }
  }
  if (rate >= 90) {
    return { textClass: 'text-success/80', dotClass: 'bg-success/70' }
  }
  if (rate >= 70) {
    return { textClass: 'text-warning', dotClass: 'bg-warning' }
  }
  return { textClass: 'text-destructive', dotClass: 'bg-destructive' }
}
