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
import { describe, expect, mock, test } from 'bun:test'

import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { renderToStaticMarkup } from 'react-dom/server'

import enLocale from '@/i18n/locales/en.json'
import frLocale from '@/i18n/locales/fr.json'
import jaLocale from '@/i18n/locales/ja.json'
import ruLocale from '@/i18n/locales/ru.json'
import viLocale from '@/i18n/locales/vi.json'
import zhTWLocale from '@/i18n/locales/zh-TW.json'
import zhLocale from '@/i18n/locales/zh.json'

import type {
  BlockedRegistrationIP,
  PageData,
  RegistrationIPAllowlistItem,
} from '../types'

const apiCalls: Array<{
  method: string
  url: string
  body?: unknown
  config?: Record<string, unknown>
}> = []

mock.module('@/lib/api', () => ({
  api: {
    get: mock(async (url: string, config?: Record<string, unknown>) => {
      apiCalls.push({ method: 'get', url, config })
      return {
        data: {
          success: true,
          data: { page: 1, page_size: 10, total: 0, items: [] },
        },
      }
    }),
    post: mock(
      async (url: string, body?: unknown, config?: Record<string, unknown>) => {
        apiCalls.push({ method: 'post', url, body, config })
        return {
          data: {
            success: true,
            data: {
              ip: '2001:db8::1',
              affected_user_ids: [],
              affected_account_count: 0,
              allowlisted: false,
            },
          },
        }
      }
    ),
    delete: mock(async (url: string, config?: Record<string, unknown>) => {
      apiCalls.push({ method: 'delete', url, config })
      return {
        data: {
          success: true,
          data: {
            ip: '2001:db8::1',
            affected_user_ids: [],
            affected_account_count: 0,
            allowlisted: false,
          },
        },
      }
    }),
  },
}))

mock.module('react-i18next', () => ({
  useTranslation: () => ({
    t: (key: string, values?: Record<string, string | number>) =>
      key.replaceAll(/\{\{(\w+)\}\}/g, (_match, name: string) =>
        String(values?.[name] ?? '')
      ),
  }),
}))

const {
  addRegistrationIPAllowlist,
  getBlockedRegistrationIPsQueryKey,
  getRegistrationIPAllowlistQueryKey,
  removeRegistrationIPAllowlist,
  unblockRegistrationIP,
} = await import('../api')
const { RegistrationIPAbuseSection } =
  await import('../registration-ip-abuse-section')
const { isExactIPAddress } = await import('../ip-validation')

const blockedPage: PageData<BlockedRegistrationIP> = {
  page: 1,
  page_size: 10,
  total: 11,
  items: [
    {
      ip: '203.0.113.80',
      current_cycle: 2,
      registration_count: 4,
      blocked_at: 1_750_000_100,
      associated_account_count: 4,
      accounts: [
        {
          user_id: 51,
          username: 'blocked-user',
          display_name: 'Blocked Display',
          status: 2,
          user_created_at: 1_750_000_000,
          registration_at: 1_750_000_000,
          deleted: false,
          restore_eligible: true,
          auto_disabled_at: 1_750_000_100,
        },
      ],
    },
  ],
}

const allowlistPage: PageData<RegistrationIPAllowlistItem> = {
  page: 1,
  page_size: 10,
  total: 1,
  items: [
    {
      ip: '2001:db8::10',
      current_cycle: 3,
      registration_count: 0,
      created_at: 1_750_000_000,
      updated_at: 1_750_000_200,
    },
  ],
}

function renderSection(
  blocked: PageData<BlockedRegistrationIP>,
  allowlist: PageData<RegistrationIPAllowlistItem>
): string {
  const queryClient = new QueryClient()
  queryClient.setQueryData(
    getBlockedRegistrationIPsQueryKey(1, 10, ''),
    blocked
  )
  queryClient.setQueryData(getRegistrationIPAllowlistQueryKey(1, 10), allowlist)

  return renderToStaticMarkup(
    <QueryClientProvider client={queryClient}>
      <RegistrationIPAbuseSection />
    </QueryClientProvider>
  )
}

