import { useState } from 'react'
import { Check, Eye, X } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { Button } from '@/components/ui/button'
import type { RebateApprovalRequest } from '../types'
import { RebateApprovalDetailDialog } from './rebate-approval-detail-dialog'

interface RebateApprovalRowActionsProps {
  request: RebateApprovalRequest
  onApprove: (id: number) => Promise<void>
  onReject: (id: number) => Promise<void>
}

export function RebateApprovalRowActions({
  request,
  onApprove,
  onReject,
}: RebateApprovalRowActionsProps) {
  const { t } = useTranslation()
  const [detailsOpen, setDetailsOpen] = useState(false)
  const pending = request.status === 'pending'

  return (
    <>
      <div className='flex items-center justify-end gap-2'>
        <Button
          size='sm'
          variant='outline'
          onClick={() => setDetailsOpen(true)}
        >
          <Eye className='size-4' />
          {t('Details')}
        </Button>
        <Button size='sm' disabled={!pending} onClick={() => onApprove(request.id)}>
          <Check className='size-4' />
          {t('Approve')}
        </Button>
        <Button
          size='sm'
          variant='outline'
          disabled={!pending}
          onClick={() => onReject(request.id)}
        >
          <X className='size-4' />
          {t('Reject')}
        </Button>
      </div>
      <RebateApprovalDetailDialog
        requestId={request.id}
        open={detailsOpen}
        onOpenChange={setDetailsOpen}
      />
    </>
  )
}
