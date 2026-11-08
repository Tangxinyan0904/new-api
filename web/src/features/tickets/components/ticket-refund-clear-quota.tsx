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
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { ConfirmDialog } from '@/components/confirm-dialog'
import { Button } from '@/components/ui/button'
import { formatQuota } from '@/lib/format'
import { handleServerError } from '@/lib/handle-server-error'

import { clearStaffTicketOwnerQuota } from '../api'
import { ticketQueryKeys } from '../constants'

interface TicketRefundClearQuotaProps {
  ticketId: number
  ownerName: string
  quota: number
}

/** Lets staff set a refund ticket owner's balance to zero after refunding. */
export function TicketRefundClearQuota(props: TicketRefundClearQuotaProps) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [confirmOpen, setConfirmOpen] = useState(false)
  const mutation = useMutation({
    mutationFn: () => clearStaffTicketOwnerQuota(props.ticketId),
    onSuccess: async (result) => {
      if (!result.success) {
        handleServerError(result)
        return
      }
      setConfirmOpen(false)
      toast.success(t('Balance cleared'))
      await queryClient.invalidateQueries({
        queryKey: ticketQueryKeys.staffRefund(props.ticketId),
      })
    },
  })

  return (
    <>
      <Button
        variant='outline'
        size='sm'
        disabled={props.quota <= 0 || mutation.isPending}
        onClick={() => setConfirmOpen(true)}
      >
        {t('Clear balance')}
      </Button>
      <ConfirmDialog
        open={confirmOpen}
        onOpenChange={setConfirmOpen}
        title={t('Clear the balance of {{name}}?', { name: props.ownerName })}
        desc={t(
          'The remaining {{quota}} will be set to 0. Send the refund first: this is recorded in the logs and cannot be undone.',
          { quota: formatQuota(props.quota) }
        )}
        confirmText={t('Clear balance')}
        destructive
        isLoading={mutation.isPending}
        handleConfirm={() => mutation.mutate()}
      />
    </>
  )
}
