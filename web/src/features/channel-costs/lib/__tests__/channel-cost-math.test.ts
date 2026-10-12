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
import { describe, expect, it } from 'vitest'

import type { ChannelCostChannel } from '../../types'
import {
  channelCostFigures,
  getChannelCostRangeStarts,
  parseCostRatioInput,
  sumChannelCostFigures,
} from '../channel-cost-math'

function channel(
  id: number,
  ratio: number | null,
  quotas: [number, number, number]
): ChannelCostChannel {
  return {
    id,
    name: `channel-${id}`,
    type: 1,
    status: 1,
    deleted: false,
    ratio,
    today_quota: quotas[0],
    yesterday_quota: quotas[1],
    week_quota: quotas[2],
  }
}

function localMidnight(year: number, month: number, day: number): number {
  return new Date(year, month, day).getTime() / 1000
}

describe('getChannelCostRangeStarts', () => {
  it('starts today, yesterday and the 7-day range (including today) at local midnight', () => {
    const starts = getChannelCostRangeStarts(new Date(2026, 9, 13, 15, 42))

    expect(starts).toEqual({
      today_start: localMidnight(2026, 9, 13),
      yesterday_start: localMidnight(2026, 9, 12),
      week_start: localMidnight(2026, 9, 7),
    })
  })

  it('treats the first minute of a day as part of that day', () => {
    const starts = getChannelCostRangeStarts(new Date(2026, 9, 13, 0, 0, 30))

    expect(starts.today_start).toBe(localMidnight(2026, 9, 13))
  })
})

describe('channelCostFigures', () => {
  it('uses the channel ratio when it has one', () => {
    expect(
      channelCostFigures(channel(1, 0.4, [1000, 0, 0]), 'today', 0.9)
    ).toEqual({
      used: 1000,
      cost: 400,
      profit: 600,
    })
  })

  it('falls back to the default ratio, including an explicit zero', () => {
    expect(
      channelCostFigures(channel(1, null, [0, 500, 0]), 'yesterday', 0.5)
    ).toEqual({
      used: 500,
      cost: 250,
      profit: 250,
    })
    expect(channelCostFigures(channel(1, 0, [0, 0, 800]), 'week', 0.5)).toEqual(
      {
        used: 800,
        cost: 0,
        profit: 800,
      }
    )
  })

  it('reports a loss when the ratio is above 1', () => {
    expect(
      channelCostFigures(channel(1, 1.25, [400, 0, 0]), 'today', 1).profit
    ).toBe(-100)
  })
})

describe('sumChannelCostFigures', () => {
  it('adds up every channel with its own effective ratio', () => {
    const channels = [
      channel(1, 0.5, [100, 0, 1000]),
      channel(2, null, [200, 0, 2000]),
    ]

    expect(sumChannelCostFigures(channels, 'week', 0.25)).toEqual({
      used: 3000,
      cost: 1000,
      profit: 2000,
    })
    expect(sumChannelCostFigures([], 'today', 1)).toEqual({
      used: 0,
      cost: 0,
      profit: 0,
    })
  })
})

describe('parseCostRatioInput', () => {
  it.each([
    ['0.35', 0.35],
    [' 2 ', 2],
    ['0', 0],
    ['100', 100],
    ['', null],
    ['   ', null],
    ['-0.1', undefined],
    ['100.01', undefined],
    ['abc', undefined],
  ])('parses %j as %j', (input, expected) => {
    expect(parseCostRatioInput(input)).toBe(expected)
  })
})
