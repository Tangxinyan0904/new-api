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

import { getStaffTickets } from '../api'
import {
  TICKET_CATEGORIES,
  TICKET_CATEGORY_VALUES,
  TICKET_PRIORITIES,
  TICKET_PRIORITY_VALUES,
  TICKET_STATUSES,
  TICKET_STATUS_VALUES,
  ticketQueryKeys,
} from '../constants'
import { TicketOptionSelect } from './ticket-option-select'
import { useTicketsColumns } from './tickets-columns'

const route = getRouteApi('/_authenticated/ticket-management/')

function firstFilterValue(
  filters: { id: string; value: unknown }[],
  id: string
): string | undefined {
  return (filters.find((item) => item.id === id)?.value as string[])?.[0]
}

export function StaffTicketsTable() {
  const { t } = useTranslation()
  const columns = useTicketsColumns('staff')
  const search = route.useSearch()
  const navigate = route.useNavigate()
  const {
    globalFilter,
    onGlobalFilterChange,
    columnFilters,
    onColumnFiltersChange,
    pagination,
    onPaginationChange,
    ensurePageInRange,
  } = useTableUrlState({
    search,
    navigate,
    pagination: { defaultPage: 1, defaultPageSize: 20 },
    globalFilter: { enabled: true, key: 'filter' },
    columnFilters: [
      { columnId: 'status', searchKey: 'status', type: 'array' },
      { columnId: 'category', searchKey: 'category', type: 'array' },
      { columnId: 'priority', searchKey: 'priority', type: 'array' },
    ],
  })
  const sort = search.sort ?? 'recent'
  const priority = firstFilterValue(columnFilters, 'priority')
  const params = {
    p: pagination.pageIndex + 1,
    page_size: pagination.pageSize,
    status: firstFilterValue(columnFilters, 'status'),
    category: firstFilterValue(columnFilters, 'category'),
    priority: priority ? Number(priority) : undefined,
    keyword: globalFilter?.trim() || undefined,
    sort: sort === 'priority' ? 'priority' : undefined,
  }

  const { data, isLoading, isFetching } = useQuery({
    queryKey: ticketQueryKeys.staffList(params),
    queryFn: () => getStaffTickets(params),
    placeholderData: (previous) => previous,
  })

  const { table } = useDataTable({
    data: data?.items ?? [],
    columns,
    columnFilters,
    globalFilter,
    globalFilterFn: () => true,
    pagination,
    onPaginationChange,
    onGlobalFilterChange,
    onColumnFiltersChange,
    manualPagination: true,
    totalCount: data?.total ?? 0,
    ensurePageInRange,
  })

  return (
    <DataTablePage
      table={table}
      columns={columns}
      isLoading={isLoading}
      isFetching={isFetching}
      emptyTitle={t('No tickets found')}
      emptyDescription={t('No tickets match the current filters.')}
      skeletonKeyPrefix='ticket-management-skeleton'
      applyHeaderSize
      toolbarProps={{
        searchPlaceholder: t('Search by subject, ticket ID, or username'),
        searchDebounceMs: 400,
        hideViewOptions: true,
        filters: [
          {
            columnId: 'status',
            title: t('Status'),
            options: TICKET_STATUS_VALUES.map((value) => ({
              label: t(TICKET_STATUSES[value].labelKey),
              value,
            })),
            singleSelect: true,
          },
          {
            columnId: 'category',
            title: t('Category'),
            options: TICKET_CATEGORY_VALUES.map((value) => ({
              label: t(TICKET_CATEGORIES[value].labelKey),
              value,
            })),
            singleSelect: true,
          },
          {
            columnId: 'priority',
            title: t('Priority'),
            options: TICKET_PRIORITY_VALUES.map((value) => ({
              label: t(TICKET_PRIORITIES[value].labelKey),
              value: String(value),
            })),
            singleSelect: true,
          },
        ],
        afterFilters: (
          <TicketOptionSelect
            label={t('Sort by')}
            value={sort}
            options={[
              { value: 'recent', label: t('Latest activity') },
              { value: 'priority', label: t('Priority first') },
            ]}
            onChange={(value) =>
              void navigate({
                search: (previous) => ({
                  ...previous,
                  sort: value === 'priority' ? 'priority' : undefined,
                  page: undefined,
                }),
              })
            }
          />
        ),
      }}
    />
  )
}
