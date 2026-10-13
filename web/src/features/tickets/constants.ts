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
import type { StatusVariant } from '@/components/status-badge'

import type { TicketCategory, TicketPriority, TicketStatus } from './types'

// Limits mirror model/ticket.go; the server enforces them.
export const TICKET_LIMITS = {
  SUBJECT_MAX: 100,
  CONTENT_MAX: 5000,
  MAX_IMAGES: 3,
  MAX_IMAGE_BYTES: 2 * 1024 * 1024,
  MAX_OPEN_TICKETS: 5,
  AUTO_CLOSE_DAYS: 7,
  MAX_LOGS: 5,
} as const

/** Log type of a failed request (usage-logs LOG_TYPE_ENUM.ERROR). */
export const TICKET_LOG_TYPE_ERROR = 5

export const TICKET_IMAGE_TYPES = ['image/png', 'image/jpeg', 'image/webp']

// labelKey values are i18n keys; render them with t(config.labelKey).
export const TICKET_STATUSES: Record<
  TicketStatus,
  { labelKey: string; variant: StatusVariant }
> = {
  open: { labelKey: 'Awaiting support', variant: 'warning' },
  processing: { labelKey: 'In progress', variant: 'info' },
  waiting_user: { labelKey: 'Awaiting user reply', variant: 'purple' },
  closed: { labelKey: 'Closed', variant: 'neutral' },
}

export const TICKET_STATUS_VALUES = Object.keys(
  TICKET_STATUSES
) as TicketStatus[]

export const TICKET_CATEGORIES: Record<TicketCategory, { labelKey: string }> = {
  billing: { labelKey: 'Billing & top-up' },
  api: { labelKey: 'API & models' },
  account: { labelKey: 'Account & security' },
  refund: { labelKey: 'Refund' },
  other: { labelKey: 'Other' },
}

export const TICKET_CATEGORY_VALUES = Object.keys(
  TICKET_CATEGORIES
) as TicketCategory[]

export const TICKET_PRIORITIES: Record<
  TicketPriority,
  { labelKey: string; variant: StatusVariant }
> = {
  1: { labelKey: 'Low', variant: 'neutral' },
  2: { labelKey: 'Normal', variant: 'info' },
  3: { labelKey: 'High', variant: 'warning' },
  4: { labelKey: 'Urgent', variant: 'danger' },
}

export const TICKET_PRIORITY_VALUES = [4, 3, 2, 1] as TicketPriority[]

/** Statuses staff can pick when replying; replying never re-opens to "open". */
export const TICKET_REPLY_STATUSES: TicketStatus[] = [
  'waiting_user',
  'processing',
  'closed',
]

export const ticketQueryKeys = {
  all: ['tickets'] as const,
  userList: (params: Record<string, unknown>) =>
    ['tickets', 'user', 'list', params] as const,
  userDetail: (id: number) => ['tickets', 'user', 'detail', id] as const,
  userUnread: ['tickets', 'user', 'unread'] as const,
  staffList: (params: Record<string, unknown>) =>
    ['tickets', 'staff', 'list', params] as const,
  staffDetail: (id: number) => ['tickets', 'staff', 'detail', id] as const,
  staffSummary: ['tickets', 'staff', 'summary'] as const,
  logCandidates: (params: Record<string, unknown>) =>
    ['tickets', 'user', 'logs', params] as const,
  staffRefund: (id: number) => ['tickets', 'staff', 'refund', id] as const,
  // Outside the 'tickets' prefix: attachments never change, so refreshing a
  // ticket must not download its images again.
  attachment: (audience: string, id: number) =>
    ['ticket-attachments', audience, id] as const,
}

/** Sidebar badge polling interval. */
export const TICKET_BADGE_REFETCH_MS = 60_000
