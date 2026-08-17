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
domWindow.document.write(
  '<!doctype html><html><head></head><body></body></html>'
)
domWindow.document.close()
Object.defineProperty(domWindow.document, 'compatMode', {
  configurable: true,
  value: 'CSS1Compat',
})
const apiCalls: Array<{ url: string; params?: Record<string, unknown> }> = []
let blockedStatusDays: 3 | null = null
let releaseBlockedStatus: (() => void) | null = null

const statusByDays = {
  1: {
    last_updated_at: 1_786_841_200,
    counts: { operational: 1, degraded: 0, failed: 0, unknown: 0 },
    days: 1,
    items: [
      {
        id: 17,
        public_name: 'OpenAI global',
        name_color: '#123ABC',
        provider: 'openai',
        primary_model: 'gpt-primary',
        status: 'operational',
        latency_ms: 321,
        error_category: '',
        last_checked_at: 1_786_841_200,
        availability: 99.125,
        average_latency_ms: 310.5,
        timeline: [
          {
            status: 'operational',
            latency_ms: 321,
            error_category: '',
            checked_at: 1_786_841_200,
          },
          {
            status: 'degraded',
            latency_ms: 6321,
            error_category: '',
            checked_at: 1_786_841_140,
          },
        ],
      },
    ],
  },
  3: {
    last_updated_at: 1_786_841_200,
    counts: { operational: 1, degraded: 0, failed: 0, unknown: 0 },
    days: 3,
    items: [
      {
        id: 17,
        public_name: 'OpenAI global',
        name_color: '#123ABC',
        provider: 'openai',
        primary_model: 'gpt-primary',
        status: 'operational',
        latency_ms: 321,
        error_category: '',
        last_checked_at: 1_786_841_200,
        availability: 98.25,
        average_latency_ms: 318.75,
        timeline: [],
      },
    ],
  },
  7: {
    last_updated_at: 1_786_841_200,
    counts: { operational: 0, degraded: 1, failed: 0, unknown: 0 },
    days: 7,
    items: [
      {
        id: 17,
        public_name: 'OpenAI global',
        name_color: '#123ABC',
        provider: 'openai',
        primary_model: 'gpt-primary',
        status: 'degraded',
        latency_ms: 6321,
        error_category: '',
        last_checked_at: 1_786_841_200,
        availability: 96.5,
        average_latency_ms: 702.25,
        timeline: [],
      },
    ],
  },
} as const

const monitorDetail = {
  id: 17,
  public_name: 'OpenAI global',
  name_color: '#123ABC',
  provider: 'openai',
  primary_model: 'gpt-primary',
  status: 'operational',
  last_updated_at: 1_786_841_200,
  models: [
    {
      model: 'gpt-primary',
      is_primary: true,
      status: 'operational',
      latency_ms: 321,
      error_category: '',
      last_checked_at: 1_786_841_200,
      availability_1d: 99.125,
      availability_3d: 98.25,
      availability_7d: 96.5,
      average_latency_ms_1d: 310.5,
      average_latency_ms_3d: 318.75,
      average_latency_ms_7d: 702.25,
    },
    {
      model: 'gpt-secondary',
      is_primary: false,
      status: 'failed',
      latency_ms: null,
      error_category: 'upstream',
      last_checked_at: 1_786_841_100,
      availability_1d: 87.5,
      availability_3d: 90,
      availability_7d: 92.25,
      average_latency_ms_1d: null,
      average_latency_ms_3d: 510,
      average_latency_ms_7d: 498,
    },
  ],
  timeline: [],
} as const

