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
import { getRouteApi } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'

import { DataTablePage, useDataTable } from '@/components/data-table'
import { useTableUrlState } from '@/hooks/use-table-url-state'

import {
  approveAllPendingRebateTransferRequests,
  getRebateTransferRequests,
} from '../api'
import { useRebateApprovalsColumns } from './rebate-approvals-columns'
import { RebateApproveAllAction } from './rebate-approve-all-action'

const route = getRouteApi('/_authenticated/rebate-approvals/')

const statusOptions = [
  { label: 'Pending', value: 'pending' },
  { label: 'Approved', value: 'approved' },
  { label: 'Rejected', value: 'rejected' },
]

export function RebateApprovalsTable() {
  const { t } = useTranslation()
  const columns = useRebateApprovalsColumns()
  const {
    columnFilters,
    onColumnFiltersChange,
    pagination,
    onPaginationChange,
    ensurePageInRange,
  } = useTableUrlState({
    search: route.useSearch(),
    navigate: route.useNavigate(),
    pagination: { defaultPage: 1, defaultPageSize: 20 },
    columnFilters: [{ columnId: 'status', searchKey: 'status', type: 'array' }],
  })

  const status = columnFilters.find((item) => item.id === 'status')?.value as
    | string[]
    | undefined

  const { data, isLoading, isFetching } = useQuery({
    queryKey: [
      'rebate-approvals',
      pagination.pageIndex + 1,
      pagination.pageSize,
      status?.[0] ?? '',
    ],
    queryFn: async () => {
      const result = await getRebateTransferRequests({
        p: pagination.pageIndex + 1,
        page_size: pagination.pageSize,
        status: status?.[0],
      })
      return {
        items: result.data?.items || [],
        total: result.data?.total || 0,
      }
    },
    placeholderData: (previousData) => previousData,
  })
  const pendingCountQuery = useQuery({
    queryKey: ['rebate-approvals', 'pending-count'],
    queryFn: async () => {
      const result = await getRebateTransferRequests({
        p: 1,
        page_size: 1,
        status: 'pending',
      })
      return result.data?.total ?? 0
    },
  })

  const { table } = useDataTable({
    data: data?.items || [],
    columns,
    columnFilters,
    pagination,
    onPaginationChange,
    onColumnFiltersChange,
    manualPagination: true,
    totalCount: data?.total || 0,
    ensurePageInRange,
  })

  return (
    <DataTablePage
      table={table}
      columns={columns}
      isLoading={isLoading}
      isFetching={isFetching}
      emptyTitle={t('No Rebate Requests Found')}
      emptyDescription={t(
        'No rebate transfer requests are waiting for review.'
      )}
      skeletonKeyPrefix='rebate-approvals-skeleton'
      applyHeaderSize
      toolbarProps={{
        preActions: (
          <RebateApproveAllAction
            pendingCount={pendingCountQuery.data ?? 0}
            isCountLoading={pendingCountQuery.isLoading}
            onApproveAll={approveAllPendingRebateTransferRequests}
          />
        ),
        filters: [
          {
            columnId: 'status',
            title: t('Status'),
            options: statusOptions.map((option) => ({
              label: t(option.label),
              value: option.value,
            })),
            singleSelect: true,
          },
        ],
      }}
    />
  )
}
