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
import { useEffect, useId, useState } from 'react'
import { useForm, useWatch } from 'react-hook-form'
import { useTranslation } from 'react-i18next'

import { Dialog } from '@/components/dialog'
import { PasswordInput } from '@/components/password-input'
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
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'

import {
  upstreamBillingFormSchema,
  type UpstreamBillingFormValues,
} from '../lib/upstream-billing-form'
import type { ChannelCostChannel, ChannelUpstreamBillingInput } from '../types'

function formValues(channel: ChannelCostChannel): UpstreamBillingFormValues {
  const billing = channel.upstream_billing
  return {
    kind: billing?.kind ?? 'none',
    usd_rate: billing?.usd_rate ?? 1,
    upstream_user_id: billing?.upstream_user_id ?? 0,
    access_token: '',
    access_token_set: billing?.access_token_set ?? false,
  }
}

function parseNumber(value: string): number {
  return value.trim() === '' ? Number.NaN : Number(value)
}

/** Chooses whether a channel's cost comes from its upstream's bill. */
export function UpstreamBillingDialog(props: {
  channel: ChannelCostChannel
  open: boolean
  onOpenChange: (open: boolean) => void
  /** Resolves true when the server checked and saved the settings. */
  onSave: (input: ChannelUpstreamBillingInput) => Promise<boolean>
}) {
  const { t } = useTranslation()
  const formId = useId()
  const [saving, setSaving] = useState(false)
  const form = useForm<UpstreamBillingFormValues>({
    resolver: zodResolver(upstreamBillingFormSchema),
    defaultValues: formValues(props.channel),
  })
  const kind = useWatch({ control: form.control, name: 'kind' })
  const kindItems = [
    { value: 'none', label: t('Cost ratio (no upstream bill)') },
    { value: 'sub2api', label: 'sub2api' },
    { value: 'newapi', label: 'new-api' },
  ]

  useEffect(() => {
    if (props.open) form.reset(formValues(props.channel))
  }, [form, props.channel, props.open])

  const submit = async (values: UpstreamBillingFormValues) => {
    setSaving(true)
    const saved = await props.onSave({
      kind: values.kind === 'none' ? '' : values.kind,
      usd_rate: values.usd_rate,
      upstream_user_id: values.upstream_user_id,
      access_token: values.access_token,
    })
    setSaving(false)
    if (saved) props.onOpenChange(false)
  }

  return (
    <Dialog
      open={props.open}
      onOpenChange={props.onOpenChange}
      title={t('Upstream bill of {{name}}', { name: props.channel.name })}
      description={t(
        "Read this channel's cost from what its upstream actually charged instead of the cost ratio."
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
          <Button type='submit' form={formId} disabled={saving}>
            {saving ? t('Checking the upstream…') : t('Save')}
          </Button>
        </>
      }
    >
      <Form {...form}>
        <form
          id={formId}
          className='flex flex-col gap-4'
          onSubmit={form.handleSubmit(submit)}
        >
          <FormField
            control={form.control}
            name='kind'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('Upstream type')}</FormLabel>
                <Select
                  items={kindItems}
                  value={field.value}
                  onValueChange={(value) => {
                    if (typeof value === 'string') field.onChange(value)
                  }}
                >
                  <FormControl>
                    <SelectTrigger className='w-full'>
                      <SelectValue />
                    </SelectTrigger>
                  </FormControl>
                  <SelectContent alignItemWithTrigger={false}>
                    <SelectGroup>
                      {kindItems.map((item) => (
                        <SelectItem key={item.value} value={item.value}>
                          {item.label}
                        </SelectItem>
                      ))}
                    </SelectGroup>
                  </SelectContent>
                </Select>
                {kind === 'sub2api' && (
                  <FormDescription>
                    {t(
                      "Uses the channel's own key to read what the key was charged. Needs a sub2api released after May 2026."
                    )}
                  </FormDescription>
                )}
                {kind === 'newapi' && (
                  <FormDescription>
                    {t(
                      "Reads the account's usage statistics for the token behind the channel's key. Give that token a name no other token on the account uses."
                    )}
                  </FormDescription>
                )}
              </FormItem>
            )}
          />
          {kind !== 'none' && (
            <FormField
              control={form.control}
              name='usd_rate'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>
                    {t("1 upstream dollar = how many of this site's dollars")}
                  </FormLabel>
                  <FormControl>
                    <Input
                      type='number'
                      inputMode='decimal'
                      min={0}
                      step='0.001'
                      name={field.name}
                      ref={field.ref}
                      value={Number.isNaN(field.value) ? '' : field.value}
                      onBlur={field.onBlur}
                      onChange={(event) =>
                        field.onChange(parseNumber(event.target.value))
                      }
                    />
                  </FormControl>
                  <FormDescription>
                    {t(
                      'For example, if $1 costs ¥1 on the upstream and $1 sells for ¥7 here, enter 0.143.'
                    )}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />
          )}
          {kind === 'newapi' && (
            <>
              <FormField
                control={form.control}
                name='upstream_user_id'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Your user ID on the upstream')}</FormLabel>
                    <FormControl>
                      <Input
                        type='number'
                        inputMode='numeric'
                        min={1}
                        step={1}
                        name={field.name}
                        ref={field.ref}
                        value={
                          Number.isNaN(field.value) || field.value === 0
                            ? ''
                            : field.value
                        }
                        onBlur={field.onBlur}
                        onChange={(event) =>
                          field.onChange(parseNumber(event.target.value))
                        }
                      />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />
              <FormField
                control={form.control}
                name='access_token'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('System access token')}</FormLabel>
                    <FormControl>
                      <PasswordInput
                        {...field}
                        autoComplete='off'
                        placeholder={
                          form.getValues('access_token_set')
                            ? t('Saved. Leave empty to keep it.')
                            : undefined
                        }
                      />
                    </FormControl>
                    <FormDescription>
                      {t(
                        'Generate it on the upstream under personal settings → system access token. It is only sent to this channel’s address.'
                      )}
                    </FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />
            </>
          )}
        </form>
      </Form>
    </Dialog>
  )
}
