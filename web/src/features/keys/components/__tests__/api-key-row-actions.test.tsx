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

import type { Row } from '@tanstack/react-table'
import { afterAll, test } from 'vitest'

import type { ApiKey } from '../../types'

const { act } = await import('react')
const { createRoot } = await import('react-dom/client')
const { QueryClient, QueryClientProvider } =
  await import('@tanstack/react-query')
const { createInstance } = await import('i18next')
const { I18nextProvider, initReactI18next } = await import('react-i18next')
const { ApiKeysProvider } = await import('../api-keys-provider')
const { DataTableRowActions } = await import('../data-table-row-actions')

const i18n = createInstance()
await i18n.use(initReactI18next).init({
  lng: 'en',
  resources: { en: { translation: {} } },
})

const reactTestGlobals = globalThis as typeof globalThis & {
  IS_REACT_ACT_ENVIRONMENT?: boolean
}
reactTestGlobals.IS_REACT_ACT_ENVIRONMENT = true

const apiKey: ApiKey = {
  id: 23,
  name: 'cc-switch-key',
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
  model_limits_enabled: false,
  model_limits: '',
  allow_ips: '',
}

afterAll(() => {})

test('uses the dedicated API key action treatment for CC Switch import', async () => {
  const host = document.createElement('div')
  document.body.append(host)
  const root = createRoot(host)
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  queryClient.setQueryData(['status'], { chats: [] })

  await act(async () => {
    root.render(
      <QueryClientProvider client={queryClient}>
        <I18nextProvider i18n={i18n}>
          <ApiKeysProvider>
            <DataTableRowActions row={{ original: apiKey } as Row<ApiKey>} />
          </ApiKeysProvider>
        </I18nextProvider>
      </QueryClientProvider>
    )
  })

  const importButton = [...host.querySelectorAll('button')].find((button) =>
    button.textContent?.includes('One-click import to CC Switch')
  )
  assert.ok(importButton)
  assert.equal(importButton.classList.contains('border-api-key-action'), true)
  assert.equal(
    importButton.classList.contains('bg-api-key-action-surface'),
    true
  )
  assert.equal(importButton.classList.contains('text-api-key-action'), true)

  await act(async () => root.unmount())
  queryClient.clear()
  host.remove()
})
