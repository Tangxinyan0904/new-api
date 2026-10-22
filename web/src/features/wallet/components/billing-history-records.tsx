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
import { useTranslation } from 'react-i18next'

import { CopyButton } from '@/components/copy-button'
import {
  StaticDataTable,
  staticDataTableClassNames,
  type StaticDataTableColumn,
} from '@/components/data-table'
import { StatusBadge } from '@/components/status-badge'
import { TableId } from '@/components/table-id'
import { Button } from '@/components/ui/button'
import { useIsMobile } from '@/hooks/use-mobile'
import { formatNumber } from '@/lib/format'
import { cn } from '@/lib/utils'

import {
  formatTimestamp,
  formatTopupAmount,
  getPaymentMethodName,
  getStatusConfig,
} from '../lib/billing'
import type { TopupRecord } from '../types'

// Rows are kept short so a page fits without scrolling far.
const ROW_CELL = 'py-1'

interface BillingHistoryRecordsProps {
  records: TopupRecord[]
  isAdmin: boolean
  completing: boolean
  onComplete: (record: TopupRecord) => void
  onFilterUser: (userId: number) => void
}

/**
 * Billing records as a dense table on desktop and a compact list on phones.
 */
export function BillingHistoryRecords(props: BillingHistoryRecordsProps) {
  const { t } = useTranslation()
  const isMobile = useIsMobile()

  const renderStatus = (record: TopupRecord) => {
    const statusConfig = getStatusConfig(record.status)
    return (
      <StatusBadge
        label={t(statusConfig.label)}
        variant={statusConfig.variant}
        copyable={false}
      />
    )
  }

  const renderTradeNo = (record: TopupRecord) => (
    <div className='flex min-w-0 items-center gap-1'>
      <code className='truncate font-mono text-xs' title={record.trade_no}>
        {record.trade_no}
      </code>
      <CopyButton
        value={record.trade_no}
        className='size-6 max-md:size-5'
        iconClassName='size-3'
      />
    </div>
  )

  const renderUser = (record: TopupRecord) => (
    <Button
      variant='link'
      size='sm'
      className='h-auto max-w-full min-w-0 justify-start gap-1.5 p-0 font-normal'
      title={t("Show only this user's orders")}
      onClick={() => props.onFilterUser(record.user_id)}
    >
      <span className='truncate'>{record.username || '-'}</span>
      <TableId value={`#${record.user_id}`} className='text-xs' />
    </Button>
  )

  const renderCompleteAction = (record: TopupRecord) =>
    record.status === 'pending' ? (
      <Button
        size='sm'
        variant='outline'
        className='h-7 px-2.5 max-md:h-6'
        onClick={() => props.onComplete(record)}
        disabled={props.completing}
      >
        {t('Complete Order')}
      </Button>
    ) : null

  const columns: StaticDataTableColumn<TopupRecord>[] = [
    {
      id: 'trade_no',
      header: t('Order No.'),
      // Takes the width the other columns leave and truncates within it.
      className: cn(staticDataTableClassNames.compactHeaderCell, 'w-full'),
      cellClassName: cn(ROW_CELL, 'w-full max-w-0'),
      cell: renderTradeNo,
    },
  ]
  if (props.isAdmin) {
    columns.push({
      id: 'user',
      header: t('User'),
      className: staticDataTableClassNames.compactHeaderCell,
      cellClassName: cn(ROW_CELL, 'max-w-[180px]'),
      cell: renderUser,
    })
  }
  columns.push(
    {
      id: 'payment_method',
      header: t('Payment Method'),
      className: staticDataTableClassNames.compactHeaderCell,
      cellClassName: cn(ROW_CELL, 'whitespace-nowrap'),
      cell: (record) => getPaymentMethodName(record.payment_method, t),
    },
    {
      id: 'amount',
      header: t('Topup Amount'),
      className: staticDataTableClassNames.compactHeaderCellRight,
      cellClassName: cn(ROW_CELL, 'text-right font-medium tabular-nums'),
      cell: (record) => formatTopupAmount(record.amount),
    },
    {
      id: 'money',
      header: t('Actual Amount'),
      className: staticDataTableClassNames.compactHeaderCellRight,
      cellClassName: cn(ROW_CELL, 'text-right tabular-nums'),
      cell: (record) => formatNumber(record.money),
    },
    {
      id: 'status',
      header: t('Status'),
      className: staticDataTableClassNames.compactHeaderCell,
      cellClassName: ROW_CELL,
      cell: renderStatus,
    },
    {
      id: 'create_time',
      header: t('Created'),
      className: staticDataTableClassNames.compactHeaderCell,
      cellClassName: cn(ROW_CELL, 'text-muted-foreground whitespace-nowrap'),
      cell: (record) => formatTimestamp(record.create_time),
    }
  )
  if (props.isAdmin) {
    columns.push({
      id: 'actions',
      header: <span className='sr-only'>{t('Actions')}</span>,
      className: staticDataTableClassNames.actionHeaderCell,
      cellClassName: cn(ROW_CELL, 'text-right'),
      cell: renderCompleteAction,
    })
  }

  if (!isMobile) {
    return (
      <StaticDataTable
        className='rounded-none border-0'
        tableClassName={staticDataTableClassNames.compactTable}
        headerRowClassName={staticDataTableClassNames.mutedHeaderRow}
        tableBodyClassName='[&>tr]:h-9'
        data={props.records}
        getRowKey={(record) => record.id}
        columns={columns}
      />
    )
  }

  return (
    <ul className='divide-y'>
      {props.records.map((record) => (
        <li key={record.id} className='space-y-0.5 px-3 py-1.5'>
          <div className='flex items-center justify-between gap-2'>
            {renderTradeNo(record)}
            <span className='shrink-0 text-sm font-semibold tabular-nums'>
              {formatTopupAmount(record.amount)}
            </span>
          </div>
          <div className='text-muted-foreground flex min-w-0 items-center gap-1.5 text-xs'>
            <span className='shrink-0'>{renderStatus(record)}</span>
            <span className='truncate'>
              {getPaymentMethodName(record.payment_method, t)} ·{' '}
              {formatTimestamp(record.create_time)}
            </span>
          </div>
          <div className='flex min-h-6 items-center justify-between gap-2 text-xs'>
            <div className='min-w-0'>
              {props.isAdmin ? renderUser(record) : null}
            </div>
            <div className='flex shrink-0 items-center gap-2'>
              <span className='text-muted-foreground tabular-nums'>
                {t('Actual Amount')} {formatNumber(record.money)}
              </span>
              {props.isAdmin ? renderCompleteAction(record) : null}
            </div>
          </div>
        </li>
      ))}
    </ul>
  )
}
