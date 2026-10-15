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
import { cleanup, render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, describe, expect, it, vi } from 'vitest'

import { api } from '@/lib/api'

import { TicketOwnerBan } from '../ticket-owner-ban'

function renderBan(banned: boolean, response = { success: true }) {
  const put = vi.spyOn(api, 'put').mockResolvedValue({ data: response })
  const client = new QueryClient({
    defaultOptions: { mutations: { retry: false } },
  })
  render(
    <QueryClientProvider client={client}>
      <TicketOwnerBan ticketId={7} ownerName='alice' banned={banned} />
    </QueryClientProvider>
  )
  return put
}

afterEach(() => {
  cleanup()
  vi.restoreAllMocks()
})

describe('TicketOwnerBan', () => {
  it('blocks the owner only after the confirmation is accepted', async () => {
    const user = userEvent.setup()
    const put = renderBan(false)

    await user.click(screen.getByRole('button', { name: 'Block from tickets' }))
    expect(screen.getByRole('alertdialog')).toHaveTextContent(
      'Block alice from tickets?'
    )
    expect(put).not.toHaveBeenCalled()

    await user.click(screen.getByRole('button', { name: 'Block' }))
    await waitFor(() => expect(screen.queryByRole('alertdialog')).toBeNull())
    expect(put).toHaveBeenCalledWith(
      '/api/ticket/admin/7/ban',
      { banned: true },
      expect.anything()
    )
  })

  it('keeps the confirmation open when blocking fails', async () => {
    const user = userEvent.setup()
    const put = renderBan(false, { success: false })

    await user.click(screen.getByRole('button', { name: 'Block from tickets' }))
    await user.click(screen.getByRole('button', { name: 'Block' }))

    await waitFor(() => expect(put).toHaveBeenCalledTimes(1))
    expect(screen.getByRole('alertdialog')).toBeInTheDocument()
  })

  it('shows the block and unblocks without a confirmation', async () => {
    const user = userEvent.setup()
    const put = renderBan(true)

    expect(screen.getByText('Blocked from tickets')).toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: 'Unblock' }))

    expect(screen.queryByRole('alertdialog')).toBeNull()
    await waitFor(() =>
      expect(put).toHaveBeenCalledWith(
        '/api/ticket/admin/7/ban',
        { banned: false },
        expect.anything()
      )
    )
  })
})
