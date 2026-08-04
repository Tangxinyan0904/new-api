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
import { after, afterEach, test } from 'node:test'

import { Window } from 'happy-dom'

import type { SystemStatus } from '../../types'

const domWindow = new Window({ url: 'http://localhost/sign-in' })
const domGlobals = [
  'window',
  'document',
  'navigator',
  'HTMLElement',
  'HTMLButtonElement',
  'HTMLAnchorElement',
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
const { LegalConsent } = await import('../legal-consent')

const reactTestGlobals = globalThis as typeof globalThis & {
  IS_REACT_ACT_ENVIRONMENT?: boolean
}
reactTestGlobals.IS_REACT_ACT_ENVIRONMENT = true

const i18n = createInstance()
await i18n.use(initReactI18next).init({
  lng: 'en',
  resources: { en: { translation: {} } },
})

const legalStatus = {
  user_agreement_enabled: true,
  privacy_policy_enabled: true,
} as SystemStatus

let host: HTMLDivElement | null = null
let root: ReturnType<typeof createRoot> | null = null
let checkedValues: boolean[] = []

async function renderConsent() {
  host = document.createElement('div')
  document.body.append(host)
  root = createRoot(host)
  await act(async () => {
    root?.render(
      <I18nextProvider i18n={i18n}>
        <LegalConsent
          status={legalStatus}
          checked={false}
          onCheckedChange={(checked) => checkedValues.push(checked)}
        />
      </I18nextProvider>
    )
  })
}

function getLegalLinks() {
  assert.ok(host)
  const links = [...host.querySelectorAll<HTMLAnchorElement>('a')]
  assert.equal(links.length, 2)
  return links
}

afterEach(async () => {
  if (root) {
    await act(async () => root?.unmount())
  }
  host?.remove()
  root = null
  host = null
  checkedValues = []
  document.body.replaceChildren()
})

after(() => {
  domWindow.close()
})

test('associates only plain consent text with the checkbox label', async () => {
  await renderConsent()

  assert.ok(host)
  const checkbox = host.querySelector<HTMLElement>('#legal-consent')
  const label = host.querySelector<HTMLLabelElement>(
    'label[for="legal-consent"]'
  )
  assert.ok(checkbox)
  assert.ok(label)
  assert.equal(label.contains(checkbox), false)
  for (const link of getLegalLinks()) {
    assert.equal(link.closest('label'), null)
  }
})

test('opening either legal link with a pointer does not toggle consent', async () => {
  await renderConsent()

  for (const link of getLegalLinks()) {
    await act(async () => link.click())
  }

  assert.deepEqual(checkedValues, [])
})

test('opening a focused legal link from the keyboard does not toggle consent', async () => {
  await renderConsent()

  const [link] = getLegalLinks()
  assert.ok(link)
  link.focus()
  await act(async () => {
    link.dispatchEvent(
      new domWindow.KeyboardEvent('keydown', {
        key: 'Enter',
        bubbles: true,
        cancelable: true,
      }) as unknown as KeyboardEvent
    )
    link.dispatchEvent(
      new domWindow.MouseEvent('click', {
        bubbles: true,
        cancelable: true,
        detail: 0,
      }) as unknown as MouseEvent
    )
  })

  assert.equal(document.activeElement, link)
  assert.deepEqual(checkedValues, [])
})
