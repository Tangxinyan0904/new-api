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
export type TicketStatus = 'open' | 'processing' | 'waiting_user' | 'closed'

export type TicketCategory = 'billing' | 'api' | 'account' | 'other'

export type TicketPriority = 1 | 2 | 3 | 4

export interface Ticket {
  id: number
  user_id: number
  subject: string
  category: TicketCategory
  priority: TicketPriority
  status: TicketStatus
  user_unread: boolean
  last_reply_at: number
  last_reply_by: 'user' | 'staff' | ''
  created_at: number
  closed_at: number
  closed_by: 'user' | 'staff' | 'auto' | ''
  /** Only present in staff views. */
  username?: string
  display_name?: string
}

export interface TicketAttachment {
  id: number
  mime_type: string
  size: number
}

export interface TicketMessage {
  id: number
  is_staff: boolean
  content: string
  /** Only present in staff views; users see staff replies as the support team. */
  author_name?: string
  created_at: number
  attachments: TicketAttachment[]
}

export interface TicketDetail extends Ticket {
  messages: TicketMessage[]
}

export interface TicketPage {
  items: Ticket[]
  total: number
}

export interface ApiResponse<T = unknown> {
  success: boolean
  message?: string
  message_key?: string
  message_params?: Record<string, unknown>
  data?: T
}

/** Selects whether a ticket view talks to the owner or the staff endpoints. */
export type TicketAudience = 'user' | 'staff'
