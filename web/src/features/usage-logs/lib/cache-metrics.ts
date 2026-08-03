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
import type { LogOtherData } from '../types'

export interface CacheHitMetrics {
  cacheReadTokens: number
  cacheWriteTokens: number
  totalInputTokens: number
  percentage: number
  formattedPercentage: string
}

export function isHighCacheHitPercentage(percentage: number): boolean {
  return Number.isFinite(percentage) && percentage >= 90
}

function normalizeTokens(value: number | null | undefined): number {
  if (!Number.isFinite(value) || value == null || value < 0) return 0
  return value
}

export function getCacheHitMetrics(
  promptTokens: number | null | undefined,
  other: LogOtherData | null | undefined
): CacheHitMetrics {
  const normalizedPromptTokens = normalizeTokens(promptTokens)
  const cacheReadTokens = normalizeTokens(other?.cache_tokens)
  const normalizedCacheWriteTokens = normalizeTokens(other?.cache_write_tokens)
  const cacheWrite5m = normalizeTokens(other?.cache_creation_tokens_5m)
  const cacheWrite1h = normalizeTokens(other?.cache_creation_tokens_1h)

  let cacheWriteTokens = normalizedCacheWriteTokens
  if (other?.cache_write_tokens == null) {
    if (
      other?.cache_creation_tokens_5m != null ||
      other?.cache_creation_tokens_1h != null
    ) {
      cacheWriteTokens = cacheWrite5m + cacheWrite1h
    } else {
      cacheWriteTokens = normalizeTokens(other?.cache_creation_tokens)
    }
  }

  const isAnthropic =
    other?.usage_semantic === 'anthropic' || other?.claude === true
  const totalInputTokens = isAnthropic
    ? normalizedPromptTokens + cacheReadTokens + cacheWriteTokens
    : normalizedPromptTokens
  const percentage =
    totalInputTokens > 0
      ? Math.min(100, Math.max(0, (cacheReadTokens / totalInputTokens) * 100))
      : 0

  return {
    cacheReadTokens,
    cacheWriteTokens,
    totalInputTokens,
    percentage,
    formattedPercentage: `${percentage.toFixed(3)}%`,
  }
}
