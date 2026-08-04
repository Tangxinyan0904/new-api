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
import { after, test } from 'node:test'

import { Window } from 'happy-dom'

import type { ApiKey } from '../../types'

const domWindow = new Window()
const domGlobals = [
  'window',
  'document',
  'navigator',
  'HTMLElement',
  'HTMLButtonElement',
  'HTMLInputElement',
  'SVGElement',
  'Node',
  'Element',
  'Event',
  'KeyboardEvent',
  'PointerEvent',
  'MouseEvent',
  'FocusEvent',
  'CustomEvent',
  'MutationObserver',
  'ResizeObserver',
  'requestAnimationFrame',
  'cancelAnimationFrame',
  'getComputedStyle',
] as const

for (const key of domGlobals) {
  Object.defineProperty(globalThis, key, {
    configurable: true,
    value: domWindow[key],
  })
}

const { act } = await import('react')
const { createRoot } = await import('react-dom/client')
const { createInstance } = await import('i18next')
const { I18nextProvider, initReactI18next } = await import('react-i18next')
const { QueryClient, QueryClientProvider } =
  await import('@tanstack/react-query')
const { api } = await import('@/lib/api')
const { ApiKeysProvider } = await import('../api-keys-provider')
const { EditableApiKeyGroupCell } = await import('../api-keys-cells')

const i18n = createInstance()
await i18n.use(initReactI18next).init({
  lng: 'en',
  resources: { en: { translation: {} } },
})

const reactTestGlobals = globalThis as typeof globalThis & {
  IS_REACT_ACT_ENVIRONMENT?: boolean
}
reactTestGlobals.IS_REACT_ACT_ENVIRONMENT = true

type PutCall = { url: string; data: unknown }
type PutMethod = (
  url: string,
  data?: unknown
) => Promise<{ data: { success: boolean; message: string } }>
const apiClient = api as unknown as { put: PutMethod }
const originalPut = apiClient.put

const apiKey: ApiKey = {
  id: 17,
  name: 'stale-list-key',
  key: 'masked-key',
  status: 1,
  remain_quota: 100,
  used_quota: 0,
  today_quota: 0,
  unlimited_quota: false,
  expired_time: -1,
  created_time: 1,
  accessed_time: 1,
  group: 'default',
  auto_groups: ['default'],
  cross_group_retry: true,
  model_limits_enabled: true,
  model_limits: 'gpt-test',
  allow_ips: '127.0.0.1',
}

after(() => {
  apiClient.put = originalPut
  domWindow.close()
})

test('inline group switching sends only group-selection fields', async () => {
  const calls: PutCall[] = []
  apiClient.put = async (url, data) => {
    calls.push({ url, data })
    return { data: { success: true, message: '' } }
  }
  const host = document.createElement('div')
  document.body.append(host)
  const root = createRoot(host)
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  })

  await act(async () => {
    root.render(
      <QueryClientProvider client={queryClient}>
        <I18nextProvider i18n={i18n}>
          <ApiKeysProvider>
            <EditableApiKeyGroupCell
              apiKey={apiKey}
              groupOptions={[
                { value: 'default', label: 'default', desc: 'Default group' },
                { value: 'vip', label: 'vip', desc: 'Priority group' },
              ]}
            />
          </ApiKeysProvider>
        </I18nextProvider>
      </QueryClientProvider>
    )
  })

  const trigger = host.querySelector<HTMLButtonElement>(
    'button[role="combobox"]'
  )
  assert.ok(trigger)
  await act(async () => trigger.click())
  const vipOption = [
    ...document.querySelectorAll<HTMLElement>('[data-slot="command-item"]'),
  ].find((candidate) => candidate.textContent?.includes('Priority group'))
  assert.ok(vipOption)
  await act(async () => {
    vipOption.click()
    await Promise.resolve()
  })

  assert.deepEqual(calls, [
    {
      url: '/api/token/17/group',
      data: {
        group: 'vip',
        auto_groups: [],
        cross_group_retry: false,
      },
    },
  ])

  await act(async () => root.unmount())
  queryClient.clear()
  host.remove()
})
