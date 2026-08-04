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
import { after, afterEach, beforeEach, test } from 'node:test'

import { Window } from 'happy-dom'

import type { GeoIPSettings } from '../../types'

const domWindow = new Window()
const domGlobals = [
  'window',
  'document',
  'navigator',
  'HTMLElement',
  'HTMLButtonElement',
  'HTMLInputElement',
  'HTMLTextAreaElement',
  'HTMLFormElement',
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
const { GeoIPSection } = await import('../geoip-section')

const i18n = createInstance()
await i18n.use(initReactI18next).init({
  lng: 'en',
  resources: { en: { translation: {} } },
})

const reactTestGlobals = globalThis as typeof globalThis & {
  IS_REACT_ACT_ENVIRONMENT?: boolean
}
reactTestGlobals.IS_REACT_ACT_ENVIRONMENT = true

type PutCall = {
  url: string
  data: unknown
}
type PutMethod = (
  url: string,
  data?: unknown
) => Promise<{ data: { success: boolean; message: string } }>
type MockableApi = {
  put: PutMethod
}

const apiClient = api as unknown as MockableApi
const originalPut = apiClient.put
let host: HTMLDivElement | null = null
let root: ReturnType<typeof createRoot> | null = null
let queryClient: InstanceType<typeof QueryClient> | null = null
let putCalls: PutCall[] = []
let nextResponse = { success: true, message: '' }

const defaultValues: GeoIPSettings = {
  'geoip.mode': 'off',
  'geoip.database_path': 'Country.mmdb',
  'geoip.download_url': '',
  'geoip.maxmind_license_key': '',
  'geoip.popup_message': 'Initial popup',
  'geoip.allow_private_loopback': true,
  'geoip.blocked_countries': ['CN'],
}

async function renderSection() {
  host = document.createElement('div')
  document.body.append(host)
  root = createRoot(host)
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  })
  queryClient = client
  await act(async () => {
    root?.render(
      <QueryClientProvider client={client}>
        <I18nextProvider i18n={i18n}>
          <GeoIPSection defaultValues={defaultValues} />
        </I18nextProvider>
      </QueryClientProvider>
    )
  })
}

function getControlByLabel<T extends HTMLElement>(labelText: string): T {
  const label = [...document.querySelectorAll<HTMLLabelElement>('label')].find(
    (candidate) => candidate.textContent?.trim() === labelText
  )
  assert.ok(label, `Expected label "${labelText}"`)
  const control =
    label.control ?? document.querySelector<HTMLElement>(`#${label.htmlFor}`)
  assert.ok(control, `Expected control for "${labelText}"`)
  return control as T
}

async function changeTextControl(
  control: HTMLInputElement | HTMLTextAreaElement,
  value: string
) {
  await act(async () => {
    const prototype =
      control instanceof domWindow.HTMLTextAreaElement
        ? domWindow.HTMLTextAreaElement.prototype
        : domWindow.HTMLInputElement.prototype
    const valueSetter = Object.getOwnPropertyDescriptor(prototype, 'value')?.set
    assert.ok(valueSetter)
    valueSetter.call(control, value)
    control.dispatchEvent(
      new domWindow.Event('input', { bubbles: true }) as unknown as Event
    )
  })
}

async function submitForm() {
  const form = document.querySelector<HTMLFormElement>('form')
  assert.ok(form)
  await act(async () => {
    form.dispatchEvent(
      new domWindow.Event('submit', {
        bubbles: true,
        cancelable: true,
      }) as unknown as Event
    )
    await Promise.resolve()
  })
}

beforeEach(() => {
  putCalls = []
  nextResponse = { success: true, message: '' }
  apiClient.put = async (url, data) => {
    putCalls.push({ url, data })
    return { data: nextResponse }
  }
})

afterEach(async () => {
  apiClient.put = originalPut
  if (root) {
    await act(async () => root?.unmount())
  }
  queryClient?.clear()
  host?.remove()
  root = null
  queryClient = null
  host = null
  document.body.replaceChildren()
})

after(() => {
  domWindow.close()
})

test('saves all changed GeoIP settings in one atomic request', async () => {
  await renderSection()
  await changeTextControl(
    getControlByLabel<HTMLInputElement>('GeoIP database path'),
    'GeoLite2-Country.mmdb'
  )
  await changeTextControl(
    getControlByLabel<HTMLTextAreaElement>('GeoIP popup message'),
    'Updated popup'
  )

  await submitForm()

  assert.equal(putCalls.length, 1)
  assert.equal(putCalls[0]?.url, '/api/option/geoip')
  assert.deepEqual(putCalls[0]?.data, {
    'geoip.mode': 'off',
    'geoip.database_path': 'GeoLite2-Country.mmdb',
    'geoip.download_url': '',
    'geoip.popup_message': 'Updated popup',
    'geoip.allow_private_loopback': true,
    'geoip.blocked_countries': ['CN'],
  })
})

test('retries unchanged form values after a business failure', async () => {
  nextResponse = { success: false, message: 'save failed' }
  await renderSection()
  await changeTextControl(
    getControlByLabel<HTMLInputElement>('GeoIP database path'),
    'GeoLite2-Country.mmdb'
  )

  await submitForm()
  const callsAfterFailure = putCalls.length
  await submitForm()

  assert.equal(callsAfterFailure, 1)
  assert.equal(putCalls.length, 2)
})
