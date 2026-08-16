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
import axios from 'axios'
import { z } from 'zod'

import { api } from '@/lib/api'

import {
  channelMonitorCheckSchema,
  channelMonitorSchema,
  type ChannelMonitor,
  type ChannelMonitorCheck,
  type ChannelMonitorHistoryPage,
  type ChannelMonitorHistoryParams,
  type ChannelMonitorListParams,
  type ChannelMonitorPage,
  type ChannelMonitorPayload,
} from './types'

const channelMonitorPageSchema = z.object({
  items: z.array(channelMonitorSchema),
  total: z.number().int().nonnegative(),
  page: z.number().int().positive(),
  page_size: z.number().int().positive(),
})

const channelMonitorHistoryPageSchema = z.object({
  items: z.array(channelMonitorCheckSchema),
  total: z.number().int().nonnegative(),
  page: z.number().int().positive(),
  page_size: z.number().int().positive(),
})

const apiEnvelopeSchema = z.object({
  success: z.boolean(),
  message: z.string().optional(),
  data: z.unknown().optional(),
})

export const channelMonitorQueryKeys = {
  all: ['channel-monitors'] as const,
  lists: () => ['channel-monitors', 'list'] as const,
  list: (params: ChannelMonitorListParams) =>
    ['channel-monitors', 'list', params] as const,
  details: () => ['channel-monitors', 'detail'] as const,
  detail: (id: number) => ['channel-monitors', 'detail', id] as const,
  histories: () => ['channel-monitors', 'history'] as const,
  history: (id: number, params: ChannelMonitorHistoryParams) =>
    ['channel-monitors', 'history', id, params] as const,
}

export class ChannelMonitorApiError extends Error {
  constructor(message: string) {
    super(message)
    this.name = 'ChannelMonitorApiError'
  }
}

export function isChannelMonitorBusyError(error: unknown): boolean {
  return axios.isAxiosError(error) && error.response?.status === 409
}

export function getChannelMonitorErrorMessage(
  error: unknown
): string | undefined {
  if (axios.isAxiosError(error)) {
    const data = error.response?.data
    if (data && typeof data === 'object' && 'message' in data) {
      const message = data.message
      if (typeof message === 'string' && message.trim()) {
        return message
      }
    }
    return error.message || undefined
  }
  if (error instanceof Error) return error.message || undefined
  return undefined
}

function parseApiData<T>(payload: unknown, schema: z.ZodType<T>): T {
  const envelope = apiEnvelopeSchema.parse(payload)
  if (!envelope.success) {
    throw new ChannelMonitorApiError(envelope.message || 'Request failed')
  }
  return schema.parse(envelope.data)
}

export async function getChannelMonitors(
  params: ChannelMonitorListParams
): Promise<ChannelMonitorPage> {
  const response = await api.get('/api/channel-monitor/', { params })
  return parseApiData(response.data, channelMonitorPageSchema)
}

export async function getChannelMonitor(id: number): Promise<ChannelMonitor> {
  const response = await api.get(`/api/channel-monitor/${id}`)
  return parseApiData(response.data, channelMonitorSchema)
}

export async function createChannelMonitor(
  payload: ChannelMonitorPayload
): Promise<ChannelMonitor> {
  const response = await api.post('/api/channel-monitor/', payload)
  return parseApiData(response.data, channelMonitorSchema)
}

export async function updateChannelMonitor(
  id: number,
  payload: ChannelMonitorPayload
): Promise<ChannelMonitor> {
  const response = await api.put(`/api/channel-monitor/${id}`, payload)
  return parseApiData(response.data, channelMonitorSchema)
}

export async function deleteChannelMonitor(id: number): Promise<void> {
  const response = await api.delete(`/api/channel-monitor/${id}`)
  parseApiData(response.data, z.null())
}

export async function runChannelMonitorNow(
  id: number
): Promise<ChannelMonitorCheck[]> {
  const response = await api.post(`/api/channel-monitor/${id}/run`, undefined, {
    skipBusinessError: true,
    skipErrorHandler: true,
  })
  return parseApiData(response.data, z.array(channelMonitorCheckSchema))
}

export async function getChannelMonitorHistory(
  id: number,
  params: ChannelMonitorHistoryParams
): Promise<ChannelMonitorHistoryPage> {
  const response = await api.get(`/api/channel-monitor/${id}/history`, {
    params,
  })
  return parseApiData(response.data, channelMonitorHistoryPageSchema)
}
