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
import dayjs from 'dayjs'
import { Pencil, Trash2 } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { formatNumber, formatPercent } from '@/lib/format'
import { cn } from '@/lib/utils'

import { formatAverageDuration } from '../lib/status-format'
import type {
  ChannelStatus,
  ChannelStatusItem,
  ChannelStatusTimelinePoint,
} from '../types'
import { ChannelStatusIndicator } from './status-summary'

function timelineColor(status: ChannelStatus): string {
  if (status === 'operational') return 'bg-emerald-500 dark:bg-emerald-400'
  if (status === 'degraded') return 'bg-amber-500 dark:bg-amber-400'
  if (status === 'failed') return 'bg-red-500 dark:bg-red-400'
  return 'bg-muted-foreground/20'
}

function useStatusLabel() {
  const { t } = useTranslation()
  return (status: ChannelStatus) => {
    if (status === 'operational') return t('Operational')
    if (status === 'degraded') return t('Degraded')
    if (status === 'failed') return t('Failed')
    return t('No calls')
  }
}

function TimelinePoint(props: {
  point: ChannelStatusTimelinePoint
  bucketSeconds: number
}) {
  const { t } = useTranslation()
  const statusLabel = useStatusLabel()
  const range = `${dayjs.unix(props.point.start_at).format('HH:mm')}–${dayjs
    .unix(props.point.start_at + props.bucketSeconds)
    .format('HH:mm')}`
  const label = `${range} ${statusLabel(props.point.status)} · ${t(
    '{{success}} succeeded, {{failure}} failed',
    {
      success: props.point.success_count,
      failure: props.point.failure_count,
    }
  )}`
  return (
    <span
      data-slot='timeline-point'
      data-status={props.point.status}
      className={cn(
        'h-7 min-w-0 rounded-[2px]',
        timelineColor(props.point.status)
      )}
      title={label}
      aria-label={label}
    />
  )
}

function Metric(props: { label: string; value: string }) {
  return (
    <div className='min-w-0'>
      <span className='text-muted-foreground block truncate text-xs'>
        {props.label}
      </span>
      <span className='mt-1 block truncate font-semibold tabular-nums'>
        {props.value}
      </span>
    </div>
  )
}

export function MonitorStatusCard(props: {
  monitor: ChannelStatusItem
  bucketSeconds: number
  onSelect: (monitor: ChannelStatusItem) => void
  /** Shown to staff who may change monitors. */
  onEdit?: (monitor: ChannelStatusItem) => void
  onDelete?: (monitor: ChannelStatusItem) => void
}) {
  const { t } = useTranslation()
  const monitor = props.monitor

  return (
    <div className='bg-card flex min-h-[14rem] min-w-0 flex-col overflow-hidden rounded-lg border shadow-sm'>
      <button
        type='button'
        className='hover:bg-muted/30 focus-visible:border-ring focus-visible:ring-ring/50 flex min-w-0 flex-1 flex-col gap-4 p-4 text-left transition-colors focus-visible:ring-[3px] focus-visible:outline-none'
        aria-label={t('View {{name}} details', { name: monitor.public_name })}
        onClick={() => props.onSelect(monitor)}
      >
        <div className='flex w-full min-w-0 items-start justify-between gap-3'>
          <div className='min-w-0 flex-1'>
            <span
              data-slot='monitor-name'
              className='block truncate text-base font-semibold'
              title={monitor.public_name}
              style={
                monitor.name_color ? { color: monitor.name_color } : undefined
              }
            >
              {monitor.public_name}
            </span>
            {monitor.group !== undefined ? (
              <div className='mt-1 flex min-w-0 items-center gap-2'>
                <Badge variant='outline' className='min-w-0 truncate'>
                  {t('Group')}: {monitor.group}
                </Badge>
                {monitor.public_enabled ? null : (
                  <Badge variant='secondary' className='shrink-0'>
                    {t('Staff only')}
                  </Badge>
                )}
              </div>
            ) : null}
          </div>
          <ChannelStatusIndicator status={monitor.status} />
        </div>

        <div className='grid w-full grid-cols-3 gap-3'>
          <Metric
            label={t('Success rate')}
            value={formatPercent(monitor.success_rate)}
          />
          <Metric
            label={t('Requests')}
            value={formatNumber(monitor.success_count + monitor.failure_count)}
          />
          <Metric
            label={t('Average duration')}
            value={formatAverageDuration(monitor.average_duration_ms)}
          />
        </div>

        <div className='mt-auto w-full min-w-0'>
          <div className='text-muted-foreground mb-2 flex min-w-0 items-center justify-between gap-2 text-xs'>
            <span className='truncate'>{t('Last hour')}</span>
            <span className='truncate'>
              {t('{{minutes}} min per bar', {
                minutes: props.bucketSeconds / 60,
              })}
            </span>
          </div>
          <div
            className='grid h-7 w-full gap-px overflow-hidden'
            style={{
              gridTemplateColumns: `repeat(${monitor.timeline.length}, minmax(0, 1fr))`,
            }}
            aria-label={t('Status over the last hour')}
          >
            {monitor.timeline.map((point) => (
              <TimelinePoint
                key={point.start_at}
                point={point}
                bucketSeconds={props.bucketSeconds}
              />
            ))}
          </div>
        </div>
      </button>

      {props.onEdit || props.onDelete ? (
        <div className='flex items-center justify-end gap-1 border-t px-2 py-1.5'>
          {props.onEdit ? (
            <Button
              type='button'
              variant='ghost'
              size='sm'
              onClick={() => props.onEdit?.(monitor)}
            >
              <Pencil data-icon='inline-start' aria-hidden='true' />
              {t('Edit')}
            </Button>
          ) : null}
          {props.onDelete ? (
            <Button
              type='button'
              variant='ghost'
              size='sm'
              onClick={() => props.onDelete?.(monitor)}
            >
              <Trash2 data-icon='inline-start' aria-hidden='true' />
              {t('Delete')}
            </Button>
          ) : null}
        </div>
      ) : null}
    </div>
  )
}
