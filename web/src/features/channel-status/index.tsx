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
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import {
  Activity,
  CircleAlert,
  Loader2,
  Plus,
  RefreshCw,
  Timer,
} from 'lucide-react'
import { useCallback, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { ConfirmDialog } from '@/components/confirm-dialog'
import { SectionPageLayout } from '@/components/layout'
import { Button } from '@/components/ui/button'
import {
  Empty,
  EmptyDescription,
  EmptyHeader,
  EmptyMedia,
  EmptyTitle,
} from '@/components/ui/empty'
import { Skeleton } from '@/components/ui/skeleton'
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from '@/components/ui/tooltip'
import { hasPermission } from '@/lib/admin-permissions'
import { handleServerError } from '@/lib/handle-server-error'
import { useAuthStore } from '@/stores/auth-store'

import {
  channelStatusQueryKeys,
  deleteChannelStatusMonitor,
  getChannelStatus,
} from './api'
import { MonitorFormDialog } from './components/monitor-form-dialog'
import { MonitorStatusCard } from './components/monitor-status-card'
import { MonitorStatusDetails } from './components/monitor-status-details'
import { StatusSummary } from './components/status-summary'
import { useChannelStatusRefresh } from './hooks/use-channel-status-refresh'
import type { ChannelStatusItem } from './types'

function ChannelStatusLoading() {
  return (
    <div className='space-y-4'>
      <Skeleton className='h-24 w-full' />
      <div className='grid gap-4 md:grid-cols-2 xl:grid-cols-3'>
        {Array.from({ length: 3 }, (_, index) => (
          <Skeleton key={index} className='h-[15.5rem] w-full' />
        ))}
      </div>
    </div>
  )
}

function RefreshControls(props: {
  enabled: boolean
  secondsRemaining: number
  isRefreshing: boolean
  isFetching: boolean
  onToggle: () => void
  onRefresh: () => Promise<void>
}) {
  const { t } = useTranslation()
  const refreshing = props.isRefreshing || props.isFetching

  return (
    <div className='flex min-w-0 items-center gap-1.5'>
      <Tooltip>
        <TooltipTrigger
          render={
            <Button
              type='button'
              size='icon-sm'
              variant='outline'
              aria-label={t('Refresh channel status')}
              disabled={refreshing}
              onClick={() => void props.onRefresh()}
            />
          }
        >
          {refreshing ? (
            <Loader2 className='animate-spin' aria-hidden='true' />
          ) : (
            <RefreshCw aria-hidden='true' />
          )}
        </TooltipTrigger>
        <TooltipContent>{t('Refresh channel status')}</TooltipContent>
      </Tooltip>

      <Tooltip>
        <TooltipTrigger
          render={
            <Button
              type='button'
              size='icon-sm'
              variant={props.enabled ? 'secondary' : 'outline'}
              aria-label={
                props.enabled
                  ? t('Pause automatic refresh')
                  : t('Resume automatic refresh')
              }
              aria-pressed={props.enabled}
              onClick={props.onToggle}
            />
          }
        >
          <Timer aria-hidden='true' />
        </TooltipTrigger>
        <TooltipContent>
          {props.enabled
            ? t('Pause automatic refresh')
            : t('Resume automatic refresh')}
        </TooltipContent>
      </Tooltip>

      <span className='text-muted-foreground min-w-[6.75rem] text-xs tabular-nums'>
        {props.enabled
          ? t('Refreshes in {{seconds}}s', {
              seconds: props.secondsRemaining,
            })
          : t('Auto refresh paused')}
      </span>
    </div>
  )
}

export function ChannelStatus() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const user = useAuthStore((state) => state.auth.user)
  const canManage = hasPermission(user, 'channel', 'write')
  const [selectedMonitorId, setSelectedMonitorId] = useState<number | null>(
    null
  )
  const [formOpen, setFormOpen] = useState(false)
  const [editingMonitor, setEditingMonitor] =
    useState<ChannelStatusItem | null>(null)
  const [deletingMonitor, setDeletingMonitor] =
    useState<ChannelStatusItem | null>(null)
  const statusQuery = useQuery({
    queryKey: channelStatusQueryKeys.status,
    queryFn: getChannelStatus,
  })
  const refreshAll = useCallback(async () => {
    await queryClient.refetchQueries({
      queryKey: channelStatusQueryKeys.all,
      type: 'active',
    })
  }, [queryClient])
  const refresh = useChannelStatusRefresh(refreshAll)

  const handleMonitorSelect = useCallback((monitor: ChannelStatusItem) => {
    setSelectedMonitorId(monitor.id)
  }, [])

  const openForm = (monitor: ChannelStatusItem | null) => {
    setEditingMonitor(monitor)
    setFormOpen(true)
  }

  const deleteMutation = useMutation({
    mutationFn: (monitor: ChannelStatusItem) =>
      deleteChannelStatusMonitor(monitor.id),
    onSuccess: async (result) => {
      if (!result.success) {
        handleServerError(result)
        return
      }
      toast.success(t('Monitor deleted'))
      setDeletingMonitor(null)
      await queryClient.invalidateQueries({
        queryKey: channelStatusQueryKeys.all,
      })
    },
  })

  return (
    <TooltipProvider>
      <SectionPageLayout>
        <SectionPageLayout.Title>{t('Channel Status')}</SectionPageLayout.Title>
        <SectionPageLayout.Actions>
          <RefreshControls
            enabled={refresh.enabled}
            secondsRemaining={refresh.secondsRemaining}
            isRefreshing={refresh.isRefreshing}
            isFetching={statusQuery.isFetching}
            onToggle={refresh.toggle}
            onRefresh={refresh.refreshNow}
          />
          {canManage ? (
            <Button type='button' size='sm' onClick={() => openForm(null)}>
              <Plus data-icon='inline-start' aria-hidden='true' />
              {t('Add monitor')}
            </Button>
          ) : null}
        </SectionPageLayout.Actions>
        <SectionPageLayout.Content>
          {statusQuery.isLoading ? <ChannelStatusLoading /> : null}
          {statusQuery.isError && !statusQuery.data ? (
            <Empty className='min-h-72 border'>
              <EmptyHeader>
                <EmptyMedia variant='icon'>
                  <CircleAlert aria-hidden='true' />
                </EmptyMedia>
                <EmptyTitle>{t('Channel status unavailable')}</EmptyTitle>
                <EmptyDescription>
                  {t('Try refreshing the page in a moment.')}
                </EmptyDescription>
              </EmptyHeader>
            </Empty>
          ) : null}
          {statusQuery.data ? (
            <div className='space-y-4'>
              <StatusSummary data={statusQuery.data} />
              {statusQuery.data.items.length === 0 ? (
                <Empty className='min-h-72 border'>
                  <EmptyHeader>
                    <EmptyMedia variant='icon'>
                      <Activity aria-hidden='true' />
                    </EmptyMedia>
                    <EmptyTitle>
                      {t('No published channel monitors')}
                    </EmptyTitle>
                    <EmptyDescription>
                      {canManage
                        ? t('Add a monitor to show how a group is doing.')
                        : t('No channel status data is currently published.')}
                    </EmptyDescription>
                  </EmptyHeader>
                </Empty>
              ) : (
                <div className='grid min-w-0 gap-4 md:grid-cols-2 xl:grid-cols-3'>
                  {statusQuery.data.items.map((monitor) => (
                    <MonitorStatusCard
                      key={monitor.id}
                      monitor={monitor}
                      bucketSeconds={statusQuery.data.bucket_seconds}
                      onSelect={handleMonitorSelect}
                      onEdit={canManage ? openForm : undefined}
                      onDelete={canManage ? setDeletingMonitor : undefined}
                    />
                  ))}
                </div>
              )}
            </div>
          ) : null}
        </SectionPageLayout.Content>
      </SectionPageLayout>

      <MonitorStatusDetails
        monitorId={selectedMonitorId}
        open={selectedMonitorId !== null}
        onOpenChange={(open) => {
          if (!open) setSelectedMonitorId(null)
        }}
      />
      {canManage ? (
        <>
          <MonitorFormDialog
            open={formOpen}
            monitor={editingMonitor}
            onOpenChange={setFormOpen}
          />
          <ConfirmDialog
            open={deletingMonitor !== null}
            onOpenChange={(open) => {
              if (!open) setDeletingMonitor(null)
            }}
            title={t('Delete monitor {{name}}?', {
              name: deletingMonitor?.public_name ?? '',
            })}
            desc={t(
              'The group is no longer shown on the status page. Its call logs are kept.'
            )}
            confirmText={t('Delete')}
            destructive
            isLoading={deleteMutation.isPending}
            handleConfirm={() => {
              if (deletingMonitor) deleteMutation.mutate(deletingMonitor)
            }}
          />
        </>
      ) : null}
    </TooltipProvider>
  )
}
