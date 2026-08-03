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

interface AffiliateTransferHistoryViewStateInput {
  isLoading: boolean
  isError: boolean
  recordCount: number
  page: number
}

interface AffiliateTransferHistoryViewState {
  display: 'loading' | 'fatal-error' | 'empty' | 'records'
  showPagination: boolean
  showPreviousPageAction: boolean
}

interface AffiliateTransferStatusConfig {
  labelKey: 'Approved' | 'Rejected' | 'Pending' | 'Unknown'
  variant: 'success' | 'danger' | 'warning' | 'neutral'
}

const AFFILIATE_TRANSFER_HISTORY_QUERY_KEY = 'affiliate-transfer-history'

const TRANSFER_STATUS_CONFIG: Partial<
  Record<string, AffiliateTransferStatusConfig>
> = {
  approved: { labelKey: 'Approved', variant: 'success' },
  rejected: { labelKey: 'Rejected', variant: 'danger' },
  pending: { labelKey: 'Pending', variant: 'warning' },
}

const UNKNOWN_TRANSFER_STATUS_CONFIG: AffiliateTransferStatusConfig = {
  labelKey: 'Unknown',
  variant: 'neutral',
}

export function getAffiliateTransferHistoryQueryPrefix(userId: number) {
  return [AFFILIATE_TRANSFER_HISTORY_QUERY_KEY, userId] as const
}

export function getAffiliateTransferHistoryQueryKey(
  userId: number | undefined,
  page: number,
  pageSize: number
) {
  return [AFFILIATE_TRANSFER_HISTORY_QUERY_KEY, userId, page, pageSize] as const
}

export function getAffiliateTransferHistoryViewState(
  input: AffiliateTransferHistoryViewStateInput
): AffiliateTransferHistoryViewState {
  if (input.recordCount > 0) {
    return {
      display: 'records',
      showPagination: true,
      showPreviousPageAction: false,
    }
  }

  if (input.isLoading) {
    return {
      display: 'loading',
      showPagination: false,
      showPreviousPageAction: false,
    }
  }

  if (input.isError) {
    return {
      display: 'fatal-error',
      showPagination: false,
      showPreviousPageAction: input.page > 1,
    }
  }

  return {
    display: 'empty',
    showPagination: false,
    showPreviousPageAction: false,
  }
}

export function getAffiliateTransferStatusConfig(
  status: string
): AffiliateTransferStatusConfig {
  return TRANSFER_STATUS_CONFIG[status] ?? UNKNOWN_TRANSFER_STATUS_CONFIG
}
