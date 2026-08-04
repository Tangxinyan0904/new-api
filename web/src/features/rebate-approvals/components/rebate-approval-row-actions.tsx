import { useMutation, useQueryClient } from '@tanstack/react-query'
import { Check, Eye, Loader2, X } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '@/components/ui/alert-dialog'
import { Button } from '@/components/ui/button'

import type { ApiResponse, RebateApprovalRequest } from '../types'
import { RebateApprovalDetailDialog } from './rebate-approval-detail-dialog'

interface RebateApprovalRowActionsProps {
  request: RebateApprovalRequest
  onApprove: (id: number) => Promise<ApiResponse>
  onReject: (id: number) => Promise<ApiResponse>
}

type RebateApprovalAction = 'approve' | 'reject'

class RebateApprovalBusinessError extends Error {}

export function RebateApprovalRowActions(props: RebateApprovalRowActionsProps) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [detailsOpen, setDetailsOpen] = useState(false)
  const [confirmationAction, setConfirmationAction] =
    useState<RebateApprovalAction | null>(null)
  const canReview = props.request.status === 'pending'
  const mutation = useMutation({
    mutationFn: async (action: RebateApprovalAction) => {
      const result =
        action === 'approve'
          ? await props.onApprove(props.request.id)
          : await props.onReject(props.request.id)
      if (!result.success) {
        throw new RebateApprovalBusinessError(
          result.message ||
            t(action === 'approve' ? 'Approval failed' : 'Rejection failed')
        )
      }
      return action
    },
    onSuccess: async (action) => {
      toast.success(t(action === 'approve' ? 'Approved' : 'Rejected'))
      setConfirmationAction(null)
      await queryClient.invalidateQueries({ queryKey: ['rebate-approvals'] })
    },
    onError: (error) => {
      if (error instanceof RebateApprovalBusinessError) {
        toast.error(t(error.message))
        return
      }
      toast.error(t('Network connection failed or server not responding'))
    },
  })
  const confirmingApproval = confirmationAction === 'approve'
  const confirmationTitle = confirmingApproval
    ? t('Confirm rebate approval')
    : t('Confirm rebate rejection')
  const confirmationDescription = confirmingApproval
    ? t('Approve this rebate transfer request and credit the user balance?')
    : t(
        'Reject this rebate transfer request? The submitted reward will be permanently removed.'
      )

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
        <Button
          size='sm'
          disabled={!canReview || mutation.isPending}
          onClick={() => setConfirmationAction('approve')}
        >
          {mutation.isPending && confirmingApproval ? (
            <Loader2 className='size-4 animate-spin' />
          ) : (
            <Check className='size-4' />
          )}
          {t('Approve')}
        </Button>
        <Button
          size='sm'
          variant='destructive'
          disabled={!canReview || mutation.isPending}
          onClick={() => setConfirmationAction('reject')}
        >
          {mutation.isPending && !confirmingApproval ? (
            <Loader2 className='size-4 animate-spin' />
          ) : (
            <X className='size-4' />
          )}
          {t('Reject')}
        </Button>
      </div>
      <AlertDialog
        open={confirmationAction !== null}
        onOpenChange={(open) => {
          if (!open && !mutation.isPending) setConfirmationAction(null)
        }}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{confirmationTitle}</AlertDialogTitle>
            <AlertDialogDescription>
              {confirmationDescription}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel disabled={mutation.isPending}>
              {t('Cancel')}
            </AlertDialogCancel>
            <AlertDialogAction
              variant={confirmingApproval ? 'default' : 'destructive'}
              disabled={mutation.isPending}
              onClick={(event) => {
                event.preventDefault()
                if (confirmationAction && !mutation.isPending) {
                  mutation.mutate(confirmationAction)
                }
              }}
            >
              {mutation.isPending && (
                <Loader2 className='size-4 animate-spin' />
              )}
              {t(confirmingApproval ? 'Approve' : 'Reject')}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
      <RebateApprovalDetailDialog
        requestId={props.request.id}
        open={detailsOpen}
        onOpenChange={setDetailsOpen}
      />
    </>
  )
}