describe('registration IP helpers', () => {
  test('localizes associated account counts in every supported language', () => {
    const key = '{{count}} accounts'

    expect(enLocale.translation[key]).toBe('{{count}} accounts')
    expect(zhLocale.translation[key]).toBe('{{count}} 个账号')
    expect(zhTWLocale.translation[key]).toBe('{{count}} 個帳號')
    expect(frLocale.translation[key]).toBe('{{count}} comptes')
    expect(jaLocale.translation[key]).toBe('{{count}} 個のアカウント')
    expect(ruLocale.translation[key]).toBe('Аккаунтов: {{count}}')
    expect(viLocale.translation[key]).toBe('{{count}} tài khoản')
  })

  test('accepts exact IPv4 and IPv6 while rejecting ranges and host values', () => {
    expect(isExactIPAddress('203.0.113.8')).toBe(true)
    expect(isExactIPAddress('2001:db8::8')).toBe(true)
    expect(isExactIPAddress('fe80::1%eth0')).toBe(true)
    expect(isExactIPAddress('203.0.113.0/24')).toBe(false)
    expect(isExactIPAddress('203.0.113.8:443')).toBe(false)
    expect(isExactIPAddress('example.com')).toBe(false)
    expect(isExactIPAddress('')).toBe(false)
  })

  test('builds independent blocked and allowlist query keys', () => {
    expect(getBlockedRegistrationIPsQueryKey(2, 10, 'alice')).toEqual([
      'registration-ip-abuse',
      'blocked',
      2,
      10,
      'alice',
    ])
    expect(getRegistrationIPAllowlistQueryKey(3, 10)).toEqual([
      'registration-ip-abuse',
      'allowlist',
      3,
      10,
    ])
  })
})

describe('registration IP API', () => {
  test('encodes IPv6 path parameters and disables global business errors', async () => {
    apiCalls.length = 0

    await unblockRegistrationIP('2001:db8::1')
    await addRegistrationIPAllowlist('2001:db8::1')
    await removeRegistrationIPAllowlist('2001:db8::1')

    expect(apiCalls).toEqual([
      {
        method: 'post',
        url: '/api/registration-ip-abuse/2001%3Adb8%3A%3A1/unblock',
        body: undefined,
        config: { skipBusinessError: true },
      },
      {
        method: 'post',
        url: '/api/registration-ip-abuse/allowlist',
        body: { ip: '2001:db8::1' },
        config: { skipBusinessError: true },
      },
      {
        method: 'delete',
        url: '/api/registration-ip-abuse/allowlist/2001%3Adb8%3A%3A1',
        config: { skipBusinessError: true },
      },
    ])
  })
})

describe('RegistrationIPAbuseSection', () => {
  test('renders grouped blocked accounts, controls, pagination, and responsive layouts', () => {
    const html = renderSection(blockedPage, allowlistPage)

    expect(html).toContain('Registration Abuse Protection')
    expect(html).toContain('Search by IP, user ID, username, or display name')
    expect(html).toContain('203.0.113.80')
    expect(html).toContain('4 accounts')
    expect(html).toContain('Show account details')
    expect(html).toContain('Unblock IP')
    expect(html).toContain('2001:db8::10')
    expect(html).toContain('Add to allowlist')
    expect(html).toContain('Remove')
    expect(html).toContain('Page 1 of 2')
    expect(html).toContain('md:hidden')
    expect(html).toContain('hidden md:block')
  })

  test('renders stable empty states for both lists', () => {
    const emptyPage = { page: 1, page_size: 10, total: 0, items: [] }
    const html = renderSection(emptyPage, emptyPage)

    expect(html).toContain('No blocked registration IPs')
    expect(html).toContain('No allowlisted registration IPs')
  })
})
