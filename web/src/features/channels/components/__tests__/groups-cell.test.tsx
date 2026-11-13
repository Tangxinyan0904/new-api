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
import { cleanup, render, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { createInstance } from 'i18next'
import { I18nextProvider } from 'react-i18next'
import { afterEach, expect, test, vi } from 'vitest'

import { api } from '@/lib/api'

import { ChannelGroupsCell } from '../channel-groups-cell'

const i18n = createInstance()
await i18n.init({
  lng: 'en',
  resources: { en: { translation: {} } },
  interpolation: { escapeValue: false },
  initAsync: false,
})
const clients: QueryClient[] = []

afterEach(() => {
  cleanup()
  clients.splice(0).forEach((client) => client.clear())
  vi.restoreAllMocks()
})

function renderGroupsCell(groups: string[], mask?: string) {
  vi.spyOn(api, 'get').mockResolvedValue({
    data: {
      success: true,
      data: {
        group_ratio: { standard: 1, cheap: 0.5 },
        group_group_ratio: {
          svip: { cheap: 0.3 },
          vip: { cheap: 0.4, standard: 0.9 },
        },
      },
    },
  })
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  clients.push(client)
  return render(
    <QueryClientProvider client={client}>
      <I18nextProvider i18n={i18n}>
        <ChannelGroupsCell groups={groups} mask={mask} />
      </I18nextProvider>
    </QueryClientProvider>
  )
}

/** The text of each line of the hover list, group lines and special ratios alike. */
async function hoverListLines(): Promise<string[]> {
  const list = await screen.findByRole('list')
  await within(list).findAllByText(/\dx$/)
  return within(list)
    .getAllByRole('listitem')
    .flatMap((item) => [
      item.firstElementChild,
      ...item.querySelectorAll('[data-slot="special-group-ratio"]'),
    ])
    .map((line) => line?.textContent ?? '')
}

test('a channel in several groups shows the first one and how many more there are', () => {
  const view = renderGroupsCell(['cheap', 'standard', 'unpriced'])

  expect(screen.getByText('cheap')).toBeVisible()
  expect(screen.queryByText('standard')).not.toBeInTheDocument()
  expect(
    view.container.querySelector('[data-slot="more-groups"]')
  ).toHaveTextContent('+2')
})

test('hovering lists every group with its ratio and the special ratios of user groups', async () => {
  const view = renderGroupsCell(['cheap', 'standard', 'unpriced'])
  const user = userEvent.setup()

  await user.hover(
    view.container.querySelector('[data-slot="more-groups"]') as Element
  )

  expect(await hoverListLines()).toEqual([
    'cheap0.5x',
    'svip users0.3x',
    'vip users0.4x',
    'standard1x',
    'vip users0.9x',
    'unpriced',
  ])
})

test('a channel in one group has no mark for more, and hovering shows its ratio', async () => {
  const view = renderGroupsCell(['standard'])
  const user = userEvent.setup()

  expect(
    view.container.querySelector('[data-slot="more-groups"]')
  ).not.toBeInTheDocument()

  await user.hover(screen.getByText('standard'))

  expect(await hoverListLines()).toEqual(['standard1x', 'vip users0.9x'])
})

test('the list opens from the keyboard', async () => {
  renderGroupsCell(['cheap', 'standard'])
  const user = userEvent.setup()

  await user.tab()

  expect(await screen.findByRole('list')).toBeVisible()
})

test('hidden sensitive data masks every group name but keeps the ratios', async () => {
  const view = renderGroupsCell(['cheap', 'standard'], '••••')
  const user = userEvent.setup()

  expect(screen.queryByText('cheap')).not.toBeInTheDocument()
  await user.hover(
    view.container.querySelector('[data-slot="more-groups"]') as Element
  )

  expect(await hoverListLines()).toEqual([
    '••••0.5x',
    '•••• users0.3x',
    '•••• users0.4x',
    '••••1x',
    '•••• users0.9x',
  ])
})

test('a channel without groups shows a dash', () => {
  renderGroupsCell([])

  expect(screen.getByText('-')).toBeVisible()
  expect(screen.queryByRole('list')).not.toBeInTheDocument()
})
