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
  getAffiliateTransferActionState,
  markAffiliateTransferSubmitted,
} from './affiliate.ts'

describe('markAffiliateTransferSubmitted', () => {
  const summary = {
    invited_users: [{ id: 7, display_name: 'Affiliate user' }],
    invited_count: 1,
    total_invited_recharge_quota: 6_000,
    invite_reward_quota: 200,
    recharge_rebate_quota: 300,
    total_pending_quota: 500,
    submitted_today: false,
  }

  test('marks the summary submitted and attaches the created request', () => {
    const createdRequest = {
      id: 19,
      user_id: 8,
      invite_reward_quota: 200,
      recharge_rebate_quota: 300,
      total_quota: 500,
      status: 'pending' as const,
      created_at: 1_234,
    }

    assert.deepEqual(markAffiliateTransferSubmitted(summary, createdRequest), {
      ...summary,
      pending_request: createdRequest,
      submitted_today: true,
    })
  })

  test('preserves an existing pending request when no response data is available', () => {
    const pendingRequest = {
      id: 18,
      user_id: 8,
      invite_reward_quota: 100,
      recharge_rebate_quota: 200,
      total_quota: 300,
      status: 'pending' as const,
      created_at: 1_000,
    }
    const summaryWithPendingRequest = {
      ...summary,
      pending_request: pendingRequest,
    }

    assert.deepEqual(
      markAffiliateTransferSubmitted(summaryWithPendingRequest),
      {
        ...summaryWithPendingRequest,
        submitted_today: true,
      }
    )
  })
})

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