for (const key of [
  'window',
  'document',
  'navigator',
  'HTMLElement',
  'HTMLButtonElement',
  'HTMLDivElement',
  'SVGElement',
  'Node',
  'Element',
  'Event',
  'MouseEvent',
  'PointerEvent',
  'KeyboardEvent',
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

Object.defineProperty(globalThis, 'matchMedia', {
  configurable: true,
  value: () => ({
    matches: false,
    media: '',
    onchange: null,
    addEventListener: () => undefined,
    removeEventListener: () => undefined,
    addListener: () => undefined,
    removeListener: () => undefined,
    dispatchEvent: () => false,
  }),
})

mock.module('@/lib/http-client', () => ({
  api: {
    get: async (url: string, config?: { params?: Record<string, unknown> }) => {
      apiCalls.push({ url, params: config?.params })
      if (url === '/api/channel-monitor/status') {
        const days = Number(config?.params?.days ?? 1) as 1 | 3 | 7
        if (days === blockedStatusDays) {
          await new Promise<void>((resolve) => {
            releaseBlockedStatus = resolve
          })
        }
        return { data: { success: true, data: statusByDays[days] } }
      }
      if (url === '/api/channel-monitor/status/17') {
        return { data: { success: true, data: monitorDetail } }
      }
      throw new Error(`Unexpected API request: ${url}`)
    },
  },
}))

mock.module('react-i18next', () => ({
  useTranslation: () => ({
    t: (key: string, values?: Record<string, unknown>) =>
      key.replaceAll(/\{\{(\w+)\}\}/g, (_match, name: string) =>
        String(values?.[name] ?? `{{${name}}}`)
      ),
  }),
}))

const { act } = await import('react')
const { createRoot } = await import('react-dom/client')
const { QueryClient, QueryClientProvider } =
  await import('@tanstack/react-query')
const { channelStatusQueryKeys, getChannelStatus, getChannelStatusDetail } =
  await import('@/features/channel-status/api')
const { ChannelStatus } = await import('@/features/channel-status')
const { MonitorStatusCard } =
  await import('@/features/channel-status/components/monitor-status-card')
const { useSidebarData } = await import('@/hooks/use-sidebar-data')

const reactTestGlobals = globalThis as typeof globalThis & {
  IS_REACT_ACT_ENVIRONMENT?: boolean
}
reactTestGlobals.IS_REACT_ACT_ENVIRONMENT = true

let host: HTMLDivElement | null = null
let root: ReturnType<typeof createRoot> | null = null
let queryClient: InstanceType<typeof QueryClient> | null = null

function SidebarHarness() {
  const general = useSidebarData().navGroups.find(
    (group) => group.id === 'general'
  )
  return (
    <output data-testid='general-navigation'>
      {general?.items.map((item) => `${item.title}:${item.url}`).join('|')}
    </output>
  )
}

async function renderWithQueries(node: React.ReactNode) {
  const client = new QueryClient({
    defaultOptions: {
      queries: { retry: false, gcTime: Infinity, staleTime: Infinity },
    },
  })
  queryClient = client
  await Promise.all([
    client.fetchQuery({
      queryKey: channelStatusQueryKeys.status(1),
      queryFn: () => getChannelStatus(1),
    }),
    client.fetchQuery({
      queryKey: channelStatusQueryKeys.status(3),
      queryFn: () => getChannelStatus(3),
    }),
    client.fetchQuery({
      queryKey: channelStatusQueryKeys.status(7),
      queryFn: () => getChannelStatus(7),
    }),
    client.fetchQuery({
      queryKey: channelStatusQueryKeys.detail(17),
      queryFn: () => getChannelStatusDetail(17),
    }),
  ])
  await act(async () => {
    root?.render(
      <QueryClientProvider client={client}>{node}</QueryClientProvider>
    )
  })
}

beforeEach(() => {
  apiCalls.length = 0
  blockedStatusDays = null
  releaseBlockedStatus = null
  host = document.createElement('div')
  document.body.append(host)
  root = createRoot(host)
})

afterEach(async () => {
  await act(async () => root?.unmount())
  queryClient?.clear()
  host?.remove()
  document.body.replaceChildren()
  queryClient = null
  root = null
  host = null
})

afterAll(() => {
  domWindow.close()
})

test('switching the availability window updates the monitor metrics', async () => {
  await renderWithQueries(<ChannelStatus />)

  expect(host?.textContent).toContain('99.13%')

  const threeDayButton = host?.querySelector<HTMLButtonElement>(
    'button[aria-label="Show 3-day availability"]'
  )
  expect(threeDayButton).not.toBeNull()

  await act(async () => threeDayButton?.click())

  expect(host?.textContent).toContain('98.25%')
  expect(apiCalls).toContainEqual({
    url: '/api/channel-monitor/status',
    params: { days: 3 },
  })
})

test('shows status counts without a cross-monitor aggregate status', async () => {
  await renderWithQueries(<ChannelStatus />)

  const summary = host?.querySelector('[aria-label="Channel status summary"]')
  expect(summary).not.toBeNull()
  expect(summary?.textContent).not.toContain('Overall status')
  expect(summary?.querySelector('[role="status"]')).toBeNull()
  expect(summary?.textContent).toContain('Operational')
})

test('does not label stale metrics as the newly selected window', async () => {
  const client = new QueryClient({
    defaultOptions: {
      queries: { retry: false, gcTime: Infinity, staleTime: Infinity },
    },
  })
  queryClient = client
  await client.fetchQuery({
    queryKey: channelStatusQueryKeys.status(1),
    queryFn: () => getChannelStatus(1),
  })
  blockedStatusDays = 3

  await act(async () => {
    root?.render(
      <QueryClientProvider client={client}>
        <ChannelStatus />
      </QueryClientProvider>
    )
  })

  const threeDayButton = host?.querySelector<HTMLButtonElement>(
    'button[aria-label="Show 3-day availability"]'
  )
  if (!threeDayButton) throw new Error('Three-day selector was not rendered')

  await act(async () => threeDayButton.click())

  expect(host?.textContent).not.toContain('99.13%')

  await act(async () => root?.unmount())
  root = null
  client.clear()
  releaseBlockedStatus?.()
  await Promise.resolve()
})

test('monitor cards are keyboard focusable and open primary plus extra model details', async () => {
  await renderWithQueries(<ChannelStatus />)

  const card = host?.querySelector<HTMLButtonElement>(
    'button[aria-label="View OpenAI global details"]'
  )
  expect(card).not.toBeNull()
  if (!card) throw new Error('Monitor card was not rendered')
  card.focus()
  expect(document.activeElement).toBe(card)
  expect(
    card.querySelector<HTMLElement>('[data-slot="monitor-name"]')?.style.color
  ).toBe('#123ABC')

  await act(async () => card.click())

  expect(document.body.textContent).toContain('gpt-primary')
  expect(document.body.textContent).toContain('gpt-secondary')
  expect(document.body.textContent).toContain('Primary')
  expect(document.body.textContent).toContain('Additional')
  expect(
    document.body.querySelector<HTMLElement>(
      '[data-slot="monitor-detail-name"]'
    )?.style.color
  ).toBe('#123ABC')
})

test('renders real timeline points oldest to newest across the full row', async () => {
  await act(async () => {
    root?.render(
      <MonitorStatusCard
        days={1}
        monitor={{
          ...statusByDays[1].items[0],
          timeline: [...statusByDays[1].items[0].timeline],
        }}
        onSelect={() => undefined}
      />
    )
  })

  const timeline = host?.querySelector<HTMLElement>(
    '[aria-label="Recent channel checks"]'
  )
  const points = [
    ...(host?.querySelectorAll('[data-slot="timeline-point"]') ?? []),
  ]
  expect(points).toHaveLength(2)
  expect(points[0]?.getAttribute('data-status')).toBe('degraded')
  expect(points[1]?.getAttribute('data-status')).toBe('operational')
  expect(timeline?.style.gridTemplateColumns).toBe('repeat(2, minmax(0, 1fr))')
})

test('unknown state keeps text, an icon, bounded names, and an empty timeline message', async () => {
  const longName =
    'A very long public monitor name that must remain inside its card'
  const onSelect = mock(() => undefined)

  await act(async () => {
    root?.render(
      <MonitorStatusCard
        days={1}
        monitor={{
          ...statusByDays[1].items[0],
          public_name: longName,
          primary_model: 'a-model-name-that-is-also-intentionally-very-long',
          status: 'unknown',
          latency_ms: null,
          last_checked_at: 0,
          availability: null,
          average_latency_ms: null,
          timeline: [],
        }}
        onSelect={onSelect}
      />
    )
  })

  const status = host?.querySelector('[role="status"]')
  expect(status?.textContent).toContain('Unknown')
  expect(status?.querySelector('svg')).not.toBeNull()

  const name = host?.querySelector('[data-slot="monitor-name"]')
  expect(name?.getAttribute('title')).toBe(longName)
  expect(name?.classList.contains('truncate')).toBe(true)

  const card = host?.querySelector<HTMLButtonElement>(
    `button[aria-label="View ${longName} details"]`
  )
  if (!card) throw new Error('Unknown monitor card was not rendered')
  card.focus()
  expect(document.activeElement).toBe(card)

  const timeline = host?.querySelectorAll('[data-slot="timeline-point"]') ?? []
  expect(timeline.length).toBe(0)
  const emptyTimeline = host?.querySelector('[data-slot="timeline-empty"]')
  expect(emptyTimeline?.textContent).toContain('No recent checks')
})

test('places channel status directly after dashboard for every signed-in user', async () => {
  await act(async () => root?.render(<SidebarHarness />))

  const navigation = host?.querySelector(
    '[data-testid="general-navigation"]'
  )?.textContent
  expect(navigation).toContain(
    'Dashboard:/dashboard/models|Channel Status:/channel-status|API Keys:/keys'
  )

  const { Route } = await import('@/routes/_authenticated/channel-status')
  expect((Route as { beforeLoad?: unknown }).beforeLoad).toBeUndefined()
})
