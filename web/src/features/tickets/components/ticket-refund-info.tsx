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
import { useQuery } from '@tanstack/react-query'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import { Skeleton } from '@/components/ui/skeleton'
import { getPaymentMethodName } from '@/features/wallet/lib/billing'
import { toIntlLocale } from '@/i18n/languages'
import { formatCurrencyFromUSD } from '@/lib/currency'
import dayjs from '@/lib/dayjs'
import { formatNumber, formatQuota, formatTimestamp } from '@/lib/format'

import { getStaffTicketRefundInfo } from '../api'
import { ticketQueryKeys } from '../constants'

/** Local midnight today and six days earlier, so the week includes today. */
function refundUsageStarts(now: Date) {
  const today = dayjs(now).startOf('day')
  return {
    today_start: today.unix(),
    week_start: today.subtract(6, 'day').unix(),
  }
}

/** Payments and recent usage of a refund ticket's owner, for staff. */
export function TicketRefundInfoPanel(props: { ticketId: number }) {
  const { t, i18n } = useTranslation()
  const locale = toIntlLocale(i18n.resolvedLanguage || i18n.language)
  const [starts] = useState(() => refundUsageStarts(new Date()))
  const query = useQuery({
    queryKey: ticketQueryKeys.staffRefund(props.ticketId),
    queryFn: () => getStaffTicketRefundInfo(props.ticketId, starts),
  })
  const info = query.data

  let body: React.ReactNode
  if (query.isPending) {
    body = <Skeleton className='h-24 w-full' />
  } else if (query.isError || !info) {
    body = (
      <p className='text-destructive text-sm'>
        {t('Failed to load refund details')}
      </p>
    )
  } else {
    const latest = info.latest_top_up
    const items = [
      { label: t('Remaining quota'), value: formatQuota(info.quota) },
      {
        label: t('Online top-ups'),
        value: formatNumber(info.top_up_money, locale),
        detail: t('{{count}} orders, {{amount}} credited', {
          count: info.top_up_count,
          amount: formatCurrencyFromUSD(info.top_up_amount, {
            digitsLarge: 2,
            digitsSmall: 2,
            abbreviate: false,
          }),
        }),
      },
      {
        label: t('Latest top-up'),
        value: latest
          ? getPaymentMethodName(latest.payment_method, t)
          : t('None'),
        detail: latest
          ? `${formatNumber(latest.money, locale)} · ${formatTimestamp(latest.complete_time)}`
          : undefined,
      },
      {
        label: t('Redemption codes'),
        value: formatQuota(info.redemption_quota),
        detail: t('{{count}} codes', { count: info.redemption_count }),
      },
      { label: t('Used today'), value: formatQuota(info.today_quota) },
      {
        label: t('Used in the last 7 days'),
        value: formatQuota(info.week_quota),
      },
    ]
    body = (
      <dl className='grid grid-cols-2 gap-x-4 gap-y-3 sm:grid-cols-3'>
        {items.map((item) => (
          <div key={item.label} className='flex min-w-0 flex-col gap-0.5'>
            <dt className='text-muted-foreground text-xs'>{item.label}</dt>
            <dd className='truncate text-sm font-semibold tabular-nums'>
              {item.value}
            </dd>
            {item.detail && (
              <dd className='text-muted-foreground truncate text-xs tabular-nums'>
                {item.detail}
              </dd>
            )}
          </div>
        ))}
      </dl>
    )
  }

  return (
    <section
      aria-label={t('Refund reference')}
      className='bg-card flex flex-col gap-3 rounded-lg border p-3 sm:p-4'
    >
      <div className='flex flex-col gap-0.5'>
        <h3 className='text-sm font-semibold'>{t('Refund reference')}</h3>
        <p className='text-muted-foreground text-xs'>
          {t(
            'Online top-ups count successful orders by the amount paid; usage is net of refunds and follows your local time.'
          )}
        </p>
      </div>
      {body}
    </section>
  )
}
