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

mock.module('../../hooks/use-update-option', () => ({
  useUpdateOption: () => ({
    isPending: false,
    mutateAsync: async () => ({ success: true }),
  }),
}))

const { ApiKeyNoticeSettings } = await import('../api-key-notice-settings')

describe('ApiKeyNoticeSettings', () => {
  test('renders the saved value as a bounded plain-text setting', () => {
    const html = renderToStaticMarkup(
      <ApiKeyNoticeSettings value='Short operational notice' />
    )

    expect(html).toContain('API Key Notice')
    expect(html).toContain('Displayed beside the API key filters.')
    expect(html).toContain('Short operational notice')
    expect(html).toContain('24 / 500')
    expect(html).toContain('Save Settings')
  })

  test('counts supplementary Unicode characters consistently with the API', () => {
    const html = renderToStaticMarkup(<ApiKeyNoticeSettings value='🔐🔐' />)

    expect(html).toContain('2 / 500')
  })
})
