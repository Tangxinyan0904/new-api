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
  Activity01Icon,
  Delete02Icon,
  Edit02Icon,
  HistoryIcon,
  PlayIcon,
} from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
import { useTranslation } from 'react-i18next'

import { StatusBadge, type StatusVariant } from '@/components/status-badge'
import { Button } from '@/components/ui/button'
import {
  Empty,
  EmptyDescription,
  EmptyHeader,
  EmptyMedia,
  EmptyTitle,
} from '@/components/ui/empty'
import { Skeleton } from '@/components/ui/skeleton'
import { Spinner } from '@/components/ui/spinner'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from '@/components/ui/tooltip'
import { formatTimestampToDate } from '@/lib/format'

import type { ChannelMonitor, ChannelMonitorStatus } from '../types'

type ChannelMonitorTableProps = {
  monitors: ChannelMonitor[]
  isLoading: boolean
  runningMonitorId: number | null
  onEdit: (monitor: ChannelMonitor) => void
  onRunNow: (monitor: ChannelMonitor) => void
  onHistory: (monitor: ChannelMonitor) => void
  onDelete: (monitor: ChannelMonitor) => void
}

function statusVariant(status: ChannelMonitorStatus): StatusVariant {
  if (status === 'operational') return 'success'
  if (status === 'degraded') return 'warning'
  if (status === 'failed') return 'danger'
  return 'neutral'
}

function MonitorStatus(props: { monitor: ChannelMonitor }) {
  const { t } = useTranslation()
  const latency =
    props.monitor.latest_latency_ms === null
      ? t('No latency data')
      : `${props.monitor.latest_latency_ms} ms`

  return (
    <div className='flex min-w-0 flex-col items-start gap-1'>
      <StatusBadge
        label={t(props.monitor.latest_status)}
        variant={statusVariant(props.monitor.latest_status)}
        copyable={false}
        className='capitalize'
      />
      <span className='text-muted-foreground text-xs'>{latency}</span>
    </div>
  )
}

function MonitorActions(props: {
  monitor: ChannelMonitor
  running: boolean
  onEdit: (monitor: ChannelMonitor) => void
  onRunNow: (monitor: ChannelMonitor) => void
  onHistory: (monitor: ChannelMonitor) => void
  onDelete: (monitor: ChannelMonitor) => void
}) {
  const { t } = useTranslation()
  const actions = [
    {
      label: t('Edit'),
      icon: Edit02Icon,
      onClick: props.onEdit,
      disabled: false,
      destructive: false,
      loading: false,
    },
    {
      label: t('Run now'),
      icon: PlayIcon,
      onClick: props.onRunNow,
      disabled: props.running,
      destructive: false,
      loading: props.running,
    },
    {
      label: t('View history'),
      icon: HistoryIcon,
      onClick: props.onHistory,
      disabled: false,
      destructive: false,
      loading: false,
    },
    {
      label: t('Delete'),
      icon: Delete02Icon,
      onClick: props.onDelete,
      disabled: false,
      destructive: true,
      loading: false,
    },
  ]

  return (
    <div className='flex w-36 items-center justify-end gap-1'>
      {actions.map((action) => (
        <Tooltip key={action.label}>
          <TooltipTrigger
            render={
              <Button
                type='button'
                size='icon-sm'
                variant={action.destructive ? 'destructive' : 'ghost'}
                aria-label={action.label}
                disabled={action.disabled}
                onClick={() => action.onClick(props.monitor)}
              />
            }
          >
            {action.loading ? (
              <Spinner />
            ) : (
              <HugeiconsIcon icon={action.icon} strokeWidth={2} />
            )}
          </TooltipTrigger>
          <TooltipContent>{action.label}</TooltipContent>
        </Tooltip>
      ))}
    </div>
  )
}

function MonitorNames(props: { monitor: ChannelMonitor }) {
  return (
    <div className='flex max-w-64 min-w-0 flex-col gap-0.5'>
      <span
        className='truncate font-medium'
        title={props.monitor.internal_name}
      >
        {props.monitor.internal_name}
      </span>
      <span
        className='text-muted-foreground truncate text-xs'
        title={props.monitor.public_name}
      >
        {props.monitor.public_name}
      </span>
    </div>
  )
}

function LoadingRows() {
  return (
    <div className='flex flex-col gap-2 p-3'>
      {Array.from({ length: 4 }, (_, index) => (
        <Skeleton key={index} className='h-14 w-full' />
      ))}
    </div>
  )
}

