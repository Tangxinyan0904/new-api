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
  type ChannelStatusDays,
  type ChannelStatusDetail,
  type ChannelStatusResponse,
} from './types'

const apiEnvelopeSchema = z.object({
  success: z.boolean(),
  message: z.string().optional(),
  data: z.unknown().optional(),
})

export const channelStatusQueryKeys = {
  all: ['channel-monitors'] as const,
  status: (days: ChannelStatusDays) =>
    ['channel-monitors', 'status', days] as const,
  detail: (id: number) => ['channel-monitors', 'status-detail', id] as const,
}

function parseApiData<T>(payload: unknown, schema: z.ZodType<T>): T {
  const envelope = apiEnvelopeSchema.parse(payload)
  if (!envelope.success) {
    throw new Error(envelope.message || 'Request failed')
  }
  return schema.parse(envelope.data)
}

export async function getChannelStatus(
  days: ChannelStatusDays
): Promise<ChannelStatusResponse> {
  const response = await api.get('/api/channel-monitor/status', {
    params: { days },
  })
  return parseApiData(response.data, channelStatusResponseSchema)
}

export async function getChannelStatusDetail(
  id: number
): Promise<ChannelStatusDetail> {
  const response = await api.get(`/api/channel-monitor/status/${id}`)
  return parseApiData(response.data, channelStatusDetailSchema)
}
