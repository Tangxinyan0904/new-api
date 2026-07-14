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
import assert from 'node:assert/strict'
import { describe, test } from 'node:test'

import { QueryClient } from '@tanstack/react-query'

import {
  getAffiliateTransferHistoryQueryKey,
  getAffiliateTransferHistoryQueryPrefix,
  getAffiliateTransferHistoryViewState,
  getAffiliateTransferStatusConfig,
} from './transfer-history'

describe('affiliate transfer history query keys', () => {
  test('isolates cached history by user and removes one user by prefix', () => {
    const queryClient = new QueryClient()
    const userAKey = getAffiliateTransferHistoryQueryKey(101, 1, 10)
    const userBKey = getAffiliateTransferHistoryQueryKey(202, 1, 10)
    const userAHistory = { items: [{ id: 1 }], total: 1 }

    queryClient.setQueryData(userAKey, userAHistory)

    assert.deepEqual(queryClient.getQueryData(userAKey), userAHistory)
    assert.equal(queryClient.getQueryData(userBKey), undefined)

    queryClient.removeQueries({
      queryKey: getAffiliateTransferHistoryQueryPrefix(101),
    })

    assert.equal(queryClient.getQueryData(userAKey), undefined)
  })

  test('keeps page number and page size as separate cache dimensions', () => {
    const queryClient = new QueryClient()
    const firstPageKey = getAffiliateTransferHistoryQueryKey(101, 1, 10)
    const secondPageKey = getAffiliateTransferHistoryQueryKey(101, 2, 10)
    const largerPageKey = getAffiliateTransferHistoryQueryKey(101, 1, 20)

    queryClient.setQueryData(firstPageKey, 'first-page')
    queryClient.setQueryData(secondPageKey, 'second-page')
    queryClient.setQueryData(largerPageKey, 'larger-page')

    assert.equal(queryClient.getQueryData(firstPageKey), 'first-page')
    assert.equal(queryClient.getQueryData(secondPageKey), 'second-page')
    assert.equal(queryClient.getQueryData(largerPageKey), 'larger-page')
  })
})

describe('getAffiliateTransferHistoryViewState', () => {
  test('keeps cached records and pagination visible after a request error', () => {
    assert.deepEqual(
      getAffiliateTransferHistoryViewState({
        isLoading: false,
        isError: true,
        recordCount: 2,
        page: 1,
      }),
      {
        display: 'records',
        showPagination: true,
        showPreviousPageAction: false,
      }
    )
  })

  test('offers previous-page recovery when an uncached later page fails', () => {
    assert.deepEqual(
      getAffiliateTransferHistoryViewState({
        isLoading: false,
        isError: true,
        recordCount: 0,
        page: 2,
      }),
      {
        display: 'fatal-error',
        showPagination: false,
        showPreviousPageAction: true,
      }
    )
  })

  test('keeps first-page fatal errors limited to retry', () => {
    assert.deepEqual(
      getAffiliateTransferHistoryViewState({
        isLoading: false,
        isError: true,
        recordCount: 0,
        page: 1,
      }),
      {
        display: 'fatal-error',
        showPagination: false,
        showPreviousPageAction: false,
      }
    )
  })
})

describe('getAffiliateTransferStatusConfig', () => {
  test('uses a neutral accessible label for unknown statuses', () => {
    assert.deepEqual(getAffiliateTransferStatusConfig('legacy-review'), {
      labelKey: 'Unknown',
      variant: 'neutral',
    })
  })

  test('preserves known status labels and variants', () => {
    assert.deepEqual(getAffiliateTransferStatusConfig('pending'), {
      labelKey: 'Pending',
      variant: 'warning',
    })
    assert.deepEqual(getAffiliateTransferStatusConfig('approved'), {
      labelKey: 'Approved',
      variant: 'success',
    })
    assert.deepEqual(getAffiliateTransferStatusConfig('rejected'), {
      labelKey: 'Rejected',
      variant: 'danger',
    })
  })
})
