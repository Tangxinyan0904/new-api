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
import { beforeEach, expect, test } from 'vitest'

import { THEME_STORAGE_KEYS } from '@/lib/theme-storage'

import {
  ThemeCustomizationProvider,
  useThemeCustomization,
} from '../theme-customization-provider'

function ThemeProbe() {
  const theme = useThemeCustomization()

  return (
    <div>
      <output data-testid='preset'>{theme.customization.preset}</output>
      <button type='button' onClick={() => theme.setPreset('default')}>
        Select base preset
      </button>
    </div>
  )
}

function renderProvider() {
  return render(
    <ThemeCustomizationProvider>
      <ThemeProbe />
    </ThemeCustomizationProvider>
  )
}

beforeEach(() => {
  localStorage.clear()
  document.body.removeAttribute('data-theme-preset')
})

test('uses simple-large when the browser has no saved preset', () => {
  renderProvider()

  expect(screen.getByTestId('preset')).toHaveTextContent('simple-large')
  expect(document.body).toHaveAttribute('data-theme-preset', 'simple-large')
  expect(localStorage.getItem(THEME_STORAGE_KEYS.preset)).toBeNull()
})

test('persists an explicit base preset and restores it after remount', async () => {
  const user = userEvent.setup()
  const first = renderProvider()

  await user.click(screen.getByRole('button', { name: 'Select base preset' }))

  expect(document.body).not.toHaveAttribute('data-theme-preset')
  expect(localStorage.getItem(THEME_STORAGE_KEYS.preset)).toBe('default')

  first.unmount()
  renderProvider()

  expect(screen.getByTestId('preset')).toHaveTextContent('default')
  expect(document.body).not.toHaveAttribute('data-theme-preset')
})
