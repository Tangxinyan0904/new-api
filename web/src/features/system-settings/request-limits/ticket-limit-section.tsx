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
import { useEffect } from 'react'
import { useForm } from 'react-hook-form'
import { useTranslation } from 'react-i18next'
import * as z from 'zod'

import {
  Form,
  FormControl,
  FormDescription,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from '@/components/ui/form'
import { Input } from '@/components/ui/input'

import { SettingsForm } from '../components/settings-form-layout'
import { SettingsPageFormActions } from '../components/settings-page-context'
import { SettingsSection } from '../components/settings-section'
import { useUpdateOption } from '../hooks/use-update-option'

const DAILY_LIMIT_KEY = 'ticket_setting.daily_limit'

const ticketLimitSchema = z.object({
  daily_limit: z.number().int().min(0),
})

type TicketLimitFormValues = z.output<typeof ticketLimitSchema>
type TicketLimitFormInput = z.input<typeof ticketLimitSchema>

type TicketLimitSectionProps = {
  defaultValues: {
    'ticket_setting.daily_limit': number
  }
}

export function TicketLimitSection(props: TicketLimitSectionProps) {
  const { t } = useTranslation()
  const updateOption = useUpdateOption()
  const defaultLimit = props.defaultValues[DAILY_LIMIT_KEY]
  const form = useForm<TicketLimitFormInput, unknown, TicketLimitFormValues>({
    resolver: zodResolver(ticketLimitSchema),
    mode: 'onChange',
    defaultValues: { daily_limit: defaultLimit },
  })

  useEffect(() => {
    form.reset({ daily_limit: defaultLimit })
  }, [defaultLimit, form])

  const onSubmit = async (values: TicketLimitFormValues) => {
    if (values.daily_limit !== defaultLimit) {
      await updateOption.mutateAsync({
        key: DAILY_LIMIT_KEY,
        value: values.daily_limit,
      })
    }
  }

  return (
    <SettingsSection title={t('Ticket Limits')}>
      <Form {...form}>
        <SettingsForm onSubmit={form.handleSubmit(onSubmit)}>
          <SettingsPageFormActions
            onSave={form.handleSubmit(onSubmit)}
            isSaving={updateOption.isPending}
            saveLabel='Save ticket limits'
          />
          <FormField
            control={form.control}
            name='daily_limit'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('Tickets per user in 24 hours')}</FormLabel>
                <FormControl>
                  <Input
                    type='number'
                    min={0}
                    step={1}
                    {...field}
                    onChange={(e) =>
                      field.onChange(Number.parseInt(e.target.value) || 0)
                    }
                  />
                </FormControl>
                <FormDescription>
                  {t(
                    'Maximum number of tickets each user can open in any 24 hours, closed tickets included. 0 means no limit.'
                  )}
                </FormDescription>
                <FormMessage />
              </FormItem>
            )}
          />
        </SettingsForm>
      </Form>
    </SettingsSection>
  )
}
