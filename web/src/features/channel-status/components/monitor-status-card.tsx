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

import { Badge } from '@/components/ui/badge'
import { formatTimestampToDate } from '@/lib/format'
import { cn } from '@/lib/utils'

import {
  CHANNEL_STATUS_PROVIDER_LABELS,
  type ChannelStatus,
  type ChannelStatusDays,
  type ChannelStatusItem,
  type ChannelStatusTimelinePoint,
} from '../types'
import { ChannelStatusIndicator } from './status-summary'

const TIMELINE_POINT_COUNT = 60

function formatAvailability(value: number | null): string {
  return value === null ? '-' : `${value.toFixed(2)}%`
}

function formatLatency(value: number | null): string {
  return value === null ? '-' : `${Math.round(value)} ms`
}

function timelineColor(status: ChannelStatus): string {
  if (status === 'operational') return 'bg-emerald-500 dark:bg-emerald-400'
  if (status === 'degraded') return 'bg-amber-500 dark:bg-amber-400'
  if (status === 'failed') return 'bg-red-500 dark:bg-red-400'
  return 'bg-muted-foreground/20'
}

function TimelinePoint(props: {
  point: ChannelStatusTimelinePoint
  label: string
}) {
  return (
    <span
      data-slot='timeline-point'
      data-status={props.point.status}
      className={cn(
        'h-7 min-w-0 rounded-[2px]',
        timelineColor(props.point.status)
      )}
      title={props.label}
      aria-label={props.label}
    />
  )
}

export function MonitorStatusCard(props: {
  days: ChannelStatusDays
  monitor: ChannelStatusItem
  onSelect: (monitor: ChannelStatusItem) => void
}) {
  const { t } = useTranslation()
  const recentPoints = props.monitor.timeline
    .slice(0, TIMELINE_POINT_COUNT)
    .reverse()
  const availabilityLabel =
    props.days === 1
      ? t('24-hour availability')
      : t('{{days}}-day availability', { days: props.days })

  return (
    <button
      type='button'
      className='bg-card hover:bg-muted/30 focus-visible:border-ring focus-visible:ring-ring/50 flex min-h-[15.5rem] min-w-0 flex-col gap-4 overflow-hidden rounded-lg border p-4 text-left shadow-sm transition-colors focus-visible:ring-[3px] focus-visible:outline-none'
      aria-label={t('View {{name}} details', {
        name: props.monitor.public_name,
      })}
      onClick={() => props.onSelect(props.monitor)}
    >
      <div className='flex w-full min-w-0 items-start justify-between gap-3'>
        <div className='min-w-0 flex-1'>
          <span
            data-slot='monitor-name'
            className='block truncate text-base font-semibold'
            title={props.monitor.public_name}
            style={
              props.monitor.name_color
                ? { color: props.monitor.name_color }
                : undefined
            }
          >
            {props.monitor.public_name}
          </span>
          <div className='mt-1 flex min-w-0 items-center gap-2'>
            <Badge variant='outline' className='shrink-0'>
              {CHANNEL_STATUS_PROVIDER_LABELS[props.monitor.provider]}
            </Badge>
            <span
              className='text-muted-foreground min-w-0 truncate text-xs'
              title={props.monitor.primary_model}
            >
              {props.monitor.primary_model}
            </span>
          </div>
        </div>
        <ChannelStatusIndicator status={props.monitor.status} />
      </div>

      <div className='grid w-full grid-cols-3 gap-3'>
        <div className='min-w-0'>
          <span className='text-muted-foreground block truncate text-xs'>
            {availabilityLabel}
          </span>
          <span className='mt-1 block truncate font-semibold tabular-nums'>
            {formatAvailability(props.monitor.availability)}
          </span>
        </div>
        <div className='min-w-0'>
          <span className='text-muted-foreground block truncate text-xs'>
            {t('Latest latency')}
          </span>
          <span className='mt-1 block truncate font-semibold tabular-nums'>
            {formatLatency(props.monitor.latency_ms)}
          </span>
        </div>
        <div className='min-w-0'>
          <span className='text-muted-foreground block truncate text-xs'>
            {t('Average latency')}
          </span>
          <span className='mt-1 block truncate font-semibold tabular-nums'>
            {formatLatency(props.monitor.average_latency_ms)}
          </span>
        </div>
      </div>

      <div className='mt-auto w-full min-w-0'>
        <div className='mb-2 flex min-w-0 items-center justify-between gap-2'>
          <span className='text-muted-foreground truncate text-xs'>
            {t('Recent checks')}
          </span>
          <span className='text-muted-foreground truncate text-xs tabular-nums'>
            {props.monitor.last_checked_at
              ? formatTimestampToDate(props.monitor.last_checked_at)
              : t('Never checked')}
          </span>
        </div>
        {recentPoints.length === 0 ? (
          <div
            data-slot='timeline-empty'
            className='bg-muted/30 text-muted-foreground flex h-7 w-full items-center justify-center rounded-[2px] px-2 text-xs'
            aria-label={t('Recent channel checks')}
          >
            {t('No recent checks')}
          </div>
        ) : (
          <div
            className='grid h-7 w-full gap-px overflow-hidden'
            style={{
              gridTemplateColumns: `repeat(${recentPoints.length}, minmax(0, 1fr))`,
            }}
            aria-label={t('Recent channel checks')}
          >
            {recentPoints.map((point) => {
              let statusLabel = t('Unknown')
              if (point.status === 'operational') {
                statusLabel = t('Operational')
              } else if (point.status === 'degraded') {
                statusLabel = t('Degraded')
              } else if (point.status === 'failed') {
                statusLabel = t('Failed')
              }
              return (
                <TimelinePoint
                  key={point.checked_at}
                  point={point}
                  label={`${statusLabel}: ${formatTimestampToDate(point.checked_at)}`}
                />
              )
            })}
          </div>
        )}
      </div>
    </button>
  )
}
