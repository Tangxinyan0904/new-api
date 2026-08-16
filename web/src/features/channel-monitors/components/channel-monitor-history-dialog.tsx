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
  ArrowLeft01Icon,
  ArrowRight01Icon,
  Clock01Icon,
} from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
import { useQuery } from '@tanstack/react-query'
import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { StatusBadge, type StatusVariant } from '@/components/status-badge'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import {
  Empty,
  EmptyDescription,
  EmptyHeader,
  EmptyMedia,
  EmptyTitle,
} from '@/components/ui/empty'
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Skeleton } from '@/components/ui/skeleton'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { formatTimestampToDate } from '@/lib/format'

import { channelMonitorQueryKeys, getChannelMonitorHistory } from '../api'
import type {
  ChannelMonitor,
  ChannelMonitorCheck,
  ChannelMonitorErrorCategory,
  ChannelMonitorHistoryParams,
  ChannelMonitorStatus,
} from '../types'

type ChannelMonitorHistoryDialogProps = {
  open: boolean
  monitor: ChannelMonitor | null
  onOpenChange: (open: boolean) => void
}

const HISTORY_STATUSES: Array<Exclude<ChannelMonitorStatus, 'unknown'>> = [
  'operational',
  'degraded',
  'failed',
]

const HISTORY_ERROR_CATEGORIES: Array<
  Exclude<ChannelMonitorErrorCategory, ''>
> = [
  'network',
  'tls',
  'auth',
  'rate_limit',
  'timeout',
  'upstream',
  'invalid_response',
  'config',
]

function statusVariant(status: ChannelMonitorStatus): StatusVariant {
  if (status === 'operational') return 'success'
  if (status === 'degraded') return 'warning'
  if (status === 'failed') return 'danger'
  return 'neutral'
}

export function ChannelMonitorHistoryTable(props: {
  items: ChannelMonitorCheck[]
  isLoading: boolean
}) {
  const { t } = useTranslation()

  if (props.isLoading) {
    return (
      <div className='flex flex-col gap-2 p-3'>
        {Array.from({ length: 5 }, (_, index) => (
          <Skeleton key={index} className='h-12 w-full' />
        ))}
      </div>
    )
  }

  if (props.items.length === 0) {
    return (
      <Empty className='min-h-56'>
        <EmptyHeader>
          <EmptyMedia variant='icon'>
            <HugeiconsIcon icon={Clock01Icon} strokeWidth={2} />
          </EmptyMedia>
          <EmptyTitle>{t('No probe history found')}</EmptyTitle>
          <EmptyDescription>
            {t('No checks match the selected filters.')}
          </EmptyDescription>
        </EmptyHeader>
      </Empty>
    )
  }

  return (
    <Table>
      <TableHeader>
        <TableRow>
          <TableHead>{t('Checked at')}</TableHead>
          <TableHead>{t('Model')}</TableHead>
          <TableHead>{t('Status')}</TableHead>
          <TableHead>{t('Latency')}</TableHead>
          <TableHead>{t('HTTP')}</TableHead>
          <TableHead>{t('Category')}</TableHead>
          <TableHead className='min-w-64'>{t('Error')}</TableHead>
        </TableRow>
      </TableHeader>
      <TableBody>
        {props.items.map((item) => (
          <TableRow key={item.id}>
            <TableCell>{formatTimestampToDate(item.checked_at)}</TableCell>
            <TableCell className='max-w-52 truncate' title={item.model}>
              {item.model}
            </TableCell>
            <TableCell>
              <StatusBadge
                label={t(item.status)}
                variant={statusVariant(item.status)}
                copyable={false}
                className='capitalize'
              />
            </TableCell>
            <TableCell>{item.latency_ms} ms</TableCell>
            <TableCell>{item.http_status || '-'}</TableCell>
            <TableCell>
              {item.error_category ? t(item.error_category) : '-'}
            </TableCell>
            <TableCell className='max-w-96 whitespace-normal'>
              {item.error_message || '-'}
            </TableCell>
          </TableRow>
        ))}
      </TableBody>
    </Table>
  )
}

