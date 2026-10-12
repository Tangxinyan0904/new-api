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
import { ImagePlus, X } from 'lucide-react'
import { useEffect, useMemo, useRef } from 'react'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'

import { TICKET_IMAGE_TYPES, TICKET_LIMITS } from '../constants'
import { useTicketImageAdder } from '../hooks/use-ticket-image-adder'

export interface TicketImagePickerProps {
  value: File[]
  onChange: (files: File[]) => void
  disabled?: boolean
}

export function TicketImagePicker(props: TicketImagePickerProps) {
  const { t } = useTranslation()
  const inputRef = useRef<HTMLInputElement>(null)
  const addImages = useTicketImageAdder(props.value, props.onChange)
  const previews = useMemo(
    () => props.value.map((file) => URL.createObjectURL(file)),
    [props.value]
  )
  useEffect(
    () => () => previews.forEach((url) => URL.revokeObjectURL(url)),
    [previews]
  )
  const full = props.value.length >= TICKET_LIMITS.MAX_IMAGES

  return (
    <div className='flex flex-col gap-2'>
      {props.value.length > 0 && (
        <ul className='flex flex-wrap gap-2' aria-label={t('Attached images')}>
          {props.value.map((file, index) => (
            <li
              key={previews[index]}
              className='group relative size-16 overflow-hidden rounded-md border'
            >
              <img
                src={previews[index]}
                alt={file.name}
                className='size-full object-cover'
              />
              <Button
                type='button'
                size='icon-xs'
                variant='secondary'
                className='absolute top-1 right-1 opacity-90'
                aria-label={t('Remove image {{name}}', { name: file.name })}
                disabled={props.disabled}
                onClick={() =>
                  props.onChange(props.value.filter((_, i) => i !== index))
                }
              >
                <X aria-hidden='true' />
              </Button>
            </li>
          ))}
        </ul>
      )}
      <div className='flex flex-wrap items-center gap-2'>
        <Button
          type='button'
          size='sm'
          variant='outline'
          disabled={props.disabled || full}
          onClick={() => inputRef.current?.click()}
        >
          <ImagePlus data-icon='inline-start' aria-hidden='true' />
          {t('Add image')}
        </Button>
        <span className='text-muted-foreground text-xs'>
          {t(
            'PNG, JPEG, or WebP. Up to {{count}} images, {{size}} MB each. You can also paste screenshots.',
            {
              count: TICKET_LIMITS.MAX_IMAGES,
              size: TICKET_LIMITS.MAX_IMAGE_BYTES / 1024 / 1024,
            }
          )}
        </span>
      </div>
      <input
        ref={inputRef}
        type='file'
        accept={TICKET_IMAGE_TYPES.join(',')}
        multiple
        hidden
        data-testid='ticket-image-input'
        onChange={(event) => {
          addImages([...(event.currentTarget.files ?? [])])
          event.currentTarget.value = ''
        }}
      />
    </div>
  )
}
