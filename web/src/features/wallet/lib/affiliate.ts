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
// ============================================================================
// Affiliate Functions
// ============================================================================

import type {
  AffiliateRebateSummary,
  AffiliateTransferRequestRecord,
} from '../types'

export function markAffiliateTransferSubmitted(
  summary: AffiliateRebateSummary,
  createdRequest?: AffiliateTransferRequestRecord
): AffiliateRebateSummary {
  const todayRequestCount = (summary.today_request_count ?? 0) + 1
  const dailyLimitReached =
    summary.daily_request_limit > 0 &&
    todayRequestCount >= summary.daily_request_limit
  const next: AffiliateRebateSummary = {
    ...summary,
    today_request_count: todayRequestCount,
    submitted_today: dailyLimitReached,
  }
  if (createdRequest?.status === 'pending') {
    return { ...next, pending_request: createdRequest }
  }
  if (createdRequest?.status === 'approved') {
    // Credited immediately: everything that was transferable has moved.
    return {
      ...next,
      invite_reward_quota: 0,
      recharge_rebate_quota: 0,
      total_pending_quota: 0,
    }
  }
  return next
}

interface AffiliateTransferActionStateInput {
  totalPendingQuota: number
  /** 0 means any positive amount can be transferred. */
  minimumQuota: number
  pendingRequest: boolean
  submittedToday: boolean
}

interface AffiliateTransferActionState {
  disabled: boolean
  labelKey: 'Request Transfer' | 'Submitted'
  showMinimum: boolean
}

export function getAffiliateTransferActionState(
  input: AffiliateTransferActionStateInput
): AffiliateTransferActionState {
  if (input.pendingRequest || input.submittedToday) {
    return {
      disabled: true,
      labelKey: 'Submitted',
      showMinimum: false,
    }
  }

  const belowMinimum =
    input.minimumQuota > 0 && input.totalPendingQuota < input.minimumQuota
  return {
    disabled: belowMinimum || input.totalPendingQuota <= 0,
    labelKey: 'Request Transfer',
    showMinimum: belowMinimum,
  }
}

/**
 * Generate affiliate registration link
 */
export function generateAffiliateLink(affCode: string): string {
  if (typeof window === 'undefined') return ''
  return `${window.location.origin}/sign-up?aff=${affCode}`
}
