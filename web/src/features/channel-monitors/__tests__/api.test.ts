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
import { afterAll, beforeEach, expect, mock, test } from 'bun:test'

import { api } from '@/lib/api'

const get = mock(async () => ({ data: {} }))
const originalGet = api.get

const {
  channelMonitorQueryKeys,
  getChannelMonitorErrorMessage,
  getChannelMonitors,
  isChannelMonitorBusyError,
} = await import('@/features/channel-monitors/api')

const MONITOR = {
  id: 17,
  internal_name: 'Primary OpenAI monitor',
  public_name: 'OpenAI availability',
  name_color: '#123ABC',
  provider: 'openai',
  api_mode: 'chat_completions',
  endpoint: 'https://api.example.com',
  credential_configured: true,
  credential_masked: '****cret',
  primary_model: 'gpt-primary',
  extra_models: [],
  interval_seconds: 60,
  degraded_threshold_ms: 6000,
  timeout_seconds: 30,
  enabled: true,
  public_enabled: true,
  last_checked_at: 1_723_456_789,
  next_check_at: 1_723_456_849,
  schedule_blocked_reason: '',
  latest_status: 'operational',
  latest_latency_ms: 321,
  latest_error_category: '',
  lease_owner: '',
  lease_until: 0,
  created_by: 1,
  created_at: 1_723_450_000,
  updated_at: 1_723_456_789,
}

beforeEach(() => {
  get.mockReset()
  api.get = get as unknown as typeof api.get
})

afterAll(() => {
  api.get = originalGet
})

test('validates and returns the administrator monitor list response', async () => {
  get.mockResolvedValueOnce({
    data: {
      success: true,
      message: '',
      data: {
        items: [MONITOR],
        total: 1,
        page: 1,
        page_size: 20,
      },
    },
  })

  const result = await getChannelMonitors({ p: 1, page_size: 20 })

  expect(result.items).toHaveLength(1)
  expect(result.items[0]?.name_color).toBe('#123ABC')
  expect(result.items[0]?.latest_latency_ms).toBe(321)
  expect(result.total).toBe(1)
})

test('rejects a malformed administrator monitor response', async () => {
  get.mockResolvedValueOnce({
    data: {
      success: true,
      message: '',
      data: {
        items: [{ ...MONITOR, latest_latency_ms: '321' }],
        total: 1,
        page: 1,
        page_size: 20,
      },
    },
  })

  await expect(getChannelMonitors({ p: 1, page_size: 20 })).rejects.toThrow()
})

test('roots every channel monitor query key under one namespace', () => {
  expect(channelMonitorQueryKeys.list({ p: 1, page_size: 20 })[0]).toBe(
    'channel-monitors'
  )
  expect(channelMonitorQueryKeys.detail(17)[0]).toBe('channel-monitors')
  expect(channelMonitorQueryKeys.history(17, { p: 1, page_size: 20 })[0]).toBe(
    'channel-monitors'
  )
})

test('recognizes only an HTTP 409 response as a busy monitor', () => {
  expect(
    isChannelMonitorBusyError({
      isAxiosError: true,
      response: { status: 409 },
    })
  ).toBe(true)
  expect(
    isChannelMonitorBusyError({
      isAxiosError: true,
      response: { status: 500 },
    })
  ).toBe(false)
  expect(isChannelMonitorBusyError(new Error('network error'))).toBe(false)
})

test('reads the backend message for non-busy monitor failures', () => {
  expect(
    getChannelMonitorErrorMessage({
      isAxiosError: true,
      message: 'Request failed with status code 500',
      response: {
        status: 500,
        data: { message: 'upstream probe failed' },
      },
    })
  ).toBe('upstream probe failed')
  expect(getChannelMonitorErrorMessage(new Error('network unavailable'))).toBe(
    'network unavailable'
  )
})
