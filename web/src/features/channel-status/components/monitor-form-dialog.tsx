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
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { RotateCcw } from 'lucide-react'
import { useEffect, useId } from 'react'
import { useForm, useWatch } from 'react-hook-form'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Dialog } from '@/components/dialog'
import { Button } from '@/components/ui/button'
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
import {
  InputGroup,
  InputGroupAddon,
  InputGroupButton,
  InputGroupInput,
} from '@/components/ui/input-group'
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Switch } from '@/components/ui/switch'
import { getGroups } from '@/features/users/api'
import { handleServerError } from '@/lib/handle-server-error'
import { requireServerSuccess } from '@/lib/server-error-message'

import { channelStatusQueryKeys, saveChannelStatusMonitor } from '../api'
import {
  monitorFormSchema,
  NAME_COLOR_PATTERN,
  type MonitorFormInput,
  type MonitorFormValues,
} from '../lib/monitor-form'
import type { ChannelStatusItem } from '../types'

function formValues(monitor: ChannelStatusItem | null): MonitorFormInput {
  return {
    group: monitor?.group ?? '',
    public_name: monitor?.public_name ?? '',
    name_color: monitor?.name_color ?? '',
    public_enabled: monitor?.public_enabled ?? true,
  }
}

/** Adds a group to the status page, or edits the monitor given. */
export function MonitorFormDialog(props: {
  open: boolean
  monitor: ChannelStatusItem | null
  onOpenChange: (open: boolean) => void
}) {
  const { t } = useTranslation()
  const formId = useId()
  const queryClient = useQueryClient()
  const form = useForm<MonitorFormInput, unknown, MonitorFormValues>({
    resolver: zodResolver(monitorFormSchema),
    defaultValues: formValues(props.monitor),
  })
  const publicName = useWatch({ control: form.control, name: 'public_name' })
  const groupsQuery = useQuery({
    queryKey: ['channel-status', 'groups'],
    queryFn: async () => requireServerSuccess(await getGroups()).data ?? [],
    enabled: props.open,
  })
  const groupItems = (groupsQuery.data ?? []).map((group) => ({
    value: group,
    label: group,
  }))

  useEffect(() => {
    if (props.open) form.reset(formValues(props.monitor))
  }, [form, props.monitor, props.open])

  const mutation = useMutation({
    mutationFn: (values: MonitorFormValues) =>
      saveChannelStatusMonitor(values, props.monitor?.id),
    onSuccess: async (result) => {
      if (!result.success) {
        handleServerError(result)
        return
      }
      toast.success(props.monitor ? t('Monitor updated') : t('Monitor added'))
      props.onOpenChange(false)
      await queryClient.invalidateQueries({
        queryKey: channelStatusQueryKeys.all,
      })
    },
  })

  return (
    <Dialog
      open={props.open}
      onOpenChange={props.onOpenChange}
      title={props.monitor ? t('Edit monitor') : t('Add monitor')}
      description={t(
        'The status comes from the calls of this group in the last hour.'
      )}
      contentClassName='sm:max-w-lg'
      footer={
        <>
          <Button
            type='button'
            variant='outline'
            onClick={() => props.onOpenChange(false)}
          >
            {t('Cancel')}
          </Button>
          <Button type='submit' form={formId} disabled={mutation.isPending}>
            {t('Save')}
          </Button>
        </>
      }
    >
      <Form {...form}>
        <form
          id={formId}
          className='flex flex-col gap-4'
          onSubmit={form.handleSubmit((values) => mutation.mutate(values))}
        >
          <FormField
            control={form.control}
            name='group'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('Group')}</FormLabel>
                <Select
                  items={groupItems}
                  value={field.value || null}
                  onValueChange={(value) => {
                    if (typeof value !== 'string') return
                    field.onChange(value)
                    if (!form.getValues('public_name').trim()) {
                      form.setValue('public_name', value)
                    }
                  }}
                >
                  <FormControl>
                    <SelectTrigger className='w-full'>
                      <SelectValue placeholder={t('Choose a group')} />
                    </SelectTrigger>
                  </FormControl>
                  <SelectContent alignItemWithTrigger={false}>
                    <SelectGroup>
                      {groupItems.map((item) => (
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
            name='public_name'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('Display name')}</FormLabel>
                <FormControl>
                  <Input {...field} />
                </FormControl>
                <FormDescription>
                  {t('Users see this name instead of the group.')}
                </FormDescription>
                <FormMessage />
              </FormItem>
            )}
          />
          <FormField
            control={form.control}
            name='name_color'
            render={({ field }) => {
              const hasValidColor = NAME_COLOR_PATTERN.test(field.value)
              return (
                <FormItem>
                  <FormLabel>{t('Name color')}</FormLabel>
                  <InputGroup>
                    <InputGroupAddon>
                      <input
                        type='color'
                        value={hasValidColor ? field.value : '#737373'}
                        aria-label={t('Choose name color')}
                        className='size-5 cursor-pointer border-0 bg-transparent p-0'
                        onChange={(event) =>
                          field.onChange(event.target.value.toUpperCase())
                        }
                      />
                    </InputGroupAddon>
                    <FormControl>
                      <InputGroupInput
                        {...field}
                        placeholder='#RRGGBB'
                        autoCapitalize='characters'
                        spellCheck={false}
                      />
                    </FormControl>
                    <InputGroupAddon align='inline-end'>
                      <InputGroupButton
                        size='icon-xs'
                        aria-label={t('Use theme default color')}
                        title={t('Use theme default color')}
                        onClick={() => field.onChange('')}
                      >
                        <RotateCcw aria-hidden='true' />
                      </InputGroupButton>
                    </InputGroupAddon>
                  </InputGroup>
                  <span
                    data-slot='monitor-name-preview'
                    className='block truncate text-sm font-semibold'
                    style={hasValidColor ? { color: field.value } : undefined}
                  >
                    {publicName.trim() || t('Display name')}
                  </span>
                  <FormDescription>
                    {t('Leave empty to use the theme default color.')}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )
            }}
          />
          <FormField
            control={form.control}
            name='public_enabled'
            render={({ field }) => (
              <FormItem className='flex items-center justify-between gap-4 rounded-lg border p-3'>
                <div className='space-y-1'>
                  <FormLabel>{t('Show to users')}</FormLabel>
                  <FormDescription>
                    {t(
                      'When off, only staff see this monitor. Public monitors also appear on the model square.'
                    )}
                  </FormDescription>
                </div>
                <FormControl>
                  <Switch
                    checked={field.value}
                    onCheckedChange={field.onChange}
                  />
                </FormControl>
              </FormItem>
            )}
          />
        </form>
      </Form>
    </Dialog>
  )
}
