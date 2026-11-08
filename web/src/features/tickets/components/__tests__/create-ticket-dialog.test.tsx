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

import { CreateTicketDialog } from '../create-ticket-dialog'

const navigate = vi.hoisted(() => vi.fn())

vi.mock('@tanstack/react-router', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@tanstack/react-router')>()),
  useNavigate: () => navigate,
}))

const QR_CODE_ERROR = 'Refund tickets need an image of your payment QR code'

/** Opens the dialog and fills a ticket in category, without images. */
async function fillTicket(category: string) {
  const user = userEvent.setup()
  const post = vi
    .spyOn(api, 'post')
    .mockResolvedValue({ data: { success: true, data: { id: 9 } } })
  render(
    <QueryClientProvider client={new QueryClient()}>
      <CreateTicketDialog />
    </QueryClientProvider>
  )
  await user.click(screen.getByRole('button', { name: 'New ticket' }))
  await user.type(
    screen.getByRole('textbox', { name: 'Subject' }),
    'Please refund'
  )
  await user.type(
    screen.getByRole('textbox', { name: 'Problem description' }),
    'I no longer need the balance.'
  )
  await user.click(screen.getByRole('combobox', { name: 'Category' }))
  await user.click(await screen.findByRole('option', { name: category }))
  return { user, post }
}

afterEach(() => {
  cleanup()
  vi.restoreAllMocks()
  navigate.mockReset()
})

describe('CreateTicketDialog refund QR code', () => {
  it('does not submit a refund ticket without a payment QR code image', async () => {
    const { user, post } = await fillTicket('Refund')
    expect(screen.getByText('Attach your payment QR code')).toBeInTheDocument()
    expect(screen.queryByText(QR_CODE_ERROR)).toBeNull()

    await user.click(screen.getByRole('button', { name: 'Submit' }))

    expect(await screen.findByText(QR_CODE_ERROR)).toBeInTheDocument()
    expect(post).not.toHaveBeenCalled()
  })

  it('submits a refund ticket once a QR code image is attached', async () => {
    const { user, post } = await fillTicket('Refund')
    await user.upload(
      screen.getByTestId('ticket-image-input'),
      new File(['qr'], 'qr.png', { type: 'image/png' })
    )

    await user.click(screen.getByRole('button', { name: 'Submit' }))

    await waitFor(() => expect(post).toHaveBeenCalledTimes(1))
    const form = post.mock.calls[0][1] as FormData
    expect(form.get('category')).toBe('refund')
    expect(form.getAll('images')).toHaveLength(1)
    expect(screen.queryByText(QR_CODE_ERROR)).toBeNull()
  })

  it('does not ask for a QR code on other categories', async () => {
    const { user, post } = await fillTicket('Billing & top-up')
    expect(screen.queryByText('Attach your payment QR code')).toBeNull()

    await user.click(screen.getByRole('button', { name: 'Submit' }))

    await waitFor(() => expect(post).toHaveBeenCalledTimes(1))
    expect((post.mock.calls[0][1] as FormData).get('category')).toBe('billing')
  })
})
