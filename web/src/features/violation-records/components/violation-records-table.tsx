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

import { getViolationRecords } from '../api'
import { useViolationRecordsColumns } from './violation-records-columns'

const route = getRouteApi('/_authenticated/violations/')

export function ViolationRecordsTable() {
  const { t } = useTranslation()
  const columns = useViolationRecordsColumns()
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
    globalFilter: { enabled: false },
  })

  const { data, isLoading, isFetching } = useQuery({
    queryKey: [
      'violation-records',
      pagination.pageIndex + 1,
      pagination.pageSize,
    ],
    queryFn: () =>
      getViolationRecords(pagination.pageIndex + 1, pagination.pageSize),
    placeholderData: (previousData) => previousData,
  })

  const { table } = useDataTable({
    data: data?.data?.items ?? [],
    columns,
    columnFilters,
    pagination,
    onColumnFiltersChange,
    onPaginationChange,
    manualPagination: true,
    totalCount: data?.data?.total ?? 0,
    ensurePageInRange,
  })

  return (
    <DataTablePage
      table={table}
      columns={columns}
      isLoading={isLoading}
      isFetching={isFetching}
      emptyTitle={t('No Violation Records Found')}
      emptyDescription={t(
        'No distillation violations have been recorded for this account.'
      )}
      skeletonKeyPrefix='violation-records-skeleton'
      applyHeaderSize
    />
  )
}
