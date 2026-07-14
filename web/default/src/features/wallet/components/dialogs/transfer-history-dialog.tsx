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
import { ChevronLeft, ChevronRight } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import { Dialog } from '@/components/dialog'
import { StatusBadge, type StatusVariant } from '@/components/status-badge'
import { Button } from '@/components/ui/button'
import {
  Empty,
  EmptyDescription,
  EmptyHeader,
  EmptyTitle,
} from '@/components/ui/empty'
import { Skeleton } from '@/components/ui/skeleton'
import { formatQuota, formatTimestamp } from '@/lib/format'

import { getAffiliateTransferHistory, isApiSuccess } from '../../api'
import type { AffiliateTransferHistoryItem } from '../../types'

const PAGE_SIZE = 10
const LOADING_RECORD_KEYS = ['record-1', 'record-2', 'record-3']
const LOADING_FIELD_KEYS = [
  'created',
  'invitation-reward',
  'recharge-rebate',
  'total',
  'status',
  'reviewed',
]

const TRANSFER_STATUS_CONFIG: Record<
  AffiliateTransferHistoryItem['status'],
  { labelKey: 'Approved' | 'Rejected' | 'Pending'; variant: StatusVariant }
> = {
  approved: { labelKey: 'Approved', variant: 'success' },
  rejected: { labelKey: 'Rejected', variant: 'danger' },
  pending: { labelKey: 'Pending', variant: 'warning' },
}

interface TransferHistoryDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
}

