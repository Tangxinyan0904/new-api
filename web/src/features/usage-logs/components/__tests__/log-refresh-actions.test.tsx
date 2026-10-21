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
import {
  QueryClient,
  QueryClientProvider,
  useQuery,
} from '@tanstack/react-query'
import {
  act,
  cleanup,
  fireEvent,
  render,
  screen,
  waitFor,
} from '@testing-library/react'
import { afterEach, describe, expect, test, vi } from 'vitest'

import { AUTO_REFRESH_SECONDS } from '../../lib/auto-refresh'
import type { LogCategory } from '../../types'
import { LogRefreshActions } from '../log-refresh-actions'
import type { LogsViewAccess } from '../usage-logs-provider'

/**
 * Registers the list and stats queries with the key shapes the usage log
 * table (usage-logs-table.tsx) and the stats bar (common-logs-stats.tsx) use.
 */
function LogQueries(props: {
  logCategory: LogCategory
  viewAccess: LogsViewAccess
  fetchList: () => Promise<string>
  fetchStats: () => Promise<string>
}) {
  useQuery({
    queryKey: [
      'logs',
      props.logCategory,
      props.viewAccess,
      1,
      20,
      [],
      { page: 1 },
    ],
    queryFn: props.fetchList,
  })
  useQuery({
    queryKey: ['usage-logs-stats', props.viewAccess !== 'self', { page: 1 }],
    queryFn: props.fetchStats,
    enabled: props.logCategory === 'common',
  })
  return null
}

function renderRefreshActions(
  logCategory: LogCategory,
  viewAccess: LogsViewAccess
) {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false, staleTime: Infinity } },
  })
  const fetchList = vi.fn(async () => 'list')
  const fetchStats = vi.fn(async () => 'stats')
  render(
    <QueryClientProvider client={client}>
      <LogQueries
        logCategory={logCategory}
        viewAccess={viewAccess}
        fetchList={fetchList}
        fetchStats={fetchStats}
      />
      <LogRefreshActions
        logCategory={logCategory}
        viewAccess={viewAccess}
        scopeKey='{"page":1}'
      />
    </QueryClientProvider>
  )
  return { client, fetchList, fetchStats }
}

/** The first load has settled once Refresh is enabled again. */
async function waitForFirstLoad() {
  await waitFor(() =>
    expect(screen.getByRole('button', { name: 'Refresh' })).toBeEnabled()
  )
}

afterEach(() => {
  cleanup()
  vi.useRealTimers()
})

describe('usage log refresh actions', () => {
  test.each(['self', 'admin', 'root'] as const)(
    'Refresh reloads the visible log list and stats for the %s view',
    async (viewAccess) => {
      const { fetchList, fetchStats } = renderRefreshActions(
        'common',
        viewAccess
      )
      await waitForFirstLoad()
      expect(fetchList).toHaveBeenCalledTimes(1)
      expect(fetchStats).toHaveBeenCalledTimes(1)

      fireEvent.click(screen.getByRole('button', { name: 'Refresh' }))

      await waitFor(() => expect(fetchList).toHaveBeenCalledTimes(2))
      expect(fetchStats).toHaveBeenCalledTimes(2)
    }
  )

  test('Refresh reloads a task log list', async () => {
    const { fetchList } = renderRefreshActions('task', 'root')
    await waitForFirstLoad()
    expect(fetchList).toHaveBeenCalledTimes(1)

    fireEvent.click(screen.getByRole('button', { name: 'Refresh' }))

    await waitFor(() => expect(fetchList).toHaveBeenCalledTimes(2))
  })

  test('Auto refresh reloads the log list when the countdown ends', async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true })
    const { fetchList } = renderRefreshActions('common', 'root')
    await waitForFirstLoad()
    expect(fetchList).toHaveBeenCalledTimes(1)

    fireEvent.click(screen.getByRole('button', { name: 'Auto refresh' }))
    expect(
      screen.getByRole('button', { name: 'Cancel auto refresh (30s)' })
    ).toHaveTextContent('30s')
    await act(async () => {
      await vi.advanceTimersByTimeAsync(AUTO_REFRESH_SECONDS * 1000)
    })

    await waitFor(() => expect(fetchList).toHaveBeenCalledTimes(2))
  })

  test('Refresh is disabled and spins while the log list is loading', async () => {
    const { client, fetchList } = renderRefreshActions('task', 'admin')
    await waitForFirstLoad()
    let finishLoad = () => {}
    fetchList.mockImplementationOnce(
      () =>
        new Promise<string>((resolve) => {
          finishLoad = () => resolve('list')
        })
    )

    // A search or a page change reloads the list.
    void client.invalidateQueries({ queryKey: ['logs'] })

    await waitFor(() =>
      expect(screen.getByRole('button', { name: 'Refresh' })).toBeDisabled()
    )
    act(() => finishLoad())
    await waitForFirstLoad()
  })
})
