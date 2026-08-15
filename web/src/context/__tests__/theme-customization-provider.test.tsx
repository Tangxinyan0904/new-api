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
import { after, beforeEach, test } from 'node:test'

import { Window } from 'happy-dom'

const domWindow = new Window({ url: 'https://example.test/' })
const domGlobals = [
  'window',
  'document',
  'navigator',
  'HTMLElement',
  'HTMLButtonElement',
  'Node',
  'Element',
  'Event',
  'MouseEvent',
] as const

for (const key of domGlobals) {
  Object.defineProperty(globalThis, key, {
    configurable: true,
    value: domWindow[key],
  })
}

const { act } = await import('react')
const { createRoot } = await import('react-dom/client')
const { ThemeCustomizationProvider, useThemeCustomization } =
  await import('../theme-customization-provider')

const reactTestGlobals = globalThis as typeof globalThis & {
  IS_REACT_ACT_ENVIRONMENT?: boolean
}
reactTestGlobals.IS_REACT_ACT_ENVIRONMENT = true

function clearPresetCookie() {
  document.cookie = 'theme_preset=; path=/; max-age=0'
}

function ThemeProbe() {
  const theme = useThemeCustomization()

  return (
    <div>
      <output data-testid='preset'>{theme.customization.preset}</output>
      <button type='button' onClick={() => theme.setPreset('default')}>
        Select base preset
      </button>
    </div>
  )
}

beforeEach(() => {
  clearPresetCookie()
  document.body.removeAttribute('data-theme-preset')
  document.body.replaceChildren()
})

after(() => {
  domWindow.close()
})

test('uses and persists simple-large when the browser has no saved preset', async () => {
  const host = document.createElement('div')
  document.body.append(host)
  const root = createRoot(host)

  await act(async () => {
    root.render(
      <ThemeCustomizationProvider>
        <ThemeProbe />
      </ThemeCustomizationProvider>
    )
  })

  assert.equal(
    host.querySelector('[data-testid="preset"]')?.textContent,
    'simple-large'
  )
  assert.equal(document.body.dataset.themePreset, 'simple-large')
  assert.match(document.cookie, /(?:^|; )theme_preset=simple-large(?:;|$)/)

  await act(async () => root.unmount())
  host.remove()
})

test('persists an explicit base preset and restores it after remount', async () => {
  document.cookie = 'theme_preset=simple-large; path=/'
  const firstHost = document.createElement('div')
  document.body.append(firstHost)
  const firstRoot = createRoot(firstHost)

  await act(async () => {
    firstRoot.render(
      <ThemeCustomizationProvider>
        <ThemeProbe />
      </ThemeCustomizationProvider>
    )
  })

  const selectBaseButton = firstHost.querySelector<HTMLButtonElement>('button')
  assert.ok(selectBaseButton)
  await act(async () => selectBaseButton.click())

  assert.equal(document.body.hasAttribute('data-theme-preset'), false)
  assert.match(document.cookie, /(?:^|; )theme_preset=default(?:;|$)/)

  await act(async () => firstRoot.unmount())
  firstHost.remove()

  const secondHost = document.createElement('div')
  document.body.append(secondHost)
  const secondRoot = createRoot(secondHost)
  await act(async () => {
    secondRoot.render(
      <ThemeCustomizationProvider>
        <ThemeProbe />
      </ThemeCustomizationProvider>
    )
  })

  assert.equal(
    secondHost.querySelector('[data-testid="preset"]')?.textContent,
    'default'
  )
  assert.equal(document.body.hasAttribute('data-theme-preset'), false)

  await act(async () => secondRoot.unmount())
  secondHost.remove()
})
