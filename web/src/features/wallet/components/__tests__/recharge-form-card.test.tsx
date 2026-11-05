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
import { describe, expect, test } from 'vitest'

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
  const view = window.document.createElement('div')
  view.innerHTML = html
  return view
}

async function renderTopupForm(selectedPreset: number | null = null) {
  const i18n = createInstance()
  await i18n.init({ lng: 'zh', resources: { zh } })

  const html = renderToStaticMarkup(
    <I18nextProvider i18n={i18n}>
      <RechargeFormCard
        topupInfo={{
          enable_online_topup: true,
          enable_stripe_topup: false,
          pay_methods: [],
          min_topup: 1,
          stripe_min_topup: 1,
          amount_options: [100, 20],
          discount: {},
        }}
        presetAmounts={[{ value: 100, discount: 0.8 }, { value: 20 }]}
        selectedPreset={selectedPreset}
        onSelectPreset={() => undefined}
        topupAmount={100}
        onTopupAmountChange={() => undefined}
        paymentAmount={80}
        calculating={false}
        onPaymentMethodSelect={() => undefined}
        paymentLoading={null}
        redemptionCode=''
        onRedemptionCodeChange={() => undefined}
        onRedeem={() => undefined}
        redeeming={false}
      />
    </I18nextProvider>
  )
  const view = window.document.createElement('div')
  view.innerHTML = html
  return view
}

function presetTile(view: HTMLElement, amount: string) {
  const tile = [...view.querySelectorAll('button[aria-pressed]')].find(
    (button) => button.firstElementChild?.textContent === amount
  )
  if (!tile) throw new Error(`preset ${amount} not rendered`)
  return tile
}

describe('RechargeFormCard preset amounts', () => {
  test('in a card narrowed by side panels, sizes preset columns by the card width instead of the viewport', async () => {
    const view = await renderTopupForm()

    const grid = presetTile(view, '100').parentElement

    expect(grid?.closest('[class~="@container"]')).not.toBeNull()
    expect(grid).toHaveClass('grid-cols-2', '@xs:grid-cols-3')
    expect(grid?.className).not.toMatch(/(^|\s)(sm|md|lg):grid-cols-/)
  })

  test('with a selected preset, marks only that preset as pressed', async () => {
    const view = await renderTopupForm(100)

    expect(presetTile(view, '100').getAttribute('aria-pressed')).toBe('true')
    expect(presetTile(view, '20').getAttribute('aria-pressed')).toBe('false')
  })

  test('with a discount, places the discount badge on the tile border so the amount keeps the full width', async () => {
    const view = await renderTopupForm()

    const badge = [...presetTile(view, '100').children].find(
      (child) => child.textContent === '优惠 20%'
    )

    expect(badge).toHaveClass('absolute')
  })

  test('at price ratio 1 without a discount, hides the pay line that would repeat the amount', async () => {
    const view = await renderTopupForm()

    expect(presetTile(view, '20').textContent).toBe('20')
    expect(presetTile(view, '100').textContent).toContain('支付 80')
  })
})

describe('RechargeFormCard amount to pay', () => {
  test('on a narrow card, keeps the amount-to-pay label whole instead of truncating it', async () => {
    const view = await renderTopupForm()

    const label = [...view.querySelectorAll('span')].find(
      (span) => span.textContent === '待支付金额：'
    )

    expect(label).toHaveClass('whitespace-nowrap')
    expect(label).not.toHaveClass('truncate')
    expect(label?.parentElement?.parentElement).toHaveClass(
      'grid-cols-[minmax(0,1fr)_auto]'
    )
  })
})

describe('RechargeFormCard redemption code entry', () => {
  test('with a top-up link, renders the get-code entry as a button that opens the link in a new tab', async () => {
    const view = await renderRechargeForm('https://shop.example.com/codes')

    const entry = view.querySelector('a[href="https://shop.example.com/codes"]')

    expect(entry?.getAttribute('data-slot')).toBe('button')
    expect(entry?.getAttribute('target')).toBe('_blank')
    expect(entry?.getAttribute('rel')).toContain('noopener')
    expect(entry?.textContent).toContain('点此获取')
    expect(view.textContent).toContain('需要兑换码？')
  })

  test('without a top-up link, hides the get-code entry', async () => {
    const view = await renderRechargeForm()

    expect(view.textContent).not.toContain('点此获取')
    expect(view.textContent).not.toContain('需要兑换码？')
  })
})
