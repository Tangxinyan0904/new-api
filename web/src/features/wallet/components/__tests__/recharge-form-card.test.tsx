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

import { Window } from 'happy-dom'
import { createInstance } from 'i18next'
import { renderToStaticMarkup } from 'react-dom/server'
import { I18nextProvider } from 'react-i18next'

import zh from '@/i18n/locales/zh.json'

import { RechargeFormCard } from '../recharge-form-card'

async function renderRechargeForm(topupLink?: string) {
  const i18n = createInstance()
  await i18n.init({ lng: 'zh', resources: { zh } })

  const html = renderToStaticMarkup(
    <I18nextProvider i18n={i18n}>
      <RechargeFormCard
        topupInfo={null}
        presetAmounts={[]}
        selectedPreset={null}
        onSelectPreset={() => undefined}
        topupAmount={0}
        onTopupAmountChange={() => undefined}
        paymentAmount={0}
        calculating={false}
        onPaymentMethodSelect={() => undefined}
        paymentLoading={null}
        redemptionCode=''
        onRedemptionCodeChange={() => undefined}
        onRedeem={() => undefined}
        redeeming={false}
        topupLink={topupLink}
      />
    </I18nextProvider>
  )
  const document = new Window().document
  document.body.innerHTML = html
  return document
}

describe('RechargeFormCard redemption code entry', () => {
  test('with a top-up link, renders the get-code entry as a button that opens the link in a new tab', async () => {
    const document = await renderRechargeForm('https://shop.example.com/codes')

    const entry = document.querySelector(
      'a[href="https://shop.example.com/codes"]'
    )

    expect(entry?.getAttribute('data-slot')).toBe('button')
    expect(entry?.getAttribute('target')).toBe('_blank')
    expect(entry?.getAttribute('rel')).toContain('noopener')
    expect(entry?.textContent).toContain('点此获取')
    expect(document.body.textContent).toContain('需要兑换码？')
  })

  test('without a top-up link, hides the get-code entry', async () => {
    const document = await renderRechargeForm()

    expect(document.body.textContent).not.toContain('点此获取')
    expect(document.body.textContent).not.toContain('需要兑换码？')
  })
})
