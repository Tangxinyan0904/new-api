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
import { describe, expect, mock, test } from 'bun:test'

import { renderToStaticMarkup } from 'react-dom/server'

mock.module('react-i18next', () => ({
  useTranslation: () => ({ t: (key: string) => key }),
}))

const { ComboboxInput } = await import('../combobox-input')

describe('ComboboxInput', () => {
  test('preserves a password input for masked selectable values', () => {
    const html = renderToStaticMarkup(
      <ComboboxInput
        options={[{ value: 'private-group', label: 'Private group' }]}
        value='private-group'
        onValueChange={() => undefined}
        type='password'
      />
    )

    expect(html).toContain('type="password"')
    expect(html).toContain('value="private-group"')
    expect(html).not.toContain('role="combobox"')
    expect(html).not.toContain('aria-haspopup="listbox"')
  })
})
