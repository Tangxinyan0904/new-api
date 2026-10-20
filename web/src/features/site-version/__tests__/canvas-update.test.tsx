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
import { QueryClientProvider, type QueryClient } from '@tanstack/react-query'
import {
  cleanup,
  render,
  screen,
  waitFor,
  within,
} from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import type { ReactNode } from 'react'
import { afterEach, beforeEach, describe, expect, test, vi } from 'vitest'

import { api } from '@/lib/api'
import { createAppQueryClient } from '@/lib/query-client'
import { ROLE } from '@/lib/roles'
import { STATUS_QUERY_KEY } from '@/lib/status-query'
import { useAuthStore } from '@/stores/auth-store'

import { CanvasUpdateAction } from '../canvas-update-action'
import { SystemVersionLabel } from '../system-version-label'
import type { CanvasStatus } from '../types'

let client: QueryClient

function Wrapper(props: { children: ReactNode }) {
  return (
    <QueryClientProvider client={client}>{props.children}</QueryClientProvider>
  )
}

function canvasStatus(overrides: Partial<CanvasStatus> = {}): CanvasStatus {
  return {
    configured: true,
    container: 'infinite-canvas',
    current_version: 'v0.15.1',
    update_available: true,
    checked_at: 1_760_000_000,
    check_error: null,
    release: {
      version: 'v0.20.0',
      published_at: '2026-10-10T08:47:20Z',
      url: 'https://github.com/basketikun/infinite-canvas/releases/tag/v0.20.0',
      notes: 'Canvas release notes.',
    },
    release_error: null,
    task: null,
    ...overrides,
  }
}

/** Answers the canvas status requests in order, then repeats the last one. */
function serveCanvasStatus(...statuses: CanvasStatus[]) {
  const get = vi.spyOn(api, 'get')
  for (const status of statuses) {
    get.mockResolvedValueOnce({ data: { success: true, data: status } })
  }
  get.mockResolvedValue({ data: { success: true, data: statuses.at(-1) } })
  return get
}

function signIn(role: number) {
  useAuthStore.getState().auth.setUser({ id: 1, username: 'root', role })
}

beforeEach(() => {
  client = createAppQueryClient()
  client.setQueryData(STATUS_QUERY_KEY, { version: '2026.10.20-26844859' })
})

afterEach(() => {
  cleanup()
  client.clear()
  useAuthStore.getState().auth.reset()
  vi.restoreAllMocks()
})

