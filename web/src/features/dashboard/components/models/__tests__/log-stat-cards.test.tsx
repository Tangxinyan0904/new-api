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
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { cleanup, render, screen } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { api } from '@/lib/api'
import { useAuthStore } from '@/stores/auth-store'

import { LogStatCards } from '../log-stat-cards'

const ACTIVITY: Record<string, { concurrency: number; rpm: number }> = {
  '/api/data/activity': { concurrency: 30, rpm: 420 },
  '/api/data/self/activity': { concurrency: 1, rpm: 5 },
}

function renderAs(role: number) {
  useAuthStore.getState().auth.setUser({
    id: 1,
    username: 'dashboard-user',
    role,
    quota: 0,
    used_quota: 0,
    request_count: 0,
  })
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  render(
    <QueryClientProvider client={client}>
      <LogStatCards />
    </QueryClientProvider>
  )
}

beforeEach(() => {
  vi.spyOn(api, 'get').mockImplementation(async (url: string) => {
    if (url in ACTIVITY) {
      return { data: { success: true, data: ACTIVITY[url] } }
    }
    return { data: { success: true, data: [] } }
  })
})

afterEach(() => {
  cleanup()
  vi.restoreAllMocks()
})

describe('LogStatCards concurrency and RPM', () => {
  it("shows the whole site's live figures to admins", async () => {
    renderAs(10)

    expect(await screen.findByText('30 / 420')).toBeInTheDocument()
    expect(
      screen.getByText('Whole site, live, refreshed every minute')
    ).toBeInTheDocument()
    expect(api.get).toHaveBeenCalledWith('/api/data/activity')
  })

  it("shows a user only their own account's live figures", async () => {
    renderAs(1)

    expect(await screen.findByText('1 / 5')).toBeInTheDocument()
    expect(
      screen.getByText('Your account, live, refreshed every minute')
    ).toBeInTheDocument()
    expect(api.get).not.toHaveBeenCalledWith('/api/data/activity')
  })
})
