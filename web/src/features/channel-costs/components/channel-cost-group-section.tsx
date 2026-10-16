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
import { ReceiptText } from 'lucide-react'
import { useId, useState } from 'react'
import { useTranslation } from 'react-i18next'

import {
  StaticDataTable,
  type StaticDataTableColumn,
} from '@/components/data-table'
import { StatusBadge } from '@/components/status-badge'
import { Button } from '@/components/ui/button'
import { formatQuota } from '@/lib/format'
import { cn } from '@/lib/utils'

import {
  CHANNEL_COST_RANGES,
  channelCostFigures,
  sumGroupCostFigures,
  upstreamShareKey,
  type UpstreamCostShares,
} from '../lib/channel-cost-math'
import type {
  ChannelCostChannel,
  ChannelCostFigures,
  ChannelCostGroup,
  ChannelCostRange,
  ChannelUpstreamBillingInput,
  ChannelUpstreamCost,
} from '../types'
import { CostRatioInput } from './cost-ratio-input'
import { UpstreamBillingDialog } from './upstream-billing-dialog'

const RANGE_HEADER: Record<ChannelCostRange, string> = {
  today: 'Today',
  yesterday: 'Yesterday',
  week: 'Last 7 days',
}

interface ChannelCostGroupSectionProps {
  group: ChannelCostGroup
  canEdit: boolean
  onSaveRatio: (channelId: number, ratio: number | null) => Promise<boolean>
  /** Each channel's upstream cost within each group, in quota units. */
  shares: UpstreamCostShares
  /** What was read from the upstreams, by channel id. */
  upstreamCosts: Map<number, ChannelUpstreamCost>
  upstreamLoading: boolean
  onSaveUpstream: (
    channelId: number,
    input: ChannelUpstreamBillingInput
  ) => Promise<boolean>
}

/** Says whether a channel's upstream bill is being read or failed. */
function UpstreamBillState(props: {
  channel: ChannelCostChannel
  cost: ChannelUpstreamCost | undefined
  loading: boolean
}) {
  const { t } = useTranslation()
  const billing = props.channel.upstream_billing
  if (!billing) return null
  let state: React.ReactNode = null
  if (props.cost?.error) {
    state = (
      <span className='text-destructive text-xs break-all'>
        {t(props.cost.error.key, props.cost.error.params)}
      </span>
    )
  } else if (!props.cost && props.loading) {
    state = (
      <span className='text-muted-foreground text-xs'>
        {t('Reading the upstream…')}
      </span>
    )
  }
  return (
    <div className='flex flex-col items-start gap-1'>
      <StatusBadge
        label={t('Upstream bill · {{kind}}', {
          kind: billing.kind === 'newapi' ? 'new-api' : billing.kind,
        })}
        variant='info'
        copyable={false}
      />
      {state}
    </div>
  )
}

function profitClassName(figures: ChannelCostFigures): string {
  return figures.profit < 0 ? 'text-destructive' : 'text-success'
}

function ChannelCostFiguresCell(props: {
  channel: ChannelCostChannel
  range: ChannelCostRange
  upstreamCost: number | undefined
}) {
  const { t } = useTranslation()
  const figures = channelCostFigures(
    props.channel,
    props.range,
    props.upstreamCost
  )
  return (
    <div className='flex flex-col gap-0.5 whitespace-nowrap tabular-nums'>
      <span className='font-medium'>{formatQuota(figures.used)}</span>
      <span className='text-muted-foreground text-xs'>
        {t('Upstream cost')} {formatQuota(figures.cost)}
      </span>
      <span className={cn('text-xs', profitClassName(figures))}>
        {t('Profit')} {formatQuota(figures.profit)}
      </span>
    </div>
  )
}

