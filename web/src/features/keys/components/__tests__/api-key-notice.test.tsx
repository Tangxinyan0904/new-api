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
import { describe, expect, test } from 'bun:test'

import { createInstance } from 'i18next'
import { renderToStaticMarkup } from 'react-dom/server'
import { I18nextProvider } from 'react-i18next'

const { ApiKeyNotice } = await import('../api-key-notice')

const i18n = createInstance()
await i18n.init({
  lng: 'en',
  resources: { en: { translation: {} } },
})

describe('ApiKeyNotice', () => {
  test('hides an empty notice', () => {
    expect(
      renderToStaticMarkup(
        <I18nextProvider i18n={i18n}>
          <ApiKeyNotice notice={'  \n '} />
        </I18nextProvider>
      )
    ).toBe('')
  })

  test('renders configured plain text with line breaks preserved', () => {
    const html = renderToStaticMarkup(
      <I18nextProvider i18n={i18n}>
        <ApiKeyNotice notice={'Keep this key private.\nRotate it regularly.'} />
      </I18nextProvider>
    )

    expect(html).toContain('API Key Notice')
    expect(html).toContain('whitespace-pre-wrap')
    expect(html).toContain('Keep this key private.\nRotate it regularly.')
  })
})
