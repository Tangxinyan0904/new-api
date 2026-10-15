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
import { z } from 'zod'

import { api } from '@/lib/api'

import {
  channelStatusDetailSchema,
  channelStatusResponseSchema,
  publicChannelStatusSummarySchema,
  type ChannelStatusDetail,
  type ChannelStatusMonitorInput,
  type ChannelStatusResponse,
  type PublicChannelStatusSummary,
} from './types'

const apiEnvelopeSchema = z.object({
  success: z.boolean(),
  message: z.string().optional(),
  data: z.unknown().optional(),
})

export interface ChannelStatusMutationResponse {
  success: boolean
  message?: string
  message_key?: string
  message_params?: Record<string, unknown>
}

const mutationConfig = { skipBusinessError: true, skipErrorHandler: true }

export const channelStatusQueryKeys = {
  all: ['channel-status'] as const,
  status: ['channel-status', 'status'] as const,
  detail: (id: number) => ['channel-status', 'detail', id] as const,
  publicSummary: ['channel-status', 'public-summary'] as const,
}

function parseApiData<T>(payload: unknown, schema: z.ZodType<T>): T {
  const envelope = apiEnvelopeSchema.parse(payload)
  if (!envelope.success) {
    throw new Error(envelope.message || 'Request failed')
  }
  return schema.parse(envelope.data)
}

export async function getChannelStatus(): Promise<ChannelStatusResponse> {
  const response = await api.get('/api/channel-monitor/status')
  return parseApiData(response.data, channelStatusResponseSchema)
}

export async function getChannelStatusDetail(
  id: number
): Promise<ChannelStatusDetail> {
  const response = await api.get(`/api/channel-monitor/status/${id}`)
  return parseApiData(response.data, channelStatusDetailSchema)
}

export async function getPublicChannelStatus(): Promise<PublicChannelStatusSummary> {
  const response = await api.get('/api/channel-monitor/public-summary')
  return parseApiData(response.data, publicChannelStatusSummarySchema)
}

/** Creates a monitor, or updates it when id is given. */
export async function saveChannelStatusMonitor(
  input: ChannelStatusMonitorInput,
  id?: number
): Promise<ChannelStatusMutationResponse> {
  const response = id
    ? await api.put(`/api/channel-monitor/${id}`, input, mutationConfig)
    : await api.post('/api/channel-monitor/', input, mutationConfig)
  return response.data
}

export async function deleteChannelStatusMonitor(
  id: number
): Promise<ChannelStatusMutationResponse> {
  const response = await api.delete(
    `/api/channel-monitor/${id}`,
    mutationConfig
  )
  return response.data
}
