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
import { render, screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'

import type { Ticket } from '../../types'
import { TicketSummary } from '../ticket-summary'

const base: Ticket = {
  id: 12,
  user_id: 7,
  username: 'alice',
  subject: 'API returns 401',
  category: 'api',
  priority: 4,
  status: 'open',
  user_unread: false,
  last_reply_at: 1_700_000_600,
  last_reply_by: 'user',
  created_at: 1_700_000_000,
  closed_at: 0,
  closed_by: '',
}

describe('TicketSummary', () => {
  it('shows priority and owner only to staff', () => {
    const { rerender } = render(<TicketSummary ticket={base} audience='user' />)
    expect(screen.queryByText('Urgent')).not.toBeInTheDocument()
    expect(screen.queryByText(/alice/)).not.toBeInTheDocument()

    rerender(<TicketSummary ticket={base} audience='staff' />)
    expect(screen.getByText('Urgent')).toBeInTheDocument()
    expect(screen.getByText(/alice/)).toBeInTheDocument()
  })

  it('reminds only the owner about auto close while support waits for them', () => {
    const waiting: Ticket = { ...base, status: 'waiting_user' }
    const { rerender } = render(
      <TicketSummary ticket={waiting} audience='user' />
    )
    expect(
      screen.getByText(/closes automatically after 7 days/)
    ).toBeInTheDocument()

    rerender(<TicketSummary ticket={waiting} audience='staff' />)
    expect(screen.queryByText(/closes automatically/)).not.toBeInTheDocument()
  })

  it.each([
    ['auto', /Closed automatically on .* after 7 days without a reply/],
    ['user', /Closed by the user on/],
    ['staff', /Closed by support on/],
  ] as const)('explains a ticket closed by %s', (closedBy, text) => {
    render(
      <TicketSummary
        ticket={{
          ...base,
          status: 'closed',
          closed_at: 1_700_100_000,
          closed_by: closedBy,
        }}
        audience='user'
      />
    )
    expect(screen.getByText(text)).toBeInTheDocument()
  })
})
