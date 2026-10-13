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
import { useState } from 'react'
import { afterEach, describe, expect, it, vi } from 'vitest'

import { api } from '@/lib/api'

import type { TicketLog } from '../../types'
import { TicketLogList } from '../ticket-log-list'
import { TicketLogPicker } from '../ticket-log-picker'

function log(id: number, overrides: Partial<TicketLog> = {}): TicketLog {
  return {
    log_id: id,
    created_at: 1_700_000_000 + id,
    type: 2,
    model_name: `model-${id}`,
    token_name: 'main',
    group: 'default',
    quota: 1000,
    prompt_tokens: 30,
    completion_tokens: 12,
    use_time: 3,
    is_stream: false,
    request_id: `req-${id}`,
    content: '',
    ...overrides,
  }
}

/** Seven candidate logs, newest first, served five per page. */
const candidates = [7, 6, 5, 4, 3, 2, 1].map((id) => log(id))

function mockCandidateApi() {
  return vi.spyOn(api, 'get').mockImplementation(async (url: string) => {
    const params = new URLSearchParams(url.split('?')[1])
    const page = Number(params.get('p'))
    const size = Number(params.get('page_size'))
    return {
      data: {
        success: true,
        data: {
          items: candidates.slice((page - 1) * size, page * size),
          total: candidates.length,
        },
      },
    }
  })
}

function Picker(props: {
  initial?: number[]
  onChange?: (ids: number[]) => void
}) {
  const [value, setValue] = useState<number[]>(props.initial ?? [])
  return (
    <TicketLogPicker
      value={value}
      onChange={(ids) => {
        setValue(ids)
        props.onChange?.(ids)
      }}
    />
  )
}

function renderPicker(props: Parameters<typeof Picker>[0] = {}) {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  render(
    <QueryClientProvider client={client}>
      <Picker {...props} />
    </QueryClientProvider>
  )
}

afterEach(() => {
  cleanup()
  vi.restoreAllMocks()
})

describe('TicketLogPicker', () => {
  it('selects and unselects a log by ticking it', async () => {
    mockCandidateApi()
    const onChange = vi.fn()
    const user = userEvent.setup()
    renderPicker({ onChange })

    await user.click(await screen.findByText('model-7'))
    expect(onChange).toHaveBeenLastCalledWith([7])
    expect(screen.getByText('1 of 5 selected')).toBeInTheDocument()

    await user.click(screen.getByText('model-7'))
    expect(onChange).toHaveBeenLastCalledWith([])
  })

  it('keeps selections across pages and stops at five', async () => {
    mockCandidateApi()
    const user = userEvent.setup()
    renderPicker({ initial: [1, 2, 3, 4] })

    await user.click(await screen.findByText('model-7'))
    expect(screen.getByText('5 of 5 selected')).toBeInTheDocument()
    const unchecked = screen
      .getAllByRole('checkbox')
      .filter((box) => box.getAttribute('aria-checked') === 'false')
    expect(unchecked).not.toHaveLength(0)
    for (const box of unchecked) {
      expect(box).toHaveAttribute('aria-disabled', 'true')
    }

    await user.click(screen.getByRole('button', { name: 'Next' }))
    await waitFor(() => expect(screen.getByText('model-2')).toBeInTheDocument())
    expect(screen.getByText('2 / 2')).toBeInTheDocument()
    const checked = screen
      .getAllByRole('checkbox')
      .filter((box) => box.getAttribute('aria-checked') === 'true')
    expect(checked).toHaveLength(2)
  })

  it('filters the logs by model name', async () => {
    const get = mockCandidateApi()
    const user = userEvent.setup()
    renderPicker()

    await screen.findByText('model-7')
    await user.type(
      screen.getByRole('textbox', { name: 'Filter by model' }),
      'gpt-4o'
    )

    await waitFor(() =>
      expect(get).toHaveBeenLastCalledWith(
        expect.stringContaining('model_name=gpt-4o')
      )
    )
  })
})

describe('TicketLogList', () => {
  it('shows a failed call with its error and request ID, and the channel only to staff', () => {
    const failed = log(9, {
      type: 5,
      content: 'upstream timeout',
      channel_id: 12,
    })
    const { rerender } = render(
      <TicketLogList logs={[failed]} audience='user' />
    )

    expect(screen.getByText('Failed')).toBeInTheDocument()
    expect(screen.getByText('upstream timeout')).toBeInTheDocument()
    expect(screen.getByText('req-9')).toBeInTheDocument()
    expect(
      screen.getByRole('button', { name: 'Copy request ID' })
    ).toBeInTheDocument()
    expect(screen.queryByText(/#12/)).not.toBeInTheDocument()

    rerender(<TicketLogList logs={[failed]} audience='staff' />)
    expect(screen.getByText(/#12/)).toBeInTheDocument()
  })

  it('renders nothing when no logs were attached', () => {
    const { container } = render(<TicketLogList logs={[]} audience='staff' />)
    expect(container).toBeEmptyDOMElement()
  })
})
