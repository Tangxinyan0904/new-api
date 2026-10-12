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
import { Send } from 'lucide-react'
import { useId, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'
import { Label } from '@/components/ui/label'
import { Spinner } from '@/components/ui/spinner'
import { Textarea } from '@/components/ui/textarea'

import type { TicketMessageInput } from '../api'
import { TICKET_LIMITS } from '../constants'
import { useTicketImageAdder } from '../hooks/use-ticket-image-adder'
import { pastedImages } from '../lib/ticket-images'
import { TicketImagePicker } from './ticket-image-picker'

interface TicketReplyFormProps {
  /** Resolves true when the reply was saved; the form then clears. */
  onSubmit: (input: TicketMessageInput) => Promise<boolean>
  isSubmitting: boolean
  /** Controls placed before the send button, such as the status after reply. */
  actions?: React.ReactNode
}

export function TicketReplyForm(props: TicketReplyFormProps) {
  const { t } = useTranslation()
  const fieldId = useId()
  const [content, setContent] = useState('')
  const [images, setImages] = useState<File[]>([])
  const addImages = useTicketImageAdder(images, setImages)
  const trimmed = content.trim()
  const tooLong = content.length > TICKET_LIMITS.CONTENT_MAX
  const canSubmit =
    !props.isSubmitting && !tooLong && (trimmed !== '' || images.length > 0)

  const submit = async () => {
    if (!canSubmit) return
    const saved = await props.onSubmit({ content: trimmed, images })
    if (saved) {
      setContent('')
      setImages([])
    }
  }

  return (
    <form
      className='flex flex-col gap-3 rounded-lg border p-3 sm:p-4'
      onSubmit={(event) => {
        event.preventDefault()
        void submit()
      }}
    >
      <Label htmlFor={fieldId}>{t('Reply')}</Label>
      <Textarea
        id={fieldId}
        value={content}
        rows={4}
        placeholder={t('Describe the details. Press Ctrl+Enter to send.')}
        aria-invalid={tooLong}
        disabled={props.isSubmitting}
        onChange={(event) => setContent(event.target.value)}
        onPaste={(event) => {
          const files = pastedImages(event)
          if (files.length > 0) {
            event.preventDefault()
            addImages(files)
          }
        }}
        onKeyDown={(event) => {
          if (event.key === 'Enter' && (event.ctrlKey || event.metaKey)) {
            event.preventDefault()
            void submit()
          }
        }}
      />
      <TicketImagePicker
        value={images}
        onChange={setImages}
        disabled={props.isSubmitting}
      />
      <div className='flex flex-wrap items-center justify-between gap-2'>
        <span
          className={
            tooLong
              ? 'text-destructive text-xs tabular-nums'
              : 'text-muted-foreground text-xs tabular-nums'
          }
        >
          {content.length} / {TICKET_LIMITS.CONTENT_MAX}
        </span>
        <div className='flex flex-wrap items-center gap-2'>
          {props.actions}
          <Button type='submit' size='sm' disabled={!canSubmit}>
            {props.isSubmitting ? (
              <Spinner data-icon='inline-start' />
            ) : (
              <Send data-icon='inline-start' aria-hidden='true' />
            )}
            {t('Send reply')}
          </Button>
        </div>
      </div>
    </form>
  )
}
