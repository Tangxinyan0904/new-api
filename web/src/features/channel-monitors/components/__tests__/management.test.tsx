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
import { afterAll, afterEach, beforeEach, expect, test, vi } from 'vitest'

const domWindow = window
window.history.replaceState({}, '', '/channels')
const navigationCalls: Array<Record<string, unknown>> = []
let authUserRole = 100
let queryResult: Record<string, unknown> = {
  data: undefined,
  isLoading: false,
  isFetching: false,
}

vi.mock('@tanstack/react-query', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@tanstack/react-query')>()),
  useMutation: () => ({
    isPending: false,
    mutate: () => undefined,
    mutateAsync: async () => undefined,
  }),
  useQuery: () => queryResult,
  useQueryClient: () => ({
    invalidateQueries: async () => undefined,
  }),
}))

vi.mock('@tanstack/react-router', () => ({
  createFileRoute: () => (config: Record<string, unknown>) => config,
  redirect: (options: Record<string, unknown>) =>
    Object.assign(new Error('redirect'), { options }),
  useNavigate: () => (options: Record<string, unknown>) => {
    navigationCalls.push(options)
  },
}))

vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (key: string, values?: Record<string, unknown>) =>
      key.replaceAll(/\{\{(\w+)\}\}/g, (_match, name: string) =>
        String(values?.[name] ?? `{{${name}}}`)
      ),
  }),
}))

vi.mock('@/stores/auth-store', () => {
  const useAuthStore = Object.assign(
    (selector: (state: Record<string, unknown>) => unknown) =>
      selector({
        auth: { user: { role: authUserRole, permissions: {} } },
      }),
    {
      getState: () => ({
        auth: { user: { role: authUserRole, permissions: {} } },
      }),
    }
  )
  return { useAuthStore }
})

vi.mock('@/lib/admin-permissions', () => ({
  ADMIN_PERMISSION_ACTIONS: {
    READ: 'read',
    SENSITIVE_WRITE: 'sensitive_write',
  },
  ADMIN_PERMISSION_RESOURCES: { CHANNEL: 'channel' },
  hasPermission: () => true,
}))

vi.mock('@/features/channels/components/channels-provider', () => ({
  useChannels: () => ({
    setOpen: () => undefined,
    setCurrentRow: () => undefined,
    enableTagMode: false,
    setEnableTagMode: () => undefined,
    idSort: false,
    setIdSort: () => undefined,
    batchMode: false,
    setBatchMode: () => undefined,
    upstream: {
      detectAllUpdates: () => undefined,
      applyAllUpdates: () => undefined,
      detectAllLoading: false,
      applyAllLoading: false,
    },
  }),
}))

const { act } = await import('react')
const { createRoot } = await import('react-dom/client')
const { ChannelsPrimaryButtons } =
  await import('@/features/channels/components/channels-primary-buttons')

const reactTestGlobals = globalThis as typeof globalThis & {
  IS_REACT_ACT_ENVIRONMENT?: boolean
}
reactTestGlobals.IS_REACT_ACT_ENVIRONMENT = true

let host: HTMLDivElement | null = null
let root: ReturnType<typeof createRoot> | null = null

function createFreshRoot() {
  if (!host) throw new Error('test host is not initialized')
  return createRoot(host)
}

function channelMonitorFixture() {
  return {
    id: 17,
    internal_name: 'Primary OpenAI monitor',
    public_name: 'OpenAI availability',
    name_color: '#123ABC',
    provider: 'openai' as const,
    api_mode: 'chat_completions' as const,
    endpoint: 'https://api.example.com',
    credential_configured: true,
    credential_masked: '****cret',
    primary_model: 'gpt-primary',
    extra_models: ['gpt-secondary'],
    interval_seconds: 60,
    degraded_threshold_ms: 6000,
    timeout_seconds: 30,
    enabled: true,
    public_enabled: true,
    last_checked_at: 1_723_456_789,
    next_check_at: 1_723_456_849,
    schedule_blocked_reason: '',
    latest_status: 'operational' as const,
    latest_latency_ms: 321,
    latest_error_category: '' as const,
    lease_owner: '',
    lease_until: 0,
    created_by: 1,
    created_at: 1_723_450_000,
    updated_at: 1_723_456_789,
  }
}

