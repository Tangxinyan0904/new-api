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

const domWindow = new Window({ url: 'http://localhost/pricing' })
Object.defineProperty(domWindow.document, 'compatMode', {
  configurable: true,
  value: 'CSS1Compat',
})

for (const key of [
  'window',
  'document',
  'navigator',
  'HTMLElement',
  'HTMLButtonElement',
  'HTMLInputElement',
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

const navigationCalls: Array<Record<string, unknown>> = []
let authUser: { id: number; role: number } | null = null
let publicSummaryQuery: Record<string, unknown> = {
  data: undefined,
  isLoading: true,
  isError: false,
}
let pricingLoading = false

const publicSummary = {
  overall_status: 'operational',
  last_updated_at: 1_786_841_200,
  counts: { operational: 1, degraded: 1, failed: 1, unknown: 1 },
  monitors: [
    {
      id: 17,
      public_name: 'OpenAI global',
      provider: 'openai',
      primary_model: 'gpt-primary',
      status: 'operational',
      latency_ms: 321,
      error_category: '',
      availability_1d: 99.125,
      last_checked_at: 1_786_841_200,
    },
    {
      id: 18,
      public_name: 'Claude edge',
      provider: 'anthropic',
      primary_model: 'claude-primary',
      status: 'degraded',
      latency_ms: 6321,
      error_category: '',
      availability_1d: 98.25,
      last_checked_at: 1_786_841_100,
    },
    {
      id: 19,
      public_name: 'Gemini regional',
      provider: 'gemini',
      primary_model: 'gemini-primary',
      status: 'failed',
      latency_ms: null,
      error_category: 'upstream',
      availability_1d: 87.5,
      last_checked_at: 1_786_841_000,
    },
    {
      id: 20,
      public_name: 'New monitor',
      provider: 'openai',
      primary_model: 'gpt-new',
      status: 'unknown',
      latency_ms: null,
      error_category: '',
      availability_1d: null,
      last_checked_at: 0,
    },
  ],
} as const

const actualReactQuery = await import('@tanstack/react-query')
const actualRouter = await import('@tanstack/react-router')
const realUseQuery = actualReactQuery.useQuery

mock.module('@tanstack/react-query', () => ({
  ...actualReactQuery,
  useQuery: (...args: Parameters<typeof actualReactQuery.useQuery>) => {
    const options = args[0] as { queryKey?: readonly unknown[] }
    if (options.queryKey?.[1] === 'public-summary') {
      return publicSummaryQuery
    }
    return realUseQuery(...args)
  },
}))

mock.module('@tanstack/react-router', () => ({
  ...actualRouter,
  useNavigate: () => (options: Record<string, unknown>) => {
    navigationCalls.push(options)
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

mock.module('@/stores/auth-store', () => ({
  useAuthStore: (selector: (state: Record<string, unknown>) => unknown) =>
    selector({ auth: { user: authUser } }),
}))

mock.module('@/components/layout', () => ({
  PublicLayout: (props: { children: React.ReactNode }) => (
    <div data-testid='public-layout'>{props.children}</div>
  ),
  SectionPageLayout: Object.assign(
    (props: { children: React.ReactNode }) => <main>{props.children}</main>,
    {
      Title: (props: { children: React.ReactNode }) => (
        <div>{props.children}</div>
      ),
      Actions: (props: { children: React.ReactNode }) => (
        <div>{props.children}</div>
      ),
      Content: (props: { children: React.ReactNode }) => (
        <div>{props.children}</div>
      ),
      Breadcrumb: (props: { children: React.ReactNode }) => (
        <div>{props.children}</div>
      ),
    }
  ),
}))

mock.module('@/components/page-transition', () => ({
  PageTransition: (props: {
    children: React.ReactNode
    className?: string
  }) => <div className={props.className}>{props.children}</div>,
}))

mock.module('@/features/pricing/components', () => ({
  LoadingSkeleton: () => <div data-testid='pricing-loading' />,
  EmptyState: () => <div data-testid='pricing-empty' />,
  SearchBar: (props: { value: string; onChange: (value: string) => void }) => (
    <input
      aria-label='Search models'
      value={props.value}
      onChange={(event) => props.onChange(event.target.value)}
    />
  ),
  PricingTable: () => <div data-testid='pricing-table' />,
  PricingSidebar: () => <aside data-testid='pricing-sidebar' />,
  PricingToolbar: () => <div data-testid='pricing-toolbar' />,
  ModelCardGrid: () => <div data-testid='model-card-grid' />,
  ModelDetailsDrawer: () => <div data-testid='model-details' />,
}))

mock.module('@/features/pricing/hooks/use-pricing-data', () => ({
  usePricingData: () => ({
    models: [{ model_name: 'gpt-primary' }],
    vendors: [],
    groupRatio: {},
    usableGroup: {},
    endpointMap: {},
    autoGroups: [],
    isLoading: pricingLoading,
    priceRate: 1,
    usdExchangeRate: 1,
  }),
}))

mock.module('@/features/pricing/hooks/use-filters', () => ({
  useFilters: (models: Array<{ model_name: string }>) => ({
    searchInput: '',
    sortBy: 'name',
    vendorFilter: 'all',
    groupFilter: 'all',
    quotaTypeFilter: 'all',
    endpointTypeFilter: 'all',
    tagFilter: 'all',
    tokenUnit: 'M',
    viewMode: 'card',
    showRechargePrice: false,
    setSearchInput: () => undefined,
    setSortBy: () => undefined,
    setVendorFilter: () => undefined,
    setGroupFilter: () => undefined,
    setQuotaTypeFilter: () => undefined,
    setEndpointTypeFilter: () => undefined,
    setTagFilter: () => undefined,
    setTokenUnit: () => undefined,
    setViewMode: () => undefined,
    setShowRechargePrice: () => undefined,
    filteredModels: models,
    hasActiveFilters: false,
    activeFilterCount: 0,
    availableTags: [],
    clearFilters: () => undefined,
    clearSearch: () => undefined,
  }),
}))

const { act } = await import('react')
const { createRoot } = await import('react-dom/client')
const { PublicStatusSummary } =
  await import('@/features/channel-status/components/public-status-summary')
const { Pricing } = await import('@/features/pricing')

const reactTestGlobals = globalThis as typeof globalThis & {
  IS_REACT_ACT_ENVIRONMENT?: boolean
}
reactTestGlobals.IS_REACT_ACT_ENVIRONMENT = true

let host: HTMLDivElement | null = null
let root: ReturnType<typeof createRoot> | null = null

beforeEach(() => {
  navigationCalls.length = 0
  authUser = null
  pricingLoading = false
  publicSummaryQuery = {
    data: undefined,
    isLoading: true,
    isError: false,
  }
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
  domWindow.close()
})

test('keeps a stable public status region while loading', async () => {
  await act(async () => root?.render(<PublicStatusSummary />))

  const loading = host?.querySelector('[data-slot="public-status-loading"]')
  expect(loading).not.toBeNull()
  expect(loading?.classList.contains('min-h-32')).toBe(true)
})

test('shows a compact empty state when no monitors are published', async () => {
  publicSummaryQuery = {
    data: { ...publicSummary, monitors: [] },
    isLoading: false,
    isError: false,
  }

  await act(async () => root?.render(<PublicStatusSummary />))

  expect(host?.textContent).toContain('No published channel monitors')
  expect(
    host?.querySelector('[data-slot="public-status-summary"]')
  ).not.toBeNull()
})

test('renders operational, degraded, failed, and unknown states with text and icons', async () => {
  publicSummaryQuery = {
    data: publicSummary,
    isLoading: false,
    isError: false,
  }

  await act(async () => root?.render(<PublicStatusSummary />))

  for (const label of ['Operational', 'Degraded', 'Failed', 'Unknown']) {
    const status = [...(host?.querySelectorAll('[role="status"]') ?? [])].find(
      (element) => element.textContent?.includes(label)
    )
    expect(status).toBeDefined()
    expect(status?.querySelector('svg')).not.toBeNull()
  }
  expect(host?.textContent).toContain('OpenAI global')
  expect(host?.textContent).toContain('321 ms')
  expect(host?.textContent).toContain('99.13%')
})

test('keeps model search usable when the summary request fails', async () => {
  publicSummaryQuery = {
    data: undefined,
    isLoading: false,
    isError: true,
  }

  await act(async () => root?.render(<Pricing />))

  expect(host?.textContent).toContain('Channel status unavailable')
  expect(
    host?.querySelector('input[aria-label="Search models"]')
  ).not.toBeNull()
})

test('loads public status independently while model pricing is loading', async () => {
  pricingLoading = true
  publicSummaryQuery = {
    data: publicSummary,
    isLoading: false,
    isError: false,
  }

  await act(async () => root?.render(<Pricing />))

  expect(
    host?.querySelector('[data-slot="public-status-summary"]')
  ).not.toBeNull()
  expect(host?.querySelector('[data-testid="pricing-loading"]')).not.toBeNull()
})

test('replaces the former model-square heading block above search', async () => {
  publicSummaryQuery = {
    data: publicSummary,
    isLoading: false,
    isError: false,
  }

  await act(async () => root?.render(<Pricing />))

  expect(host?.textContent).not.toContain('Model Square')
  expect(host?.textContent).not.toContain('This site currently has')
  expect(host?.textContent).not.toContain('Discover curated AI models')

  const summary = host?.querySelector('[data-slot="public-status-summary"]')
  const search = host?.querySelector('input[aria-label="Search models"]')
  expect(summary).not.toBeNull()
  expect(search).not.toBeNull()
  if (!summary || !search) throw new Error('Summary or search was not rendered')
  expect(
    Boolean(
      summary.compareDocumentPosition(search) & Node.DOCUMENT_POSITION_FOLLOWING
    )
  ).toBe(true)
})

test('routes guests through sign-in and signed-in users directly to details', async () => {
  publicSummaryQuery = {
    data: publicSummary,
    isLoading: false,
    isError: false,
  }

  await act(async () => root?.render(<PublicStatusSummary />))
  const guestButton = host?.querySelector<HTMLButtonElement>(
    'button[aria-label="View channel status details"]'
  )
  if (!guestButton) throw new Error('View details button was not rendered')
  await act(async () => guestButton.click())

  expect(navigationCalls).toEqual([
    {
      to: '/sign-in',
      search: { redirect: '/channel-status' },
    },
  ])

  navigationCalls.length = 0
  authUser = { id: 1, role: 1 }
  await act(async () => root?.render(<PublicStatusSummary />))
  const userButton = host?.querySelector<HTMLButtonElement>(
    'button[aria-label="View channel status details"]'
  )
  if (!userButton) throw new Error('Signed-in details button was not rendered')
  await act(async () => userButton.click())

  expect(navigationCalls).toEqual([{ to: '/channel-status' }])
})
