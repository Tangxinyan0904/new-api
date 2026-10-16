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

import type {
  ChannelCostChannel,
  ChannelCostGroup,
  ChannelUpstreamCost,
} from '../../types'
import {
  allocateUpstreamCosts,
  channelCostFigures,
  getChannelCostRangeStarts,
  parseCostRatioInput,
  sumGroupCostFigures,
  upstreamShareKey,
} from '../channel-cost-math'

/** Usage per range as [charged quota, quota at the original price]. */
function channel(
  ratio: number | null,
  usage: Partial<Record<'today' | 'yesterday' | 'week', [number, number]>>,
  id = 1
): ChannelCostChannel {
  const range = (pair?: [number, number]) => ({
    quota: pair?.[0] ?? 0,
    base_quota: pair?.[1] ?? 0,
  })
  return {
    id,
    name: `channel-${id}`,
    type: 1,
    status: 1,
    deleted: false,
    ratio,
    upstream_billing: null,
    today: range(usage.today),
    yesterday: range(usage.yesterday),
    week: range(usage.week),
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
  it('prices the original-price usage at the cost ratio', () => {
    // Sold at group ratio 0.1, bought at 0.08: 1000 charged is 10000 at the
    // original price, which costs 800.
    expect(
      channelCostFigures(channel(0.08, { today: [1000, 10000] }), 'today')
    ).toEqual({ used: 1000, cost: 800, profit: 200 })
  })

  it('counts the whole usage as cost when no cost ratio is set', () => {
    expect(
      channelCostFigures(channel(null, { yesterday: [500, 2500] }), 'yesterday')
    ).toEqual({ used: 500, cost: 500, profit: 0 })
  })

  it('treats an explicit zero cost ratio as free', () => {
    expect(
      channelCostFigures(channel(0, { week: [800, 4000] }), 'week')
    ).toEqual({ used: 800, cost: 0, profit: 800 })
  })

  it('reports a loss when the cost ratio is above the charged ratio', () => {
    expect(
      channelCostFigures(channel(0.3, { today: [400, 2000] }), 'today').profit
    ).toBe(-200)
  })

  it('uses the upstream charge instead of the cost ratio when one is given', () => {
    expect(
      channelCostFigures(channel(0.08, { today: [1000, 10000] }), 'today', 650)
    ).toEqual({ used: 1000, cost: 650, profit: 350 })
  })
})

function upstreamCost(
  channelId: number,
  quota: Record<'today' | 'yesterday' | 'week', number>,
  error: ChannelUpstreamCost['error'] = null
): ChannelUpstreamCost {
  const amount = (value: number) => ({ usd: value / 500000, quota: value })
  return {
    channel_id: channelId,
    kind: 'sub2api',
    error,
    fetched_at: 0,
    today: amount(quota.today),
    yesterday: amount(quota.yesterday),
    week: amount(quota.week),
  }
}

describe('allocateUpstreamCosts', () => {
  const groups: ChannelCostGroup[] = [
    {
      name: 'default',
      ratio: 1,
      channels: [
        channel(null, { today: [300, 300], week: [0, 0] }, 7),
        channel(null, { today: [50, 50] }, 8),
      ],
    },
    {
      name: 'vip',
      ratio: 0.5,
      channels: [channel(null, { today: [50, 100], week: [0, 0] }, 7)],
    },
  ]

  it("splits a channel's charge by each group's share of its original-price usage", () => {
    const shares = allocateUpstreamCosts(groups, [
      upstreamCost(7, { today: 800, yesterday: 0, week: 1000 }),
    ])

    expect(shares.get(upstreamShareKey('default', 7))?.today).toBe(600)
    expect(shares.get(upstreamShareKey('vip', 7))?.today).toBe(200)
  })

  it('splits equally when the channel had no usage of its own', () => {
    const shares = allocateUpstreamCosts(groups, [
      upstreamCost(7, { today: 0, yesterday: 0, week: 1000 }),
    ])

    expect(shares.get(upstreamShareKey('default', 7))?.week).toBe(500)
    expect(shares.get(upstreamShareKey('vip', 7))?.week).toBe(500)
  })

  it('leaves out channels whose upstream could not be read', () => {
    const shares = allocateUpstreamCosts(groups, [
      upstreamCost(
        7,
        { today: 800, yesterday: 0, week: 0 },
        {
          key: 'Upstream unreachable: {{error}}',
        }
      ),
    ])

    expect(shares.size).toBe(0)
  })
})

describe('sumGroupCostFigures', () => {
  it('adds up every channel, using upstream shares where there are any', () => {
    const groups: ChannelCostGroup[] = [
      {
        name: 'default',
        ratio: 1,
        channels: [
          channel(0.05, { week: [1000, 10000] }, 1),
          channel(null, { week: [2000, 4000] }, 2),
        ],
      },
    ]
    const shares = new Map([
      [upstreamShareKey('default', 2), { today: 0, yesterday: 0, week: 700 }],
    ])

    expect(sumGroupCostFigures(groups, 'week', shares)).toEqual({
      used: 3000,
      cost: 1200,
      profit: 1800,
    })
    expect(sumGroupCostFigures([], 'today', new Map())).toEqual({
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
