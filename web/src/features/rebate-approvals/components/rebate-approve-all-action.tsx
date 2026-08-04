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
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { CheckCheck, Loader2 } from 'lucide-react'
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

import type { ApiResponse, RebateApproveAllResult } from '../types'

interface RebateApproveAllActionProps {
  pendingCount: number
  isCountLoading: boolean
  onApproveAll: () => Promise<ApiResponse<RebateApproveAllResult>>
}

class RebateApproveAllBusinessError extends Error {}

export function RebateApproveAllAction(props: RebateApproveAllActionProps) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [confirmationOpen, setConfirmationOpen] = useState(false)
  const mutation = useMutation({
    mutationFn: async () => {
      const result = await props.onApproveAll()
      if (!result.success) {
        throw new RebateApproveAllBusinessError(
          result.message || t('Approval failed')
        )
      }
      return result.data?.approved_count ?? 0
    },
    onSuccess: async (approvedCount) => {
      toast.success(
        t('Approved {{count}} rebate requests', { count: approvedCount })
      )
      setConfirmationOpen(false)
      await queryClient.invalidateQueries({ queryKey: ['rebate-approvals'] })
    },
    onError: (error) => {
      if (error instanceof RebateApproveAllBusinessError) {
        toast.error(t(error.message))
        return
      }
      toast.error(t('Network connection failed or server not responding'))
    },
  })
  const disabled =
    props.isCountLoading || props.pendingCount === 0 || mutation.isPending

  return (
    <>
      <Button
        disabled={disabled}
        title={
          props.pendingCount === 0 ? t('No pending rebate requests') : undefined
        }
        onClick={() => setConfirmationOpen(true)}
      >
        {props.isCountLoading || mutation.isPending ? (
          <Loader2 className='size-4 animate-spin' />
        ) : (
          <CheckCheck className='size-4' />
        )}
        {t('Approve All')}
      </Button>
      <AlertDialog
        open={confirmationOpen}
        onOpenChange={(open) => {
          if (!mutation.isPending) setConfirmationOpen(open)
        }}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>
              {t('Approve all pending rebate requests?')}
            </AlertDialogTitle>
            <AlertDialogDescription>
              {t(
                'Approve all {{count}} pending rebate requests and credit every user balance? This action is atomic and cannot be partially completed.',
                { count: props.pendingCount }
              )}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel disabled={mutation.isPending}>
              {t('Cancel')}
            </AlertDialogCancel>
            <AlertDialogAction
              disabled={mutation.isPending}
              onClick={(event) => {
                event.preventDefault()
                if (!mutation.isPending) mutation.mutate()
              }}
            >
              {mutation.isPending ? (
                <Loader2 className='size-4 animate-spin' />
              ) : (
                <CheckCheck className='size-4' />
              )}
              {t('Approve All')}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </>
  )
}
