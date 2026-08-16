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
  Add01Icon,
  ArrowLeft01Icon,
  ArrowRight01Icon,
  Refresh01Icon,
  SearchIcon,
} from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { ConfirmDialog } from '@/components/confirm-dialog'
import { SectionPageLayout } from '@/components/layout/components/section-page-layout'
import { Button } from '@/components/ui/button'
import {
  InputGroup,
  InputGroupAddon,
  InputGroupInput,
} from '@/components/ui/input-group'
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Spinner } from '@/components/ui/spinner'
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from '@/components/ui/tooltip'
import { useDebounce } from '@/hooks/use-debounce'

import {
  channelMonitorQueryKeys,
  createChannelMonitor,
  deleteChannelMonitor,
  getChannelMonitorErrorMessage,
  getChannelMonitors,
  isChannelMonitorBusyError,
  runChannelMonitorNow,
  updateChannelMonitor,
} from './api'
import { ChannelMonitorFormDrawer } from './components/channel-monitor-form-drawer'
import { ChannelMonitorHistoryDialog } from './components/channel-monitor-history-dialog'
import { ChannelMonitorTable } from './components/channel-monitor-table'
import type { ChannelMonitorFormValues } from './lib/schema'
import type {
  ChannelMonitor,
  ChannelMonitorListParams,
  ChannelMonitorProvider,
} from './types'

type BooleanFilter = 'all' | 'true' | 'false'
type ProviderFilter = 'all' | ChannelMonitorProvider

