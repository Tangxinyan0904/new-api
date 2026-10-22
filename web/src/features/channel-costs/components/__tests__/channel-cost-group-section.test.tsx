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
import { render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi } from 'vitest'

import { formatQuota } from '@/lib/format'

import {
  upstreamShareKey,
  type UpstreamCostShares,
} from '../../lib/channel-cost-math'
import type {
  ChannelCostChannel,
  ChannelCostGroup,
  ChannelUpstreamBilling,
  ChannelUpstreamCost,
} from '../../types'
import { ChannelCostGroupSection } from '../channel-cost-group-section'

function channel(
  id: number,
  name: string,
  ratio: number | null,
  week: [number, number],
  upstreamBilling: ChannelUpstreamBilling | null = null
): ChannelCostChannel {
  const none = { quota: 0, base_quota: 0 }
  return {
    id,
    name,
    type: 1,
    status: 1,
    ratio,
    upstream_billing: upstreamBilling,
    newapi_login: null,
    today: none,
    yesterday: none,
    week: { quota: week[0], base_quota: week[1] },
  }
}

function renderSection(
  group: ChannelCostGroup,
  onSaveRatio = vi.fn(async () => true),
  upstream: {
    shares?: UpstreamCostShares
    costs?: ChannelUpstreamCost[]
  } = {}
) {
  const onSaveUpstream = vi.fn(async () => true)
  render(
    <ChannelCostGroupSection
      group={group}
      canEdit
      onSaveRatio={onSaveRatio}
      shares={upstream.shares ?? new Map()}
      upstreamCosts={
        new Map((upstream.costs ?? []).map((cost) => [cost.channel_id, cost]))
      }
      upstreamLoading={false}
      onSaveUpstream={onSaveUpstream}
    />
  )
  return {
    onSaveRatio,
    onSaveUpstream,
    section: screen.getByRole('region', { name: group.name }),
  }
}

const sub2apiBilling: ChannelUpstreamBilling = {
  kind: 'sub2api',
  usd_rate: 0.5,
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

  it('uses the upstream bill instead of the ratio for a channel that reads it', () => {
    const { section } = renderSection(
      {
        name: 'vip',
        ratio: 0.5,
        channels: [channel(7, 'relay', 0.08, [1000, 5000], sub2apiBilling)],
      },
      undefined,
      {
        shares: new Map([
          [upstreamShareKey('vip', 7), { today: 0, yesterday: 0, week: 300 }],
        ]),
      }
    )

    expect(
      within(section).getByText(`Upstream cost ${formatQuota(300)}`, {
        selector: 'span:not(.text-xs)',
      })
    ).toBeInTheDocument()
    expect(
      within(section).getByText('Upstream bill · sub2api')
    ).toBeInTheDocument()
    expect(
      within(section).getByText('Upstream $1 = site $0.5')
    ).toBeInTheDocument()
    expect(within(section).queryByRole('spinbutton')).not.toBeInTheDocument()
  })

  it('shows why the upstream could not be read', () => {
    renderSection(
      {
        name: 'vip',
        ratio: 0.5,
        channels: [channel(7, 'relay', null, [10, 10], sub2apiBilling)],
      },
      undefined,
      {
        costs: [
          {
            channel_id: 7,
            kind: 'sub2api',
            error: {
              key: 'Upstream answered {{status}}: {{error}}',
              params: { status: 401, error: 'Invalid API key' },
            },
            fetched_at: 0,
            today: { usd: 0, quota: 0 },
            yesterday: { usd: 0, quota: 0 },
            week: { usd: 0, quota: 0 },
          },
        ],
      }
    )

    expect(
      screen.getByText('Upstream answered 401: Invalid API key')
    ).toBeInTheDocument()
  })

  it('saves a sub2api upstream bill with the dollar rate', async () => {
    const user = userEvent.setup()
    const { onSaveUpstream } = renderSection({
      name: 'vip',
      ratio: 0.5,
      channels: [channel(7, 'relay', null, [10, 10])],
    })

    await user.click(screen.getByRole('button', { name: 'Upstream bill' }))
    await user.click(screen.getByRole('combobox'))
    await user.click(await screen.findByRole('option', { name: 'sub2api' }))
    const rate = screen.getByLabelText(
      "1 upstream dollar = how many of this site's dollars"
    )
    await user.clear(rate)
    await user.type(rate, '0.143')
    await user.click(screen.getByRole('button', { name: 'Save' }))

    await waitFor(() =>
      expect(onSaveUpstream).toHaveBeenCalledWith(7, {
        kind: 'sub2api',
        usd_rate: 0.143,
        upstream_user_id: 0,
        access_token: '',
      })
    )
  })

  it('asks for the user ID and access token of a new-api upstream', async () => {
    const user = userEvent.setup()
    const { onSaveUpstream } = renderSection({
      name: 'vip',
      ratio: 0.5,
      channels: [channel(7, 'relay', null, [10, 10])],
    })

    await user.click(screen.getByRole('button', { name: 'Upstream bill' }))
    await user.click(screen.getByRole('combobox'))
    await user.click(await screen.findByRole('option', { name: 'new-api' }))
    await user.click(screen.getByRole('button', { name: 'Save' }))

    expect(
      await screen.findByText('Enter your user ID on the upstream')
    ).toBeInTheDocument()
    expect(
      screen.getByText('Enter the system access token of your upstream account')
    ).toBeInTheDocument()
    expect(onSaveUpstream).not.toHaveBeenCalled()
  })

  it('reuses the new-api login saved for the same upstream address', async () => {
    const user = userEvent.setup()
    const { onSaveUpstream } = renderSection({
      name: 'vip',
      ratio: 0.5,
      channels: [
        {
          ...channel(8, 'relay-b', null, [10, 10]),
          newapi_login: {
            upstream_user_id: 42,
            shared_with: [{ id: 7, name: 'relay-a' }],
          },
        },
      ],
    })

    await user.click(screen.getByRole('button', { name: 'Upstream bill' }))
    await user.click(screen.getByRole('combobox'))
    await user.click(await screen.findByRole('option', { name: 'new-api' }))

    expect(screen.getByLabelText('Your user ID on the upstream')).toHaveValue(
      42
    )
    expect(
      screen.getByText(
        'Shared with relay-a on the same upstream address. Saving here updates it everywhere.'
      )
    ).toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: 'Save' }))
    await waitFor(() =>
      expect(onSaveUpstream).toHaveBeenCalledWith(8, {
        kind: 'newapi',
        usd_rate: 1,
        upstream_user_id: 42,
        access_token: '',
      })
    )
  })
})
