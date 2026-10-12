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
import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi } from 'vitest'

import type { TicketMessageInput } from '../../api'
import { TicketReplyForm } from '../ticket-reply-form'

function renderForm(onSubmit: (input: TicketMessageInput) => Promise<boolean>) {
  render(<TicketReplyForm onSubmit={onSubmit} isSubmitting={false} />)
  return {
    textbox: screen.getByRole('textbox', { name: 'Reply' }),
    send: screen.getByRole('button', { name: 'Send reply' }),
  }
}

describe('TicketReplyForm', () => {
  it('keeps send disabled until there is text', async () => {
    const user = userEvent.setup()
    const { textbox, send } = renderForm(vi.fn(async () => true))

    expect(send).toBeDisabled()
    await user.type(textbox, '   ')
    expect(send).toBeDisabled()
    await user.type(textbox, 'hello')
    expect(send).toBeEnabled()
  })

  it('sends trimmed text and clears the form when the reply is saved', async () => {
    const user = userEvent.setup()
    const onSubmit = vi.fn(async () => true)
    const { textbox, send } = renderForm(onSubmit)

    await user.type(textbox, '  Still failing  ')
    await user.click(send)

    expect(onSubmit).toHaveBeenCalledWith({
      content: 'Still failing',
      images: [],
    })
    expect(textbox).toHaveValue('')
  })

  it('keeps the text when the reply fails', async () => {
    const user = userEvent.setup()
    const { textbox, send } = renderForm(vi.fn(async () => false))

    await user.type(textbox, 'Please check')
    await user.click(send)

    expect(textbox).toHaveValue('Please check')
  })

  it('sends with Ctrl+Enter', async () => {
    const user = userEvent.setup()
    const onSubmit = vi.fn(async () => true)
    const { textbox } = renderForm(onSubmit)

    await user.type(textbox, 'Quick reply')
    await user.keyboard('{Control>}{Enter}{/Control}')

    expect(onSubmit).toHaveBeenCalledWith({
      content: 'Quick reply',
      images: [],
    })
  })

  it('marks text over the limit invalid and blocks sending', async () => {
    const user = userEvent.setup()
    const { textbox, send } = renderForm(vi.fn(async () => true))

    await user.click(textbox)
    await user.paste('a'.repeat(5001))

    expect(textbox).toHaveAttribute('aria-invalid', 'true')
    expect(send).toBeDisabled()
  })
})
