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
  BlockedRegistrationIP,
  PageData,
  RegistrationIPAllowlistItem,
  RegistrationIPApiResponse,
  RegistrationIPMutationResult,
} from './types'

const EMPTY_PAGE = {
  page: 1,
  page_size: 10,
  total: 0,
  items: [],
} as const

function requireSuccess<T>(
  response: RegistrationIPApiResponse<T>,
  fallbackMessage: string
): T {
  if (!response.success || response.data === undefined) {
    throw new Error(response.message || fallbackMessage)
  }
  return response.data
}

export function getBlockedRegistrationIPsQueryKey(
  page: number,
  pageSize: number,
  keyword: string
) {
  return ['registration-ip-abuse', 'blocked', page, pageSize, keyword] as const
}

export function getRegistrationIPAllowlistQueryKey(
  page: number,
  pageSize: number
) {
  return ['registration-ip-abuse', 'allowlist', page, pageSize] as const
}

export async function getBlockedRegistrationIPs(params: {
  page: number
  pageSize: number
  keyword: string
}): Promise<PageData<BlockedRegistrationIP>> {
  const response = await api.get<
    RegistrationIPApiResponse<PageData<BlockedRegistrationIP>>
  >('/api/registration-ip-abuse/blocked', {
    params: {
      p: params.page,
      page_size: params.pageSize,
      keyword: params.keyword,
    },
    skipBusinessError: true,
  })
  if (response.data.success && response.data.data === undefined) {
    return {
      ...EMPTY_PAGE,
      page: params.page,
      page_size: params.pageSize,
      items: [],
    }
  }
  return requireSuccess(
    response.data,
    'Failed to load blocked registration IPs'
  )
}

export async function getRegistrationIPAllowlist(params: {
  page: number
  pageSize: number
}): Promise<PageData<RegistrationIPAllowlistItem>> {
  const response = await api.get<
    RegistrationIPApiResponse<PageData<RegistrationIPAllowlistItem>>
  >('/api/registration-ip-abuse/allowlist', {
    params: { p: params.page, page_size: params.pageSize },
    skipBusinessError: true,
  })
  if (response.data.success && response.data.data === undefined) {
    return {
      ...EMPTY_PAGE,
      page: params.page,
      page_size: params.pageSize,
      items: [],
    }
  }
  return requireSuccess(
    response.data,
    'Failed to load registration IP allowlist'
  )
}

export async function unblockRegistrationIP(
  ip: string
): Promise<RegistrationIPMutationResult> {
  const response = await api.post<
    RegistrationIPApiResponse<RegistrationIPMutationResult>
  >(`/api/registration-ip-abuse/${encodeURIComponent(ip)}/unblock`, undefined, {
    skipBusinessError: true,
  })
  return requireSuccess(response.data, 'Failed to unblock registration IP')
}

export async function addRegistrationIPAllowlist(
  ip: string
): Promise<RegistrationIPMutationResult> {
  const response = await api.post<
    RegistrationIPApiResponse<RegistrationIPMutationResult>
  >('/api/registration-ip-abuse/allowlist', { ip }, { skipBusinessError: true })
  return requireSuccess(
    response.data,
    'Failed to add registration IP allowlist'
  )
}

export async function removeRegistrationIPAllowlist(
  ip: string
): Promise<RegistrationIPMutationResult> {
  const response = await api.delete<
    RegistrationIPApiResponse<RegistrationIPMutationResult>
  >(`/api/registration-ip-abuse/allowlist/${encodeURIComponent(ip)}`, {
    skipBusinessError: true,
  })
  return requireSuccess(
    response.data,
    'Failed to remove registration IP allowlist'
  )
}
