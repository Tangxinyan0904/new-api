import { useQueryClient } from '@tanstack/react-query'
import type { ColumnDef } from '@tanstack/react-table'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { StatusBadge } from '@/components/status-badge'
import { formatQuota, formatTimestamp } from '@/lib/format'

import {
  approveRebateTransferRequest,
  rejectRebateTransferRequest,
} from '../api'
import { getRebateApprovalStatusConfig } from '../lib/rebate-approval-status'
import type { RebateApprovalRequest } from '../types'
import { RebateApprovalRowActions } from './rebate-approval-row-actions'

export function useRebateApprovalsColumns(): ColumnDef<RebateApprovalRequest>[] {
  const { t } = useTranslation()
  const queryClient = useQueryClient()

  const refresh = () =>
    queryClient.invalidateQueries({ queryKey: ['rebate-approvals'] })

  const approve = async (id: number) => {
    const res = await approveRebateTransferRequest(id)
    if (res.success) {
      toast.success(t('Approved'))
      refresh()
    } else {
      toast.error(res.message || t('Approval failed'))
    }
  }

  const reject = async (id: number) => {
    const res = await rejectRebateTransferRequest(id)
    if (res.success) {
      toast.success(t('Rejected'))
      refresh()
    } else {
      toast.error(res.message || t('Rejection failed'))
    }
  }

  return [
    {
      accessorKey: 'id',
      header: t('ID'),
      cell: ({ row }) => (
        <span className='text-muted-foreground font-mono text-xs'>
          #{row.original.id}
        </span>
      ),
      size: 64,
    },
    {
      accessorKey: 'username',
      header: t('Applicant'),
      cell: ({ row }) => (
        <div className='min-w-[140px]'>
          <div className='font-medium'>
            {row.original.display_name ||
              row.original.username ||
              row.original.user_id}
          </div>
          <div className='text-muted-foreground text-xs'>
            ID: {row.original.user_id}
          </div>
        </div>
      ),
      size: 170,
    },
    {
      accessorKey: 'invite_reward_quota',
      header: t('Invitation Reward'),
      cell: ({ row }) => (
        <span className='font-medium tabular-nums'>
          {formatQuota(row.original.invite_reward_quota)}
        </span>
      ),
      size: 108,
    },
    {
      accessorKey: 'recharge_rebate_quota',
      header: t('Recharge Rebate'),
      cell: ({ row }) => (
        <span className='font-medium tabular-nums'>
          {formatQuota(row.original.recharge_rebate_quota)}
        </span>
      ),
      size: 108,
    },
    {
      accessorKey: 'total_quota',
      header: t('Total'),
      cell: ({ row }) => (
        <span className='font-semibold tabular-nums'>
          {formatQuota(row.original.total_quota)}
        </span>
      ),
      size: 96,
    },
    {
      accessorKey: 'status',
      header: t('Status'),
      cell: ({ row }) => {
        const statusConfig = getRebateApprovalStatusConfig(row.original.status)
        return (
          <StatusBadge
            label={t(statusConfig.labelKey)}
            variant={statusConfig.variant}
            copyable={false}
          />
        )
      },
      filterFn: (row, id, value) => value.includes(String(row.getValue(id))),
      size: 92,
    },
    {
      accessorKey: 'created_at',
      header: t('Created'),
      cell: ({ row }) => (
        <span className='text-muted-foreground text-xs'>
          {formatTimestamp(row.original.created_at)}
        </span>
      ),
      size: 150,
    },
    {
      id: 'actions',
      header: t('Actions'),
      cell: ({ row }) => {
        return (
          <RebateApprovalRowActions
            request={row.original}
            onApprove={approve}
            onReject={reject}
          />
        )
      },
      size: 270,
      meta: { pinned: 'right' as const },
    },
  ]
}