export function ChannelMonitorTable(props: ChannelMonitorTableProps) {
  const { t } = useTranslation()

  if (props.isLoading) {
    return <LoadingRows />
  }

  if (props.monitors.length === 0) {
    return (
      <Empty className='min-h-72'>
        <EmptyHeader>
          <EmptyMedia variant='icon'>
            <HugeiconsIcon icon={Activity01Icon} strokeWidth={2} />
          </EmptyMedia>
          <EmptyTitle>{t('No channel monitors found')}</EmptyTitle>
          <EmptyDescription>
            {t('Create a monitor to start checking upstream availability.')}
          </EmptyDescription>
        </EmptyHeader>
      </Empty>
    )
  }

  return (
    <div className='overflow-hidden rounded-lg border'>
      <div className='hidden md:block'>
        <Table className='table-fixed'>
          <TableHeader>
            <TableRow>
              <TableHead className='w-[20%]'>{t('Name')}</TableHead>
              <TableHead className='w-[14%]'>{t('Provider / mode')}</TableHead>
              <TableHead className='w-[7%]'>{t('Models')}</TableHead>
              <TableHead className='w-[11%]'>{t('Latest status')}</TableHead>
              <TableHead className='w-[8%]'>{t('Interval')}</TableHead>
              <TableHead className='w-[17%]'>
                {t('Last / next check')}
              </TableHead>
              <TableHead className='w-[9%]'>{t('Visibility')}</TableHead>
              <TableHead className='w-36 text-right'>{t('Actions')}</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {props.monitors.map((monitor) => (
              <TableRow key={monitor.id}>
                <TableCell>
                  <MonitorNames monitor={monitor} />
                </TableCell>
                <TableCell>
                  <div className='flex min-w-0 flex-col gap-0.5'>
                    <span className='truncate font-medium capitalize'>
                      {monitor.provider}
                    </span>
                    <span className='text-muted-foreground truncate text-xs'>
                      {monitor.api_mode.replaceAll('_', ' ')}
                    </span>
                  </div>
                </TableCell>
                <TableCell>{1 + monitor.extra_models.length}</TableCell>
                <TableCell>
                  <MonitorStatus monitor={monitor} />
                </TableCell>
                <TableCell>{monitor.interval_seconds} s</TableCell>
                <TableCell>
                  <div className='flex min-w-0 flex-col gap-0.5 text-xs'>
                    <span>
                      {formatTimestampToDate(monitor.last_checked_at)}
                    </span>
                    <span className='text-muted-foreground'>
                      {formatTimestampToDate(monitor.next_check_at)}
                    </span>
                  </div>
                </TableCell>
                <TableCell>
                  <div className='flex flex-col items-start gap-1'>
                    <StatusBadge
                      label={monitor.enabled ? t('Enabled') : t('Disabled')}
                      variant={monitor.enabled ? 'success' : 'neutral'}
                      copyable={false}
                    />
                    <span className='text-muted-foreground text-xs'>
                      {monitor.public_enabled ? t('Public') : t('Private')}
                    </span>
                  </div>
                </TableCell>
                <TableCell>
                  <MonitorActions
                    monitor={monitor}
                    running={props.runningMonitorId === monitor.id}
                    onEdit={props.onEdit}
                    onRunNow={props.onRunNow}
                    onHistory={props.onHistory}
                    onDelete={props.onDelete}
                  />
                </TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      </div>

      <div className='divide-y md:hidden'>
        {props.monitors.map((monitor) => (
          <div key={monitor.id} className='flex flex-col gap-3 p-3'>
            <div className='flex min-w-0 items-start justify-between gap-3'>
              <MonitorNames monitor={monitor} />
              <MonitorStatus monitor={monitor} />
            </div>
            <div className='grid grid-cols-2 gap-x-4 gap-y-2 text-xs'>
              <div className='min-w-0'>
                <span className='text-muted-foreground block'>
                  {t('Provider / mode')}
                </span>
                <span className='block truncate capitalize'>
                  {monitor.provider} / {monitor.api_mode.replaceAll('_', ' ')}
                </span>
              </div>
              <div>
                <span className='text-muted-foreground block'>
                  {t('Models')}
                </span>
                <span>{1 + monitor.extra_models.length}</span>
              </div>
              <div>
                <span className='text-muted-foreground block'>
                  {t('Interval')}
                </span>
                <span>{monitor.interval_seconds} s</span>
              </div>
              <div>
                <span className='text-muted-foreground block'>
                  {t('Last check')}
                </span>
                <span>{formatTimestampToDate(monitor.last_checked_at)}</span>
              </div>
              <div>
                <span className='text-muted-foreground block'>
                  {t('Next check')}
                </span>
                <span>{formatTimestampToDate(monitor.next_check_at)}</span>
              </div>
            </div>
            <div className='flex items-center justify-between gap-3'>
              <span className='text-muted-foreground text-xs'>
                {monitor.enabled ? t('Enabled') : t('Disabled')} ·{' '}
                {monitor.public_enabled ? t('Public') : t('Private')}
              </span>
              <MonitorActions
                monitor={monitor}
                running={props.runningMonitorId === monitor.id}
                onEdit={props.onEdit}
                onRunNow={props.onRunNow}
                onHistory={props.onHistory}
                onDelete={props.onDelete}
              />
            </div>
          </div>
        ))}
      </div>
    </div>
  )
}
