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
import { afterEach, expect, test } from 'vitest'

import { useSidebarData } from '@/hooks/use-sidebar-data'

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

afterEach(() => {
  cleanup()
})

test('places channel status directly after dashboard for every signed-in user', async () => {
  // The sidebar reads ticket badge counts through React Query.
  render(
    <QueryClientProvider client={new QueryClient()}>
      <SidebarHarness />
    </QueryClientProvider>
  )

  expect(screen.getByTestId('general-navigation').textContent).toContain(
    'Dashboard:/dashboard/models|Channel Status:/channel-status|API Keys:/keys'
  )

  const { Route } = await import('@/routes/_authenticated/channel-status')
  expect((Route as { beforeLoad?: unknown }).beforeLoad).toBeUndefined()
})
