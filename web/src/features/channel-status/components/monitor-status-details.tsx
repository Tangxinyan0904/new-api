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

import { Badge } from '@/components/ui/badge'
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
import { formatTimestampToDate } from '@/lib/format'

import { channelStatusQueryKeys, getChannelStatusDetail } from '../api'
import {
  CHANNEL_STATUS_PROVIDER_LABELS,
  type ChannelModelStatus,
  type ChannelStatusDays,
  type ChannelStatusErrorCategory,
} from '../types'
import { ChannelStatusIndicator } from './status-summary'

function modelAvailability(
  model: ChannelModelStatus,
  days: ChannelStatusDays
): number | null {
  if (days === 1) return model.availability_1d
  if (days === 3) return model.availability_3d
  return model.availability_7d
}

function modelLatency(
  model: ChannelModelStatus,
  days: ChannelStatusDays
): number | null {
  if (days === 1) return model.average_latency_ms_1d
  if (days === 3) return model.average_latency_ms_3d
  return model.average_latency_ms_7d
}

function formatMetric(value: number | null, suffix: string): string {
  if (value === null) return '-'
  return `${value.toFixed(2)}${suffix}`
}

function ErrorCategory(props: { category: ChannelStatusErrorCategory }) {
  const { t } = useTranslation()
  if (!props.category) return null

  let label = t('Configuration')
  if (props.category === 'network') label = t('Network')
  else if (props.category === 'tls') label = t('TLS')
  else if (props.category === 'auth') label = t('Authentication')
  else if (props.category === 'rate_limit') label = t('Rate limit')
  else if (props.category === 'timeout') label = t('Timeout')
  else if (props.category === 'upstream') label = t('Upstream')
  else if (props.category === 'invalid_response') label = t('Invalid response')

  return (
    <span className='text-muted-foreground truncate text-xs' title={label}>
      {t('Error category')}: {label}
    </span>
  )
}

function ModelRow(props: { model: ChannelModelStatus }) {
  const { t } = useTranslation()
  const windows: ChannelStatusDays[] = [1, 3, 7]

  return (
    <div className='flex min-w-0 flex-col gap-3 border-b px-4 py-4 last:border-b-0'>
      <div className='flex min-w-0 items-start justify-between gap-3'>
        <div className='min-w-0 flex-1'>
          <div className='flex min-w-0 items-center gap-2'>
            <span className='truncate font-semibold' title={props.model.model}>
              {props.model.model}
            </span>
            <Badge variant='outline' className='shrink-0'>
              {props.model.is_primary ? t('Primary') : t('Additional')}
            </Badge>
          </div>
          <div className='mt-1 flex min-w-0 flex-col gap-0.5'>
            <span className='text-muted-foreground truncate text-xs'>
              {props.model.last_checked_at
                ? `${t('Last checked')}: ${formatTimestampToDate(props.model.last_checked_at)}`
                : t('Never checked')}
            </span>
            <ErrorCategory category={props.model.error_category} />
          </div>
        </div>
        <ChannelStatusIndicator status={props.model.status} />
      </div>

      <div className='grid grid-cols-3 gap-2'>
        {windows.map((days) => (
          <div key={days} className='bg-muted/30 min-w-0 rounded-md px-2 py-2'>
            <span className='text-muted-foreground block truncate text-xs'>
              {t('{{days}} days', { days })}
            </span>
            <span className='mt-1 block truncate text-sm font-semibold tabular-nums'>
              {formatMetric(modelAvailability(props.model, days), '%')}
            </span>
            <span className='text-muted-foreground block truncate text-xs tabular-nums'>
              {formatMetric(modelLatency(props.model, days), ' ms')}
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
        <Skeleton key={index} className='h-36 w-full' />
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

  return (
    <Sheet open={props.open} onOpenChange={props.onOpenChange}>
      <SheetContent className='sm:max-w-2xl'>
        <SheetHeader className='border-b'>
          <SheetTitle>{t('Channel details')}</SheetTitle>
          <SheetDescription>
            {detailQuery.data
              ? `${detailQuery.data.public_name} · ${CHANNEL_STATUS_PROVIDER_LABELS[detailQuery.data.provider]}`
              : t('Availability by model and time window.')}
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
          {detailQuery.data ? (
            <div>
              <div className='bg-muted/20 flex min-w-0 items-center justify-between gap-3 border-b px-4 py-3'>
                <div className='min-w-0'>
                  <span
                    className='block truncate font-semibold'
                    title={detailQuery.data.public_name}
                  >
                    {detailQuery.data.public_name}
                  </span>
                  <span className='text-muted-foreground block truncate text-xs'>
                    {t('Last updated')}:{' '}
                    {formatTimestampToDate(detailQuery.data.last_updated_at)}
                  </span>
                </div>
                <ChannelStatusIndicator status={detailQuery.data.status} />
              </div>
              <div className='divide-y-0'>
                {detailQuery.data.models.map((model) => (
                  <ModelRow key={model.model} model={model} />
                ))}
              </div>
              {detailQuery.data.models.length === 0 ? (
                <Empty className='min-h-64'>
                  <EmptyHeader>
                    <EmptyMedia variant='icon'>
                      <Activity aria-hidden='true' />
                    </EmptyMedia>
                    <EmptyTitle>{t('No model status data')}</EmptyTitle>
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
