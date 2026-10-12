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
import { useTranslation } from 'react-i18next'

import { formatTimestamp } from '@/lib/format'

import { TICKET_CATEGORIES, TICKET_LIMITS } from '../constants'
import type { Ticket, TicketAudience } from '../types'
import { TicketPriorityBadge, TicketStatusBadge } from './ticket-badges'

interface TicketSummaryProps {
  ticket: Ticket
  audience: TicketAudience
}

export function TicketSummary(props: TicketSummaryProps) {
  const { t } = useTranslation()
  const ticket = props.ticket
  const closedNote = getClosedNote(ticket, props.audience, t)

  return (
    <section
      aria-label={t('Ticket details')}
      className='bg-card flex flex-col gap-3 rounded-lg border p-3 sm:p-4'
    >
      <div className='flex flex-wrap items-start justify-between gap-2'>
        <h3 className='min-w-0 text-base font-semibold break-words'>
          <span className='text-muted-foreground me-2 tabular-nums'>
            #{ticket.id}
          </span>
          {ticket.subject}
        </h3>
        <div className='flex flex-wrap items-center gap-2'>
          {props.audience === 'staff' && (
            <TicketPriorityBadge priority={ticket.priority} />
          )}
          <TicketStatusBadge status={ticket.status} />
        </div>
      </div>
      <dl className='text-muted-foreground grid grid-cols-1 gap-x-6 gap-y-1 text-xs sm:grid-cols-3'>
        {props.audience === 'staff' && (
          <div className='flex gap-1'>
            <dt>{t('User')}:</dt>
            <dd className='text-foreground'>
              {ticket.username || '-'} (ID {ticket.user_id})
            </dd>
          </div>
        )}
        <div className='flex gap-1'>
          <dt>{t('Category')}:</dt>
          <dd className='text-foreground'>
            {t(TICKET_CATEGORIES[ticket.category]?.labelKey ?? 'Other')}
          </dd>
        </div>
        <div className='flex gap-1'>
          <dt>{t('Created')}:</dt>
          <dd className='text-foreground tabular-nums'>
            {formatTimestamp(ticket.created_at)}
          </dd>
        </div>
        <div className='flex gap-1'>
          <dt>{t('Last reply')}:</dt>
          <dd className='text-foreground tabular-nums'>
            {formatTimestamp(ticket.last_reply_at)}
          </dd>
        </div>
      </dl>
      {closedNote && (
        <p className='bg-muted text-muted-foreground rounded-md px-3 py-2 text-xs'>
          {closedNote}
        </p>
      )}
    </section>
  )
}

function getClosedNote(
  ticket: Ticket,
  audience: TicketAudience,
  t: (key: string, options?: Record<string, unknown>) => string
): string | null {
  if (ticket.status === 'waiting_user' && audience === 'user') {
    return t(
      'Support is waiting for your reply. The ticket closes automatically after {{days}} days without a reply.',
      { days: TICKET_LIMITS.AUTO_CLOSE_DAYS }
    )
  }
  if (ticket.status !== 'closed') return null
  const when = formatTimestamp(ticket.closed_at)
  if (ticket.closed_by === 'auto') {
    return t(
      'Closed automatically on {{time}} after {{days}} days without a reply.',
      { time: when, days: TICKET_LIMITS.AUTO_CLOSE_DAYS }
    )
  }
  if (ticket.closed_by === 'user') {
    return t('Closed by the user on {{time}}.', { time: when })
  }
  return t('Closed by support on {{time}}.', { time: when })
}
