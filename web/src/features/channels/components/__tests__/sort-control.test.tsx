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
import { afterEach, expect, test, vi } from 'vitest'

import { ChannelSortControl } from '../channel-priority-controls'

afterEach(cleanup)

test('the sort direction button matches the height and outline of the sort select', () => {
  render(<ChannelSortControl field='priority' desc onChange={vi.fn()} />)
  const group = within(screen.getByRole('group', { name: 'Sort channels by' }))

  // The default select trigger is h-8; the direction button must be size-8
  // with a solid border, not the theme's taller dashed outline button.
  expect(
    group.getByRole('combobox', { name: 'Sort channels by' })
  ).toHaveAttribute('data-size', 'default')
  const direction = group.getByRole('button', { name: 'Descending' })
  expect(direction).toHaveClass('size-8', 'border-input')
  expect(direction).not.toHaveClass('border-dashed')
})
