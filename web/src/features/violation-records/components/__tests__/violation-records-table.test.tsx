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
import { afterAll, afterEach, beforeEach, mock, test } from 'bun:test'
import assert from 'node:assert/strict'

import { Window } from 'happy-dom'

import type { ViolationRecordsApiResponse } from '../../types'

const domWindow = new Window({ url: 'http://localhost/violations' })
const domGlobals = [
  'window',
  'document',
  'navigator',
  'localStorage',
  'HTMLElement',
  'HTMLButtonElement',
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

let apiResponse: ViolationRecordsApiResponse
const apiCalls: string[] = []
const routeSearch = { page: 1, pageSize: 20 }

mock.module('@/lib/api', () => ({
  api: {
    get: async (url: string) => {
      apiCalls.push(url)
      return { data: apiResponse }
    },
  },
  getNotice: async () => ({ success: true, data: '' }),
  getStatus: async () => ({}),
}))

mock.module('@tanstack/react-router', () => ({
  getRouteApi: () => ({
    useSearch: () => routeSearch,
    useNavigate: () => () => undefined,
  }),
}))

const { act } = await import('react')
const { createRoot } = await import('react-dom/client')
const { createInstance } = await import('i18next')
const { I18nextProvider, initReactI18next } = await import('react-i18next')
const { QueryClient, QueryClientProvider } =
  await import('@tanstack/react-query')
const { PageFooterProvider } =
  await import('@/components/layout/components/page-footer')
const { ViolationRecordsTable } = await import('../violation-records-table')

const reactTestGlobals = globalThis as typeof globalThis & {
  IS_REACT_ACT_ENVIRONMENT?: boolean
}
reactTestGlobals.IS_REACT_ACT_ENVIRONMENT = true

const i18n = createInstance()
await i18n.use(initReactI18next).init({
  lng: 'en',
  resources: { en: { translation: {} } },
})

let host: HTMLDivElement | null = null
let footer: HTMLDivElement | null = null
let root: ReturnType<typeof createRoot> | null = null
let queryClient: InstanceType<typeof QueryClient> | null = null

async function renderTable() {
  host = document.createElement('div')
  footer = document.createElement('div')
  document.body.append(host, footer)
  root = createRoot(host)
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  queryClient = client

  await act(async () => {
    root?.render(
      <QueryClientProvider client={client}>
        <I18nextProvider i18n={i18n}>
          <PageFooterProvider container={footer}>
            <ViolationRecordsTable />
          </PageFooterProvider>
        </I18nextProvider>
      </QueryClientProvider>
    )
  })
}

async function waitForCondition(assertion: () => void) {
  let lastError: unknown
  for (let attempt = 0; attempt < 30; attempt += 1) {
    try {
      assertion()
      return
    } catch (error) {
      lastError = error
    }
    await act(
      async () =>
        new Promise<void>((resolve) => domWindow.setTimeout(resolve, 0))
    )
  }
  throw lastError
}

beforeEach(() => {
  apiCalls.length = 0
  localStorage.clear()
})

afterEach(async () => {
  if (root) {
    await act(async () => root?.unmount())
  }
  queryClient?.clear()
  host?.remove()
  footer?.remove()
  root = null
  queryClient = null
  host = null
  footer = null
  document.body.replaceChildren()
})

afterAll(() => {
  domWindow.close()
})

test('renders violation records and enables next page from server total', async () => {
  apiResponse = {
    success: true,
    data: {
      page: 1,
      page_size: 20,
      total: 21,
      items: [
        {
          id: 1,
          cycle_started_at: 1_700_000_000,
          triggered_at: 1_700_000_000,
          request_count: 200,
          detection_threshold: 200,
          penalty_rpm: 10,
          action: 'temporary_limit',
          effective_until: 1_700_000_600,
          created_at: 1_700_000_000,
        },
        {
          id: 2,
          cycle_started_at: 1_700_000_000,
          triggered_at: 1_700_000_700,
          request_count: 200,
          detection_threshold: 200,
          penalty_rpm: 10,
          action: 'permanent_ban',
          effective_until: 0,
          created_at: 1_700_000_700,
        },
      ],
    },
  }

  await renderTable()

  await waitForCondition(() => {
    assert.ok(host)
    assert.match(host.textContent || '', /2023-11-14/)
    assert.match(host.textContent || '', /200 \/ 200/)
    assert.match(host.textContent || '', /Temporary limit/)
    assert.match(host.textContent || '', /Permanent non-stream ban/)
    assert.match(host.textContent || '', /Permanent/)
    assert.deepEqual(apiCalls, [
      '/api/user/distillation/violations/self?p=1&page_size=20',
    ])
    assert.ok(footer)
    const nextLabel = [...footer.querySelectorAll('span')].find(
      (element) => element.textContent === 'Go to next page'
    )
    assert.ok(nextLabel)
    const nextButton = nextLabel.closest('button')
    assert.ok(nextButton)
    assert.equal(nextButton.disabled, false)
  })
})

test('renders the dedicated empty state', async () => {
  apiResponse = {
    success: true,
    data: { page: 1, page_size: 20, total: 0, items: [] },
  }

  await renderTable()

  await waitForCondition(() => {
    assert.ok(host)
    assert.match(host.textContent || '', /No Violation Records Found/)
    assert.match(
      host.textContent || '',
      /No distillation violations have been recorded for this account\./
    )
  })
})
