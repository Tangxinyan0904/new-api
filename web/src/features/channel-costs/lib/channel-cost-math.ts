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
import dayjs from 'dayjs'

import type {
  ChannelCostChannel,
  ChannelCostFigures,
  ChannelCostRange,
} from '../types'

export const CHANNEL_COST_RANGES: ChannelCostRange[] = [
  'today',
  'yesterday',
  'week',
]

/** Matches setting/operation_setting MaxChannelCostRatio. */
export const MAX_CHANNEL_COST_RATIO = 100

export interface ChannelCostRangeStarts {
  today_start: number
  yesterday_start: number
  /** The 7-day range starts six days before today, so it includes today. */
  week_start: number
}

/** Range starts in Unix seconds at local midnight of the viewer's time zone. */
export function getChannelCostRangeStarts(now: Date): ChannelCostRangeStarts {
  const today = dayjs(now).startOf('day')
  return {
    today_start: today.unix(),
    yesterday_start: today.subtract(1, 'day').unix(),
    week_start: today.subtract(6, 'day').unix(),
  }
}

/**
 * cost = usage at the original price × cost ratio; without a cost ratio the
 * whole usage counts as cost. profit = usage − cost.
 */
export function channelCostFigures(
  channel: ChannelCostChannel,
  range: ChannelCostRange
): ChannelCostFigures {
  const usage = channel[range]
  const cost =
    channel.ratio === null ? usage.quota : usage.base_quota * channel.ratio
  return { used: usage.quota, cost, profit: usage.quota - cost }
}

export function sumChannelCostFigures(
  channels: ChannelCostChannel[],
  range: ChannelCostRange
): ChannelCostFigures {
  const total: ChannelCostFigures = { used: 0, cost: 0, profit: 0 }
  for (const channel of channels) {
    const figures = channelCostFigures(channel, range)
    total.used += figures.used
    total.cost += figures.cost
    total.profit += figures.profit
  }
  return total
}

/**
 * Parses ratio input. Empty input returns null (clear the ratio); anything
 * that is not a number within the allowed range returns undefined.
 */
export function parseCostRatioInput(value: string): number | null | undefined {
  const trimmed = value.trim()
  if (trimmed === '') return null
  const ratio = Number(trimmed)
  if (!Number.isFinite(ratio) || ratio < 0 || ratio > MAX_CHANNEL_COST_RATIO) {
    return undefined
  }
  return ratio
}
