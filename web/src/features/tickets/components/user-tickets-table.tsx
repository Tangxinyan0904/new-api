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

import { getUserTickets } from '../api'
import {
  TICKET_STATUSES,
  TICKET_STATUS_VALUES,
  ticketQueryKeys,
} from '../constants'
import { useTicketsColumns } from './tickets-columns'

const route = getRouteApi('/_authenticated/tickets/')

export function UserTicketsTable() {
  const { t } = useTranslation()
  const columns = useTicketsColumns('user')
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
  const status = (
    columnFilters.find((item) => item.id === 'status')?.value as
      | string[]
      | undefined
  )?.[0]
  const params = {
    p: pagination.pageIndex + 1,
    page_size: pagination.pageSize,
    status,
  }

  const { data, isLoading, isFetching } = useQuery({
    queryKey: ticketQueryKeys.userList(params),
    queryFn: () => getUserTickets(params),
    placeholderData: (previous) => previous,
  })

  const { table } = useDataTable({
    data: data?.items ?? [],
    columns,
    columnFilters,
    pagination,
    onPaginationChange,
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
      emptyTitle={t('No tickets yet')}
      emptyDescription={t(
        'Open a ticket when you need help, and our support team will reply here.'
      )}
      skeletonKeyPrefix='tickets-skeleton'
      applyHeaderSize
      toolbarProps={{
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
        ],
      }}
    />
  )
}