export function ChannelMonitors() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [page, setPage] = useState(1)
  const [search, setSearch] = useState('')
  const [provider, setProvider] = useState<ProviderFilter>('all')
  const [enabled, setEnabled] = useState<BooleanFilter>('all')
  const [publicEnabled, setPublicEnabled] = useState<BooleanFilter>('all')
  const [formOpen, setFormOpen] = useState(false)
  const [editingMonitor, setEditingMonitor] = useState<ChannelMonitor | null>(
    null
  )
  const [historyMonitor, setHistoryMonitor] = useState<ChannelMonitor | null>(
    null
  )
  const [deleteMonitor, setDeleteMonitor] = useState<ChannelMonitor | null>(
    null
  )
  const debouncedSearch = useDebounce(search.trim(), 300)
  const params: ChannelMonitorListParams = {
    p: page,
    page_size: 20,
    search: debouncedSearch || undefined,
    provider: provider === 'all' ? undefined : provider,
    enabled: enabled === 'all' ? undefined : enabled === 'true',
    public_enabled:
      publicEnabled === 'all' ? undefined : publicEnabled === 'true',
  }
  const listQuery = useQuery({
    queryKey: channelMonitorQueryKeys.list(params),
    queryFn: () => getChannelMonitors(params),
    placeholderData: (previousData) => previousData,
  })
  const total = listQuery.data?.total ?? 0
  const pageSize = listQuery.data?.page_size ?? 20
  const totalPages = Math.max(1, Math.ceil(total / pageSize))

  const formMutation = useMutation({
    mutationFn: (values: ChannelMonitorFormValues) =>
      editingMonitor
        ? updateChannelMonitor(editingMonitor.id, values)
        : createChannelMonitor(values),
    onSuccess: async () => {
      toast.success(
        editingMonitor
          ? t('Channel monitor updated')
          : t('Channel monitor created')
      )
      setFormOpen(false)
      setEditingMonitor(null)
      await queryClient.invalidateQueries({
        queryKey: channelMonitorQueryKeys.all,
      })
    },
  })

  const deleteMutation = useMutation({
    mutationFn: (monitor: ChannelMonitor) => deleteChannelMonitor(monitor.id),
    onSuccess: async () => {
      toast.success(t('Channel monitor deleted'))
      setDeleteMonitor(null)
      await queryClient.invalidateQueries({
        queryKey: channelMonitorQueryKeys.all,
      })
    },
  })

  const runMutation = useMutation({
    mutationFn: (monitor: ChannelMonitor) => runChannelMonitorNow(monitor.id),
    onSuccess: async () => {
      toast.success(t('Channel monitor check completed'))
      await queryClient.invalidateQueries({
        queryKey: channelMonitorQueryKeys.all,
      })
    },
    onError: (error) => {
      if (isChannelMonitorBusyError(error)) {
        toast.error(t('This channel monitor is already running.'))
        return
      }
      toast.error(
        getChannelMonitorErrorMessage(error) ||
          t('Channel monitor check failed')
      )
    },
  })
  const runningMonitorId = runMutation.isPending
    ? (runMutation.variables?.id ?? null)
    : null

  const openCreateForm = () => {
    setEditingMonitor(null)
    setFormOpen(true)
  }

  const resetPage = () => setPage(1)

  return (
    <>
      <SectionPageLayout fixedContent>
        <SectionPageLayout.Title>
          {t('Channel monitors')}
        </SectionPageLayout.Title>
        <SectionPageLayout.Actions>
          <Button type='button' size='sm' onClick={openCreateForm}>
            <HugeiconsIcon
              icon={Add01Icon}
              strokeWidth={2}
              data-icon='inline-start'
            />
            {t('Create monitor')}
          </Button>
        </SectionPageLayout.Actions>
        <SectionPageLayout.Content>
          <div className='flex h-full min-h-0 flex-col gap-3'>
            <div className='grid shrink-0 gap-2 sm:grid-cols-2 lg:grid-cols-[minmax(16rem,1fr)_11rem_10rem_10rem_auto]'>
              <InputGroup>
                <InputGroupAddon>
                  <HugeiconsIcon icon={SearchIcon} strokeWidth={2} />
                </InputGroupAddon>
                <InputGroupInput
                  aria-label={t('Search monitors')}
                  placeholder={t('Search names or primary model')}
                  value={search}
                  onChange={(event) => {
                    setSearch(event.target.value)
                    resetPage()
                  }}
                />
              </InputGroup>

              <Select
                items={[
                  { value: 'all', label: t('All providers') },
                  { value: 'openai', label: 'OpenAI' },
                  { value: 'anthropic', label: 'Anthropic' },
                  { value: 'gemini', label: 'Gemini' },
                ]}
                value={provider}
                onValueChange={(value) => {
                  if (!value) return
                  setProvider(value)
                  resetPage()
                }}
              >
                <SelectTrigger
                  className='w-full'
                  aria-label={t('Filter by provider')}
                >
                  <SelectValue />
                </SelectTrigger>
                <SelectContent alignItemWithTrigger={false}>
                  <SelectGroup>
                    <SelectItem value='all'>{t('All providers')}</SelectItem>
                    <SelectItem value='openai'>OpenAI</SelectItem>
                    <SelectItem value='anthropic'>Anthropic</SelectItem>
                    <SelectItem value='gemini'>Gemini</SelectItem>
                  </SelectGroup>
                </SelectContent>
              </Select>

              <Select
                items={[
                  { value: 'all', label: t('Any schedule') },
                  { value: 'true', label: t('Enabled') },
                  { value: 'false', label: t('Disabled') },
                ]}
                value={enabled}
                onValueChange={(value) => {
                  if (!value) return
                  setEnabled(value)
                  resetPage()
                }}
              >
                <SelectTrigger
                  className='w-full'
                  aria-label={t('Filter by schedule state')}
                >
                  <SelectValue />
                </SelectTrigger>
                <SelectContent alignItemWithTrigger={false}>
                  <SelectGroup>
                    <SelectItem value='all'>{t('Any schedule')}</SelectItem>
                    <SelectItem value='true'>{t('Enabled')}</SelectItem>
                    <SelectItem value='false'>{t('Disabled')}</SelectItem>
                  </SelectGroup>
                </SelectContent>
              </Select>

              <Select
                items={[
                  { value: 'all', label: t('Any visibility') },
                  { value: 'true', label: t('Public') },
                  { value: 'false', label: t('Private') },
                ]}
                value={publicEnabled}
                onValueChange={(value) => {
                  if (!value) return
                  setPublicEnabled(value)
                  resetPage()
                }}
              >
                <SelectTrigger
                  className='w-full'
                  aria-label={t('Filter by visibility')}
                >
                  <SelectValue />
                </SelectTrigger>
                <SelectContent alignItemWithTrigger={false}>
                  <SelectGroup>
                    <SelectItem value='all'>{t('Any visibility')}</SelectItem>
                    <SelectItem value='true'>{t('Public')}</SelectItem>
                    <SelectItem value='false'>{t('Private')}</SelectItem>
                  </SelectGroup>
                </SelectContent>
              </Select>

              <Tooltip>
                <TooltipTrigger
                  render={
                    <Button
                      type='button'
                      variant='outline'
                      size='icon-sm'
                      aria-label={t('Refresh monitors')}
                      disabled={listQuery.isFetching}
                      onClick={() => void listQuery.refetch()}
                    />
                  }
                >
                  {listQuery.isFetching ? (
                    <Spinner />
                  ) : (
                    <HugeiconsIcon icon={Refresh01Icon} strokeWidth={2} />
                  )}
                </TooltipTrigger>
                <TooltipContent>{t('Refresh monitors')}</TooltipContent>
              </Tooltip>
            </div>

            <div className='min-h-0 flex-1 overflow-auto'>
              <ChannelMonitorTable
                monitors={listQuery.data?.items ?? []}
                isLoading={listQuery.isLoading}
                runningMonitorId={runningMonitorId}
                onEdit={(monitor) => {
                  setEditingMonitor(monitor)
                  setFormOpen(true)
                }}
                onRunNow={(monitor) => runMutation.mutate(monitor)}
                onHistory={setHistoryMonitor}
                onDelete={setDeleteMonitor}
              />
            </div>

            <div className='flex shrink-0 items-center justify-between gap-3'>
              <span className='text-muted-foreground text-xs'>
                {t('Page {{page}} of {{totalPages}} · {{total}} monitors', {
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
                  disabled={page <= 1 || listQuery.isFetching}
                  onClick={() => setPage((current) => Math.max(1, current - 1))}
                >
                  <HugeiconsIcon icon={ArrowLeft01Icon} strokeWidth={2} />
                </Button>
                <Button
                  type='button'
                  variant='outline'
                  size='icon-sm'
                  aria-label={t('Next page')}
                  disabled={page >= totalPages || listQuery.isFetching}
                  onClick={() =>
                    setPage((current) => Math.min(totalPages, current + 1))
                  }
                >
                  <HugeiconsIcon icon={ArrowRight01Icon} strokeWidth={2} />
                </Button>
              </div>
            </div>
          </div>
        </SectionPageLayout.Content>
      </SectionPageLayout>

      <ChannelMonitorFormDrawer
        open={formOpen}
        monitor={editingMonitor}
        isSubmitting={formMutation.isPending}
        onOpenChange={(open) => {
          setFormOpen(open)
          if (!open) setEditingMonitor(null)
        }}
        onSubmit={(values) => formMutation.mutateAsync(values).then(() => {})}
      />

      <ChannelMonitorHistoryDialog
        open={historyMonitor !== null}
        monitor={historyMonitor}
        onOpenChange={(open) => {
          if (!open) setHistoryMonitor(null)
        }}
      />

      <ConfirmDialog
        open={deleteMonitor !== null}
        onOpenChange={(open) => {
          if (!open) setDeleteMonitor(null)
        }}
        title={t('Delete channel monitor?')}
        desc={t(
          'This permanently deletes the monitor and all retained probe history. This action cannot be undone.'
        )}
        confirmText={t('Delete')}
        destructive
        isLoading={deleteMutation.isPending}
        handleConfirm={() => {
          if (deleteMonitor) deleteMutation.mutate(deleteMonitor)
        }}
      />
    </>
  )
}
