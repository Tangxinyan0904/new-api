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
import { cleanup, render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, describe, expect, it, vi } from 'vitest'

import { api } from '@/lib/api'

import type { ChannelStatusItem } from '../../types'
import { MonitorFormDialog } from '../monitor-form-dialog'

const existing: ChannelStatusItem = {
  id: 17,
  public_name: 'VIP lane',
  name_color: '',
  group: 'vip',
  public_enabled: true,
  status: 'operational',
  success_rate: 100,
  success_count: 1,
  failure_count: 0,
  average_duration_ms: 1000,
  timeline: [],
}

function renderDialog(monitor: ChannelStatusItem | null) {
  vi.spyOn(api, 'get').mockResolvedValue({
    data: { success: true, data: ['default', 'vip'] },
  })
  const save = vi.fn().mockResolvedValue({ data: { success: true } })
  vi.spyOn(api, 'post').mockImplementation(save)
  vi.spyOn(api, 'put').mockImplementation(save)
  const onOpenChange = vi.fn()
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  })
  render(
    <QueryClientProvider client={client}>
      <MonitorFormDialog open monitor={monitor} onOpenChange={onOpenChange} />
    </QueryClientProvider>
  )
  return { save, onOpenChange }
}

afterEach(() => {
  cleanup()
  vi.restoreAllMocks()
})

describe('MonitorFormDialog', () => {
  it('fills the display name from the chosen group and creates the monitor', async () => {
    const user = userEvent.setup()
    const { save, onOpenChange } = renderDialog(null)

    await user.click(screen.getByRole('combobox'))
    await user.click(await screen.findByRole('option', { name: 'default' }))
    expect(screen.getByLabelText('Display name')).toHaveValue('default')

    await user.click(screen.getByRole('button', { name: 'Save' }))

    await waitFor(() => expect(onOpenChange).toHaveBeenCalledWith(false))
    expect(save).toHaveBeenCalledWith(
      '/api/channel-monitor/',
      {
        group: 'default',
        public_name: 'default',
        name_color: '',
        public_enabled: true,
      },
      expect.anything()
    )
  })

  it('updates an existing monitor in place', async () => {
    const user = userEvent.setup()
    const { save } = renderDialog(existing)

    const name = screen.getByLabelText('Display name')
    await user.clear(name)
    await user.type(name, 'Premium')
    await user.click(screen.getByRole('switch'))
    await user.click(screen.getByRole('button', { name: 'Save' }))

    await waitFor(() =>
      expect(save).toHaveBeenCalledWith(
        '/api/channel-monitor/17',
        {
          group: 'vip',
          public_name: 'Premium',
          name_color: '',
          public_enabled: false,
        },
        expect.anything()
      )
    )
  })

  it('rejects a malformed name color without saving', async () => {
    const user = userEvent.setup()
    const { save } = renderDialog(existing)

    await user.type(screen.getByPlaceholderText('#RRGGBB'), 'red')
    await user.click(screen.getByRole('button', { name: 'Save' }))

    expect(
      await screen.findByText('Name color must use #RRGGBB format')
    ).toBeInTheDocument()
    expect(save).not.toHaveBeenCalled()
  })
})
