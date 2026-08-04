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

import type { RebateApprovalDetail } from '../../types'

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
const { api } = await import('@/lib/api')
const { RebateApprovalDetailDialog } =
  await import('../rebate-approval-detail-dialog')

const reactTestGlobals = globalThis as typeof globalThis & {
  IS_REACT_ACT_ENVIRONMENT?: boolean
}
reactTestGlobals.IS_REACT_ACT_ENVIRONMENT = true

const i18n = createInstance()
await i18n.use(initReactI18next).init({
  lng: 'en',
  resources: {
    en: {
      translation: {
        'detail unavailable': 'Translated detail unavailable',
        'Failed to load rebate request details.':
          'Unable to load rebate request details.',
        Retry: 'Retry',
      },
    },
  },
})

type GetMethod = (
  url: string,
  config?: unknown
) => Promise<{
  data: {
    success: boolean
    message?: string
    data?: RebateApprovalDetail
  }
}>

const apiClient = api as unknown as { get: GetMethod }
const originalGet = apiClient.get

const detail: RebateApprovalDetail = {
  id: 42,
  user_id: 7,
  username: 'rebate-user',
  display_name: 'Rebate User',
  invite_reward_quota: 100_000,
  recharge_rebate_quota: 50_000,
  total_quota: 150_000,
  status: 'pending',
  created_at: 1_700_000_000,
  invited_users: [],
  invited_count: 0,
  total_invited_recharge_quota: 0,
  recharge_rebate_rate: 0.1,
  recharge_sources: [],
}

let host: HTMLDivElement | null = null
let root: ReturnType<typeof createRoot> | null = null
let queryClient: InstanceType<typeof QueryClient> | null = null

async function renderDialog(cachedDetail?: RebateApprovalDetail) {
  host = document.createElement('div')
  document.body.append(host)
  root = createRoot(host)
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  })
  queryClient = client
  if (cachedDetail) {
    client.setQueryData(
      ['rebate-approval-detail', cachedDetail.id],
      cachedDetail
    )
  }

  await act(async () => {
    root?.render(
      <QueryClientProvider client={client}>
        <I18nextProvider i18n={i18n}>
          <RebateApprovalDetailDialog
            requestId={42}
            open
            onOpenChange={() => undefined}
          />
        </I18nextProvider>
      </QueryClientProvider>
    )
    await Promise.resolve()
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

function findButton(label: string) {
  const button = [
    ...document.querySelectorAll<HTMLButtonElement>('button'),
  ].find((candidate) => candidate.textContent?.trim() === label)
  assert.ok(button, `Expected button "${label}"`)
  return button
}

afterEach(async () => {
  apiClient.get = originalGet
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

test('shows a translated business error and retries the detail request', async () => {
  let calls = 0
  apiClient.get = async () => {
    calls += 1
    if (calls === 1) {
      return { data: { success: false, message: 'detail unavailable' } }
    }
    return { data: { success: true, data: detail } }
  }

  await renderDialog()

  await waitForCondition(() => {
    assert.match(
      document.body.textContent ?? '',
      /Translated detail unavailable/
    )
    findButton('Retry')
  })
  await act(async () => {
    findButton('Retry').click()
    await new Promise<void>((resolve) => domWindow.setTimeout(resolve, 0))
  })

  await waitForCondition(() => {
    assert.equal(calls, 2)
    assert.match(document.body.textContent ?? '', /Rebate User/)
  })
})

test('shows a translated retry state after a network failure', async () => {
  apiClient.get = async () => {
    throw new Error('socket closed')
  }

  await renderDialog()

  await waitForCondition(() => {
    assert.match(
      document.body.textContent ?? '',
      /Unable to load rebate request details\./
    )
    findButton('Retry')
  })
})

test('keeps cached detail visible when a background refresh fails', async () => {
  apiClient.get = async () => {
    throw new Error('socket closed')
  }

  await renderDialog(detail)

  await waitForCondition(() => {
    assert.match(document.body.textContent ?? '', /Rebate User/)
    assert.doesNotMatch(
      document.body.textContent ?? '',
      /Unable to load rebate request details\./
    )
  })
})
