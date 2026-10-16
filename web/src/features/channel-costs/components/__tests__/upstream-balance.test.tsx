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
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { cleanup, render, screen, within } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'

import { useSidebarData } from '@/hooks/use-sidebar-data'
import { api } from '@/lib/api'

import type { ChannelCostChannel, ChannelUpstreamAccount } from '../../types'
import { UpstreamBalanceSection } from '../upstream-balance-section'

let authUser: { id: number; role: number } | null = null

vi.mock('@/stores/auth-store', () => ({
  useAuthStore: (selector: (state: Record<string, unknown>) => unknown) =>
    selector({ auth: { user: authUser } }),
}))

function account(
  overrides: Partial<ChannelUpstreamAccount>
): ChannelUpstreamAccount {
  return {
    kind: 'sub2api',
    base_url: 'https://api.example.com',
    channel_ids: [7],
    balance: 100,
    daily_spend: 40,
    days_left: 2.5,
    low: false,
    ...overrides,
  }
}

const channels = [{ id: 7, name: 'claude-relay' }] as ChannelCostChannel[]

function rowOf(text: string) {
  return screen.getByText(text).closest('tr') as HTMLElement
}

afterEach(() => {
  cleanup()
  vi.restoreAllMocks()
  authUser = null
})

describe('UpstreamBalanceSection', () => {
  it('shows each account with its balance, spend and days left', () => {
    render(
      <UpstreamBalanceSection accounts={[account({})]} channels={channels} />
    )

    const row = rowOf('claude-relay')
    expect(within(row).getByText('api.example.com')).toBeInTheDocument()
    expect(within(row).getByText('$100')).toBeInTheDocument()
    expect(within(row).getByText('$40')).toBeInTheDocument()
    expect(within(row).getByText('About 2.5 days')).toBeInTheDocument()
    expect(within(row).queryByText('Less than a day left')).toBeNull()
  })

  it('counts hours and warns when less than a day is left', () => {
    render(
      <UpstreamBalanceSection
        accounts={[account({ balance: 10, days_left: 0.25, low: true })]}
        channels={channels}
      />
    )

    expect(screen.getByText('About 6 hours')).toBeInTheDocument()
    expect(screen.getByText('Less than a day left')).toBeInTheDocument()
  })

  it('tells used-up balances from accounts without recent spending', () => {
    render(
      <UpstreamBalanceSection
        accounts={[
          account({ balance: 0, days_left: 0, low: true }),
          account({
            base_url: 'https://quiet.example.com',
            channel_ids: [8],
            daily_spend: 0,
            days_left: null,
          }),
        ]}
        channels={channels}
      />
    )

    expect(screen.getByText('Used up')).toBeInTheDocument()
    expect(screen.getByText('No recent spending')).toBeInTheDocument()
  })
})

function SidebarHarness() {
  const admin = useSidebarData().navGroups.find((group) => group.id === 'admin')
  const costs = admin?.items.find((item) => item.url === '/channel-costs')
  return <output data-testid='cost-badge'>{costs?.badge ?? 'none'}</output>
}

describe('low balance sidebar badge', () => {
  it('counts the upstream accounts with less than a day left for staff', async () => {
    authUser = { id: 1, role: 100 }
    vi.spyOn(api, 'get').mockImplementation(async (url: string) => {
      if (url === '/api/channel-cost/upstream') {
        return {
          data: {
            success: true,
            data: {
              channels: [],
              accounts: [
                account({ low: true }),
                account({ channel_ids: [8], low: true }),
                account({ channel_ids: [9] }),
              ],
            },
          },
        }
      }
      return { data: { success: true, data: {} } }
    })
    render(
      <QueryClientProvider client={new QueryClient()}>
        <SidebarHarness />
      </QueryClientProvider>
    )

    expect(await screen.findByText('2')).toBeInTheDocument()
  })

  it('does not ask signed-out viewers', () => {
    const get = vi.spyOn(api, 'get')
    render(
      <QueryClientProvider client={new QueryClient()}>
        <SidebarHarness />
      </QueryClientProvider>
    )

    expect(screen.getByTestId('cost-badge')).toHaveTextContent('none')
    expect(get).not.toHaveBeenCalledWith(
      '/api/channel-cost/upstream',
      expect.anything()
    )
  })
})
