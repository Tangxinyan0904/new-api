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

import type {
  DistillationPenalty,
  PageData,
  RateLimitApiResponse,
  RateLimitFormValues,
  RateLimitUserSummary,
} from './types'

function requireSuccess<T>(
  response: RateLimitApiResponse<T>,
  fallbackMessage: string
): RateLimitApiResponse<T> {
  if (!response.success) {
    throw new Error(response.message || fallbackMessage)
  }
  return response
}

export async function saveRateLimitSettings(
  values: RateLimitFormValues
): Promise<void> {
  const response = await api.put<RateLimitApiResponse>(
    '/api/rate-limit',
    values,
    { skipBusinessError: true }
  )
  requireSuccess(response.data, 'Failed to save rate limits')
}

export async function searchRateLimitUsers(
  keyword: string
): Promise<RateLimitUserSummary[]> {
  const response = await api.get<
    RateLimitApiResponse<{
      items: Array<{
        id: number
        username: string
        display_name?: string
      }>
    }>
  >('/api/user/search', {
    params: { keyword, p: 1, page_size: 20 },
    skipBusinessError: true,
  })
  const result = requireSuccess(response.data, 'Failed to search users')
  return (result.data?.items ?? []).map((user) => ({
    id: user.id,
    username: user.username,
    displayName: user.display_name,
  }))
}

export async function getDistillationPenalties(params: {
  page: number
  pageSize: number
  keyword: string
}): Promise<PageData<DistillationPenalty>> {
  const response = await api.get<
    RateLimitApiResponse<PageData<DistillationPenalty>>
  >('/api/rate-limit/distillation/penalties', {
    params: {
      p: params.page,
      page_size: params.pageSize,
      keyword: params.keyword,
    },
    skipBusinessError: true,
  })
  const result = requireSuccess(
    response.data,
    'Failed to load distillation penalties'
  )
  return (
    result.data ?? {
      page: params.page,
      page_size: params.pageSize,
      total: 0,
      items: [],
    }
  )
}

export async function clearDistillationPenalty(userId: number): Promise<void> {
  const response = await api.delete<RateLimitApiResponse>(
    `/api/rate-limit/distillation/penalties/${userId}`,
    { skipBusinessError: true }
  )
  requireSuccess(response.data, 'Failed to clear distillation penalty')
}
