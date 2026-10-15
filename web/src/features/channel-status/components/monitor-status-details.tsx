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
import { Activity, CircleAlert } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import {
  Empty,
  EmptyDescription,
  EmptyHeader,
  EmptyMedia,
  EmptyTitle,
} from '@/components/ui/empty'
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetHeader,
  SheetTitle,
} from '@/components/ui/sheet'
import { Skeleton } from '@/components/ui/skeleton'
import {
  formatNumber,
  formatPercent,
  formatTimestampToDate,
} from '@/lib/format'

import { channelStatusQueryKeys, getChannelStatusDetail } from '../api'
import { formatAverageDuration } from '../lib/status-format'
import type { ChannelModelStatus } from '../types'
import { ChannelStatusIndicator } from './status-summary'

function ModelRow(props: { model: ChannelModelStatus }) {
  const { t } = useTranslation()
  const figures = [
    {
      label: t('Success rate'),
      value: formatPercent(props.model.success_rate),
    },
    {
      label: t('Succeeded / failed'),
      value: `${formatNumber(props.model.success_count)} / ${formatNumber(props.model.failure_count)}`,
    },
    {
      label: t('Average duration'),
      value: formatAverageDuration(props.model.average_duration_ms),
    },
  ]

  return (
    <div
      data-slot='model-status'
      className='flex min-w-0 flex-col gap-3 border-b px-4 py-4 last:border-b-0'
    >
      <div className='flex min-w-0 items-center justify-between gap-3'>
        <span className='truncate font-semibold' title={props.model.model}>
          {props.model.model}
        </span>
        <ChannelStatusIndicator status={props.model.status} />
      </div>
      <div className='grid grid-cols-3 gap-2'>
        {figures.map((figure) => (
          <div
            key={figure.label}
            className='bg-muted/30 min-w-0 rounded-md px-2 py-2'
          >
            <span className='text-muted-foreground block truncate text-xs'>
              {figure.label}
            </span>
            <span className='mt-1 block truncate text-sm font-semibold tabular-nums'>
              {figure.value}
            </span>
          </div>
        ))}
      </div>
    </div>
  )
}

function DetailsLoading() {
  return (
    <div className='space-y-3 p-4'>
      {Array.from({ length: 3 }, (_, index) => (
        <Skeleton key={index} className='h-28 w-full' />
      ))}
    </div>
  )
}

export function MonitorStatusDetails(props: {
  monitorId: number | null
  open: boolean
  onOpenChange: (open: boolean) => void
}) {
  const { t } = useTranslation()
  const monitorId = props.monitorId ?? 0
  const detailQuery = useQuery({
    queryKey: channelStatusQueryKeys.detail(monitorId),
    queryFn: () => getChannelStatusDetail(monitorId),
    enabled: props.open && monitorId > 0,
  })
  const monitor = detailQuery.data?.monitor

  return (
    <Sheet open={props.open} onOpenChange={props.onOpenChange}>
      <SheetContent className='sm:max-w-2xl'>
        <SheetHeader className='border-b'>
          <SheetTitle>{t('Channel details')}</SheetTitle>
          <SheetDescription>
            {t('Calls of each model in the last hour.')}
          </SheetDescription>
        </SheetHeader>

        <div className='min-h-0 flex-1 overflow-auto'>
          {detailQuery.isLoading ? <DetailsLoading /> : null}
          {detailQuery.isError ? (
            <Empty className='min-h-72'>
              <EmptyHeader>
                <EmptyMedia variant='icon'>
                  <CircleAlert aria-hidden='true' />
                </EmptyMedia>
                <EmptyTitle>{t('Channel details unavailable')}</EmptyTitle>
                <EmptyDescription>
                  {t('Try refreshing the channel status page.')}
                </EmptyDescription>
              </EmptyHeader>
            </Empty>
          ) : null}
          {detailQuery.data && monitor ? (
            <div>
              <div className='bg-muted/20 flex min-w-0 items-center justify-between gap-3 border-b px-4 py-3'>
                <div className='min-w-0'>
                  <span
                    data-slot='monitor-detail-name'
                    className='block truncate font-semibold'
                    title={monitor.public_name}
                    style={
                      monitor.name_color
                        ? { color: monitor.name_color }
                        : undefined
                    }
                  >
                    {monitor.public_name}
                  </span>
                  <span className='text-muted-foreground block truncate text-xs'>
                    {t('Last updated')}:{' '}
                    {formatTimestampToDate(detailQuery.data.last_updated_at)}
                  </span>
                </div>
                <ChannelStatusIndicator status={monitor.status} />
              </div>
              {detailQuery.data.models.map((model) => (
                <ModelRow key={model.model} model={model} />
              ))}
              {detailQuery.data.models.length === 0 ? (
                <Empty className='min-h-64'>
                  <EmptyHeader>
                    <EmptyMedia variant='icon'>
                      <Activity aria-hidden='true' />
                    </EmptyMedia>
                    <EmptyTitle>{t('No calls in the last hour')}</EmptyTitle>
                  </EmptyHeader>
                </Empty>
              ) : null}
            </div>
          ) : null}
        </div>
      </SheetContent>
    </Sheet>
  )
}
