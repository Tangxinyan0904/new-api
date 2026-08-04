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
  ApiResponse,
  RebateApproveAllResult,
  RebateApprovalDetail,
  RebateApprovalListResponse,
} from './types'

export class RebateApprovalDetailError extends Error {
  constructor(message: string) {
    super(message)
    this.name = 'RebateApprovalDetailError'
  }
}

export async function getRebateTransferRequests(params: {
  p?: number
  page_size?: number
  status?: string
}): Promise<ApiResponse<RebateApprovalListResponse>> {
  const search = new URLSearchParams()
  if (params.p) search.set('p', String(params.p))
  if (params.page_size) search.set('page_size', String(params.page_size))
  if (params.status) search.set('status', params.status)
  const res = await api.get(
    `/api/user/affiliate/transfer-requests?${search.toString()}`
  )
  return res.data
}

export async function approveRebateTransferRequest(
  id: number
): Promise<ApiResponse> {
  const res = await api.post(
    `/api/user/affiliate/transfer-requests/${id}/approve`,
    undefined,
    { skipBusinessError: true, skipErrorHandler: true }
  )
  return res.data
}

export async function approveAllPendingRebateTransferRequests(): Promise<
  ApiResponse<RebateApproveAllResult>
> {
  const res = await api.post(
    '/api/user/affiliate/transfer-requests/approve-all',
    undefined,
    { skipBusinessError: true, skipErrorHandler: true }
  )
  return res.data
}

export async function getRebateTransferRequestDetail(
  id: number
): Promise<ApiResponse<RebateApprovalDetail>> {
  const res = await api.get(
    `/api/user/affiliate/transfer-requests/${id}/detail`,
    { skipBusinessError: true, skipErrorHandler: true }
  )
  if (res.data?.success === false) {
    throw new RebateApprovalDetailError(
      res.data.message || 'Failed to load rebate request details.'
    )
  }
  if (!res.data?.data) {
    throw new RebateApprovalDetailError(
      'Failed to load rebate request details.'
    )
  }
  return res.data
}

export async function rejectRebateTransferRequest(
  id: number,
  reason = ''
): Promise<ApiResponse> {
  const res = await api.post(
    `/api/user/affiliate/transfer-requests/${id}/reject`,
    { reason },
    { skipBusinessError: true, skipErrorHandler: true }
  )
  return res.data
}
