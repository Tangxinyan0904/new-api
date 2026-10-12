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
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { toast } from 'sonner'
import { afterEach, describe, expect, it, vi } from 'vitest'

import { CostRatioInput } from '../cost-ratio-input'

function renderInput(options: {
  value: number | null
  onSave: (ratio: number | null) => Promise<boolean>
}) {
  render(
    <CostRatioInput
      value={options.value}
      label='Cost ratio of openai'
      placeholder='Not set'
      onSave={options.onSave}
    />
  )
  return screen.getByRole('spinbutton', { name: 'Cost ratio of openai' })
}

afterEach(() => {
  vi.restoreAllMocks()
})

describe('CostRatioInput', () => {
  it('saves a new ratio when Enter is pressed', async () => {
    const user = userEvent.setup()
    const onSave = vi.fn(async () => true)
    const input = renderInput({ value: 0.4, onSave })

    await user.clear(input)
    await user.type(input, '0.35{Enter}')

    expect(onSave).toHaveBeenCalledWith(0.35)
  })

  it('saves null when the ratio is cleared', async () => {
    const user = userEvent.setup()
    const onSave = vi.fn(async () => true)
    const input = renderInput({ value: 0.4, onSave })

    await user.clear(input)
    await user.tab()

    expect(onSave).toHaveBeenCalledWith(null)
  })

  it('does not save an unchanged ratio', async () => {
    const user = userEvent.setup()
    const onSave = vi.fn(async () => true)
    const input = renderInput({ value: 0.4, onSave })

    await user.click(input)
    await user.tab()

    expect(onSave).not.toHaveBeenCalled()
  })

  it('rejects an out-of-range ratio and restores the saved value', async () => {
    const user = userEvent.setup()
    const errorToast = vi.spyOn(toast, 'error').mockImplementation(() => '')
    const onSave = vi.fn(async () => true)
    const input = renderInput({ value: 0.4, onSave })

    await user.clear(input)
    await user.type(input, '101')
    await user.tab()

    expect(onSave).not.toHaveBeenCalled()
    expect(errorToast).toHaveBeenCalledWith(
      'Cost ratio must be between 0 and 100'
    )
    expect(input).toHaveValue(0.4)
  })

  it('discards the edit on Escape without saving', async () => {
    const user = userEvent.setup()
    const onSave = vi.fn(async () => true)
    const input = renderInput({ value: 0.4, onSave })

    await user.clear(input)
    await user.type(input, '0.9{Escape}')

    expect(onSave).not.toHaveBeenCalled()
    expect(input).toHaveValue(0.4)
  })

  it('restores the saved value when saving fails', async () => {
    const user = userEvent.setup()
    const input = renderInput({
      value: 0.4,
      onSave: vi.fn(async () => false),
    })

    await user.clear(input)
    await user.type(input, '0.7{Enter}')

    expect(input).toHaveValue(0.4)
  })
})
