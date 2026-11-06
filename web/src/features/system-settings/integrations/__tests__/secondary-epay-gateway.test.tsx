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
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { useState, type ComponentProps } from 'react'
import { afterEach, describe, expect, it, vi } from 'vitest'

import { api } from '@/lib/api'

import { SettingsPageProvider } from '../../components/settings-page-context'
import { PaymentMethodDialog } from '../payment-method-dialog'
import { PaymentSettingsSection } from '../payment-settings-section'

type SectionProps = ComponentProps<typeof PaymentSettingsSection>

const baseDefaults: SectionProps['defaultValues'] = {
  PayAddress: 'https://pay.example.com',
  EpayId: '10001',
  EpayKey: '',
  SecondaryEpayAddress: '',
  SecondaryEpayId: '',
  SecondaryEpayKey: '',
  Price: 7.3,
  MinTopUp: 1,
  CustomCallbackAddress: '',
  PayMethods: '[{"name":"Alipay","type":"alipay"}]',
  AmountOptions: '[10,20]',
  AmountDiscount: '{}',
  StripeApiSecret: '',
  StripeWebhookSecret: '',
  StripePriceId: '',
  StripeUnitPrice: 8,
  StripeMinTopUp: 1,
  StripePromotionCodesEnabled: false,
  CreemApiKey: '',
  CreemWebhookSecret: '',
  CreemTestMode: false,
  CreemProducts: '[]',
}

function PaymentFixture(props: { defaults: SectionProps['defaultValues'] }) {
  const [actions, setActions] = useState<HTMLDivElement | null>(null)
  return (
    <SettingsPageProvider actionsContainer={actions}>
      <div ref={setActions} />
      <PaymentSettingsSection
        defaultValues={props.defaults}
        waffoDefaultValues={{
          WaffoEnabled: false,
          WaffoApiKey: '',
          WaffoPrivateKey: '',
          WaffoPublicCert: '',
          WaffoSandboxPublicCert: '',
          WaffoSandboxApiKey: '',
          WaffoSandboxPrivateKey: '',
          WaffoSandbox: false,
          WaffoMerchantId: '',
          WaffoCurrency: 'USD',
          WaffoUnitPrice: 1,
          WaffoMinTopUp: 1,
          WaffoNotifyUrl: '',
          WaffoReturnUrl: '',
          WaffoPayMethods: '[]',
        }}
        waffoPancakeDefaultValues={{
          WaffoPancakeMerchantID: '',
          WaffoPancakePrivateKey: '',
          WaffoPancakeReturnURL: '',
        }}
        complianceDefaults={{
          confirmed: true,
          termsVersion: 'v1',
          confirmedAt: 1,
          confirmedBy: 1,
        }}
      />
    </SettingsPageProvider>
  )
}

async function openEpayTab(
  defaults: Partial<SectionProps['defaultValues']> = {}
) {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  })
  render(
    <QueryClientProvider client={client}>
      <PaymentFixture defaults={{ ...baseDefaults, ...defaults }} />
    </QueryClientProvider>
  )
  const user = userEvent.setup()
  await user.click(screen.getByRole('tab', { name: 'Epay' }))
  return user
}

afterEach(() => vi.restoreAllMocks())

describe('secondary Epay gateway settings', () => {
  it('when the secondary gateway is filled in, saves its address, merchant ID and key as payment settings', async () => {
    const request = vi
      .spyOn(api, 'put')
      .mockResolvedValue({ data: { success: true } })
    const user = await openEpayTab()

    await user.type(
      screen.getByRole('textbox', { name: 'Secondary Epay endpoint' }),
      'https://usdt.example.com/'
    )
    await user.type(
      screen.getByRole('textbox', { name: 'Secondary Epay merchant ID' }),
      '1000'
    )
    const key = screen.getByLabelText('Secondary Epay secret key')
    expect(key).toHaveAttribute('type', 'password')
    await user.type(key, 'token-1')
    await user.click(screen.getByRole('button', { name: 'Save all settings' }))

    await waitFor(() => expect(request).toHaveBeenCalledTimes(3))
    expect(request).toHaveBeenCalledWith('/api/option/', {
      key: 'payment_setting.secondary_epay_address',
      value: 'https://usdt.example.com',
    })
    expect(request).toHaveBeenCalledWith('/api/option/', {
      key: 'payment_setting.secondary_epay_id',
      value: '1000',
    })
    expect(request).toHaveBeenCalledWith('/api/option/', {
      key: 'payment_setting.secondary_epay_key',
      value: 'token-1',
    })
  })

  it('when the key field is left blank, keeps the stored secondary key', async () => {
    const request = vi
      .spyOn(api, 'put')
      .mockResolvedValue({ data: { success: true } })
    const user = await openEpayTab({
      SecondaryEpayAddress: 'https://usdt.example.com',
      SecondaryEpayId: '1000',
    })

    const address = screen.getByRole('textbox', {
      name: 'Secondary Epay endpoint',
    })
    await user.clear(address)
    await user.type(address, 'https://usdt2.example.com')
    await user.click(screen.getByRole('button', { name: 'Save all settings' }))

    await waitFor(() =>
      expect(request).toHaveBeenCalledWith('/api/option/', {
        key: 'payment_setting.secondary_epay_address',
        value: 'https://usdt2.example.com',
      })
    )
    expect(request).toHaveBeenCalledTimes(1)
  })
})

describe('payment method gateway', () => {
  function renderDialog(
    editData: ComponentProps<typeof PaymentMethodDialog>['editData']
  ) {
    const onSave = vi.fn()
    render(
      <PaymentMethodDialog
        open
        onOpenChange={() => undefined}
        onSave={onSave}
        editData={editData}
      />
    )
    return { onSave, user: userEvent.setup() }
  }

  it('when the secondary gateway is chosen, saves the method with gateway secondary', async () => {
    const { onSave, user } = renderDialog({ name: 'USDT', type: 'usdt.trc20' })

    await user.click(screen.getByRole('combobox', { name: 'Gateway' }))
    await user.click(
      await screen.findByRole('option', { name: 'Secondary Epay gateway' })
    )
    await user.click(screen.getByRole('button', { name: 'Update' }))

    await waitFor(() =>
      expect(onSave).toHaveBeenCalledWith({
        name: 'USDT',
        type: 'usdt.trc20',
        gateway: 'secondary',
      })
    )
  })

  it('when a secondary method is edited without changing the gateway, keeps it on the secondary gateway', async () => {
    const { onSave, user } = renderDialog({
      name: 'USDT',
      type: 'usdt.trc20',
      gateway: 'secondary',
    })

    expect(screen.getByRole('combobox', { name: 'Gateway' })).toHaveTextContent(
      'Secondary Epay gateway'
    )
    await user.click(screen.getByRole('button', { name: 'Update' }))

    await waitFor(() =>
      expect(onSave).toHaveBeenCalledWith(
        expect.objectContaining({ gateway: 'secondary' })
      )
    )
  })

  it('when the primary gateway is kept, saves the method without a gateway field', async () => {
    const { onSave, user } = renderDialog({ name: 'Alipay', type: 'alipay' })

    await user.click(screen.getByRole('button', { name: 'Update' }))

    await waitFor(() => expect(onSave).toHaveBeenCalledTimes(1))
    expect(onSave.mock.calls[0][0]).not.toHaveProperty('gateway')
  })
})
