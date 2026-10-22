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
import { render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, expect, it, vi } from 'vitest'

import { api } from '@/lib/api'
import { ROLE } from '@/lib/roles'
import { useAuthStore } from '@/stores/auth-store'

import type { TopupRecord } from '../../types'
import { BillingHistoryDialog } from '../dialogs/billing-history-dialog'

const originalAuth = useAuthStore.getState().auth

function record(overrides: Partial<TopupRecord>): TopupRecord {
  return {
    id: 1,
    user_id: 7,
    username: 'alice',
    amount: 10,
    money: 73,
    trade_no: 'USR7NOabc',
    payment_method: 'alipay',
    create_time: 1_790_000_000,
    status: 'success',
    ...overrides,
  }
}

function mockList(items: TopupRecord[], total = items.length) {
  return vi.spyOn(api, 'get').mockResolvedValue({
    data: { success: true, data: { items, total } },
  })
}

function signInAs(role: number) {
  useAuthStore.setState({
    auth: { ...originalAuth, user: { id: 1, username: 'root', role } },
  })
}

function requestedParams(get: ReturnType<typeof mockList>) {
  const url = String(get.mock.calls.at(-1)?.[0])
  return new URLSearchParams(url.slice(url.indexOf('?') + 1))
}

beforeEach(() => {
  signInAs(ROLE.ADMIN)
})

afterEach(() => {
  vi.restoreAllMocks()
  useAuthStore.setState({ auth: originalAuth })
})

it('sends the user and status filters and goes back to the first page', async () => {
  const user = userEvent.setup()
  const get = mockList([record({})], 45)
  render(<BillingHistoryDialog open onOpenChange={vi.fn()} />)
  await screen.findByText('USR7NOabc')
  await user.click(screen.getByRole('button', { name: 'Next' }))
  await waitFor(() => expect(requestedParams(get).get('p')).toBe('2'))

  await user.type(
    screen.getByRole('textbox', { name: 'Search by username or user ID' }),
    'alice'
  )
  await waitFor(() => expect(requestedParams(get).get('user')).toBe('alice'))
  expect(requestedParams(get).get('p')).toBe('1')

  await user.click(screen.getByRole('combobox', { name: 'Status' }))
  await user.click(await screen.findByRole('option', { name: 'Pending' }))
  await waitFor(() =>
    expect(requestedParams(get).get('status')).toBe('pending')
  )
  expect(requestedParams(get).get('user')).toBe('alice')
  expect(String(get.mock.calls.at(-1)?.[0])).toMatch(/^\/api\/user\/topup\?/)
})

it('filters to a user when their name is clicked', async () => {
  const user = userEvent.setup()
  const get = mockList([record({})])
  render(<BillingHistoryDialog open onOpenChange={vi.fn()} />)

  await user.click(await screen.findByRole('button', { name: /alice/ }))

  await waitFor(() => expect(requestedParams(get).get('user')).toBe('7'))
  expect(
    screen.getByRole('textbox', { name: 'Search by username or user ID' })
  ).toHaveValue('7')
})

it('clears every filter at once', async () => {
  const user = userEvent.setup()
  const get = mockList([record({})])
  render(<BillingHistoryDialog open onOpenChange={vi.fn()} />)
  await screen.findByText('USR7NOabc')
  await user.type(
    screen.getByRole('textbox', { name: 'Search by username or user ID' }),
    'alice'
  )
  await waitFor(() => expect(requestedParams(get).get('user')).toBe('alice'))

  await user.click(screen.getByRole('button', { name: 'Clear filters' }))

  await waitFor(() => expect(requestedParams(get).has('user')).toBe(false))
  expect(
    screen.queryByRole('button', { name: 'Clear filters' })
  ).not.toBeInTheDocument()
})

it('shows a failed order as Failed rather than Pending', async () => {
  mockList([record({ status: 'failed' })])
  render(<BillingHistoryDialog open onOpenChange={vi.fn()} />)

  const row = (await screen.findByText('USR7NOabc')).closest('tr')
  expect(row).not.toBeNull()
  expect(within(row as HTMLElement).getByText('Failed')).toBeVisible()
  expect(within(row as HTMLElement).queryByText('Pending')).toBeNull()
})

it('shows the order before an admin completes it', async () => {
  const user = userEvent.setup()
  mockList([record({ status: 'pending' })])
  const post = vi
    .spyOn(api, 'post')
    .mockResolvedValue({ data: { success: true } })
  render(<BillingHistoryDialog open onOpenChange={vi.fn()} />)

  await user.click(
    await screen.findByRole('button', { name: 'Complete Order' })
  )
  const confirm = await screen.findByRole('alertdialog')
  expect(within(confirm).getByText('USR7NOabc')).toBeVisible()
  expect(within(confirm).getByText('alice #7')).toBeVisible()
  await user.click(within(confirm).getByRole('button', { name: 'Confirm' }))

  await waitFor(() =>
    expect(post).toHaveBeenCalledWith('/api/user/topup/complete', {
      trade_no: 'USR7NOabc',
    })
  )
})

it('keeps the admin filters away from ordinary users', async () => {
  signInAs(ROLE.USER)
  const get = mockList([record({ username: undefined })])
  render(<BillingHistoryDialog open onOpenChange={vi.fn()} />)
  await screen.findByText('USR7NOabc')

  expect(
    screen.queryByRole('textbox', { name: 'Search by username or user ID' })
  ).toBeNull()
  expect(screen.queryByRole('combobox', { name: 'Status' })).toBeNull()
  expect(screen.queryByRole('button', { name: 'Complete Order' })).toBeNull()
  expect(String(get.mock.calls.at(-1)?.[0])).toMatch(
    /^\/api\/user\/topup\/self\?/
  )
})
