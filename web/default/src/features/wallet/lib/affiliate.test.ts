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

import { getAffiliateTransferActionState } from './affiliate'

describe('getAffiliateTransferActionState', () => {
  test('disables transfers below the configured minimum', () => {
    assert.deepEqual(
      getAffiliateTransferActionState({
        totalPendingQuota: 499_999,
        minimumQuota: 500_000,
        pendingRequest: false,
        submittedToday: false,
      }),
      {
        disabled: true,
        labelKey: 'Request Transfer',
        showMinimum: true,
      }
    )
  })

  test('enables transfers at the configured minimum', () => {
    assert.deepEqual(
      getAffiliateTransferActionState({
        totalPendingQuota: 500_000,
        minimumQuota: 500_000,
        pendingRequest: false,
        submittedToday: false,
      }),
      {
        disabled: false,
        labelKey: 'Request Transfer',
        showMinimum: false,
      }
    )
  })

  test('marks a pending request as submitted', () => {
    assert.deepEqual(
      getAffiliateTransferActionState({
        totalPendingQuota: 100,
        minimumQuota: 500_000,
        pendingRequest: true,
        submittedToday: false,
      }),
      {
        disabled: true,
        labelKey: 'Submitted',
        showMinimum: false,
      }
    )
  })

  test('marks a request submitted today as submitted', () => {
    assert.deepEqual(
      getAffiliateTransferActionState({
        totalPendingQuota: 100,
        minimumQuota: 500_000,
        pendingRequest: false,
        submittedToday: true,
      }),
      {
        disabled: true,
        labelKey: 'Submitted',
        showMinimum: false,
      }
    )
  })

  test('disables transfers when the minimum is unavailable', () => {
    assert.deepEqual(
      getAffiliateTransferActionState({
        totalPendingQuota: 500_000,
        minimumQuota: 0,
        pendingRequest: false,
        submittedToday: false,
      }),
      {
        disabled: true,
        labelKey: 'Request Transfer',
        showMinimum: true,
      }
    )
  })
})
