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
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Link } from '@tanstack/react-router'
import { ArrowLeft, CircleCheck } from 'lucide-react'
import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { ConfirmDialog } from '@/components/confirm-dialog'
import { ErrorState } from '@/components/error-state'
import { SectionPageLayout } from '@/components/layout'
import { LoadingState } from '@/components/loading-state'
import { Button } from '@/components/ui/button'
import { handleServerError } from '@/lib/handle-server-error'

import { closeUserTicket, getUserTicket, replyUserTicket } from '../api'
import { ticketQueryKeys } from '../constants'
import { TicketLogList } from './ticket-log-list'
import { TicketReplyForm } from './ticket-reply-form'
import { TicketSummary } from './ticket-summary'
import { TicketThread } from './ticket-thread'

export function UserTicketDetail(props: { ticketId: number }) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [confirmClose, setConfirmClose] = useState(false)
  const query = useQuery({
    queryKey: ticketQueryKeys.userDetail(props.ticketId),
    queryFn: () => getUserTicket(props.ticketId),
  })
  const ticket = query.data

  // Opening the ticket marks it read on the server; refresh the badge.
  useEffect(() => {
    if (!query.isSuccess) return
    void queryClient.invalidateQueries({ queryKey: ticketQueryKeys.userUnread })
  }, [query.isSuccess, query.dataUpdatedAt, queryClient])

  const refresh = () =>
    queryClient.invalidateQueries({ queryKey: ticketQueryKeys.all })

  const closeMutation = useMutation({
    mutationFn: () => closeUserTicket(props.ticketId),
    onSuccess: async (result) => {
      if (!result.success) {
        handleServerError(result)
        return
      }
      toast.success(t('Ticket closed'))
      setConfirmClose(false)
      await refresh()
    },
  })

  const replyMutation = useMutation({
    mutationFn: (input: Parameters<typeof replyUserTicket>[1]) =>
      replyUserTicket(props.ticketId, input),
  })

  const submitReply = async (input: Parameters<typeof replyUserTicket>[1]) => {
    try {
      const result = await replyMutation.mutateAsync(input)
      if (!result.success) {
        handleServerError(result)
        return false
      }
      toast.success(t('Reply sent'))
      await refresh()
      return true
    } catch (error) {
      handleServerError(error)
      return false
    }
  }

  let content: React.ReactNode
  if (query.isLoading) {
    content = <LoadingState />
  } else if (query.isError || !ticket) {
    content = (
      <ErrorState
        title={t('Failed to load ticket')}
        onRetry={() => void query.refetch()}
      />
    )
  } else {
    content = (
      <div className='mx-auto flex w-full max-w-4xl flex-col gap-4'>
        <TicketSummary ticket={ticket} audience='user' />
        <TicketLogList logs={ticket.logs ?? []} audience='user' />
        <TicketThread audience='user' messages={ticket.messages} />
        {ticket.status === 'closed' ? (
          <p className='text-muted-foreground rounded-lg border border-dashed p-4 text-center text-sm'>
            {t('This ticket is closed. Please open a new ticket.')}
          </p>
        ) : (
          <TicketReplyForm
            onSubmit={submitReply}
            isSubmitting={replyMutation.isPending}
          />
        )}
      </div>
    )
  }

  return (
    <SectionPageLayout>
      <SectionPageLayout.Breadcrumb>
        <Button
          variant='ghost'
          size='sm'
          className='-ms-2'
          render={<Link to='/tickets' />}
        >
          <ArrowLeft data-icon='inline-start' aria-hidden='true' />
          {t('Back to tickets')}
        </Button>
      </SectionPageLayout.Breadcrumb>
      <SectionPageLayout.Title>
        {t('Ticket #{{id}}', { id: props.ticketId })}
      </SectionPageLayout.Title>
      {ticket && ticket.status !== 'closed' && (
        <SectionPageLayout.Actions>
          <Button
            variant='outline'
            size='sm'
            onClick={() => setConfirmClose(true)}
          >
            <CircleCheck data-icon='inline-start' aria-hidden='true' />
            {t('Close ticket')}
          </Button>
        </SectionPageLayout.Actions>
      )}
      <SectionPageLayout.Content>{content}</SectionPageLayout.Content>
      <ConfirmDialog
        open={confirmClose}
        onOpenChange={setConfirmClose}
        title={t('Close this ticket?')}
        desc={t(
          'Close the ticket once your problem is solved. Closed tickets cannot be replied to.'
        )}
        confirmText={t('Close ticket')}
        isLoading={closeMutation.isPending}
        handleConfirm={() => closeMutation.mutate()}
      />
    </SectionPageLayout>
  )
}
