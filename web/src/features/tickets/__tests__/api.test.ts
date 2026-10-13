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
import { afterEach, describe, expect, it, vi } from 'vitest'

import { api } from '@/lib/api'

import { createTicket } from '../api'

afterEach(() => {
  vi.restoreAllMocks()
})

describe('createTicket', () => {
  it('sends the attached call logs as a comma-separated list', async () => {
    const post = vi
      .spyOn(api, 'post')
      .mockResolvedValue({ data: { success: true } })

    await createTicket({
      subject: 'Slow',
      category: 'api',
      content: 'See logs',
      images: [],
      logIds: [11, 4],
    })

    const form = post.mock.calls[0][1] as FormData
    expect(form.get('log_ids')).toBe('11,4')
    expect(form.get('category')).toBe('api')
  })

  it('sends an empty list when no logs are attached', async () => {
    const post = vi
      .spyOn(api, 'post')
      .mockResolvedValue({ data: { success: true } })

    await createTicket({
      subject: 'Refund',
      category: 'refund',
      content: 'x',
      images: [],
    })

    expect((post.mock.calls[0][1] as FormData).get('log_ids')).toBe('')
  })
})
