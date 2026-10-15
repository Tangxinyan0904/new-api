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
import {
  CheckCircle2,
  CircleHelp,
  CircleX,
  TriangleAlert,
  type LucideIcon,
} from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { StatusBadge, type StatusVariant } from '@/components/status-badge'
import { formatTimestampToDate } from '@/lib/format'

import type {
  ChannelStatus,
  ChannelStatusCounts,
  ChannelStatusResponse,
} from '../types'

type StatusPresentation = {
  icon: LucideIcon
  label: string
  variant: StatusVariant
}

export function ChannelStatusIndicator(props: {
  status: ChannelStatus
  size?: 'sm' | 'md' | 'lg'
}) {
  const { t } = useTranslation()
  let presentation: StatusPresentation

  if (props.status === 'operational') {
    presentation = {
      icon: CheckCircle2,
      label: t('Operational'),
      variant: 'success',
    }
  } else if (props.status === 'degraded') {
    presentation = {
      icon: TriangleAlert,
      label: t('Degraded'),
      variant: 'warning',
    }
  } else if (props.status === 'failed') {
    presentation = {
      icon: CircleX,
      label: t('Failed'),
      variant: 'danger',
    }
  } else {
    presentation = {
      icon: CircleHelp,
      label: t('No calls'),
      variant: 'neutral',
    }
  }

  return (
    <StatusBadge
      role='status'
      label={presentation.label}
      icon={presentation.icon}
      variant={presentation.variant}
      size={props.size}
      copyable={false}
      className='font-semibold'
    />
  )
}

function StatusCount(props: { count: number; label: string }) {
  return (
    <div className='flex min-w-0 items-center justify-between gap-3 sm:block'>
      <span className='text-muted-foreground truncate text-xs'>
        {props.label}
      </span>
      <div className='mt-0 text-lg font-semibold tabular-nums sm:mt-1'>
        {props.count}
      </div>
    </div>
  )
}

function StatusCounts(props: { counts: ChannelStatusCounts }) {
  const { t } = useTranslation()
  const counts = [
    {
      status: 'operational' as const,
      label: t('Operational'),
      count: props.counts.operational,
    },
    {
      status: 'degraded' as const,
      label: t('Degraded'),
      count: props.counts.degraded,
    },
    {
      status: 'failed' as const,
      label: t('Failed'),
      count: props.counts.failed,
    },
    {
      status: 'unknown' as const,
      label: t('No calls'),
      count: props.counts.unknown,
    },
  ]

  return (
    <div className='grid min-w-0 grid-cols-2 gap-x-6 gap-y-2 sm:grid-cols-4'>
      {counts.map((item) => (
        <StatusCount key={item.status} count={item.count} label={item.label} />
      ))}
    </div>
  )
}

export function StatusSummary(props: { data: ChannelStatusResponse }) {
  const { t } = useTranslation()
  const updatedAt = props.data.last_updated_at
    ? formatTimestampToDate(props.data.last_updated_at)
    : t('No data')

  return (
    <section
      className='bg-muted/30 flex min-w-0 flex-col gap-4 rounded-lg border px-4 py-3 lg:flex-row lg:items-center lg:justify-between'
      aria-label={t('Channel status summary')}
    >
      <div className='text-muted-foreground min-w-0 text-xs'>
        <span className='block truncate'>
          {t('Based on user calls in the last hour')}
        </span>
        <span className='block truncate' title={updatedAt}>
          {t('Last updated')}: {updatedAt}
        </span>
      </div>
      <StatusCounts counts={props.data.counts} />
    </section>
  )
}
