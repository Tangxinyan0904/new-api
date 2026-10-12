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
import type { TFunction } from 'i18next'
import { z } from 'zod'

import { TICKET_CATEGORY_VALUES, TICKET_LIMITS } from '../constants'

// Use getCreateTicketSchema(t) in components so messages follow the language.
export function getCreateTicketSchema(t: TFunction) {
  return z.object({
    subject: z
      .string()
      .trim()
      .min(1, t('Ticket subject is required'))
      .max(
        TICKET_LIMITS.SUBJECT_MAX,
        t('Ticket subject must be at most {{max}} characters', {
          max: TICKET_LIMITS.SUBJECT_MAX,
        })
      ),
    category: z.enum(TICKET_CATEGORY_VALUES, {
      message: t('Invalid ticket category'),
    }),
    content: z
      .string()
      .trim()
      .min(1, t('Ticket content is required'))
      .max(
        TICKET_LIMITS.CONTENT_MAX,
        t('Ticket content must be at most {{max}} characters', {
          max: TICKET_LIMITS.CONTENT_MAX,
        })
      ),
  })
}

export type CreateTicketFormValues = z.infer<
  ReturnType<typeof getCreateTicketSchema>
>

export const CREATE_TICKET_DEFAULT_VALUES: CreateTicketFormValues = {
  subject: '',
  category: 'api',
  content: '',
}
