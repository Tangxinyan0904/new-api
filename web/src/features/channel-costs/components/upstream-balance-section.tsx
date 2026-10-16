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
import { useTranslation } from 'react-i18next'

import { StaticDataTable } from '@/components/data-table'
import { StatusBadge } from '@/components/status-badge'
import { toIntlLocale } from '@/i18n/languages'
import { formatNumber } from '@/lib/format'
import { cn } from '@/lib/utils'

import type { ChannelCostChannel, ChannelUpstreamAccount } from '../types'

function upstreamHost(baseUrl: string): string {
  try {
    return new URL(baseUrl).host
  } catch {
    return baseUrl
  }
}

/** Remaining time, in hours below a day and in days above. */
function TimeLeft(props: { account: ChannelUpstreamAccount }) {
  const { t, i18n } = useTranslation()
  const locale = toIntlLocale(i18n.resolvedLanguage || i18n.language)
  const daysLeft = props.account.days_left
  if (props.account.balance <= 0) {
    return <span className='text-destructive font-medium'>{t('Used up')}</span>
  }
  if (daysLeft === null) {
    return (
      <span className='text-muted-foreground'>{t('No recent spending')}</span>
    )
  }
  if (daysLeft < 1) {
    return (
      <span className='text-destructive font-medium'>
        {t('About {{hours}} hours', {
          hours: formatNumber(Math.max(daysLeft * 24, 0.1), locale),
        })}
      </span>
    )
  }
  return (
    <span className={cn(daysLeft < 3 && 'text-warning', 'font-medium')}>
      {t('About {{days}} days', { days: formatNumber(daysLeft, locale) })}
    </span>
  )
}

/** Each upstream account's balance and how long it lasts. */
export function UpstreamBalanceSection(props: {
  accounts: ChannelUpstreamAccount[]
  channels: ChannelCostChannel[]
}) {
  const { t, i18n } = useTranslation()
  const locale = toIntlLocale(i18n.resolvedLanguage || i18n.language)
  const names = new Map(
    props.channels.map((channel) => [channel.id, channel.name])
  )
  const dollars = (value: number) => `$${formatNumber(value, locale)}`

  return (
    <section
      aria-label={t('Upstream balances')}
      className='bg-card flex flex-col gap-3 rounded-2xl border p-3 shadow-xs sm:p-5'
    >
      <div className='flex flex-wrap items-baseline justify-between gap-2'>
        <h2 className='text-base font-semibold'>{t('Upstream balances')}</h2>
        <span className='text-muted-foreground text-xs'>
          {t(
            'Time left = balance ÷ average daily spend since the start of yesterday. Amounts are in upstream dollars.'
          )}
        </span>
      </div>
      <StaticDataTable
        data={props.accounts}
        getRowKey={(account) =>
          `${account.kind}:${account.base_url}:${account.channel_ids.join(',')}`
        }
        columns={[
          {
            id: 'upstream',
            header: t('Upstream'),
            cell: (account) => (
              <div className='flex min-w-40 flex-col gap-1'>
                <span className='font-medium break-all'>
                  {upstreamHost(account.base_url)}
                  <span className='text-muted-foreground ms-1 text-xs'>
                    {account.kind === 'newapi' ? 'new-api' : account.kind}
                  </span>
                </span>
                <span className='text-muted-foreground text-xs break-all'>
                  {account.channel_ids
                    .map((id) => names.get(id) ?? `#${id}`)
                    .join(', ')}
                </span>
              </div>
            ),
          },
          {
            id: 'balance',
            header: t('Balance'),
            cell: (account) => (
              <span className='tabular-nums'>{dollars(account.balance)}</span>
            ),
          },
          {
            id: 'spend',
            header: t('Daily spend'),
            cell: (account) => (
              <span className='tabular-nums'>
                {dollars(account.daily_spend)}
              </span>
            ),
          },
          {
            id: 'left',
            header: t('Time left'),
            cell: (account) => (
              <div className='flex flex-col items-start gap-1 tabular-nums'>
                <TimeLeft account={account} />
                {account.low && (
                  <StatusBadge
                    label={t('Less than a day left')}
                    variant='danger'
                    copyable={false}
                  />
                )}
              </div>
            ),
          },
        ]}
      />
    </section>
  )
}
