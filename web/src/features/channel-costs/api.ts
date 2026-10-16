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

import type { ChannelCostRangeStarts } from './lib/channel-cost-math'
import type {
  ApiResponse,
  ChannelCostStats,
  ChannelUpstreamBillingInput,
  ChannelUpstreamReport,
} from './types'

const mutationConfig = { skipBusinessError: true, skipErrorHandler: true }

export const channelCostQueryKeys = {
  all: ['channel-costs'] as const,
  stats: (starts: ChannelCostRangeStarts) =>
    ['channel-costs', 'stats', starts] as const,
  upstream: (starts: ChannelCostRangeStarts, timezone: string) =>
    ['channel-costs', 'upstream', starts, timezone] as const,
}

export async function getChannelCostStats(
  starts: ChannelCostRangeStarts
): Promise<ChannelCostStats> {
  const res = await api.get('/api/channel-cost/stats', { params: starts })
  return requireServerSuccess(res.data as ApiResponse<ChannelCostStats>)
    .data as ChannelCostStats
}

/** A null ratio clears the channel's cost ratio. */
export async function updateChannelCostRatio(
  channelId: number,
  ratio: number | null
): Promise<ApiResponse> {
  const res = await api.put(
    `/api/channel-cost/channels/${channelId}/ratio`,
    { ratio },
    mutationConfig
  )
  return res.data
}

/**
 * What each channel with upstream billing was charged by its upstream, and
 * how long each upstream account's balance lasts. The server caches answers
 * for five minutes; refresh asks the upstreams again.
 */
export async function getChannelUpstreamCosts(
  starts: ChannelCostRangeStarts,
  timezone: string,
  refresh = false
): Promise<ChannelUpstreamReport> {
  const res = await api.get('/api/channel-cost/upstream', {
    params: { ...starts, timezone, refresh: refresh || undefined },
  })
  const body = requireServerSuccess(
    res.data as ApiResponse<ChannelUpstreamReport>
  )
  return {
    channels: body.data?.channels ?? [],
    accounts: body.data?.accounts ?? [],
  }
}

/** The server checks the settings against the upstream before saving. */
export async function updateChannelUpstreamBilling(
  channelId: number,
  input: ChannelUpstreamBillingInput
): Promise<ApiResponse> {
  const res = await api.put(
    `/api/channel-cost/channels/${channelId}/upstream`,
    input,
    mutationConfig
  )
  return res.data
}
