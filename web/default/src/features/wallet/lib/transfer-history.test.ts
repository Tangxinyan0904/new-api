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

import {
  getAffiliateTransferHistoryViewState,
  getAffiliateTransferStatusConfig,
} from './transfer-history'

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
