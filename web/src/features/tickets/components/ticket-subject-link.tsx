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
import { Link } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'

import type { Ticket, TicketAudience } from '../types'

export function TicketSubjectLink(props: {
  ticket: Ticket
  audience: TicketAudience
}) {
  const { t } = useTranslation()
  // Unread means new staff activity for the owner, and a ticket awaiting
  // support for staff.
  const highlighted =
    props.audience === 'user'
      ? props.ticket.user_unread
      : props.ticket.status === 'open'
  const linkProps =
    props.audience === 'user'
      ? { to: '/tickets/$ticketId' as const }
      : { to: '/ticket-management/$ticketId' as const }

  return (
    <Link
      {...linkProps}
      params={{ ticketId: String(props.ticket.id) }}
      className='hover:text-primary flex min-w-0 items-center gap-2 font-medium'
    >
      {highlighted && (
        <span
          className='bg-destructive size-2 shrink-0 rounded-full'
          role='img'
          aria-label={
            props.audience === 'user' ? t('New reply') : t('Awaiting support')
          }
        />
      )}
      <span className='text-muted-foreground shrink-0 tabular-nums'>
        #{props.ticket.id}
      </span>
      <span className='truncate'>{props.ticket.subject}</span>
    </Link>
  )
}
