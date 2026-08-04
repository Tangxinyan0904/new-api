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
