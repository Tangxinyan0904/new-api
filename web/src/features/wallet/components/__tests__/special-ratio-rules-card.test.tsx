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
import { expect, test } from 'bun:test'

import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { createInstance } from 'i18next'
import { renderToStaticMarkup } from 'react-dom/server'
import { I18nextProvider } from 'react-i18next'

import zh from '@/i18n/locales/zh.json'

import { SpecialRatioRulesCard } from '../special-ratio-rules-card'

test('shows only the current and upgraded ratios beneath each rule', async () => {
  const i18n = createInstance()
  await i18n.init({
    lng: 'zh',
    resources: { zh },
  })
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: { staleTime: Infinity },
    },
  })
  queryClient.setQueryData(
    ['wallet-special-ratios'],
    [
      {
        user_group: 'svip',
        billing_group: 'Claude-Max【禁止蒸馏】',
        base_ratio: 0.95,
        special_ratio: 0.9,
      },
    ]
  )

  const html = renderToStaticMarkup(
    <QueryClientProvider client={queryClient}>
      <I18nextProvider i18n={i18n}>
        <SpecialRatioRulesCard />
      </I18nextProvider>
    </QueryClientProvider>
  )

  expect(html).toContain('当前倍率：0.95x，升级后倍率：0.9x')
  expect(html).not.toContain('升级到 Claude-Max【禁止蒸馏】')
  expect(html).toContain('svip -&gt; Claude-Max【禁止蒸馏】')
})
