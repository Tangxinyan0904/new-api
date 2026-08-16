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
import { afterAll, afterEach, beforeEach, expect, mock, test } from 'bun:test'

import { Window } from 'happy-dom'

const domWindow = new Window({ url: 'http://localhost/channel-status' })

for (const key of [
  'window',
  'document',
  'navigator',
  'HTMLElement',
  'HTMLButtonElement',
  'Node',
  'Element',
  'Event',
  'MouseEvent',
  'MutationObserver',
  'requestAnimationFrame',
  'cancelAnimationFrame',
] as const) {
  Object.defineProperty(globalThis, key, {
    configurable: true,
    value: domWindow[key],
  })
}

const originalSetInterval = globalThis.setInterval
const originalClearInterval = globalThis.clearInterval
const intervalCallbacks = new Map<number, () => void>()
const clearedIntervals: number[] = []
let nextIntervalId = 1

const { act } = await import('react')
const { createRoot } = await import('react-dom/client')
const { useChannelStatusRefresh } =
  await import('@/features/channel-status/hooks/use-channel-status-refresh')

const reactTestGlobals = globalThis as typeof globalThis & {
  IS_REACT_ACT_ENVIRONMENT?: boolean
}
reactTestGlobals.IS_REACT_ACT_ENVIRONMENT = true

let host: HTMLDivElement | null = null
let root: ReturnType<typeof createRoot> | null = null

function RefreshHarness(props: { onRefresh: () => Promise<void> }) {
  const refresh = useChannelStatusRefresh(props.onRefresh)

  return (
    <div>
      <output data-testid='state'>
        {String(refresh.enabled)}:{refresh.secondsRemaining}:
        {String(refresh.isRefreshing)}
      </output>
      <button type='button' onClick={refresh.toggle}>
        toggle
      </button>
      <button type='button' onClick={() => void refresh.refreshNow()}>
        refresh
      </button>
    </div>
  )
}

function currentState() {
  return host?.querySelector('[data-testid="state"]')?.textContent
}

beforeEach(async () => {
  intervalCallbacks.clear()
  clearedIntervals.length = 0
  nextIntervalId = 1
  globalThis.setInterval = ((callback: TimerHandler) => {
    const id = nextIntervalId
    nextIntervalId += 1
    intervalCallbacks.set(id, callback as () => void)
    return id
  }) as typeof globalThis.setInterval
  globalThis.clearInterval = ((id: number | undefined) => {
    if (typeof id !== 'number') return
    clearedIntervals.push(id)
    intervalCallbacks.delete(id)
  }) as typeof globalThis.clearInterval

  host = document.createElement('div')
  document.body.append(host)
  root = createRoot(host)
})

afterEach(async () => {
  await act(async () => root?.unmount())
  host?.remove()
  document.body.replaceChildren()
  root = null
  host = null
})

afterAll(() => {
  globalThis.setInterval = originalSetInterval
  globalThis.clearInterval = originalClearInterval
  domWindow.close()
})

test('counts down from sixty and refreshes exactly once at zero', async () => {
  const onRefresh = mock(async () => undefined)
  await act(async () => root?.render(<RefreshHarness onRefresh={onRefresh} />))

  expect(currentState()).toBe('true:60:false')
  const tick = [...intervalCallbacks.values()][0]
  expect(tick).toBeDefined()

  await act(async () => tick?.())
  expect(currentState()).toBe('true:59:false')

  await act(async () => {
    for (let count = 0; count < 59; count += 1) tick?.()
    await Promise.resolve()
  })

  expect(onRefresh).toHaveBeenCalledTimes(1)
  expect(currentState()).toBe('true:60:false')
})

test('toggle stops and restarts the automatic countdown', async () => {
  const onRefresh = mock(async () => undefined)
  await act(async () => root?.render(<RefreshHarness onRefresh={onRefresh} />))
  const toggle = host?.querySelectorAll('button')[0]

  await act(async () =>
    toggle?.dispatchEvent(new MouseEvent('click', { bubbles: true }))
  )
  expect(currentState()).toBe('false:60:false')
  expect(intervalCallbacks.size).toBe(0)

  await act(async () =>
    toggle?.dispatchEvent(new MouseEvent('click', { bubbles: true }))
  )
  expect(currentState()).toBe('true:60:false')
  expect(intervalCallbacks.size).toBe(1)
})

test('manual refresh resets the countdown and unmount clears the timer', async () => {
  const onRefresh = mock(async () => undefined)
  await act(async () => root?.render(<RefreshHarness onRefresh={onRefresh} />))
  const tick = [...intervalCallbacks.values()][0]
  await act(async () => tick?.())
  expect(currentState()).toBe('true:59:false')

  const refreshButton = host?.querySelectorAll('button')[1]
  await act(async () => {
    refreshButton?.dispatchEvent(new MouseEvent('click', { bubbles: true }))
    await Promise.resolve()
  })
  expect(onRefresh).toHaveBeenCalledTimes(1)
  expect(currentState()).toBe('true:60:false')

  await act(async () => root?.unmount())
  root = null
  expect(intervalCallbacks.size).toBe(0)
  expect(clearedIntervals.length).toBeGreaterThan(0)
})
