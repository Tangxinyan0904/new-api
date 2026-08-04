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
import type { ColumnDef } from '@tanstack/react-table'
import { useTranslation } from 'react-i18next'

import { StatusBadge, type StatusVariant } from '@/components/status-badge'
import { formatTimestamp } from '@/lib/format'

import {
  getViolationActionLabel,
  getViolationEffectiveLabel,
} from '../lib/violation-display'
import type { DistillationViolationRecord } from '../types'

export function useViolationRecordsColumns(): ColumnDef<DistillationViolationRecord>[] {
  const { t } = useTranslation()

  return [
    {
      accessorKey: 'triggered_at',
      header: t('Detection Time'),
      cell: ({ row }) => (
        <span className='text-muted-foreground text-sm whitespace-nowrap'>
          {formatTimestamp(row.original.triggered_at)}
        </span>
      ),
      size: 180,
    },
    {
      id: 'request_threshold',
      header: t('Request Count / Threshold'),
      cell: ({ row }) => (
        <span className='font-medium tabular-nums'>
          {row.original.request_count} / {row.original.detection_threshold}
        </span>
      ),
      size: 180,
    },
    {
      accessorKey: 'action',
      header: t('Action Taken'),
      cell: ({ row }) => {
        let variant: StatusVariant = 'neutral'
        if (row.original.action === 'temporary_limit') {
          variant = 'warning'
        } else if (row.original.action === 'permanent_ban') {
          variant = 'danger'
        }
        return (
          <StatusBadge
            label={t(getViolationActionLabel(row.original.action))}
            variant={variant}
            copyable={false}
          />
        )
      },
      size: 150,
    },
    {
      accessorKey: 'effective_until',
      header: t('Effective Until'),
      cell: ({ row }) => {
        const label = getViolationEffectiveLabel(row.original)
        return (
          <span className='text-muted-foreground text-sm whitespace-nowrap'>
            {row.original.action === 'permanent_ban' ? t(label) : label}
          </span>
        )
      },
      size: 180,
    },
  ]
}
