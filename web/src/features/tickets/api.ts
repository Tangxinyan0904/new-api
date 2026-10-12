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
import { api } from '@/lib/api'
import { requireServerSuccess } from '@/lib/server-error-message'

import type {
  ApiResponse,
  Ticket,
  TicketAudience,
  TicketDetail,
  TicketPage,
  TicketPriority,
  TicketStatus,
} from './types'

const mutationConfig = { skipBusinessError: true, skipErrorHandler: true }

export interface TicketMessageInput {
  content: string
  images: File[]
}

function buildMessageForm(
  input: TicketMessageInput,
  extra: Record<string, string> = {}
): FormData {
  const form = new FormData()
  form.append('content', input.content)
  for (const [key, value] of Object.entries(extra)) form.append(key, value)
  for (const image of input.images) form.append('images', image)
  return form
}

function searchString(params: Record<string, string | number | undefined>) {
  const search = new URLSearchParams()
  for (const [key, value] of Object.entries(params)) {
    if (value !== undefined && value !== '') search.set(key, String(value))
  }
  return search.toString()
}

export async function getUserTickets(params: {
  p: number
  page_size: number
  status?: string
}): Promise<TicketPage> {
  const res = await api.get(`/api/ticket/self?${searchString(params)}`)
  const body = requireServerSuccess(res.data as ApiResponse<TicketPage>)
  return { items: body.data?.items ?? [], total: body.data?.total ?? 0 }
}

export async function getUserTicket(id: number): Promise<TicketDetail> {
  const res = await api.get(`/api/ticket/${id}`)
  return requireServerSuccess(res.data as ApiResponse<TicketDetail>)
    .data as TicketDetail
}

export async function getUserUnreadTicketCount(): Promise<number> {
  const res = await api.get('/api/ticket/self/unread', {
    skipErrorHandler: true,
  })
  const body = requireServerSuccess(res.data as ApiResponse<{ unread: number }>)
  return body.data?.unread ?? 0
}

export async function createTicket(
  input: TicketMessageInput & { subject: string; category: string }
): Promise<ApiResponse<Ticket>> {
  const form = buildMessageForm(input, {
    subject: input.subject,
    category: input.category,
  })
  const res = await api.post('/api/ticket/', form, mutationConfig)
  return res.data
}

export async function replyUserTicket(
  id: number,
  input: TicketMessageInput
): Promise<ApiResponse<Ticket>> {
  const res = await api.post(
    `/api/ticket/${id}/reply`,
    buildMessageForm(input),
    mutationConfig
  )
  return res.data
}

export async function closeUserTicket(id: number): Promise<ApiResponse> {
  const res = await api.post(
    `/api/ticket/${id}/close`,
    undefined,
    mutationConfig
  )
  return res.data
}

export async function getStaffTickets(params: {
  p: number
  page_size: number
  status?: string
  category?: string
  priority?: number
  keyword?: string
  sort?: string
}): Promise<TicketPage> {
  const res = await api.get(`/api/ticket/admin?${searchString(params)}`)
  const body = requireServerSuccess(res.data as ApiResponse<TicketPage>)
  return { items: body.data?.items ?? [], total: body.data?.total ?? 0 }
}

export async function getStaffTicket(id: number): Promise<TicketDetail> {
  const res = await api.get(`/api/ticket/admin/${id}`)
  return requireServerSuccess(res.data as ApiResponse<TicketDetail>)
    .data as TicketDetail
}

export async function getStaffTicketSummary(): Promise<number> {
  const res = await api.get('/api/ticket/admin/summary', {
    skipErrorHandler: true,
  })
  const body = requireServerSuccess(
    res.data as ApiResponse<{ awaiting: number }>
  )
  return body.data?.awaiting ?? 0
}

export async function replyStaffTicket(
  id: number,
  input: TicketMessageInput & { status: TicketStatus }
): Promise<ApiResponse<Ticket>> {
  const res = await api.post(
    `/api/ticket/admin/${id}/reply`,
    buildMessageForm(input, { status: input.status }),
    mutationConfig
  )
  return res.data
}

export async function updateStaffTicket(
  id: number,
  changes: { status?: TicketStatus; priority?: TicketPriority }
): Promise<ApiResponse<Ticket>> {
  const res = await api.put(`/api/ticket/admin/${id}`, changes, mutationConfig)
  return res.data
}

export async function fetchTicketAttachment(
  audience: TicketAudience,
  id: number
): Promise<Blob> {
  const path =
    audience === 'staff'
      ? `/api/ticket/admin/attachment/${id}`
      : `/api/ticket/attachment/${id}`
  const res = await api.get(path, {
    responseType: 'blob',
    skipErrorHandler: true,
  })
  return res.data as Blob
}
