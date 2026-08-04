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

import type { RebateApprovalRequest } from '../../types'

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
const { RebateApprovalRowActions } =
  await import('../rebate-approval-row-actions')

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
type ActionResult = { success: boolean; message?: string }
type ApprovalAction = (id: number) => Promise<ActionResult>

const toastClient = toast as unknown as {
  success: ToastMethod
  error: ToastMethod
}
const originalToastSuccess = toastClient.success
const originalToastError = toastClient.error

const request: RebateApprovalRequest = {
  id: 42,
  user_id: 7,
  username: 'rebate-user',
  invite_reward_quota: 100_000,
  recharge_rebate_quota: 50_000,
  total_quota: 150_000,
  status: 'pending',
  created_at: 1_700_000_000,
}

let host: HTMLDivElement | null = null
let root: ReturnType<typeof createRoot> | null = null
let queryClient: InstanceType<typeof QueryClient> | null = null
let successMessages: string[] = []
let errorMessages: string[] = []
let approveAction: ApprovalAction
let rejectAction: ApprovalAction

async function renderActions() {
  host = document.createElement('div')
  document.body.append(host)
  root = createRoot(host)
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  })
  queryClient = client
  client.setQueryData(['rebate-approvals'], { items: [request] })

  await act(async () => {
    root?.render(
      <QueryClientProvider client={client}>
        <I18nextProvider i18n={i18n}>
          <RebateApprovalRowActions
            request={request}
            onApprove={approveAction}
            onReject={rejectAction}
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
  approveAction = async () => ({ success: true })
  rejectAction = async () => ({ success: true })
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

test('requires approval confirmation and blocks duplicate actions while pending', async () => {
  let resolveRequest: ((value: ActionResult) => void) | null = null
  const calls: number[] = []
  approveAction = (id) => {
    calls.push(id)
    return new Promise((resolve) => {
      resolveRequest = resolve
    })
  }
  await renderActions()

  assert.ok(host)
  const approveButton = findButton('Approve', host)
  const rejectButton = findButton('Reject', host)
  await click(approveButton)

  assert.deepEqual(calls, [])
  const dialog = document.querySelector<HTMLElement>(
    '[data-slot="alert-dialog-content"]'
  )
  assert.ok(dialog)
  const confirmButton = findButton('Approve', dialog)
  await click(confirmButton)

  assert.deepEqual(calls, [42])
  await waitForCondition(() => {
    assert.equal(approveButton.disabled, true)
    assert.equal(rejectButton.disabled, true)
    assert.equal(confirmButton.disabled, true)
  })
  await click(confirmButton)
  assert.equal(calls.length, 1)

  await act(async () => {
    assert.ok(resolveRequest)
    resolveRequest({ success: true })
    await Promise.resolve()
  })

  await waitForCondition(() => {
    assert.deepEqual(successMessages, ['Approved'])
    assert.equal(errorMessages.length, 0)
    assert.equal(
      queryClient?.getQueryState(['rebate-approvals'])?.isInvalidated,
      true
    )
    assert.equal(
      document.querySelector('[data-slot="alert-dialog-content"]'),
      null
    )
    assert.equal(approveButton.disabled, false)
    assert.equal(rejectButton.disabled, false)
  })
})

test('keeps destructive rejection confirmation open after a business failure', async () => {
  rejectAction = async () => ({
    success: false,
    message: 'Request was already reviewed',
  })
  await renderActions()

  assert.ok(host)
  const rejectButton = findButton('Reject', host)
  assert.match(rejectButton.className, /text-destructive/)
  await click(rejectButton)
  const dialog = document.querySelector<HTMLElement>(
    '[data-slot="alert-dialog-content"]'
  )
  assert.ok(dialog)
  const confirmButton = findButton('Reject', dialog)
  assert.match(confirmButton.className, /text-destructive/)

  await click(confirmButton)

  await waitForCondition(() => {
    assert.deepEqual(errorMessages, ['Request was already reviewed'])
    assert.ok(document.querySelector('[data-slot="alert-dialog-content"]'))
    assert.equal(confirmButton.disabled, false)
  })
})

test('shows a translated network failure and allows retrying the same action', async () => {
  let attempts = 0
  approveAction = async () => {
    attempts += 1
    if (attempts === 1) throw new Error('offline')
    return { success: true }
  }
  await renderActions()

  assert.ok(host)
  await click(findButton('Approve', host))
  let dialog = document.querySelector<HTMLElement>(
    '[data-slot="alert-dialog-content"]'
  )
  assert.ok(dialog)
  await click(findButton('Approve', dialog))

  await waitForCondition(() => {
    assert.deepEqual(errorMessages, [
      'Network connection failed or server not responding',
    ])
    const currentDialog = document.querySelector<HTMLElement>(
      '[data-slot="alert-dialog-content"]'
    )
    assert.ok(currentDialog)
    assert.equal(findButton('Approve', currentDialog).disabled, false)
  })
  dialog = document.querySelector<HTMLElement>(
    '[data-slot="alert-dialog-content"]'
  )
  assert.ok(dialog)
  await click(findButton('Approve', dialog))

  await waitForCondition(() => {
    assert.equal(attempts, 2)
    assert.deepEqual(successMessages, ['Approved'])
    assert.equal(
      document.querySelector('[data-slot="alert-dialog-content"]'),
      null
    )
  })
})
