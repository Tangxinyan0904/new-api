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
import { cleanup, render, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, describe, expect, it } from 'vitest'

import type { QuotaDataItem } from '../../../types'
import { ModelTokenUsageCard } from '../model-token-usage'

/**
 * Seven models; model-1 used the most tokens and model-7 the fewest. A
 * quarter of each model's input was cached, except model-7, which was only
 * used before input tokens were recorded.
 */
const data: QuotaDataItem[] = Array.from({ length: 7 }, (_, index) => ({
  created_at: 0,
  model_name: `model-${index + 1}`,
  token_used: (7 - index) * 1000,
  cache_tokens: index === 6 ? 0 : (7 - index) * 100,
  input_tokens: index === 6 ? 0 : (7 - index) * 400,
}))

afterEach(() => {
  cleanup()
})

describe('ModelTokenUsageCard', () => {
  it('shows the tokens and cached tokens of the five busiest models', () => {
    render(<ModelTokenUsageCard data={data} loading={false} />)

    const card = screen.getByRole('region', { name: 'Model token usage' })
    for (const model of [
      'model-1',
      'model-2',
      'model-3',
      'model-4',
      'model-5',
    ]) {
      expect(within(card).getByText(model)).toBeInTheDocument()
    }
    expect(within(card).queryByText('model-6')).not.toBeInTheDocument()
    expect(within(card).getByText('7,000')).toBeInTheDocument()
    expect(within(card).getByText('Cached 700')).toBeInTheDocument()
    expect(within(card).getAllByText('Cache hit rate 25%')).toHaveLength(5)
  })

  it('lists every model when View all is opened', async () => {
    const user = userEvent.setup()
    render(<ModelTokenUsageCard data={data} loading={false} />)

    await user.click(screen.getByRole('button', { name: 'View all' }))

    const dialog = await screen.findByRole('dialog')
    expect(within(dialog).getAllByRole('row')).toHaveLength(8)
    const firstRow = within(dialog).getAllByRole('row')[1]
    expect(within(firstRow).getByText('model-1')).toBeInTheDocument()
    expect(within(firstRow).getByText('25%')).toBeInTheDocument()
    const lastRow = within(dialog).getAllByRole('row')[7]
    expect(within(lastRow).getByText('model-7')).toBeInTheDocument()
    expect(within(lastRow).getByText('1,000')).toBeInTheDocument()
    expect(within(lastRow).getByText('-')).toBeInTheDocument()
  })

  it('says there is no data and disables View all without usage', () => {
    render(<ModelTokenUsageCard data={[]} loading={false} />)

    expect(screen.getByText('No data')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'View all' })).toBeDisabled()
  })

  it('shows placeholders instead of models while loading', () => {
    render(<ModelTokenUsageCard data={data} loading />)

    expect(screen.queryByText('model-1')).not.toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'View all' })).toBeDisabled()
  })
})
