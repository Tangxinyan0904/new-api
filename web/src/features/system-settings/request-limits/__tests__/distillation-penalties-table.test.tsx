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
import { describe, expect, mock, test } from 'bun:test'

import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { renderToStaticMarkup } from 'react-dom/server'

import { formatTimestampToDate } from '@/lib/format'

import type { PageData, DistillationPenalty } from '../types'

mock.module('react-i18next', () => ({
  useTranslation: () => ({
    t: (key: string, values?: Record<string, string | number>) =>
      key.replaceAll(/\{\{(\w+)\}\}/g, (_match, name: string) =>
        String(values?.[name] ?? '')
      ),
  }),
}))

const { DistillationPenaltiesTable } =
  await import('../distillation-penalties-table')
const { getDistillationPenaltiesQueryKey, getDistillationPenaltyPhaseConfig } =
  await import('../distillation-penalties')

const penalties: PageData<DistillationPenalty> = {
  page: 1,
  page_size: 10,
  total: 12,
  items: [
    {
      user_id: 41,
      username: 'alice',
      phase: 'temporary',
      first_triggered_at: 1_750_000_000,
      temporary_limited_until: 1_750_001_800,
      observation_until: 1_750_088_200,
      permanently_banned_at: 0,
      created_at: 1_750_000_000,
      updated_at: 1_750_000_100,
    },
    {
      user_id: 42,
      username: 'bob',
      phase: 'permanent',
      first_triggered_at: 1_750_000_200,
      temporary_limited_until: 1_750_002_000,
      observation_until: 1_750_088_400,
      permanently_banned_at: 1_750_003_000,
      created_at: 1_750_000_200,
      updated_at: 1_750_003_000,
    },
  ],
}

function renderPenaltiesTable(data: PageData<DistillationPenalty>): string {
  const queryClient = new QueryClient()
  queryClient.setQueryData(getDistillationPenaltiesQueryKey(1, 10, ''), data)

  return renderToStaticMarkup(
    <QueryClientProvider client={queryClient}>
      <DistillationPenaltiesTable />
    </QueryClientProvider>
  )
}

describe('distillation penalty display helpers', () => {
  test('uses a destructive badge for permanent bans', () => {
    expect(getDistillationPenaltyPhaseConfig('permanent')).toEqual({
      labelKey: 'Permanent ban',
      variant: 'destructive',
    })
  })

  test('builds the required paginated search query key', () => {
    expect(getDistillationPenaltiesQueryKey(2, 20, 'alice')).toEqual([
      'distillation-penalties',
      2,
      20,
      'alice',
    ])
  })
})

describe('DistillationPenaltiesTable', () => {
  test('renders search, refresh, phases, timestamps, actions, and pagination', () => {
    const html = renderPenaltiesTable(penalties)

    expect(html).toContain('Search by username or user ID')
    expect(html).toContain('Refresh')
    expect(html).toContain('alice')
    expect(html).toContain('User ID 41')
    expect(html).toContain('Temporary limit')
    expect(html).toContain('Permanent ban')
    expect(html).toContain(formatTimestampToDate(1_750_000_000))
    expect(html).toContain(formatTimestampToDate(1_750_003_000))
    expect(html).toContain('Clear penalty')
    expect(html).toContain('Page 1 of 2')
    expect(html).toContain('md:hidden')
    expect(html).toContain('hidden md:block')
  })

  test('renders a clear empty state when no penalties are active', () => {
    const html = renderPenaltiesTable({
      page: 1,
      page_size: 10,
      total: 0,
      items: [],
    })

    expect(html).toContain('No active distillation penalties')
    expect(html).toContain(
      'Temporary limits, observation periods, and permanent bans will appear here.'
    )
  })
})