export function TransferHistoryDialog(props: TransferHistoryDialogProps) {
  const { t } = useTranslation()
  const [page, setPage] = useState(1)

  const { data, isLoading, isError, refetch } = useQuery({
    queryKey: ['affiliate-transfer-history', page, PAGE_SIZE],
    queryFn: async () => {
      const response = await getAffiliateTransferHistory(page, PAGE_SIZE)
      if (!isApiSuccess(response) || !response.data) {
        throw new Error(
          response.message || 'Failed to load affiliate transfer history'
        )
      }
      return response.data
    },
    enabled: props.open,
    staleTime: 0,
  })

  const records = data?.items ?? []
  const totalPages = Math.max(1, Math.ceil((data?.total ?? 0) / PAGE_SIZE))

  const handleOpenChange = (open: boolean) => {
    if (!open) {
      setPage(1)
    }
    props.onOpenChange(open)
  }

  return (
    <Dialog
      open={props.open}
      onOpenChange={handleOpenChange}
      title={t('Transfer History')}
      description={t(
        'View your affiliate transfer requests and review results.'
      )}
      contentClassName='max-h-[calc(100dvh-1.5rem)] max-sm:w-[calc(100vw-1.5rem)] sm:max-w-2xl'
      contentHeight='min(62dvh, 34rem)'
      bodyClassName='flex h-full min-h-0 flex-col gap-3'
    >
      <div className='min-h-0 flex-1 overflow-y-auto pr-1'>
        {isLoading && (
          <div className='flex flex-col gap-3' aria-busy='true'>
            {LOADING_RECORD_KEYS.map((recordKey) => (
              <div key={recordKey} className='rounded-md border p-3 sm:p-4'>
                <div className='grid grid-cols-2 gap-3 sm:grid-cols-3'>
                  {LOADING_FIELD_KEYS.map((fieldKey) => (
                    <div key={fieldKey} className='flex flex-col gap-2'>
                      <Skeleton className='h-3 w-20 max-w-full' />
                      <Skeleton className='h-4 w-28 max-w-full' />
                    </div>
                  ))}
                </div>
              </div>
            ))}
          </div>
        )}
        {!isLoading && isError && (
          <div
            className='flex min-h-48 flex-col items-center justify-center gap-3 text-center'
            role='alert'
          >
            <p className='text-muted-foreground text-sm'>
              {t('Failed to load transfer history')}
            </p>
            <Button variant='outline' size='sm' onClick={() => void refetch()}>
              {t('Retry')}
            </Button>
          </div>
        )}
        {!isLoading && !isError && records.length === 0 && (
          <Empty className='min-h-48'>
            <EmptyHeader>
              <EmptyTitle>{t('No transfer records found.')}</EmptyTitle>
              <EmptyDescription>
                {t('Your transfer history will appear here.')}
              </EmptyDescription>
            </EmptyHeader>
          </Empty>
        )}
        {!isLoading && !isError && records.length > 0 && (
          <div className='flex flex-col gap-3'>
            {records.map((record) => {
              const statusConfig = TRANSFER_STATUS_CONFIG[record.status]
              return (
                <article
                  key={record.id}
                  className='rounded-md border p-3 sm:p-4'
                >
                  <dl className='grid min-w-0 grid-cols-2 gap-x-3 gap-y-3 sm:grid-cols-3 sm:gap-x-4'>
                    <div className='min-w-0'>
                      <dt className='text-muted-foreground text-xs'>
                        {t('Created')}
                      </dt>
                      <dd className='mt-1 text-sm font-medium break-words tabular-nums'>
                        {formatTimestamp(record.created_at)}
                      </dd>
                    </div>
                    <div className='min-w-0'>
                      <dt className='text-muted-foreground text-xs'>
                        {t('Invitation Reward')}
                      </dt>
                      <dd className='mt-1 text-sm font-medium break-words tabular-nums'>
                        {formatQuota(record.invite_reward_quota)}
                      </dd>
                    </div>
                    <div className='min-w-0'>
                      <dt className='text-muted-foreground text-xs'>
                        {t('Recharge Rebate')}
                      </dt>
                      <dd className='mt-1 text-sm font-medium break-words tabular-nums'>
                        {formatQuota(record.recharge_rebate_quota)}
                      </dd>
                    </div>
                    <div className='min-w-0'>
                      <dt className='text-muted-foreground text-xs'>
                        {t('Total')}
                      </dt>
                      <dd className='mt-1 text-sm font-semibold break-words tabular-nums'>
                        {formatQuota(record.total_quota)}
                      </dd>
                    </div>
                    <div className='min-w-0'>
                      <dt className='text-muted-foreground text-xs'>
                        {t('Status')}
                      </dt>
                      <dd className='mt-1'>
                        <StatusBadge
                          label={t(statusConfig.labelKey)}
                          variant={statusConfig.variant}
                          copyable={false}
                        />
                      </dd>
                    </div>
                    <div className='min-w-0'>
                      <dt className='text-muted-foreground text-xs'>
                        {t('Reviewed')}
                      </dt>
                      <dd className='mt-1 text-sm font-medium break-words tabular-nums'>
                        {record.reviewed_at > 0
                          ? formatTimestamp(record.reviewed_at)
                          : '-'}
                      </dd>
                    </div>
                  </dl>

                  {record.reject_reason ? (
                    <div className='mt-3 border-t pt-3'>
                      <div className='text-muted-foreground text-xs'>
                        {t('Reject Reason')}
                      </div>
                      <p className='mt-1 text-sm break-words'>
                        {record.reject_reason}
                      </p>
                    </div>
                  ) : null}
                </article>
              )
            })}
          </div>
        )}
      </div>

      {!isLoading && !isError && records.length > 0 ? (
        <div className='grid shrink-0 grid-cols-[2rem_minmax(0,1fr)_2rem] items-center gap-2 border-t pt-3'>
          <Button
            type='button'
            variant='outline'
            size='icon-sm'
            onClick={() => setPage((currentPage) => currentPage - 1)}
            disabled={page <= 1}
            aria-label={t('Previous')}
          >
            <ChevronLeft aria-hidden='true' />
          </Button>
          <p className='text-muted-foreground min-w-0 text-center text-xs leading-4 whitespace-normal sm:text-sm'>
            {t('Page {{current}} of {{total}}', {
              current: page,
              total: totalPages,
            })}
          </p>
          <Button
            type='button'
            variant='outline'
            size='icon-sm'
            onClick={() => setPage((currentPage) => currentPage + 1)}
            disabled={page >= totalPages}
            aria-label={t('Next')}
          >
            <ChevronRight aria-hidden='true' />
          </Button>
        </div>
      ) : null}
    </Dialog>
  )
}
