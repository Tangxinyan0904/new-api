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
import { expect, test } from 'vitest'

import { formatLogQuota } from '@/lib/format'

import { getTokensPerSecond, renderAuditContent } from '../format'

test('calculates generated tokens per second for a completed request', () => {
  expect(getTokensPerSecond(120, 2)).toBe(60)
})

test('omits throughput when output tokens or request duration are invalid', () => {
  expect(getTokensPerSecond(0, 2)).toBeNull()
  expect(getTokensPerSecond(120, 0)).toBeNull()
  expect(getTokensPerSecond(Number.POSITIVE_INFINITY, 2)).toBeNull()
  expect(getTokensPerSecond(120, Number.NaN)).toBeNull()
})

test('renders the distillation penalty clear audit action', () => {
  const content = renderAuditContent(
    {
      op: {
        action: 'rate_limit.distillation_clear',
        params: { target_user_id: 51 },
      },
    },
    (key, params) =>
      key.replace('{{target_user_id}}', String(params?.target_user_id ?? ''))
  )

  expect(content).toBe('Cleared distillation penalty for user 51')
})

test('renders the user-facing affiliate approval with formatted balance', () => {
  const content = renderAuditContent(
    {
      op: {
        action: 'affiliate.transfer.approved_for_user',
        params: { request_id: 7, quota: 500 },
      },
    },
    (key, params) => key.replace('{{amount}}', String(params?.amount ?? ''))
  )

  expect(content).toBe(`Administrator approved ${formatLogQuota(500)} balance`)
})

test('renders an automatically credited rebate transfer with formatted balance', () => {
  const content = renderAuditContent(
    {
      op: {
        action: 'affiliate.transfer.auto_approved',
        params: { request_id: 8, quota: 750 },
      },
    },
    (key, params) => key.replace('{{amount}}', String(params?.amount ?? ''))
  )

  expect(content).toBe(
    `Rebate transfer of ${formatLogQuota(750)} credited automatically`
  )
})
