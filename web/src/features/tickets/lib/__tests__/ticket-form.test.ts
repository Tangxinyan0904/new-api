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
import i18next from 'i18next'
import { describe, expect, it } from 'vitest'

import { TICKET_LIMITS } from '../../constants'
import { getCreateTicketSchema } from '../ticket-form'

// Built per test: i18next is initialized by the shared setup's beforeAll.
function firstError(values: Record<string, unknown>): string | undefined {
  const result = getCreateTicketSchema(i18next.t.bind(i18next)).safeParse(
    values
  )
  return result.success ? undefined : result.error.issues[0]?.message
}

describe('getCreateTicketSchema', () => {
  it('accepts a valid ticket and trims the text fields', () => {
    const result = getCreateTicketSchema(i18next.t.bind(i18next)).safeParse({
      subject: '  API returns 401  ',
      category: 'api',
      content: '  Every request fails.  ',
    })

    expect(result.success).toBe(true)
    expect(result.data).toEqual({
      subject: 'API returns 401',
      category: 'api',
      content: 'Every request fails.',
    })
  })

  it.each([
    {
      name: 'blank subject',
      values: { subject: '   ', category: 'api', content: 'x' },
      message: 'Ticket subject is required',
    },
    {
      name: 'subject over the limit',
      values: {
        subject: 'a'.repeat(TICKET_LIMITS.SUBJECT_MAX + 1),
        category: 'api',
        content: 'x',
      },
      message: 'Ticket subject must be at most 100 characters',
    },
    {
      name: 'unknown category',
      values: { subject: 's', category: 'sales', content: 'x' },
      message: 'Invalid ticket category',
    },
    {
      name: 'blank content',
      values: { subject: 's', category: 'api', content: ' ' },
      message: 'Ticket content is required',
    },
    {
      name: 'content over the limit',
      values: {
        subject: 's',
        category: 'api',
        content: 'a'.repeat(TICKET_LIMITS.CONTENT_MAX + 1),
      },
      message: 'Ticket content must be at most 5000 characters',
    },
  ])('rejects $name', (testCase) => {
    expect(firstError(testCase.values)).toBe(testCase.message)
  })
})
