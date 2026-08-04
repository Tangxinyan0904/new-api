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
