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
import { useNavigate } from '@tanstack/react-router'
import { ArrowRight, CircleAlert, RadioTower } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'
import { formatTimestampToDate } from '@/lib/format'
import { useAuthStore } from '@/stores/auth-store'

import { channelStatusQueryKeys, getPublicChannelStatus } from '../api'
import {
  CHANNEL_STATUS_PROVIDER_LABELS,
  type ChannelStatusCounts,
  type PublicChannelStatusMonitor,
} from '../types'
import { ChannelStatusIndicator } from './status-summary'

const PUBLIC_STATUS_LOADING_SLOTS = ['first', 'second', 'third'] as const

function formatAvailability(value: number | null): string {
  return value === null ? '-' : `${value.toFixed(2)}%`
}

function formatLatency(value: number | null): string {
  return value === null ? '-' : `${Math.round(value)} ms`
}

function SummaryCounts(props: { counts: ChannelStatusCounts }) {
  const { t } = useTranslation()
  const items = [
    { label: t('Operational'), value: props.counts.operational },
    { label: t('Degraded'), value: props.counts.degraded },
    { label: t('Failed'), value: props.counts.failed },
    { label: t('Unknown'), value: props.counts.unknown },
  ]

  return (
    <div className='flex min-w-0 flex-wrap items-center gap-x-4 gap-y-1.5'>
      {items.map((item) => (
        <span key={item.label} className='text-muted-foreground text-xs'>
          {item.label}{' '}
          <strong className='text-foreground font-semibold tabular-nums'>
            {item.value}
          </strong>
        </span>
      ))}
    </div>
  )
}

function MonitorSummary(props: { monitor: PublicChannelStatusMonitor }) {
  const { t } = useTranslation()

  return (
    <div className='flex min-h-20 min-w-0 items-center gap-3 border-t py-3 pr-3'>
      <div className='min-w-0 flex-1'>
        <div className='flex min-w-0 items-center gap-2'>
          <span
            className='truncate text-sm font-semibold'
            title={props.monitor.public_name}
          >
            {props.monitor.public_name}
          </span>
          <span className='text-muted-foreground shrink-0 text-xs'>
            {CHANNEL_STATUS_PROVIDER_LABELS[props.monitor.provider]}
          </span>
        </div>
        <span
          className='text-muted-foreground mt-0.5 block truncate text-xs'
          title={props.monitor.primary_model}
        >
          {props.monitor.primary_model}
        </span>
        <div className='text-muted-foreground mt-1.5 flex min-w-0 items-center gap-3 text-xs tabular-nums'>
          <span className='truncate'>
            {t('Latency')}: {formatLatency(props.monitor.latency_ms)}
          </span>
          <span className='truncate'>
            {t('Today')}: {formatAvailability(props.monitor.availability_1d)}
          </span>
        </div>
      </div>
      <ChannelStatusIndicator status={props.monitor.status} />
    </div>
  )
}

function PublicStatusLoading() {
  return (
    <section
      data-slot='public-status-loading'
      className='bg-muted/20 min-h-32 border-y px-1 py-4'
      aria-hidden='true'
    >
      <div className='flex items-center justify-between gap-4'>
        <div className='space-y-2'>
          <Skeleton className='h-5 w-36' />
          <Skeleton className='h-3 w-56' />
        </div>
        <Skeleton className='h-8 w-24' />
      </div>
      <div className='mt-4 grid grid-cols-1 gap-x-5 sm:grid-cols-2 xl:grid-cols-3'>
        {PUBLIC_STATUS_LOADING_SLOTS.map((slot) => (
          <Skeleton key={slot} className='h-16 w-full' />
        ))}
      </div>
    </section>
  )
}

export function PublicStatusSummary() {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const signedIn = useAuthStore((state) => Boolean(state.auth.user))
  const summaryQuery = useQuery({
    queryKey: channelStatusQueryKeys.publicSummary,
    queryFn: getPublicChannelStatus,
    staleTime: 15_000,
    refetchInterval: 60_000,
    refetchIntervalInBackground: false,
  })

  if (summaryQuery.isLoading) {
    return <PublicStatusLoading />
  }

  if (summaryQuery.isError || !summaryQuery.data) {
    return (
      <section
        data-slot='public-status-summary'
        className='bg-muted/20 flex min-h-20 items-center gap-3 border-y px-1 py-4'
        aria-label={t('Public channel status')}
      >
        <CircleAlert
          className='text-muted-foreground size-5 shrink-0'
          aria-hidden='true'
        />
        <div className='min-w-0'>
          <div className='text-sm font-semibold'>
            {t('Channel status unavailable')}
          </div>
          <p className='text-muted-foreground mt-0.5 text-xs'>
            {t('Model pricing remains available while status data reloads.')}
          </p>
        </div>
      </section>
    )
  }

  const updatedAt = summaryQuery.data.last_updated_at
    ? formatTimestampToDate(summaryQuery.data.last_updated_at)
    : t('No data')

  return (
    <section
      data-slot='public-status-summary'
      className='bg-muted/20 min-w-0 border-y px-1 py-4'
      aria-label={t('Public channel status')}
    >
      <div className='flex min-w-0 flex-col gap-3 sm:flex-row sm:items-start sm:justify-between'>
        <div className='min-w-0 space-y-1.5'>
          <div className='flex min-w-0 flex-wrap items-center gap-2'>
            <RadioTower
              className='text-muted-foreground size-4 shrink-0'
              aria-hidden='true'
            />
            <span className='text-muted-foreground text-xs font-medium tracking-normal uppercase'>
              {t('Channel status')}
            </span>
            <ChannelStatusIndicator status={summaryQuery.data.overall_status} />
          </div>
          <span
            className='text-muted-foreground block truncate text-xs'
            title={updatedAt}
          >
            {t('Last updated')}: {updatedAt}
          </span>
          <SummaryCounts counts={summaryQuery.data.counts} />
        </div>

        <Button
          type='button'
          size='sm'
          variant='ghost'
          className='shrink-0 self-start'
          aria-label={t('View channel status details')}
          onClick={() => {
            if (signedIn) {
              void navigate({ to: '/channel-status' })
              return
            }
            void navigate({
              to: '/sign-in',
              search: { redirect: '/channel-status' },
            })
          }}
        >
          {t('View details')}
          <ArrowRight data-icon='inline-end' aria-hidden='true' />
        </Button>
      </div>

      {summaryQuery.data.monitors.length === 0 ? (
        <div className='text-muted-foreground mt-4 flex min-h-16 items-center border-t text-sm'>
          {t('No published channel monitors')}
        </div>
      ) : (
        <div className='mt-3 grid max-h-52 min-w-0 grid-cols-1 gap-x-5 overflow-y-auto sm:grid-cols-2 xl:grid-cols-3'>
          {summaryQuery.data.monitors.map((monitor) => (
            <MonitorSummary key={monitor.id} monitor={monitor} />
          ))}
        </div>
      )}
    </section>
  )
}
