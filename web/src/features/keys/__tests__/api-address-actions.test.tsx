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
import { afterAll, afterEach, beforeEach, expect, test } from 'bun:test'

import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { Window } from 'happy-dom'
import { createInstance } from 'i18next'
import { I18nextProvider } from 'react-i18next'

const domWindow = new Window({ url: 'http://localhost/keys' })

for (const key of [
  'window',
  'document',
  'navigator',
  'HTMLElement',
  'HTMLButtonElement',
  'SVGElement',
  'Node',
  'Element',
  'Event',
  'MouseEvent',
  'PointerEvent',
  'FocusEvent',
  'CustomEvent',
  'MutationObserver',
  'ResizeObserver',
  'requestAnimationFrame',
  'cancelAnimationFrame',
  'getComputedStyle',
] as const) {
  Object.defineProperty(globalThis, key, {
    configurable: true,
    value: domWindow[key],
  })
}

const { act } = await import('react')
const { createRoot } = await import('react-dom/client')
const { ApiKeysApiAddresses } =
  await import('../components/api-keys-api-addresses')

const i18n = createInstance()
await i18n.init({
  lng: 'en',
  resources: { en: { translation: {} } },
})

const reactTestGlobals = globalThis as typeof globalThis & {
  IS_REACT_ACT_ENVIRONMENT?: boolean
}
reactTestGlobals.IS_REACT_ACT_ENVIRONMENT = true

let host: HTMLDivElement | null = null
let root: ReturnType<typeof createRoot> | null = null
let queryClient: QueryClient | null = null

beforeEach(() => {
  host = document.createElement('div')
  document.body.append(host)
  root = createRoot(host)
  queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  queryClient.setQueryData(['status'], {
    api_info_enabled: true,
    api_info: [
      {
        url: 'https://api.example.com',
        route: 'Primary API',
        color: '#16a34a',
      },
    ],
  })
})

afterEach(async () => {
  await act(async () => root?.unmount())
  host?.remove()
  document.body.replaceChildren()
  queryClient?.clear()
  root = null
  host = null
  queryClient = null
})

afterAll(() => {
  domWindow.close()
})

test('uses the dedicated API key action treatment for both API address copy buttons', async () => {
  if (!queryClient) throw new Error('Query client was not initialized')
  const client = queryClient

  await act(async () =>
    root?.render(
      <QueryClientProvider client={client}>
        <I18nextProvider i18n={i18n}>
          <ApiKeysApiAddresses />
        </I18nextProvider>
      </QueryClientProvider>
    )
  )

  const copyButtons = [
    host?.querySelector<HTMLButtonElement>('button[aria-label="Copy URL"]'),
    host?.querySelector<HTMLButtonElement>(
      'button[aria-label="Copy with /v1"]'
    ),
  ]

  for (const button of copyButtons) {
    expect(button).not.toBeNull()
    expect(button?.classList.contains('border-api-key-action')).toBe(true)
    expect(button?.classList.contains('bg-api-key-action-surface')).toBe(true)
    expect(button?.classList.contains('text-api-key-action')).toBe(true)
  }
})
