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
import {
  cleanup,
  fireEvent,
  render,
  screen,
  waitFor,
} from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, describe, expect, it, vi } from 'vitest'

import { api } from '@/lib/api'

import { TicketLimitSection } from '../ticket-limit-section'

function renderSection() {
  const put = vi
    .spyOn(api, 'put')
    .mockResolvedValue({ data: { success: true } })
  const client = new QueryClient({
    defaultOptions: { mutations: { retry: false } },
  })
  const { container } = render(
    <QueryClientProvider client={client}>
      <TicketLimitSection defaultValues={{ 'ticket_setting.daily_limit': 3 }} />
    </QueryClientProvider>
  )
  const form = container.querySelector('form') as HTMLFormElement
  return {
    put,
    form,
    input: screen.getByLabelText('Tickets per user in 24 hours'),
  }
}

afterEach(() => {
  cleanup()
  vi.restoreAllMocks()
})

describe('TicketLimitSection', () => {
  it('saves 0 to turn the daily limit off', async () => {
    const user = userEvent.setup()
    const { put, form, input } = renderSection()

    await user.clear(input)
    await user.type(input, '0')
    fireEvent.submit(form)

    await waitFor(() =>
      expect(put).toHaveBeenCalledWith('/api/option/', {
        key: 'ticket_setting.daily_limit',
        value: 0,
      })
    )
  })

  it('rejects a negative limit', async () => {
    const { put, form, input } = renderSection()

    fireEvent.change(input, { target: { value: '-2' } })
    fireEvent.submit(form)

    await waitFor(() => expect(input).toHaveAttribute('aria-invalid', 'true'))
    expect(put).not.toHaveBeenCalled()
  })
})
