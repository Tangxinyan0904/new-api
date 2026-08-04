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

import { createInstance } from 'i18next'
import { renderToStaticMarkup } from 'react-dom/server'
import { I18nextProvider } from 'react-i18next'

import { AffiliateRewardsCard } from '../affiliate-rewards-card'

test('uses a neutral promotion message without a fixed reward claim', async () => {
  const i18n = createInstance()
  await i18n.init({
    lng: 'en',
    resources: { en: { translation: {} } },
  })

  const html = renderToStaticMarkup(
    <I18nextProvider i18n={i18n}>
      <AffiliateRewardsCard
        user={null}
        affiliateLink='https://example.com/sign-up?aff=test'
        rebateSummary={null}
        minimumTransferQuota={500_000}
        onRefresh={() => undefined}
        onTransfer={() => undefined}
        onOpenTransferHistory={() => undefined}
      />
    </I18nextProvider>
  )

  expect(html).toContain('Share your referral link to invite others to join.')
  expect(html).not.toContain('注册送1刀')
})
