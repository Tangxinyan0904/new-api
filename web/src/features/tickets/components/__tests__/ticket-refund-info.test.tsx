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
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { api } from '@/lib/api'
import { formatQuota } from '@/lib/format'

import type { TicketRefundInfo } from '../../types'
import { TicketRefundInfoPanel } from '../ticket-refund-info'

const info: TicketRefundInfo = {
  quota: 2_500_000,
  top_up_count: 3,
  top_up_amount: 30,
  top_up_money: 210.5,
  latest_top_up: {
    payment_method: 'wxpay',
    amount: 10,
    money: 70,
    complete_time: 1_790_000_000,
  },
  redemption_count: 2,
  redemption_quota: 1_000_000,
  today_quota: 120_000,
  week_quota: 900_000,
}

function renderPanel(data: TicketRefundInfo) {
  const get = vi
    .spyOn(api, 'get')
    .mockResolvedValue({ data: { success: true, data } })
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  render(
    <QueryClientProvider client={client}>
      <TicketRefundInfoPanel ticketId={5} />
    </QueryClientProvider>
  )
  return get
}

function figure(label: string) {
  const term = screen.getByText(label)
  return within(term.parentElement as HTMLElement)
}

beforeEach(() => {
  vi.useFakeTimers({ toFake: ['Date'] })
  vi.setSystemTime(new Date(2026, 9, 13, 15, 30))
})

afterEach(() => {
  cleanup()
  vi.useRealTimers()
  vi.restoreAllMocks()
})

describe('TicketRefundInfoPanel', () => {
  it('asks for usage since local midnight today and six days earlier', async () => {
    const get = renderPanel(info)
    await screen.findByText('Remaining quota')

    const todayStart = new Date(2026, 9, 13).getTime() / 1000
    const weekStart = new Date(2026, 9, 7).getTime() / 1000
    expect(get).toHaveBeenCalledWith(
      `/api/ticket/admin/5/refund?today_start=${todayStart}&week_start=${weekStart}`
    )
  })

  it('shows what the user paid, redeemed, has left and used', async () => {
    renderPanel(info)
    await screen.findByText('Remaining quota')

    expect(
      figure('Remaining quota').getByText(formatQuota(2_500_000))
    ).toBeInTheDocument()
    expect(figure('Online top-ups').getByText('210.5')).toBeInTheDocument()
    expect(figure('Online top-ups').getByText(/^3 orders/)).toBeInTheDocument()
    expect(figure('Latest top-up').getByText('WeChat Pay')).toBeInTheDocument()
    expect(
      figure('Redemption codes').getByText(formatQuota(1_000_000))
    ).toBeInTheDocument()
    expect(figure('Redemption codes').getByText('2 codes')).toBeInTheDocument()
    expect(
      figure('Used today').getByText(formatQuota(120_000))
    ).toBeInTheDocument()
    expect(
      figure('Used in the last 7 days').getByText(formatQuota(900_000))
    ).toBeInTheDocument()
  })

  it('says None when the user never topped up online', async () => {
    renderPanel({
      ...info,
      top_up_count: 0,
      top_up_money: 0,
      latest_top_up: null,
    })
    await screen.findByText('Remaining quota')

    expect(figure('Latest top-up').getByText('None')).toBeInTheDocument()
  })
})