describe('canvas update entry', () => {
  test('stays hidden from admins who are not root and asks the server nothing', () => {
    signIn(ROLE.ADMIN)
    const get = serveCanvasStatus(canvasStatus())
    render(<CanvasUpdateAction />, { wrapper: Wrapper })
    expect(screen.queryByRole('button')).not.toBeInTheDocument()
    expect(get).not.toHaveBeenCalled()
  })

  test('shows the running version and a pending update next to the site name', async () => {
    signIn(ROLE.SUPER_ADMIN)
    serveCanvasStatus(canvasStatus())
    render(<CanvasUpdateAction presentation='version' />, { wrapper: Wrapper })

    const button = await screen.findByRole('button', {
      name: 'Canvas v0.15.1 Update available',
    })
    expect(button).toHaveAttribute(
      'title',
      expect.stringContaining('Infinite canvas, current version: v0.15.1')
    )
    expect(within(button).getByText('Canvas v0.15.1')).toHaveClass(
      'hidden',
      '@min-[22rem]/system-brand:inline'
    )
    expect(within(button).getByText('Update available')).toBeInTheDocument()
  })

  test('explains the setup in settings and shows nothing next to the site name until the updater is configured', async () => {
    signIn(ROLE.SUPER_ADMIN)
    const get = serveCanvasStatus(canvasStatus({ configured: false }))
    render(
      <>
        <CanvasUpdateAction presentation='version' />
        <CanvasUpdateAction presentation='action' />
      </>,
      { wrapper: Wrapper }
    )
    expect(
      await screen.findByText(/Set CANVAS_UPDATER_URL and CANVAS_UPDATER_TOKEN/)
    ).toBeInTheDocument()
    expect(screen.queryByRole('button')).not.toBeInTheDocument()
    expect(get).toHaveBeenCalledTimes(1)
  })

  test('queues the update after confirmation and follows it while it runs', async () => {
    const user = userEvent.setup()
    signIn(ROLE.SUPER_ADMIN)
    const running = canvasStatus({
      current_version: '',
      update_available: false,
      task: {
        task_id: 'task-1',
        status: 'running',
        result: null,
        error: '',
        created_at: 1_760_000_100,
        updated_at: 1_760_000_100,
      },
    })
    serveCanvasStatus(canvasStatus(), running)
    const post = vi
      .spyOn(api, 'post')
      .mockResolvedValue({ data: { success: true, data: running.task } })
    render(<CanvasUpdateAction presentation='action' />, { wrapper: Wrapper })

    await user.click(
      await screen.findByRole('button', { name: /^Canvas v0\.15\.1/ })
    )
    const dialog = await screen.findByRole('dialog', {
      name: 'Infinite canvas updates',
    })
    expect(within(dialog).getByText('v0.20.0')).toBeInTheDocument()
    expect(
      within(dialog).getByText('A new canvas version is available.')
    ).toBeInTheDocument()
    expect(
      await within(dialog).findByText('Canvas release notes.')
    ).toBeInTheDocument()

    await user.click(within(dialog).getByRole('button', { name: 'Update now' }))
    const confirm = await screen.findByRole('alertdialog', {
      name: 'Update the infinite canvas?',
    })
    await user.click(
      within(confirm).getByRole('button', { name: 'Update now' })
    )

    await waitFor(() =>
      expect(post).toHaveBeenCalledWith('/api/canvas/update', null, {
        skipBusinessError: true,
        skipErrorHandler: true,
      })
    )
    expect(
      await within(dialog).findByText(
        'Updating the canvas. This usually takes a minute or two.'
      )
    ).toBeInTheDocument()
    expect(
      within(dialog).getByRole('button', { name: 'Updating...' })
    ).toBeDisabled()
    // The dialog hides its trigger from the accessibility tree while open.
    expect(screen.getByText('Canvas Updating...')).toBeInTheDocument()
  })

  test('follows the update another root started when the server answers 409', async () => {
    const user = userEvent.setup()
    signIn(ROLE.SUPER_ADMIN)
    const runningTask = {
      task_id: 'task-2',
      status: 'running' as const,
      result: null,
      error: '',
      created_at: 1_760_000_100,
      updated_at: 1_760_000_100,
    }
    serveCanvasStatus(
      canvasStatus(),
      canvasStatus({
        current_version: '',
        update_available: false,
        task: runningTask,
      })
    )
    vi.spyOn(api, 'post').mockRejectedValue({
      isAxiosError: true,
      response: {
        status: 409,
        data: {
          success: false,
          message: 'A canvas update is already running',
          message_key: 'A canvas update is already running',
          data: runningTask,
        },
      },
    })
    render(<CanvasUpdateAction presentation='action' />, { wrapper: Wrapper })

    await user.click(
      await screen.findByRole('button', { name: /^Canvas v0\.15\.1/ })
    )
    const dialog = await screen.findByRole('dialog', {
      name: 'Infinite canvas updates',
    })
    await user.click(within(dialog).getByRole('button', { name: 'Update now' }))
    const confirm = await screen.findByRole('alertdialog')
    await user.click(
      within(confirm).getByRole('button', { name: 'Update now' })
    )

    await waitFor(() =>
      expect(screen.queryByRole('alertdialog')).not.toBeInTheDocument()
    )
    expect(
      await within(dialog).findByText(
        'Updating the canvas. This usually takes a minute or two.'
      )
    ).toBeInTheDocument()
  })

  test('explains a failed update with the server message', async () => {
    const user = userEvent.setup()
    signIn(ROLE.SUPER_ADMIN)
    serveCanvasStatus(
      canvasStatus({
        task: {
          task_id: 'task-1',
          status: 'failed',
          result: {
            from_version: 'v0.15.1',
            to_version: '',
            updated: 0,
            error: {
              key: 'The canvas updater could not update the container. Check its logs.',
            },
          },
          error: 'raw',
          created_at: 1_760_000_100,
          updated_at: 1_760_000_200,
        },
      })
    )
    render(<CanvasUpdateAction presentation='action' />, { wrapper: Wrapper })
    await user.click(
      await screen.findByRole('button', { name: /^Canvas v0\.15\.1/ })
    )
    const dialog = await screen.findByRole('dialog')
    expect(
      within(dialog).getByText(
        /Last update failed .*: The canvas updater could not update the container\. Check its logs\./
      )
    ).toBeInTheDocument()
  })
})

describe('site version label', () => {
  test.each([
    ['2026.10.20-26844859', '2026.10.20-26844859'],
    ['', 'Unknown version'],
    ['v0.0.0', 'Unknown version'],
  ])('shows the build %j to admins as %j', (version, shown) => {
    signIn(ROLE.ADMIN)
    client.setQueryData(STATUS_QUERY_KEY, { version })
    render(<SystemVersionLabel />, { wrapper: Wrapper })
    expect(screen.getByText(shown)).toHaveClass('font-mono')
  })

  test('stays hidden from users', () => {
    signIn(ROLE.USER)
    render(<SystemVersionLabel />, { wrapper: Wrapper })
    expect(screen.queryByText('2026.10.20-26844859')).not.toBeInTheDocument()
  })
})
