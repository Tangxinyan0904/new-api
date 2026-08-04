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

import type { ApiResponse, RebateApproveAllResult } from '../../types'

const domWindow = new Window({ url: 'http://localhost' })
const domGlobals = [
  'window',
  'document',
  'navigator',
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

const { act } = await import('react')
const { createRoot } = await import('react-dom/client')
const { createInstance } = await import('i18next')
const { I18nextProvider, initReactI18next } = await import('react-i18next')
const { QueryClient, QueryClientProvider } =
  await import('@tanstack/react-query')
const { toast } = await import('sonner')
const { RebateApproveAllAction } = await import('../rebate-approve-all-action')

const reactTestGlobals = globalThis as typeof globalThis & {
  IS_REACT_ACT_ENVIRONMENT?: boolean
}
reactTestGlobals.IS_REACT_ACT_ENVIRONMENT = true

const i18n = createInstance()
await i18n.use(initReactI18next).init({
  lng: 'en',
  resources: { en: { translation: {} } },
})

type ToastMethod = (message: string) => string | number
type ApproveAllAction = () => Promise<ApiResponse<RebateApproveAllResult>>

const toastClient = toast as unknown as {
  success: ToastMethod
  error: ToastMethod
}
const originalToastSuccess = toastClient.success
const originalToastError = toastClient.error

let host: HTMLDivElement | null = null
let root: ReturnType<typeof createRoot> | null = null
let queryClient: InstanceType<typeof QueryClient> | null = null
let successMessages: string[] = []
let errorMessages: string[] = []
let approveAllAction: ApproveAllAction

async function renderAction(pendingCount: number, isCountLoading = false) {
  host = document.createElement('div')
  document.body.append(host)
  root = createRoot(host)
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  })
  queryClient = client
  client.setQueryData(['rebate-approvals'], { items: [] })

  await act(async () => {
    root?.render(
      <QueryClientProvider client={client}>
        <I18nextProvider i18n={i18n}>
          <RebateApproveAllAction
            pendingCount={pendingCount}
            isCountLoading={isCountLoading}
            onApproveAll={approveAllAction}
          />
        </I18nextProvider>
      </QueryClientProvider>
    )
  })
}

function findButton(label: string, scope: ParentNode = document) {
  const button = [...scope.querySelectorAll<HTMLButtonElement>('button')].find(
    (candidate) => candidate.textContent?.trim() === label
  )
  assert.ok(button, `Expected button "${label}"`)
  return button
}

async function click(button: HTMLButtonElement) {
  await act(async () => {
    button.click()
    await Promise.resolve()
    await new Promise<void>((resolve) => domWindow.setTimeout(resolve, 0))
  })
}

async function waitForCondition(assertion: () => void) {
  let lastError: unknown
  for (let attempt = 0; attempt < 20; attempt += 1) {
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
  successMessages = []
  errorMessages = []
  toastClient.success = (message) => {
    successMessages.push(message)
    return successMessages.length
  }
  toastClient.error = (message) => {
    errorMessages.push(message)
    return errorMessages.length
  }
  approveAllAction = async () => ({
    success: true,
    data: { approved_count: 0 },
  })
})

afterEach(async () => {
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
  toastClient.success = originalToastSuccess
  toastClient.error = originalToastError
  domWindow.close()
})

test('disables approve all when there are no pending requests', async () => {
  await renderAction(0)

  assert.ok(host)
  assert.equal(findButton('Approve All', host).disabled, true)
})

test('confirms the total and invalidates rebate queries after one request', async () => {
  let resolveApproval:
    | ((value: ApiResponse<RebateApproveAllResult>) => void)
    | null = null
  let callCount = 0
  approveAllAction = () => {
    callCount += 1
    return new Promise((resolve) => {
      resolveApproval = resolve
    })
  }
  await renderAction(3)

  assert.ok(host)
  const actionButton = findButton('Approve All', host)
  await click(actionButton)
  const dialog = document.querySelector<HTMLElement>(
    '[data-slot="alert-dialog-content"]'
  )
  assert.ok(dialog)
  assert.match(dialog.textContent || '', /3/)
  const confirmButton = findButton('Approve All', dialog)
  await click(confirmButton)

  assert.equal(callCount, 1)
  assert.equal(confirmButton.disabled, true)
  await click(confirmButton)
  assert.equal(callCount, 1)

  await act(async () => {
    assert.ok(resolveApproval)
    resolveApproval({ success: true, data: { approved_count: 3 } })
    await Promise.resolve()
  })

  await waitForCondition(() => {
    assert.deepEqual(successMessages, ['Approved 3 rebate requests'])
    assert.deepEqual(errorMessages, [])
    assert.equal(
      queryClient?.getQueryState(['rebate-approvals'])?.isInvalidated,
      true
    )
    assert.equal(
      document.querySelector('[data-slot="alert-dialog-content"]'),
      null
    )
  })
})

test('keeps approve all available after a business failure', async () => {
  approveAllAction = async () => ({
    success: false,
    message: 'Batch approval failed',
  })
  await renderAction(2)

  assert.ok(host)
  await click(findButton('Approve All', host))
  const dialog = document.querySelector<HTMLElement>(
    '[data-slot="alert-dialog-content"]'
  )
  assert.ok(dialog)
  const confirmButton = findButton('Approve All', dialog)
  await click(confirmButton)

  await waitForCondition(() => {
    assert.deepEqual(errorMessages, ['Batch approval failed'])
    assert.deepEqual(successMessages, [])
    assert.ok(document.querySelector('[data-slot="alert-dialog-content"]'))
    assert.equal(confirmButton.disabled, false)
  })
})
