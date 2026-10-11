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
import { createInstance } from 'i18next'
import { renderToStaticMarkup } from 'react-dom/server'
import { I18nextProvider } from 'react-i18next'
import { expect, test } from 'vitest'

import zhLocale from '@/i18n/locales/zh.json'

import { MonitorStatusCard } from '../monitor-status-card'

test('renders the one-day card availability label as 24 hours in Chinese', async () => {
  const i18n = createInstance()
  await i18n.init({
    lng: 'zh',
    resources: { zh: zhLocale },
  })

  const html = renderToStaticMarkup(
    <I18nextProvider i18n={i18n}>
      <MonitorStatusCard
        days={1}
        monitor={{
          id: 17,
          public_name: 'OpenAI global',
          name_color: '',
          provider: 'openai',
          primary_model: 'gpt-primary',
          status: 'operational',
          latency_ms: 321,
          error_category: '',
          last_checked_at: 1_786_841_200,
          availability: 99.125,
          average_latency_ms: 310.5,
          timeline: [],
        }}
        onSelect={() => undefined}
      />
    </I18nextProvider>
  )

  expect(html).toContain('24小时可用性')
  expect(html).not.toContain('1-day availability')
})
