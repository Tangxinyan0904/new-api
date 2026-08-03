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
import { describe, expect, mock, test } from 'bun:test'

import type { Table } from '@tanstack/react-table'
import { renderToStaticMarkup } from 'react-dom/server'

mock.module('react-i18next', () => ({
  useTranslation: () => ({ t: (key: string) => key }),
}))

mock.module('../faceted-filter', () => ({
  DataTableFacetedFilter: (props: { title: string }) => (
    <span>{props.title} filter</span>
  ),
}))

mock.module('../view-options', () => ({
  DataTableViewOptions: () => null,
}))

const { DataTableToolbar } = await import('../toolbar')

const table = {
  getState: () => ({ columnFilters: [], globalFilter: '' }),
  getColumn: () => ({
    getFilterValue: () => '',
    setFilterValue: () => undefined,
  }),
  resetColumnFilters: () => undefined,
  setGlobalFilter: () => undefined,
} as unknown as Table<unknown>

describe('DataTableToolbar', () => {
  test('renders filter-adjacent content before right-aligned actions', () => {
    const html = renderToStaticMarkup(
      <DataTableToolbar
        table={table}
        filters={[
          {
            columnId: 'status',
            title: 'Status',
            options: [],
          },
        ]}
        afterFilters={<span>API key notice</span>}
        preActions={<span>Right action</span>}
        hideViewOptions
      />
    )

    const statusIndex = html.indexOf('Status filter')
    const noticeIndex = html.indexOf('API key notice')
    const actionIndex = html.indexOf('Right action')

    expect(statusIndex).toBeGreaterThanOrEqual(0)
    expect(noticeIndex).toBeGreaterThanOrEqual(0)
    expect(actionIndex).toBeGreaterThanOrEqual(0)
    expect(statusIndex).toBeLessThan(noticeIndex)
    expect(noticeIndex).toBeLessThan(actionIndex)
  })
})
