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
import { render, screen, within } from '@testing-library/react'
import { describe, expect, test } from 'vitest'

import { formatQuota } from '@/lib/format'

import type { ChannelCostGroup } from '../../types'
import { ChannelCostSummary } from '../channel-cost-summary'

const usage = { quota: 1000, base_quota: 4000 }
const groups: ChannelCostGroup[] = [
  {
    name: 'default',
    ratio: 1,
    channels: [
      {
        id: 1,
        name: 'relay',
        type: 1,
        status: 1,
        deleted: false,
        ratio: 0.1,
        upstream_billing: null,
        newapi_login: null,
        today: usage,
        yesterday: usage,
        week: usage,
      },
    ],
  },
]

describe('ChannelCostSummary', () => {
  test('keeps upstream cost and profit visible on phones for every range', () => {
    render(
      <ChannelCostSummary groups={groups} shares={new Map()} loading={false} />
    )

    for (const range of ['today', 'yesterday', 'week']) {
      const card = screen.getByTestId(`cost-summary-${range}`)
      // 4000 × 0.1 = 400 cost against 1000 used.
      for (const [label, value] of [
        ['Upstream cost', formatQuota(400)],
        ['Profit', formatQuota(600)],
      ]) {
        // Tailwind's `hidden` (display: none) applies below the sm breakpoint.
        expect(within(card).getByText(label).closest('.hidden')).toBeNull()
        expect(within(card).getByText(value).closest('.hidden')).toBeNull()
      }
    }
  })
})
