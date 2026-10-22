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
import { Search, ChevronLeft, ChevronRight, X } from 'lucide-react'
import { useState, type ReactNode } from 'react'
import { useTranslation } from 'react-i18next'

import { ConfirmDialog } from '@/components/confirm-dialog'
import { Dialog } from '@/components/dialog'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Skeleton } from '@/components/ui/skeleton'
import { formatNumber } from '@/lib/format'

import { useBillingHistory } from '../../hooks/use-billing-history'
import { STATUS_CONFIG, formatTopupAmount } from '../../lib/billing'
import type { TopupRecord, TopupStatus } from '../../types'
import { BillingHistoryRecords } from '../billing-history-records'

interface BillingHistoryDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
}

const ALL_STATUSES = 'all'

export function BillingHistoryDialog(props: BillingHistoryDialogProps) {
  const { t } = useTranslation()
  const billing = useBillingHistory()
  const [confirmRecord, setConfirmRecord] = useState<TopupRecord | null>(null)

  const totalPages = Math.max(1, Math.ceil(billing.total / billing.pageSize))
  const hasFilters =
    billing.keyword !== '' || billing.user !== '' || billing.status !== null
  const statusItems = [
    { value: ALL_STATUSES, label: t('All Status') },
    ...Object.entries(STATUS_CONFIG).map(([value, config]) => ({
      value,
      label: t(config.label),
    })),
  ]
  const pageSizeItems = [
    { value: '10', label: t('10 / page') },
    { value: '20', label: t('20 / page') },
    { value: '50', label: t('50 / page') },
    { value: '100', label: t('100 / page') },
  ]

  const handleConfirmComplete = async () => {
    if (!confirmRecord) return
    const success = await billing.handleCompleteOrder(confirmRecord.trade_no)
    if (success) {
      setConfirmRecord(null)
    }
  }

  let content: ReactNode
  if (billing.loading) {
    content = (
      <div className='space-y-2 p-3'>
        {['a', 'b', 'c', 'd', 'e', 'f', 'g', 'h'].map((placeholder) => (
          <Skeleton key={placeholder} className='h-8 w-full' />
        ))}
      </div>
    )
  } else if (billing.records.length === 0) {
    content = (
      <div className='text-muted-foreground flex h-full min-h-40 flex-col items-center justify-center py-10 text-center'>
        <p className='text-sm font-medium'>{t('No billing records found')}</p>
        <p className='mt-1 text-xs'>
          {hasFilters
            ? t('Try adjusting your search')
            : t('Your transaction history will appear here')}
        </p>
      </div>
    )
  } else {
    content = (
      <BillingHistoryRecords
        records={billing.records}
        isAdmin={billing.isAdmin}
        completing={billing.completing}
        onComplete={setConfirmRecord}
        onFilterUser={(userId) => billing.handleUserChange(String(userId))}
      />
    )
  }

  return (
    <>
      <Dialog
        open={props.open}
        onOpenChange={props.onOpenChange}
        title={t('Billing History')}
        description={t(
          'View your topup transaction records and payment history'
        )}
        contentClassName='max-h-(--dialog-available-height) max-sm:w-screen max-sm:max-w-none max-sm:rounded-none max-sm:p-4 sm:max-w-6xl'
        contentHeight='min(80dvh, 880px)'
        bodyClassName='flex h-full flex-col gap-3'
      >
        {/* Filters */}
        <div className='flex shrink-0 flex-wrap items-center gap-2'>
          <div className='relative basis-full sm:min-w-48 sm:flex-1 sm:basis-0'>
            <Search className='text-muted-foreground absolute top-1/2 left-3 h-4 w-4 -translate-y-1/2' />
            <Input
              placeholder={t('Search by order number...')}
              aria-label={t('Search by order number...')}
              value={billing.keyword}
              onChange={(e) => billing.handleSearch(e.target.value)}
              className='h-9 pl-10'
            />
          </div>
          {billing.isAdmin && (
            <>
              <Input
                placeholder={t('Search by username or user ID')}
                aria-label={t('Search by username or user ID')}
                value={billing.user}
                onChange={(e) => billing.handleUserChange(e.target.value)}
                className='h-9 min-w-0 flex-1 sm:max-w-56'
              />
              <Select
                items={statusItems}
                value={billing.status ?? ALL_STATUSES}
                onValueChange={(value) =>
                  value !== null &&
                  billing.handleStatusChange(
                    value === ALL_STATUSES ? null : (value as TopupStatus)
                  )
                }
              >
                <SelectTrigger
                  className='h-9 w-28 sm:w-32'
                  aria-label={t('Status')}
                >
                  <SelectValue />
                </SelectTrigger>
                <SelectContent alignItemWithTrigger={false}>
                  <SelectGroup>
                    {statusItems.map((item) => (
                      <SelectItem key={item.value} value={item.value}>
                        {item.label}
                      </SelectItem>
                    ))}
                  </SelectGroup>
                </SelectContent>
              </Select>
            </>
          )}
          <Select
            items={pageSizeItems}
            value={billing.pageSize.toString()}
            onValueChange={(value) =>
              value !== null &&
              billing.handlePageSizeChange(Number.parseInt(value))
            }
          >
            <SelectTrigger className='h-9 w-28 max-sm:hidden'>
              <SelectValue />
            </SelectTrigger>
            <SelectContent alignItemWithTrigger={false}>
              <SelectGroup>
                {pageSizeItems.map((item) => (
                  <SelectItem key={item.value} value={item.value}>
                    {item.label}
                  </SelectItem>
                ))}
              </SelectGroup>
            </SelectContent>
          </Select>
          {hasFilters && (
            <Button
              variant='ghost'
              size='sm'
              className='h-9'
              onClick={billing.handleClearFilters}
            >
              <X className='h-4 w-4' />
              <span className='max-sm:sr-only'>{t('Clear filters')}</span>
            </Button>
          )}
        </div>

        {/* Records */}
        <div className='min-h-0 flex-1 overflow-auto rounded-md border'>
          {content}
        </div>

        {/* Pagination */}
        <div className='flex shrink-0 items-center justify-between gap-3'>
          <div className='text-muted-foreground text-xs sm:text-sm'>
            {billing.total > 0 && (
              <>
                {t('Showing')} {(billing.page - 1) * billing.pageSize + 1}-
                {Math.min(billing.page * billing.pageSize, billing.total)}{' '}
                {t('of')} {formatNumber(billing.total)}
              </>
            )}
          </div>
          <div className='flex items-center gap-2'>
            <Button
              variant='outline'
              size='sm'
              onClick={() => billing.handlePageChange(billing.page - 1)}
              disabled={billing.page <= 1 || billing.loading}
              className='h-8 w-8 p-0'
              aria-label={t('Previous')}
            >
              <ChevronLeft className='h-4 w-4' />
            </Button>
            <div className='text-muted-foreground flex items-center gap-1 text-sm tabular-nums'>
              <span className='text-foreground font-medium'>
                {billing.page}
              </span>
              <span>/</span>
              <span>{totalPages}</span>
            </div>
            <Button
              variant='outline'
              size='sm'
              onClick={() => billing.handlePageChange(billing.page + 1)}
              disabled={billing.page >= totalPages || billing.loading}
              className='h-8 w-8 p-0'
              aria-label={t('Next')}
            >
              <ChevronRight className='h-4 w-4' />
            </Button>
          </div>
        </div>
      </Dialog>

      <ConfirmDialog
        open={confirmRecord !== null}
        onOpenChange={(open) => !open && setConfirmRecord(null)}
        title={t('Complete Order')}
        confirmText={t('Confirm')}
        isLoading={billing.completing}
        handleConfirm={handleConfirmComplete}
        desc={
          <div className='space-y-3'>
            <p>
              {t(
                'Are you sure you want to manually complete this order? The user will be credited with the corresponding quota.'
              )}
            </p>
            {confirmRecord && (
              <dl className='bg-muted/50 text-foreground grid grid-cols-[auto_1fr] gap-x-4 gap-y-1.5 rounded-md p-3 text-sm'>
                <dt className='text-muted-foreground'>{t('Order No.')}</dt>
                <dd className='font-mono break-all'>
                  {confirmRecord.trade_no}
                </dd>
                <dt className='text-muted-foreground'>{t('User')}</dt>
                <dd>
                  {confirmRecord.username || '-'} #{confirmRecord.user_id}
                </dd>
                <dt className='text-muted-foreground'>{t('Topup Amount')}</dt>
                <dd className='tabular-nums'>
                  {formatTopupAmount(confirmRecord.amount)}
                </dd>
                <dt className='text-muted-foreground'>{t('Actual Amount')}</dt>
                <dd className='tabular-nums'>
                  {formatNumber(confirmRecord.money)}
                </dd>
              </dl>
            )}
            <p className='text-warning'>
              {t(
                'Check that the payment arrived on the payment platform before completing it.'
              )}
            </p>
          </div>
        }
      />
    </>
  )
}
