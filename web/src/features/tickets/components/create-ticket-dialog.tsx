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
import { zodResolver } from '@hookform/resolvers/zod'
import { useQueryClient } from '@tanstack/react-query'
import { useNavigate } from '@tanstack/react-router'
import { Plus } from 'lucide-react'
import { useState } from 'react'
import { useForm } from 'react-hook-form'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Dialog } from '@/components/dialog'
import { Button } from '@/components/ui/button'
import {
  Form,
  FormControl,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from '@/components/ui/form'
import { Input } from '@/components/ui/input'
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Spinner } from '@/components/ui/spinner'
import { Textarea } from '@/components/ui/textarea'
import { handleServerError } from '@/lib/handle-server-error'

import { createTicket } from '../api'
import {
  TICKET_CATEGORIES,
  TICKET_CATEGORY_VALUES,
  TICKET_LIMITS,
  ticketQueryKeys,
} from '../constants'
import { useTicketImageAdder } from '../hooks/use-ticket-image-adder'
import {
  CREATE_TICKET_DEFAULT_VALUES,
  type CreateTicketFormValues,
  getCreateTicketSchema,
} from '../lib/ticket-form'
import { pastedImages } from '../lib/ticket-images'
import { TicketImagePicker } from './ticket-image-picker'

export function CreateTicketDialog() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const navigate = useNavigate()
  const [open, setOpen] = useState(false)
  const [images, setImages] = useState<File[]>([])
  const addImages = useTicketImageAdder(images, setImages)
  const form = useForm<CreateTicketFormValues>({
    resolver: zodResolver(getCreateTicketSchema(t)),
    defaultValues: CREATE_TICKET_DEFAULT_VALUES,
  })
  const isSubmitting = form.formState.isSubmitting
  const categoryItems = TICKET_CATEGORY_VALUES.map((value) => ({
    value,
    label: t(TICKET_CATEGORIES[value].labelKey),
  }))

  const handleOpenChange = (next: boolean) => {
    if (isSubmitting) return
    setOpen(next)
    if (!next) {
      form.reset(CREATE_TICKET_DEFAULT_VALUES)
      setImages([])
    }
  }

  const onSubmit = async (values: CreateTicketFormValues) => {
    try {
      const result = await createTicket({ ...values, images })
      if (!result.success || !result.data) {
        handleServerError(result)
        return
      }
      toast.success(t('Ticket submitted'))
      await queryClient.invalidateQueries({ queryKey: ticketQueryKeys.all })
      handleOpenChange(false)
      void navigate({
        to: '/tickets/$ticketId',
        params: { ticketId: String(result.data.id) },
      })
    } catch (error) {
      handleServerError(error)
    }
  }

  return (
    <Dialog
      open={open}
      onOpenChange={handleOpenChange}
      title={t('New ticket')}
      description={t(
        'Describe your problem and our support team will reply here. You can have up to {{count}} open tickets.',
        { count: TICKET_LIMITS.MAX_OPEN_TICKETS }
      )}
      trigger={
        <Button size='sm'>
          <Plus data-icon='inline-start' aria-hidden='true' />
          {t('New ticket')}
        </Button>
      }
      footer={
        <>
          <Button
            type='button'
            variant='outline'
            disabled={isSubmitting}
            onClick={() => handleOpenChange(false)}
          >
            {t('Cancel')}
          </Button>
          <Button
            type='submit'
            form='create-ticket-form'
            disabled={isSubmitting}
          >
            {isSubmitting && <Spinner data-icon='inline-start' />}
            {t('Submit')}
          </Button>
        </>
      }
    >
      <Form {...form}>
        <form
          id='create-ticket-form'
          className='flex flex-col gap-4'
          onSubmit={(event) => void form.handleSubmit(onSubmit)(event)}
        >
          <FormField
            control={form.control}
            name='subject'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('Subject')}</FormLabel>
                <FormControl>
                  <Input
                    {...field}
                    placeholder={t('Summarize the problem in one sentence')}
                    maxLength={TICKET_LIMITS.SUBJECT_MAX}
                  />
                </FormControl>
                <FormMessage />
              </FormItem>
            )}
          />
          <FormField
            control={form.control}
            name='category'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('Category')}</FormLabel>
                <Select
                  items={categoryItems}
                  value={field.value}
                  onValueChange={field.onChange}
                >
                  <FormControl>
                    <SelectTrigger className='w-full'>
                      <SelectValue />
                    </SelectTrigger>
                  </FormControl>
                  <SelectContent alignItemWithTrigger={false}>
                    <SelectGroup>
                      {categoryItems.map((item) => (
                        <SelectItem key={item.value} value={item.value}>
                          {item.label}
                        </SelectItem>
                      ))}
                    </SelectGroup>
                  </SelectContent>
                </Select>
                <FormMessage />
              </FormItem>
            )}
          />
          <FormField
            control={form.control}
            name='content'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('Problem description')}</FormLabel>
                <FormControl>
                  <Textarea
                    {...field}
                    rows={6}
                    placeholder={t(
                      'Include what you did, what happened, and any request IDs or error messages.'
                    )}
                    onPaste={(event) => {
                      const files = pastedImages(event)
                      if (files.length > 0) {
                        event.preventDefault()
                        addImages(files)
                      }
                    }}
                  />
                </FormControl>
                <FormMessage />
              </FormItem>
            )}
          />
          <TicketImagePicker
            value={images}
            onChange={setImages}
            disabled={isSubmitting}
          />
        </form>
      </Form>
    </Dialog>
  )
}
