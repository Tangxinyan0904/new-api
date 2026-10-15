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
import { StatusBadge } from '@/components/status-badge'
import { Button } from '@/components/ui/button'
import { handleServerError } from '@/lib/handle-server-error'

import { setStaffTicketOwnerBan } from '../api'
import { ticketQueryKeys } from '../constants'

interface TicketOwnerBanProps {
  ticketId: number
  ownerName: string
  banned: boolean
}

/** Lets staff block a ticket's owner from opening and replying to tickets. */
export function TicketOwnerBan(props: TicketOwnerBanProps) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [confirmOpen, setConfirmOpen] = useState(false)
  const mutation = useMutation({
    mutationFn: (banned: boolean) =>
      setStaffTicketOwnerBan(props.ticketId, banned),
    onSuccess: async (result, banned) => {
      if (!result.success) {
        handleServerError(result)
        return
      }
      setConfirmOpen(false)
      toast.success(
        banned
          ? t('User blocked from tickets')
          : t('User unblocked from tickets')
      )
      await queryClient.invalidateQueries({ queryKey: ticketQueryKeys.all })
    },
  })

  if (props.banned) {
    return (
      <div className='flex items-center gap-2'>
        <StatusBadge
          label={t('Blocked from tickets')}
          variant='danger'
          copyable={false}
        />
        <Button
          variant='outline'
          size='sm'
          disabled={mutation.isPending}
          onClick={() => mutation.mutate(false)}
        >
          {t('Unblock')}
        </Button>
      </div>
    )
  }

  return (
    <>
      <Button
        variant='outline'
        size='sm'
        disabled={mutation.isPending}
        onClick={() => setConfirmOpen(true)}
      >
        {t('Block from tickets')}
      </Button>
      <ConfirmDialog
        open={confirmOpen}
        onOpenChange={setConfirmOpen}
        title={t('Block {{name}} from tickets?', { name: props.ownerName })}
        desc={t(
          'The user will not be able to open new tickets or reply to existing ones. The rest of the account is unaffected. You can unblock them here at any time.'
        )}
        confirmText={t('Block')}
        destructive
        isLoading={mutation.isPending}
        handleConfirm={() => mutation.mutate(true)}
      />
    </>
  )
}
