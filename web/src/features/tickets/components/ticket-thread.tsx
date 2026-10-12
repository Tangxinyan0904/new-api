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
import { Headset, User } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { formatTimestamp } from '@/lib/format'
import { cn } from '@/lib/utils'

import type { TicketAudience, TicketMessage } from '../types'
import { TicketAttachmentImage } from './ticket-attachment-image'

interface TicketThreadProps {
  audience: TicketAudience
  messages: TicketMessage[]
  /** Owner name shown on user messages in staff views. */
  ownerName?: string
}

export function TicketThread(props: TicketThreadProps) {
  const { t } = useTranslation()

  const authorLabel = (message: TicketMessage) => {
    if (message.is_staff) {
      if (props.audience === 'staff' && message.author_name) {
        return `${message.author_name} · ${t('Support team')}`
      }
      return t('Support team')
    }
    if (props.audience === 'user') return t('You')
    return props.ownerName || t('User')
  }

  return (
    <ol className='flex flex-col gap-3' aria-label={t('Conversation')}>
      {props.messages.map((message) => {
        const Icon = message.is_staff ? Headset : User
        return (
          <li
            key={message.id}
            className={cn(
              'rounded-lg border p-3 sm:p-4',
              message.is_staff ? 'border-primary/25 bg-primary/5' : 'bg-card'
            )}
          >
            <div className='flex flex-wrap items-center justify-between gap-2'>
              <span className='flex items-center gap-2 text-sm font-medium'>
                <Icon
                  className={cn(
                    'size-4',
                    message.is_staff ? 'text-primary' : 'text-muted-foreground'
                  )}
                  aria-hidden='true'
                />
                {authorLabel(message)}
              </span>
              <time className='text-muted-foreground text-xs tabular-nums'>
                {formatTimestamp(message.created_at)}
              </time>
            </div>
            {message.content && (
              <p className='mt-2 text-sm leading-relaxed break-words whitespace-pre-wrap'>
                {message.content}
              </p>
            )}
            {message.attachments.length > 0 && (
              <div className='mt-3 flex flex-wrap gap-2'>
                {message.attachments.map((attachment) => (
                  <TicketAttachmentImage
                    key={attachment.id}
                    audience={props.audience}
                    attachment={attachment}
                  />
                ))}
              </div>
            )}
          </li>
        )
      })}
    </ol>
  )
}
