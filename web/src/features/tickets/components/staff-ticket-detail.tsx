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
import { ArrowLeft } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { ErrorState } from '@/components/error-state'
import { SectionPageLayout } from '@/components/layout'
import { LoadingState } from '@/components/loading-state'
import { Button } from '@/components/ui/button'
import { hasPermission } from '@/lib/admin-permissions'
import { handleServerError } from '@/lib/handle-server-error'
import { useAuthStore } from '@/stores/auth-store'

import { getStaffTicket, replyStaffTicket, updateStaffTicket } from '../api'
import {
  TICKET_PRIORITIES,
  TICKET_PRIORITY_VALUES,
  TICKET_REPLY_STATUSES,
  TICKET_STATUSES,
  TICKET_STATUS_VALUES,
  ticketQueryKeys,
} from '../constants'
import type { TicketPriority, TicketStatus } from '../types'
import { TicketOptionSelect } from './ticket-option-select'
import { TicketReplyForm } from './ticket-reply-form'
import { TicketSummary } from './ticket-summary'
import { TicketThread } from './ticket-thread'

export function StaffTicketDetail(props: { ticketId: number }) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const user = useAuthStore((state) => state.auth.user)
  const canWrite = hasPermission(user, 'ticket', 'write')
  const [replyStatus, setReplyStatus] = useState<TicketStatus>('waiting_user')
  const query = useQuery({
    queryKey: ticketQueryKeys.staffDetail(props.ticketId),
    queryFn: () => getStaffTicket(props.ticketId),
  })
  const ticket = query.data

  const refresh = () =>
    queryClient.invalidateQueries({ queryKey: ticketQueryKeys.all })

  const updateMutation = useMutation({
    mutationFn: (changes: {
      status?: TicketStatus
      priority?: TicketPriority
    }) => updateStaffTicket(props.ticketId, changes),
    onSuccess: async (result) => {
      if (!result.success) {
        handleServerError(result)
        return
      }
      toast.success(t('Ticket updated'))
      await refresh()
    },
  })

  const replyMutation = useMutation({
    mutationFn: (input: Parameters<typeof replyStaffTicket>[1]) =>
      replyStaffTicket(props.ticketId, input),
  })

  const submitReply = async (input: { content: string; images: File[] }) => {
    try {
      const result = await replyMutation.mutateAsync({
        ...input,
        status: replyStatus,
      })
      if (!result.success) {
        handleServerError(result)
        return false
      }
      toast.success(t('Reply sent'))
      setReplyStatus('waiting_user')
      await refresh()
      return true
    } catch (error) {
      handleServerError(error)
      return false
    }
  }

  const statusOptions = TICKET_STATUS_VALUES.map((value) => ({
    value,
    label: t(TICKET_STATUSES[value].labelKey),
  }))
  const priorityOptions = TICKET_PRIORITY_VALUES.map((value) => ({
    value: String(value),
    label: t(TICKET_PRIORITIES[value].labelKey),
  }))
  const replyStatusOptions = TICKET_REPLY_STATUSES.map((value) => ({
    value,
    label: t(TICKET_STATUSES[value].labelKey),
  }))

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
        <TicketSummary ticket={ticket} audience='staff' />
        {canWrite && (
          <div className='flex flex-wrap items-center gap-3'>
            <TicketOptionSelect
              label={t('Status')}
              value={ticket.status}
              options={statusOptions}
              disabled={updateMutation.isPending}
              onChange={(value) =>
                updateMutation.mutate({ status: value as TicketStatus })
              }
            />
            <TicketOptionSelect
              label={t('Priority')}
              value={String(ticket.priority)}
              options={priorityOptions}
              disabled={updateMutation.isPending}
              onChange={(value) =>
                updateMutation.mutate({
                  priority: Number(value) as TicketPriority,
                })
              }
            />
          </div>
        )}
        <TicketThread
          audience='staff'
          messages={ticket.messages}
          ownerName={ticket.username}
        />
        {canWrite && (
          <TicketReplyForm
            onSubmit={submitReply}
            isSubmitting={replyMutation.isPending}
            actions={
              <TicketOptionSelect
                label={t('After reply')}
                value={replyStatus}
                options={replyStatusOptions}
                disabled={replyMutation.isPending}
                onChange={(value) => setReplyStatus(value as TicketStatus)}
              />
            }
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
          render={<Link to='/ticket-management' />}
        >
          <ArrowLeft data-icon='inline-start' aria-hidden='true' />
          {t('Back to ticket management')}
        </Button>
      </SectionPageLayout.Breadcrumb>
      <SectionPageLayout.Title>
        {t('Ticket #{{id}}', { id: props.ticketId })}
      </SectionPageLayout.Title>
      <SectionPageLayout.Content>{content}</SectionPageLayout.Content>
    </SectionPageLayout>
  )
}
