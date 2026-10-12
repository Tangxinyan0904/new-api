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
} from '../lib/channel-cost-math'
import type { ChannelCostChannel, ChannelCostRange } from '../types'
import { CostRatioInput } from './cost-ratio-input'

const RANGE_HEADER: Record<ChannelCostRange, string> = {
  today: 'Today',
  yesterday: 'Yesterday',
  week: 'Last 7 days',
}

interface ChannelCostsTableProps {
  channels: ChannelCostChannel[]
  defaultRatio: number
  canEdit: boolean
  onSaveRatio: (channelId: number, ratio: number | null) => Promise<boolean>
}

function ChannelCostFiguresCell(props: {
  channel: ChannelCostChannel
  range: ChannelCostRange
  defaultRatio: number
}) {
  const { t } = useTranslation()
  const figures = channelCostFigures(
    props.channel,
    props.range,
    props.defaultRatio
  )
  return (
    <div className='flex flex-col gap-0.5 whitespace-nowrap tabular-nums'>
      <span className='font-medium'>{formatQuota(figures.used)}</span>
      <span className='text-muted-foreground text-xs'>
        {t('Upstream cost')} {formatQuota(figures.cost)}
      </span>
      <span
        className={cn(
          'text-xs',
          figures.profit < 0 ? 'text-destructive' : 'text-success'
        )}
      >
        {t('Profit')} {formatQuota(figures.profit)}
      </span>
    </div>
  )
}

export function ChannelCostsTable(props: ChannelCostsTableProps) {
  const { t } = useTranslation()
  const defaultLabel = t('Default {{ratio}}', { ratio: props.defaultRatio })

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
          <span className='text-muted-foreground text-sm'>{defaultLabel}</span>
        ) : (
          <CostRatioInput
            value={channel.ratio}
            label={t('Cost ratio of {{name}}', { name: channel.name })}
            placeholder={defaultLabel}
            allowEmpty
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
          <ChannelCostFiguresCell
            channel={channel}
            range={range}
            defaultRatio={props.defaultRatio}
          />
        ),
      })
    ),
  ]

  return (
    <StaticDataTable
      columns={columns}
      data={props.channels}
      getRowKey={(channel) => channel.id}
      empty={props.channels.length === 0}
      emptyContent={t('No channels yet')}
    />
  )
}
