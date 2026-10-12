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
import { useId } from 'react'
import { useTranslation } from 'react-i18next'

import {
  StaticDataTable,
  type StaticDataTableColumn,
} from '@/components/data-table'
import { StatusBadge } from '@/components/status-badge'
import { formatQuota } from '@/lib/format'
import { cn } from '@/lib/utils'

import {
  CHANNEL_COST_RANGES,
  channelCostFigures,
  sumChannelCostFigures,
} from '../lib/channel-cost-math'
import type {
  ChannelCostChannel,
  ChannelCostFigures,
  ChannelCostGroup,
  ChannelCostRange,
} from '../types'
import { CostRatioInput } from './cost-ratio-input'

const RANGE_HEADER: Record<ChannelCostRange, string> = {
  today: 'Today',
  yesterday: 'Yesterday',
  week: 'Last 7 days',
}

interface ChannelCostGroupSectionProps {
  group: ChannelCostGroup
  canEdit: boolean
  onSaveRatio: (channelId: number, ratio: number | null) => Promise<boolean>
}

function profitClassName(figures: ChannelCostFigures): string {
  return figures.profit < 0 ? 'text-destructive' : 'text-success'
}

function ChannelCostFiguresCell(props: {
  channel: ChannelCostChannel
  range: ChannelCostRange
}) {
  const { t } = useTranslation()
  const figures = channelCostFigures(props.channel, props.range)
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
  const week = sumChannelCostFigures(props.group.channels, 'week')

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
        </div>
      ),
    },
    {
      id: 'ratio',
      header: t('Cost ratio'),
      cell: (channel) =>
        channel.deleted ? (
          <span className='text-muted-foreground text-sm tabular-nums'>
            {channel.ratio ?? t('Not set')}
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
        ),
    },
    ...CHANNEL_COST_RANGES.map(
      (range): StaticDataTableColumn<ChannelCostChannel> => ({
        id: range,
        header: t(RANGE_HEADER[range]),
        cell: (channel) => (
          <ChannelCostFiguresCell channel={channel} range={range} />
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
    </section>
  )
}