async function changeInput(input: HTMLInputElement, value: string) {
  const valueSetter = Object.getOwnPropertyDescriptor(
    domWindow.HTMLInputElement.prototype,
    'value'
  )?.set
  if (!valueSetter) throw new Error('Input value setter is unavailable')

  await act(async () => {
    valueSetter.call(input, value)
    input.dispatchEvent(
      new domWindow.Event('input', { bubbles: true }) as unknown as Event
    )
  })
}

beforeEach(async () => {
  navigationCalls.length = 0
  authUserRole = 100
  queryResult = { data: undefined, isLoading: false, isFetching: false }
  host = document.createElement('div')
  document.body.append(host)
  root = createRoot(host)
  await act(async () => {
    root?.render(<ChannelsPrimaryButtons />)
  })
})

afterEach(async () => {
  await act(async () => root?.unmount())
  host?.remove()
  document.body.replaceChildren()
  root = null
  host = null
})

afterAll(() => {})

test('opens the channel monitor administration page from channel actions', async () => {
  const button = [...(host?.querySelectorAll('button') ?? [])].find((element) =>
    element.textContent?.includes('Channel Monitor')
  )
  expect(button).toBeDefined()

  await act(async () => {
    button?.dispatchEvent(new MouseEvent('click', { bubbles: true }))
  })

  expect(navigationCalls).toEqual([{ to: '/channel-monitors' }])
})

test('renders monitor status and exposes every management action', async () => {
  const { ChannelMonitorTable } =
    await import('@/features/channel-monitors/components/channel-monitor-table')
  await act(async () => root?.unmount())
  root = createFreshRoot()

  const monitor = channelMonitorFixture()

  await act(async () => {
    root?.render(
      <ChannelMonitorTable
        monitors={[monitor]}
        isLoading={false}
        runningMonitorId={null}
        onEdit={() => undefined}
        onRunNow={() => undefined}
        onHistory={() => undefined}
        onDelete={() => undefined}
      />
    )
  })

  expect(host?.textContent).toContain('Primary OpenAI monitor')
  expect(host?.textContent).toContain('OpenAI availability')
  expect(host?.textContent).toContain('operational')
  expect(host?.textContent).toContain('321 ms')
  expect(host?.textContent).toContain('2')
  expect(host?.textContent).toContain('60 s')

  for (const label of ['Edit', 'Run now', 'View history', 'Delete']) {
    expect(host?.querySelector(`button[aria-label="${label}"]`)).not.toBeNull()
  }

  await act(async () => {
    root?.render(
      <ChannelMonitorTable
        monitors={[monitor]}
        isLoading={false}
        runningMonitorId={monitor.id}
        onEdit={() => undefined}
        onRunNow={() => undefined}
        onHistory={() => undefined}
        onDelete={() => undefined}
      />
    )
  })

  const runButton = host?.querySelector<HTMLButtonElement>(
    'button[aria-label="Run now"]'
  )
  expect(runButton?.disabled).toBe(true)
  expect(runButton?.querySelector('[role="status"]')).not.toBeNull()
})

test('applies the configured color only to the public monitor name', async () => {
  const { ChannelMonitorTable } =
    await import('@/features/channel-monitors/components/channel-monitor-table')
  await act(async () => root?.unmount())
  root = createFreshRoot()

  await act(async () => {
    root?.render(
      <ChannelMonitorTable
        monitors={[channelMonitorFixture()]}
        isLoading={false}
        runningMonitorId={null}
        onEdit={() => undefined}
        onRunNow={() => undefined}
        onHistory={() => undefined}
        onDelete={() => undefined}
      />
    )
  })

  const internalName = host?.querySelector<HTMLElement>(
    '[title="Primary OpenAI monitor"]'
  )
  const publicName = host?.querySelector<HTMLElement>(
    '[title="OpenAI availability"]'
  )
  expect(internalName?.style.color).toBe('')
  expect(publicName?.style.color).toBe('rgb(18, 58, 188)')
})

test('discloses credential, HTTP, and publication state while editing', async () => {
  const { ChannelMonitorFormDrawer } =
    await import('@/features/channel-monitors/components/channel-monitor-form-drawer')
  await act(async () => root?.unmount())
  root = createFreshRoot()

  await act(async () => {
    root?.render(
      <ChannelMonitorFormDrawer
        open
        monitor={{
          ...channelMonitorFixture(),
          endpoint: 'http://api.example.com',
          extra_models: [],
        }}
        isSubmitting={false}
        onOpenChange={() => undefined}
        onSubmit={async () => undefined}
      />
    )
  })

  expect(document.body.textContent).toContain('****cret')
  expect(document.body.textContent).toContain(
    'HTTP endpoints send monitor credentials without transport encryption.'
  )
  expect(document.body.textContent).toContain(
    'Published monitors expose their public name, provider, models, status, latency, and availability.'
  )
})

