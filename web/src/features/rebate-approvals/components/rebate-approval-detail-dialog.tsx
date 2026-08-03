import { useQuery } from '@tanstack/react-query'
import { Loader2 } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { formatQuota, formatTimestamp } from '@/lib/format'
import { cn } from '@/lib/utils'
import { Dialog } from '@/components/dialog'
import { Label } from '@/components/ui/label'
import { getRebateTransferRequestDetail } from '../api'
import type { RebateApprovalDetail } from '../types'

interface RebateApprovalDetailDialogProps {
  requestId: number
  open: boolean
  onOpenChange: (open: boolean) => void
}

function DetailRow(props: {
  label: React.ReactNode
  value: React.ReactNode
  mono?: boolean
  strong?: boolean
}) {
  return (
    <div className='grid min-w-0 grid-cols-[7.5rem_minmax(0,1fr)] gap-3 text-sm'>
      <span className='text-muted-foreground min-w-0 text-xs'>
        {props.label}
      </span>
      <span
        className={cn(
          'min-w-0 text-xs break-all',
          props.mono && 'font-mono',
          props.strong && 'font-semibold'
        )}
      >
        {props.value}
      </span>
    </div>
  )
}

function DetailSection(props: {
  label: string
  children: React.ReactNode
}) {
  return (
    <div className='min-w-0 space-y-1.5'>
      <Label className='text-xs font-semibold'>{props.label}</Label>
      <div className='bg-muted/30 min-w-0 space-y-1.5 overflow-hidden rounded-md border p-3'>
        {props.children}
      </div>
    </div>
  )
}

function DetailContent({ detail }: { detail: RebateApprovalDetail }) {
  const { t } = useTranslation()
  const applicant = detail.display_name || detail.username || detail.user_id
  const sourceCount = detail.recharge_sources?.length ?? 0

  return (
    <div className='w-full max-w-full min-w-0 space-y-4 overflow-x-hidden py-1'>
      <DetailSection label={t('Request Summary')}>
        <DetailRow
          label={t('Applicant')}
          value={`${applicant} (ID: ${detail.user_id})`}
          strong
        />
        <DetailRow label={t('Applicant ID')} value={detail.user_id} mono />
        <DetailRow
          label={t('Invitation Reward')}
          value={formatQuota(detail.invite_reward_quota)}
          mono
        />
        <DetailRow
          label={t('Recharge Rebate')}
          value={`${formatQuota(detail.recharge_rebate_quota)} (${(detail.recharge_rebate_rate * 100).toFixed(0)}%)`}
          mono
        />
        <DetailRow
          label={t('Total')}
          value={formatQuota(detail.total_quota)}
          mono
          strong
        />
        <DetailRow
          label={t('Created')}
          value={formatTimestamp(detail.created_at)}
          mono
        />
      </DetailSection>

      <DetailSection label={t('Recharge Rebate Sources')}>
        <DetailRow label={t('Invited Users')} value={detail.invited_count} mono />
        <DetailRow
          label={t('Invited Recharge')}
          value={formatQuota(detail.total_invited_recharge_quota)}
          mono
        />
        <DetailRow label={t('Source Records')} value={sourceCount} mono />
      </DetailSection>

      <div className='min-w-0 space-y-2'>
        <Label className='text-xs font-semibold'>{t('Source Details')}</Label>
        {sourceCount > 0 && (
          <div className='space-y-2'>
            {detail.recharge_sources.map((source) => (
              <div
                key={`${source.invited_user_id}-${source.complete_time}-${source.credited_quota}-${source.rebate_quota}`}
                className='bg-background min-w-0 rounded-md border p-3'
              >
                <div className='mb-2 flex min-w-0 items-center justify-between gap-2'>
                  <div className='min-w-0'>
                    <div className='truncate text-sm font-medium'>
                      {source.invited_display_name || '***'}
                    </div>
                    <div className='text-muted-foreground text-xs'>
                      {t('Invited User ID')}: {source.invited_user_id}
                    </div>
                  </div>
                  <div className='text-right'>
                    <div className='text-sm font-semibold tabular-nums'>
                      {formatQuota(source.rebate_quota)}
                    </div>
                    <div className='text-muted-foreground text-xs'>
                      {t('Rebate')}
                    </div>
                  </div>
                </div>
                <div className='grid gap-1.5 text-xs sm:grid-cols-2'>
                  <DetailRow
                    label={t('Credited')}
                    value={formatQuota(source.credited_quota)}
                    mono
                  />
                  <DetailRow
                    label={t('Completed')}
                    value={formatTimestamp(source.complete_time)}
                    mono
                  />
                  <DetailRow
                    label={t('Provider')}
                    value={source.payment_provider || '-'}
                    mono
                  />
                  <DetailRow
                    label={t('Method')}
                    value={source.payment_method || '-'}
                    mono
                  />
                </div>
              </div>
            ))}
          </div>
        )}
        {sourceCount === 0 && (
          <div className='text-muted-foreground bg-muted/30 rounded-md border p-3 text-sm'>
            {t('No recharge rebate source records found.')}
          </div>
        )}
      </div>
    </div>
  )
}

export function RebateApprovalDetailDialog({
  requestId,
  open,
  onOpenChange,
}: RebateApprovalDetailDialogProps) {
  const { t } = useTranslation()
  const { data, isLoading } = useQuery({
    queryKey: ['rebate-approval-detail', requestId],
    queryFn: async () => {
      const result = await getRebateTransferRequestDetail(requestId)
      return result.data
    },
    enabled: open,
  })
  let content = (
    <div className='text-muted-foreground flex min-h-40 items-center justify-center text-sm'>
      {t('No details found.')}
    </div>
  )
  if (isLoading) {
    content = (
      <div className='flex min-h-40 items-center justify-center'>
        <Loader2 className='text-muted-foreground size-5 animate-spin' />
      </div>
    )
  } else if (data) {
    content = <DetailContent detail={data} />
  }

  return (
    <Dialog
      open={open}
      onOpenChange={onOpenChange}
      title={t('Rebate Request Details')}
      description={t('View the pending amount sources for this rebate transfer request.')}
      contentClassName='max-sm:w-[calc(100vw-1.5rem)] sm:max-w-2xl'
      contentHeight='min(70dvh, 680px)'
      bodyClassName='pr-2 sm:pr-4'
    >
      {content}
    </Dialog>
  )
}
