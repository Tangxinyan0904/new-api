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
import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi } from 'vitest'

import { formatQuota } from '@/lib/format'

import type { ChannelCostChannel, ChannelCostGroup } from '../../types'
import { ChannelCostGroupSection } from '../channel-cost-group-section'

function channel(
  id: number,
  name: string,
  ratio: number | null,
  week: [number, number],
  deleted = false
): ChannelCostChannel {
  const none = { quota: 0, base_quota: 0 }
  return {
    id,
    name,
    type: 1,
    status: 1,
    deleted,
    ratio,
    today: none,
    yesterday: none,
    week: { quota: week[0], base_quota: week[1] },
  }
}

function renderSection(
  group: ChannelCostGroup,
  onSaveRatio = vi.fn(async () => true)
) {
  render(
    <ChannelCostGroupSection group={group} canEdit onSaveRatio={onSaveRatio} />
  )
  return {
    onSaveRatio,
    section: screen.getByRole('region', { name: group.name }),
  }
}

describe('ChannelCostGroupSection', () => {
  it('shows the group ratio and the 7-day totals of its channels', () => {
    // gemini-a costs 5000 × 0.08 = 400 and gemini-b 2000 × 0.1 = 200.
    const { section } = renderSection({
      name: 'Gemini',
      ratio: 0.25,
      channels: [
        channel(1, 'gemini-a', 0.08, [1000, 5000]),
        channel(2, 'gemini-b', 0.1, [500, 2000]),
      ],
    })

    expect(within(section).getByText('Group ratio 0.25')).toBeInTheDocument()
    expect(
      within(section).getByText(`Usage amount ${formatQuota(1500)}`)
    ).toBeInTheDocument()
    expect(
      within(section).getByText(`Upstream cost ${formatQuota(600)}`)
    ).toBeInTheDocument()
    expect(
      within(section).getByText(`Profit ${formatQuota(900)}`)
    ).toBeInTheDocument()
  })

  it('says so when the group has no configured ratio', () => {
    const { section } = renderSection({
      name: 'legacy',
      ratio: null,
      channels: [channel(3, 'old', null, [10, 10])],
    })

    expect(within(section).getByText('No group ratio')).toBeInTheDocument()
  })

  it("saves an edited ratio under that channel's id", async () => {
    const user = userEvent.setup()
    const { onSaveRatio } = renderSection({
      name: 'vip',
      ratio: 0.5,
      channels: [
        channel(1, 'first', 0.4, [100, 200]),
        channel(7, 'second', null, [50, 100]),
      ],
    })

    await user.type(
      screen.getByRole('spinbutton', { name: 'Cost ratio of second' }),
      '0.08{Enter}'
    )

    expect(onSaveRatio).toHaveBeenCalledWith(7, 0.08)
  })

  it('shows the ratio of a deleted channel as text instead of an input', () => {
    renderSection({
      name: 'vip',
      ratio: 0.5,
      channels: [channel(9, '', 0.1, [70, 140], true)],
    })

    expect(screen.getByText('Deleted channel')).toBeInTheDocument()
    expect(screen.getByText('0.1')).toBeInTheDocument()
    expect(screen.queryByRole('spinbutton')).not.toBeInTheDocument()
  })
})