test('synchronizes monitor name color controls and restores the theme default', async () => {
  const { ChannelMonitorFormDrawer } =
    await import('@/features/channel-monitors/components/channel-monitor-form-drawer')
  await act(async () => root?.unmount())
  root = createFreshRoot()

  await act(async () => {
    root?.render(
      <ChannelMonitorFormDrawer
        open
        monitor={channelMonitorFixture()}
        isSubmitting={false}
        onOpenChange={() => undefined}
        onSubmit={async () => undefined}
      />
    )
  })

  const colorCode = document.body.querySelector<HTMLInputElement>(
    'input[aria-label="Public name color code"]'
  )
  const colorSwatch = document.body.querySelector<HTMLInputElement>(
    'input[aria-label="Choose public name color"]'
  )
  const resetButton = document.body.querySelector<HTMLButtonElement>(
    'button[aria-label="Use theme default color"]'
  )
  const preview = document.body.querySelector<HTMLElement>(
    '[data-slot="monitor-name-preview"]'
  )
  expect(colorCode?.value).toBe('#123ABC')
  expect(colorSwatch?.value).toBe('#123abc')
  expect(preview?.style.color).toBe('rgb(18, 58, 188)')
  if (!colorCode || !colorSwatch || !resetButton) {
    throw new Error('Name color controls were not rendered')
  }

  await changeInput(colorCode, '#abcdef')
  expect(colorSwatch.value).toBe('#abcdef')
  expect(preview?.style.color).toBe('rgb(171, 205, 239)')

  await changeInput(colorSwatch, '#a1b2c3')
  expect(colorCode.value).toBe('#A1B2C3')
  expect(preview?.style.color).toBe('rgb(161, 178, 195)')

  await act(async () => resetButton.click())
  expect(colorCode.value).toBe('')
  expect(preview?.style.color).toBe('')
})

test('shows administrator-only probe failure details in history', async () => {
  const { ChannelMonitorHistoryTable } =
    await import('@/features/channel-monitors/components/channel-monitor-history-dialog')
  await act(async () => root?.unmount())
  root = createFreshRoot()

  await act(async () => {
    root?.render(
      <ChannelMonitorHistoryTable
        isLoading={false}
        items={[
          {
            id: 91,
            monitor_id: 17,
            model: 'gpt-primary',
            status: 'failed',
            latency_ms: 987,
            error_category: 'upstream',
            http_status: 500,
            error_message: 'upstream timeout while reading response',
            checked_at: 1_723_456_789,
          },
        ]}
      />
    )
  })

  expect(host?.textContent).toContain('gpt-primary')
  expect(host?.textContent).toContain('failed')
  expect(host?.textContent).toContain('500')
  expect(host?.textContent).toContain('upstream timeout while reading response')
})

test('renders the administrator monitor list with creation and filter controls', async () => {
  const { ChannelMonitors } = await import('@/features/channel-monitors')
  await act(async () => root?.unmount())
  root = createFreshRoot()
  queryResult = {
    data: {
      items: [{ ...channelMonitorFixture(), extra_models: [] }],
      total: 1,
      page: 1,
      page_size: 20,
    },
    isLoading: false,
    isFetching: false,
  }

  await act(async () => {
    root?.render(<ChannelMonitors />)
  })

  expect(host?.textContent).toContain('Channel monitors')
  expect(host?.textContent).toContain('Create monitor')
  expect(host?.textContent).toContain('Primary OpenAI monitor')
  expect(
    host?.querySelector('input[aria-label="Search monitors"]')
  ).not.toBeNull()
})

test('allows administrators and redirects non-admin users from the monitor route', async () => {
  const { Route } = await import('@/routes/_authenticated/channel-monitors')
  const beforeLoad = (Route as unknown as { beforeLoad: () => void }).beforeLoad

  expect(() => beforeLoad()).not.toThrow()

  authUserRole = 1
  expect(() => beforeLoad()).toThrow('redirect')
})
