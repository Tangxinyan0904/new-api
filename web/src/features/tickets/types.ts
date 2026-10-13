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

export type TicketCategory = 'billing' | 'api' | 'account' | 'refund' | 'other'

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

/** A copy of a call log attached to an API ticket when it was opened. */
export interface TicketLog {
  log_id: number
  created_at: number
  /** 2 for a consumed request, 5 for a failed one. */
  type: number
  model_name: string
  token_name: string
  group: string
  quota: number
  prompt_tokens: number
  completion_tokens: number
  use_time: number
  is_stream: boolean
  request_id: string
  content: string
  /** Only present in staff views. */
  channel_id?: number
}

export interface TicketLogPage {
  items: TicketLog[]
  total: number
}

export interface TicketDetail extends Ticket {
  messages: TicketMessage[]
  logs: TicketLog[]
}

export interface TicketRefundTopUp {
  payment_method: string
  /** Credited amount in USD, as billing history shows it. */
  amount: number
  /** Money paid, in the payment currency. */
  money: number
  complete_time: number
}

/** A refund ticket owner's payments and recent usage, for staff. */
export interface TicketRefundInfo {
  quota: number
  top_up_count: number
  top_up_amount: number
  top_up_money: number
  latest_top_up: TicketRefundTopUp | null
  redemption_count: number
  redemption_quota: number
  today_quota: number
  week_quota: number
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