export function ChannelMonitorHistoryDialog(
  props: ChannelMonitorHistoryDialogProps
) {
  const { t } = useTranslation()
  const [page, setPage] = useState(1)
  const [modelFilter, setModelFilter] = useState('all')
  const [statusFilter, setStatusFilter] = useState('all')
  const [errorFilter, setErrorFilter] = useState('all')
  const monitorId = props.monitor?.id ?? 0
  const models = props.monitor
    ? [props.monitor.primary_model, ...props.monitor.extra_models]
    : []
  const params: ChannelMonitorHistoryParams = {
    p: page,
    page_size: 20,
    model: modelFilter === 'all' ? undefined : modelFilter,
    status:
      statusFilter === 'all'
        ? undefined
        : (statusFilter as ChannelMonitorStatus),
    error_category:
      errorFilter === 'all'
        ? undefined
        : (errorFilter as Exclude<ChannelMonitorErrorCategory, ''>),
  }
  const historyQuery = useQuery({
    queryKey: channelMonitorQueryKeys.history(monitorId, params),
    queryFn: () => getChannelMonitorHistory(monitorId, params),
    enabled: props.open && monitorId > 0,
    placeholderData: (previousData) => previousData,
  })

  useEffect(() => {
    if (!props.open) return
    setPage(1)
    setModelFilter('all')
    setStatusFilter('all')
    setErrorFilter('all')
  }, [monitorId, props.open])

  const total = historyQuery.data?.total ?? 0
  const pageSize = historyQuery.data?.page_size ?? 20
  const totalPages = Math.max(1, Math.ceil(total / pageSize))

  return (
    <Dialog open={props.open} onOpenChange={props.onOpenChange}>
      <DialogContent className='grid max-h-[88dvh] grid-rows-[auto_auto_minmax(0,1fr)_auto] sm:max-w-5xl'>
        <DialogHeader>
          <DialogTitle>{t('Probe history')}</DialogTitle>
          <DialogDescription>
            {props.monitor
              ? t('Administrator probe details for {{name}}.', {
                  name: props.monitor.internal_name,
                })
              : t('Administrator probe details.')}
          </DialogDescription>
        </DialogHeader>

        <div className='grid gap-2 sm:grid-cols-3'>
          <Select
            items={[
              { value: 'all', label: t('All models') },
              ...models.map((model) => ({ value: model, label: model })),
            ]}
            value={modelFilter}
            onValueChange={(value) => {
              if (!value) return
              setModelFilter(value)
              setPage(1)
            }}
          >
            <SelectTrigger className='w-full' aria-label={t('Filter by model')}>
              <SelectValue />
            </SelectTrigger>
            <SelectContent alignItemWithTrigger={false}>
              <SelectGroup>
                <SelectItem value='all'>{t('All models')}</SelectItem>
                {models.map((model) => (
                  <SelectItem key={model} value={model}>
                    {model}
                  </SelectItem>
                ))}
              </SelectGroup>
            </SelectContent>
          </Select>

          <Select
            items={[
              { value: 'all', label: t('All statuses') },
              ...HISTORY_STATUSES.map((status) => ({
                value: status,
                label: t(status),
              })),
            ]}
            value={statusFilter}
            onValueChange={(value) => {
              if (!value) return
              setStatusFilter(value)
              setPage(1)
            }}
          >
            <SelectTrigger
              className='w-full'
              aria-label={t('Filter by status')}
            >
              <SelectValue />
            </SelectTrigger>
            <SelectContent alignItemWithTrigger={false}>
              <SelectGroup>
                <SelectItem value='all'>{t('All statuses')}</SelectItem>
                {HISTORY_STATUSES.map((status) => (
                  <SelectItem key={status} value={status}>
                    {t(status)}
                  </SelectItem>
                ))}
              </SelectGroup>
            </SelectContent>
          </Select>

          <Select
            items={[
              { value: 'all', label: t('All error categories') },
              ...HISTORY_ERROR_CATEGORIES.map((category) => ({
                value: category,
                label: t(category),
              })),
            ]}
            value={errorFilter}
            onValueChange={(value) => {
              if (!value) return
              setErrorFilter(value)
              setPage(1)
            }}
          >
            <SelectTrigger
              className='w-full'
              aria-label={t('Filter by error category')}
            >
              <SelectValue />
            </SelectTrigger>
            <SelectContent alignItemWithTrigger={false}>
              <SelectGroup>
                <SelectItem value='all'>{t('All error categories')}</SelectItem>
                {HISTORY_ERROR_CATEGORIES.map((category) => (
                  <SelectItem key={category} value={category}>
                    {t(category)}
                  </SelectItem>
                ))}
              </SelectGroup>
            </SelectContent>
          </Select>
        </div>

        <div className='min-h-0 overflow-auto rounded-lg border'>
          <ChannelMonitorHistoryTable
            items={historyQuery.data?.items ?? []}
            isLoading={historyQuery.isLoading}
          />
        </div>

        <DialogFooter className='items-center sm:justify-between'>
          <span className='text-muted-foreground text-xs'>
            {t('Page {{page}} of {{totalPages}} · {{total}} records', {
              page,
              totalPages,
              total,
            })}
          </span>
          <div className='flex items-center gap-1'>
            <Button
              type='button'
              variant='outline'
              size='icon-sm'
              aria-label={t('Previous page')}
              disabled={page <= 1 || historyQuery.isFetching}
              onClick={() => setPage((current) => Math.max(1, current - 1))}
            >
              <HugeiconsIcon icon={ArrowLeft01Icon} strokeWidth={2} />
            </Button>
            <Button
              type='button'
              variant='outline'
              size='icon-sm'
              aria-label={t('Next page')}
              disabled={page >= totalPages || historyQuery.isFetching}
              onClick={() =>
                setPage((current) => Math.min(totalPages, current + 1))
              }
            >
              <HugeiconsIcon icon={ArrowRight01Icon} strokeWidth={2} />
            </Button>
          </div>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
