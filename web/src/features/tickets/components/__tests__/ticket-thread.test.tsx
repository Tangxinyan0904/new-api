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
import { describe, expect, it } from 'vitest'

import type { TicketMessage } from '../../types'
import { TicketThread } from '../ticket-thread'

const messages: TicketMessage[] = [
  {
    id: 1,
    is_staff: false,
    content: 'Line one\nLine two',
    created_at: 1_700_000_000,
    attachments: [],
  },
  {
    id: 2,
    is_staff: true,
    content: 'Please rotate your key.',
    author_name: 'root',
    created_at: 1_700_000_600,
    attachments: [],
  },
]

function authorLabels() {
  return within(screen.getByRole('list', { name: 'Conversation' }))
    .getAllByRole('listitem')
    .map((item) => item.querySelector('span')?.textContent)
}

describe('TicketThread', () => {
  it('shows owners their own messages and the support team without staff names', () => {
    render(<TicketThread audience='user' messages={messages} />)

    expect(authorLabels()).toEqual(['You', 'Support team'])
    expect(screen.queryByText(/root/)).not.toBeInTheDocument()
  })

  it('shows staff the owner name and which admin replied', () => {
    render(
      <TicketThread audience='staff' messages={messages} ownerName='alice' />
    )

    expect(authorLabels()).toEqual(['alice', 'root · Support team'])
  })

  it('keeps line breaks in message text', () => {
    render(<TicketThread audience='user' messages={messages} />)

    const text = screen.getByText(/Line one/)
    expect(text.textContent).toBe('Line one\nLine two')
    expect(text).toHaveClass('whitespace-pre-wrap')
  })
})
