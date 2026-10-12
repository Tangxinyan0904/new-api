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
import { useQuery } from '@tanstack/react-query'
import { ImageOff } from 'lucide-react'
import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { Dialog } from '@/components/dialog'
import { Skeleton } from '@/components/ui/skeleton'

import { fetchTicketAttachment } from '../api'
import { ticketQueryKeys } from '../constants'
import type { TicketAttachment, TicketAudience } from '../types'

interface TicketAttachmentImageProps {
  audience: TicketAudience
  attachment: TicketAttachment
}

/**
 * Attachments need the signed-in session, so the image is fetched through the
 * API client and shown from an object URL.
 */
export function TicketAttachmentImage(props: TicketAttachmentImageProps) {
  const { t } = useTranslation()
  const [open, setOpen] = useState(false)
  const [url, setUrl] = useState<string>()
  const query = useQuery({
    queryKey: ticketQueryKeys.attachment(props.audience, props.attachment.id),
    queryFn: () => fetchTicketAttachment(props.audience, props.attachment.id),
    staleTime: Infinity,
    meta: { errorToast: false },
  })

  useEffect(() => {
    if (!query.data) return
    const objectUrl = URL.createObjectURL(query.data)
    setUrl(objectUrl)
    return () => URL.revokeObjectURL(objectUrl)
  }, [query.data])

  if (query.isError) {
    return (
      <div
        role='img'
        aria-label={t('Image unavailable')}
        className='text-muted-foreground flex size-20 items-center justify-center rounded-md border border-dashed'
      >
        <ImageOff className='size-5' aria-hidden='true' />
      </div>
    )
  }
  if (!url) {
    return <Skeleton className='size-20 rounded-md' />
  }

  const label = t('Attachment #{{id}}', { id: props.attachment.id })
  return (
    <>
      <button
        type='button'
        className='focus-visible:ring-ring size-20 overflow-hidden rounded-md border transition hover:opacity-90 focus-visible:ring-2 focus-visible:outline-none'
        aria-label={t('View {{name}}', { name: label })}
        onClick={() => setOpen(true)}
      >
        <img src={url} alt={label} className='size-full object-cover' />
      </button>
      <Dialog
        open={open}
        onOpenChange={setOpen}
        title={label}
        contentClassName='sm:max-w-4xl'
      >
        <img
          src={url}
          alt={label}
          className='mx-auto max-h-[75vh] w-auto rounded-md object-contain'
        />
      </Dialog>
    </>
  )
}
