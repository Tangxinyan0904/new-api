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
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { TicketAttachmentImage } from '../ticket-attachment-image'

const fetchTicketAttachment = vi.hoisted(() => vi.fn())

// Images are fetched with the signed-in session; mock that network call.
vi.mock('../../api', () => ({ fetchTicketAttachment }))

function renderImage() {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  render(
    <QueryClientProvider client={client}>
      <TicketAttachmentImage
        audience='staff'
        attachment={{ id: 9, mime_type: 'image/png', size: 4 }}
      />
    </QueryClientProvider>
  )
}

const originalCreateObjectURL = URL.createObjectURL
const originalRevokeObjectURL = URL.revokeObjectURL

beforeEach(() => {
  URL.createObjectURL = vi.fn(() => 'blob:ticket-9')
  URL.revokeObjectURL = vi.fn()
})

afterEach(() => {
  URL.createObjectURL = originalCreateObjectURL
  URL.revokeObjectURL = originalRevokeObjectURL
  fetchTicketAttachment.mockReset()
})

describe('TicketAttachmentImage', () => {
  it('loads the image through the staff endpoint and opens a preview', async () => {
    fetchTicketAttachment.mockResolvedValue(new Blob(['png']))
    const user = userEvent.setup()
    renderImage()

    const thumbnail = await screen.findByRole('button', {
      name: 'View Attachment #9',
    })
    expect(fetchTicketAttachment).toHaveBeenCalledWith('staff', 9)
    expect(screen.getByRole('img', { name: 'Attachment #9' })).toHaveAttribute(
      'src',
      'blob:ticket-9'
    )

    await user.click(thumbnail)
    expect(await screen.findByRole('dialog')).toBeInTheDocument()
  })

  it('shows a placeholder when the image cannot be loaded', async () => {
    fetchTicketAttachment.mockRejectedValue(new Error('not found'))
    renderImage()

    expect(
      await screen.findByRole('img', { name: 'Image unavailable' })
    ).toBeInTheDocument()
  })
})