/** One group: its ratio, 7-day totals and the channels serving it. */
export function ChannelCostGroupSection(props: ChannelCostGroupSectionProps) {
  const { t } = useTranslation()
  const headingId = useId()
  const [billingChannel, setBillingChannel] =
    useState<ChannelCostChannel | null>(null)
  const week = sumGroupCostFigures([props.group], 'week', props.shares)
  const shareOf = (channel: ChannelCostChannel) =>
    props.shares.get(upstreamShareKey(props.group.name, channel.id))

  const columns: StaticDataTableColumn<ChannelCostChannel>[] = [
    {
      id: 'channel',
      header: t('Channel'),
      cell: (channel) => (
        <div className='flex min-w-40 flex-col gap-1'>
          <span className='font-medium break-all'>
            {channel.deleted ? t('Deleted channel') : channel.name}
            <span className='text-muted-foreground ms-1 text-xs tabular-nums'>
              #{channel.id}
            </span>
          </span>
          {!channel.deleted && channel.status !== 1 && (
            <StatusBadge
              label={t('Disabled')}
              variant='neutral'
              copyable={false}
            />
          )}
          <UpstreamBillState
            channel={channel}
            cost={props.upstreamCosts.get(channel.id)}
            loading={props.upstreamLoading}
          />
        </div>
      ),
    },
    {
      id: 'ratio',
      header: t('Cost source'),
      cell: (channel) => {
        if (channel.deleted) {
          return (
            <span className='text-muted-foreground text-sm tabular-nums'>
              {channel.ratio ?? t('Not set')}
            </span>
          )
        }
        return (
          <div className='flex flex-col items-start gap-1'>
            {channel.upstream_billing ? (
              <span className='text-muted-foreground text-xs whitespace-nowrap tabular-nums'>
                {t('Upstream $1 = site ${{rate}}', {
                  rate: channel.upstream_billing.usd_rate,
                })}
              </span>
            ) : (
              <CostRatioInput
                value={channel.ratio}
                label={t('Cost ratio of {{name}}', { name: channel.name })}
                placeholder={t('Not set')}
                disabled={!props.canEdit}
                className='w-28'
                onSave={(ratio) => props.onSaveRatio(channel.id, ratio)}
              />
            )}
            {props.canEdit && (
              <Button
                type='button'
                variant='ghost'
                size='xs'
                className='-ms-2'
                onClick={() => setBillingChannel(channel)}
              >
                <ReceiptText data-icon='inline-start' aria-hidden='true' />
                {t('Upstream bill')}
              </Button>
            )}
          </div>
        )
      },
    },
    ...CHANNEL_COST_RANGES.map(
      (range): StaticDataTableColumn<ChannelCostChannel> => ({
        id: range,
        header: t(RANGE_HEADER[range]),
        cell: (channel) => (
          <ChannelCostFiguresCell
            channel={channel}
            range={range}
            upstreamCost={shareOf(channel)?.[range]}
          />
        ),
      })
    ),
  ]

  return (
    <section
      aria-labelledby={headingId}
      className='bg-card flex flex-col gap-3 rounded-2xl border p-3 shadow-xs sm:p-5'
    >
      <div className='flex flex-wrap items-center justify-between gap-x-4 gap-y-2'>
        <div className='flex min-w-0 flex-wrap items-center gap-2'>
          <h2 id={headingId} className='text-base font-semibold break-all'>
            {props.group.name || t('Ungrouped')}
          </h2>
          <StatusBadge
            label={
              props.group.ratio === null
                ? t('No group ratio')
                : t('Group ratio {{ratio}}', { ratio: props.group.ratio })
            }
            variant='neutral'
            copyable={false}
          />
        </div>
        <div className='text-muted-foreground flex flex-wrap gap-x-3 gap-y-1 text-xs tabular-nums'>
          <span>{t('Last 7 days')}</span>
          <span>
            {t('Usage amount')} {formatQuota(week.used)}
          </span>
          <span>
            {t('Upstream cost')} {formatQuota(week.cost)}
          </span>
          <span className={profitClassName(week)}>
            {t('Profit')} {formatQuota(week.profit)}
          </span>
        </div>
      </div>
      <StaticDataTable
        columns={columns}
        data={props.group.channels}
        getRowKey={(channel) => channel.id}
      />
      {billingChannel && (
        <UpstreamBillingDialog
          channel={billingChannel}
          open
          onOpenChange={(open) => {
            if (!open) setBillingChannel(null)
          }}
          onSave={(input) => props.onSaveUpstream(billingChannel.id, input)}
        />
      )}
    </section>
  )
}
