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
import userEvent from '@testing-library/user-event'
import { afterEach, describe, expect, it, vi } from 'vitest'

import { api } from '@/lib/api'

import type {
  ChannelStatusItem,
  ChannelStatusResponse,
  ChannelStatusTimelinePoint,
} from '../../types'

let authUser: { id: number; role: number } | null = null

vi.mock('@/stores/auth-store', () => ({
  useAuthStore: (selector: (state: Record<string, unknown>) => unknown) =>
    selector({ auth: { user: authUser } }),
}))

const { ChannelStatus } = await import('@/features/channel-status')

const WINDOW_START = 1_786_838_400

function timeline(
  first: Partial<ChannelStatusTimelinePoint>
): ChannelStatusTimelinePoint[] {
  return Array.from({ length: 12 }, (_, index) => ({
    start_at: WINDOW_START + index * 300,
    status: 'operational' as const,
    success_rate: 100,
    success_count: 10,
    failure_count: 0,
    average_duration_ms: 2000,
    ...(index === 0 ? first : {}),
  }))
}

const vip: ChannelStatusItem = {
  id: 17,
  public_name: 'VIP lane',
  name_color: '#123ABC',
  group: 'vip',
  public_enabled: false,
  status: 'operational',
  success_rate: 99.2,
  success_count: 111,
  failure_count: 1,
  average_duration_ms: 2500,
  timeline: timeline({
    status: 'failed',
    success_rate: 50,
    success_count: 1,
    failure_count: 1,
  }),
}

const quiet: ChannelStatusItem = {
  id: 18,
  public_name: 'Quiet group',
  name_color: '',
  group: 'quiet',
  public_enabled: true,
  status: 'unknown',
  success_rate: null,
  success_count: 0,
  failure_count: 0,
  average_duration_ms: null,
  timeline: timeline({}).map((point) => ({
    ...point,
    status: 'unknown' as const,
    success_rate: null,
    success_count: 0,
    average_duration_ms: null,
  })),
}

function renderPage(staff: boolean) {
  authUser = staff ? { id: 1, role: 100 } : { id: 2, role: 1 }
  const items = staff
    ? [vip, quiet]
    : [{ ...quiet, group: undefined }].map((item) => ({ ...item }))
  const status: ChannelStatusResponse = {
    last_updated_at: WINDOW_START + 3000,
    window_start: WINDOW_START,
    bucket_seconds: 300,
    counts: { operational: 1, degraded: 0, failed: 0, unknown: 1 },
    items,
  }
  const get = vi.spyOn(api, 'get').mockImplementation(async (url: string) => {
    if (url === '/api/channel-monitor/status') {
      return { data: { success: true, data: status } }
    }
    if (url === '/api/channel-monitor/status/17') {
      return {
        data: {
          success: true,
          data: {
            last_updated_at: WINDOW_START + 3000,
            monitor: vip,
            models: [
              {
                model: 'gpt-5',
                status: 'operational',
                success_rate: 100,
                success_count: 100,
                failure_count: 0,
                average_duration_ms: 2000,
              },
              {
                model: 'claude',
                status: 'failed',
                success_rate: 50,
                success_count: 1,
                failure_count: 1,
                average_duration_ms: 3000,
              },
            ],
          },
        },
      }
    }
    throw new Error(`Unexpected request ${url}`)
  })
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  render(
    <QueryClientProvider client={client}>
      <ChannelStatus />
    </QueryClientProvider>
  )
  return get
}

afterEach(() => {
  cleanup()
  vi.restoreAllMocks()
  authUser = null
})

describe('ChannelStatus', () => {
  it('shows the last-hour success rate, requests and average duration', async () => {
    renderPage(true)
    const card = await screen.findByRole('button', {
      name: 'View VIP lane details',
    })

    expect(within(card).getByText('99.2%')).toBeInTheDocument()
    expect(within(card).getByText('112')).toBeInTheDocument()
    expect(within(card).getByText('2.5s')).toBeInTheDocument()
  })

  it('draws twelve five-minute bars oldest first with their outcomes', async () => {
    renderPage(true)
    const card = await screen.findByRole('button', {
      name: 'View VIP lane details',
    })

    const bars = card.querySelectorAll('[data-slot="timeline-point"]')
    expect(bars).toHaveLength(12)
    expect(bars[0]).toHaveAttribute('data-status', 'failed')
    expect(bars[0].getAttribute('aria-label')).toContain(
      '1 succeeded, 1 failed'
    )
    expect(bars[11]).toHaveAttribute('data-status', 'operational')
  })

  it('says no calls for a group without traffic', async () => {
    renderPage(true)
    const card = await screen.findByRole('button', {
      name: 'View Quiet group details',
    })

    expect(within(card).getByRole('status')).toHaveTextContent('No calls')
    expect(within(card).getAllByText('-')).toHaveLength(2)
  })

  it('opens the per-model breakdown when a card is chosen', async () => {
    const user = userEvent.setup()
    renderPage(true)

    await user.click(
      await screen.findByRole('button', { name: 'View VIP lane details' })
    )

    const rows = await screen.findAllByText(/^(gpt-5|claude)$/)
    expect(rows.map((row) => row.textContent)).toEqual(['gpt-5', 'claude'])
  })

  it('lets staff see the group and add, edit or delete monitors', async () => {
    renderPage(true)
    await screen.findByRole('button', { name: 'View VIP lane details' })

    expect(screen.getByRole('button', { name: 'Add monitor' })).toBeVisible()
    expect(screen.getByText('Group: vip')).toBeInTheDocument()
    expect(screen.getByText('Staff only')).toBeInTheDocument()
    expect(screen.getAllByRole('button', { name: 'Edit' })).toHaveLength(2)
    expect(screen.getAllByRole('button', { name: 'Delete' })).toHaveLength(2)
  })

  it('shows users the public monitors without management controls', async () => {
    renderPage(false)
    await screen.findByRole('button', { name: 'View Quiet group details' })

    expect(screen.queryByRole('button', { name: 'Add monitor' })).toBeNull()
    expect(screen.queryByRole('button', { name: 'Edit' })).toBeNull()
    expect(screen.queryByText(/^Group:/)).toBeNull()
  })
})
