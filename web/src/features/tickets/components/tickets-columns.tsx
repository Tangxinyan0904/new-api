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
import type { ColumnDef } from '@tanstack/react-table'
import { useTranslation } from 'react-i18next'

import { formatTimestamp } from '@/lib/format'

import { TICKET_CATEGORIES } from '../constants'
import type { Ticket, TicketAudience } from '../types'
import { TicketPriorityBadge, TicketStatusBadge } from './ticket-badges'
import { TicketSubjectLink } from './ticket-subject-link'

export function useTicketsColumns(
  audience: TicketAudience
): ColumnDef<Ticket>[] {
  const { t } = useTranslation()

  const columns: (ColumnDef<Ticket> | false)[] = [
    {
      id: 'subject',
      header: t('Subject'),
      cell: ({ row }) => (
        <TicketSubjectLink ticket={row.original} audience={audience} />
      ),
      size: 320,
      enableHiding: false,
    },
    audience === 'staff' && {
      id: 'user',
      header: t('User'),
      cell: ({ row }) => (
        <span className='text-sm'>
          {row.original.username || '-'}
          <span className='text-muted-foreground ms-1 text-xs'>
            (ID {row.original.user_id})
          </span>
        </span>
      ),
      size: 160,
    },
    {
      id: 'category',
      accessorKey: 'category',
      header: t('Category'),
      cell: ({ row }) => (
        <span className='text-sm whitespace-nowrap'>
          {t(TICKET_CATEGORIES[row.original.category]?.labelKey ?? 'Other')}
        </span>
      ),
      size: 140,
    },
    audience === 'staff' && {
      id: 'priority',
      accessorKey: 'priority',
      header: t('Priority'),
      cell: ({ row }) => (
        <TicketPriorityBadge priority={row.original.priority} />
      ),
      size: 110,
    },
    {
      id: 'status',
      accessorKey: 'status',
      header: t('Status'),
      cell: ({ row }) => <TicketStatusBadge status={row.original.status} />,
      size: 150,
    },
    {
      id: 'last_reply_at',
      header: t('Last reply'),
      cell: ({ row }) => (
        <span className='text-muted-foreground text-sm whitespace-nowrap tabular-nums'>
          {formatTimestamp(row.original.last_reply_at)}
          {row.original.last_reply_by && (
            <span className='ms-1 text-xs'>
              ·{' '}
              {row.original.last_reply_by === 'staff'
                ? t('Support team')
                : t('User')}
            </span>
          )}
        </span>
      ),
      size: 220,
    },
    {
      id: 'created_at',
      header: t('Created'),
      cell: ({ row }) => (
        <span className='text-muted-foreground text-sm whitespace-nowrap tabular-nums'>
          {formatTimestamp(row.original.created_at)}
        </span>
      ),
      size: 170,
    },
  ]
  return columns.filter((column): column is ColumnDef<Ticket> => !!column)
}
