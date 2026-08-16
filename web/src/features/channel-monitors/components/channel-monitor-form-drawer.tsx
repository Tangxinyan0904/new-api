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
import { useEffect, useMemo } from 'react'
import { useForm } from 'react-hook-form'
import { useTranslation } from 'react-i18next'

import {
  SideDrawerSection,
  SideDrawerSectionHeader,
  sideDrawerContentClassName,
  sideDrawerFooterClassName,
  sideDrawerFormClassName,
  sideDrawerHeaderClassName,
} from '@/components/drawer-layout'
import { MultiSelect } from '@/components/multi-select'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
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
  Sheet,
  SheetClose,
  SheetContent,
  SheetDescription,
  SheetFooter,
  SheetHeader,
  SheetTitle,
} from '@/components/ui/sheet'
import { Spinner } from '@/components/ui/spinner'
import { Switch } from '@/components/ui/switch'

import {
  CHANNEL_MONITOR_FORM_DEFAULTS,
  getChannelMonitorFormSchema,
  type ChannelMonitorFormValues,
} from '../lib/schema'
import type {
  ChannelMonitor,
  ChannelMonitorApiMode,
  ChannelMonitorProvider,
} from '../types'

type ChannelMonitorFormDrawerProps = {
  open: boolean
  monitor: ChannelMonitor | null
  isSubmitting: boolean
  onOpenChange: (open: boolean) => void
  onSubmit: (values: ChannelMonitorFormValues) => Promise<void>
}

const PROVIDERS: Array<{ value: ChannelMonitorProvider; label: string }> = [
  { value: 'openai', label: 'OpenAI' },
  { value: 'anthropic', label: 'Anthropic' },
  { value: 'gemini', label: 'Gemini' },
]

const MODES_BY_PROVIDER: Record<
  ChannelMonitorProvider,
  Array<{ value: ChannelMonitorApiMode; label: string }>
> = {
  openai: [
    { value: 'chat_completions', label: 'Chat Completions' },
    { value: 'responses', label: 'Responses' },
  ],
  anthropic: [{ value: 'messages', label: 'Messages' }],
  gemini: [{ value: 'generate_content', label: 'Generate Content' }],
}

function formValuesForMonitor(monitor: ChannelMonitor | null) {
  if (!monitor) return CHANNEL_MONITOR_FORM_DEFAULTS
  return {
    internal_name: monitor.internal_name,
    public_name: monitor.public_name,
    provider: monitor.provider,
    api_mode: monitor.api_mode,
    endpoint: monitor.endpoint,
    api_key: '',
    primary_model: monitor.primary_model,
    extra_models: monitor.extra_models,
    interval_seconds: monitor.interval_seconds,
    degraded_threshold_ms: monitor.degraded_threshold_ms,
    timeout_seconds: monitor.timeout_seconds,
    enabled: monitor.enabled,
    public_enabled: monitor.public_enabled,
  } satisfies ChannelMonitorFormValues
}

export function ChannelMonitorFormDrawer(props: ChannelMonitorFormDrawerProps) {
  const { t } = useTranslation()
  const isEdit = props.monitor !== null
  const schema = useMemo(() => getChannelMonitorFormSchema(isEdit), [isEdit])
  const form = useForm<ChannelMonitorFormValues>({
    resolver: zodResolver(schema),
    defaultValues: CHANNEL_MONITOR_FORM_DEFAULTS,
  })
  const provider = form.watch('provider')
  const endpoint = form.watch('endpoint')
  const publicEnabled = form.watch('public_enabled')
  const modeOptions = MODES_BY_PROVIDER[provider]

  useEffect(() => {
    if (!props.open) return
    form.reset(formValuesForMonitor(props.monitor))
  }, [form, props.monitor, props.open])

  return (
    <Sheet open={props.open} onOpenChange={props.onOpenChange}>
      <SheetContent className={sideDrawerContentClassName('sm:max-w-2xl')}>
        <SheetHeader className={sideDrawerHeaderClassName()}>
          <SheetTitle>
            {isEdit ? t('Edit channel monitor') : t('Create channel monitor')}
          </SheetTitle>
          <SheetDescription>
            {t(
              'Configure an independent direct probe. Monitor requests do not consume user quota or enter usage logs.'
            )}
          </SheetDescription>
        </SheetHeader>

        <Form {...form}>
          <form
            id='channel-monitor-form'
            className={sideDrawerFormClassName()}
            onSubmit={form.handleSubmit(props.onSubmit)}
          >
            <SideDrawerSection>
              <SideDrawerSectionHeader
                title={t('Identity')}
                description={t(
                  'Use an internal name for administrators and a separate public name for status pages.'
                )}
              />
              <div className='grid gap-4 sm:grid-cols-2'>
                <FormField
                  control={form.control}
                  name='internal_name'
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>{t('Internal name')}</FormLabel>
                      <FormControl>
                        <Input
                          {...field}
                          placeholder={t('Primary upstream monitor')}
                        />
                      </FormControl>
                      <FormMessage />
                    </FormItem>
                  )}
                />
                <FormField
                  control={form.control}
                  name='public_name'
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>{t('Public name')}</FormLabel>
                      <FormControl>
                        <Input {...field} placeholder={t('OpenAI status')} />
                      </FormControl>
                      <FormMessage />
                    </FormItem>
                  )}
                />
              </div>
            </SideDrawerSection>

            <SideDrawerSection>
              <SideDrawerSectionHeader
                title={t('Protocol')}
                description={t(
                  'Choose one supported provider protocol and enter only its base endpoint.'
                )}
              />
              <div className='grid gap-4 sm:grid-cols-2'>
                <FormField
                  control={form.control}
                  name='provider'
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>{t('Provider')}</FormLabel>
                      <FormControl>
                        <Select
                          items={PROVIDERS.map((item) => ({
                            value: item.value,
                            label: item.label,
                          }))}
                          value={field.value}
                          onValueChange={(value) => {
                            if (!value) return
                            field.onChange(value)
                            const nextModes = MODES_BY_PROVIDER[value]
                            const currentMode = form.getValues('api_mode')
                            if (
                              !nextModes.some(
                                (option) => option.value === currentMode
                              )
                            ) {
                              form.setValue('api_mode', nextModes[0].value, {
                                shouldDirty: true,
                                shouldValidate: true,
                              })
                            }
                          }}
                        >
                          <SelectTrigger className='w-full'>
                            <SelectValue />
                          </SelectTrigger>
                          <SelectContent alignItemWithTrigger={false}>
                            <SelectGroup>
                              {PROVIDERS.map((item) => (
                                <SelectItem key={item.value} value={item.value}>
                                  {item.label}
                                </SelectItem>
                              ))}
                            </SelectGroup>
                          </SelectContent>
                        </Select>
                      </FormControl>
                      <FormMessage />
                    </FormItem>
                  )}
                />
                <FormField
                  control={form.control}
                  name='api_mode'
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>{t('API mode')}</FormLabel>
                      <FormControl>
                        <Select
                          items={modeOptions.map((item) => ({
                            value: item.value,
                            label: t(item.label),
                          }))}
                          value={field.value}
                          onValueChange={field.onChange}
                        >
                          <SelectTrigger className='w-full'>
                            <SelectValue />
                          </SelectTrigger>
                          <SelectContent alignItemWithTrigger={false}>
                            <SelectGroup>
                              {modeOptions.map((item) => (
                                <SelectItem key={item.value} value={item.value}>
                                  {t(item.label)}
                                </SelectItem>
                              ))}
                            </SelectGroup>
                          </SelectContent>
                        </Select>
                      </FormControl>
                      <FormMessage />
                    </FormItem>
                  )}
                />
              </div>

              <FormField
                control={form.control}
                name='endpoint'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Base endpoint')}</FormLabel>
                    <FormControl>
                      <Input
                        {...field}
                        inputMode='url'
                        placeholder='https://api.example.com'
                      />
                    </FormControl>
                    <FormDescription>
                      {t(
                        'Do not include /v1/chat/completions, /v1/responses, /v1/messages, or generateContent.'
                      )}
                    </FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />

              {endpoint.trim().toLowerCase().startsWith('http://') ? (
                <Alert>
                  <AlertTitle>{t('Unencrypted HTTP endpoint')}</AlertTitle>
                  <AlertDescription>
                    {t(
                      'HTTP endpoints send monitor credentials without transport encryption.'
                    )}
                  </AlertDescription>
                </Alert>
              ) : null}

              <FormField
                control={form.control}
                name='api_key'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Monitor credential')}</FormLabel>
                    <FormControl>
                      <Input
                        {...field}
                        type='password'
                        autoComplete='new-password'
                        placeholder={
                          isEdit
                            ? t('Leave blank to keep the existing credential')
                            : t('Enter a dedicated monitor credential')
                        }
                      />
                    </FormControl>
                    <FormDescription>
                      {isEdit && props.monitor?.credential_configured
                        ? t(
                            'Current credential: {{credential}}. Leave blank to keep it.',
                            { credential: props.monitor.credential_masked }
                          )
                        : t(
                            'Use a dedicated credential with the minimum permissions required for model requests.'
                          )}
                    </FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />
            </SideDrawerSection>

            <SideDrawerSection>
              <SideDrawerSectionHeader
                title={t('Models')}
                description={t(
                  'The primary model determines summary status. Up to nineteen extra models may be checked sequentially.'
                )}
              />
              <FormField
                control={form.control}
                name='primary_model'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Primary model')}</FormLabel>
                    <FormControl>
                      <Input {...field} placeholder='gpt-4.1-mini' />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />
              <FormField
                control={form.control}
                name='extra_models'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Extra models')}</FormLabel>
                    <FormControl>
                      <MultiSelect
                        selected={field.value}
                        options={[]}
                        onChange={field.onChange}
                        allowCreate
                        maxVisibleChips={6}
                        placeholder={t('Type a model and press Enter')}
                        createLabel={t('Add "{{value}}"')}
                        emptyText={t('Type a model name to add it')}
                      />
                    </FormControl>
                    <FormDescription>
                      {t('The primary and extra model names must be unique.')}
                    </FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />
            </SideDrawerSection>

            <SideDrawerSection>
              <SideDrawerSectionHeader
                title={t('Schedule')}
                description={t(
                  'Checks for one monitor run sequentially so upstream traffic stays bounded.'
                )}
              />
              <div className='grid gap-4 sm:grid-cols-3'>
                <FormField
                  control={form.control}
                  name='interval_seconds'
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>{t('Interval (seconds)')}</FormLabel>
                      <FormControl>
                        <Input
                          {...field}
                          type='number'
                          min={60}
                          max={3600}
                          onChange={(event) =>
                            field.onChange(Number(event.target.value))
                          }
                        />
                      </FormControl>
                      <FormMessage />
                    </FormItem>
                  )}
                />
                <FormField
                  control={form.control}
                  name='degraded_threshold_ms'
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>{t('Degraded after (ms)')}</FormLabel>
                      <FormControl>
                        <Input
                          {...field}
                          type='number'
                          min={1}
                          onChange={(event) =>
                            field.onChange(Number(event.target.value))
                          }
                        />
                      </FormControl>
                      <FormMessage />
                    </FormItem>
                  )}
                />
                <FormField
                  control={form.control}
                  name='timeout_seconds'
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>{t('Timeout (seconds)')}</FormLabel>
                      <FormControl>
                        <Input
                          {...field}
                          type='number'
                          min={1}
                          max={120}
                          onChange={(event) =>
                            field.onChange(Number(event.target.value))
                          }
                        />
                      </FormControl>
                      <FormMessage />
                    </FormItem>
                  )}
                />
              </div>
            </SideDrawerSection>

            <SideDrawerSection>
              <SideDrawerSectionHeader title={t('Access')} />
              <FormField
                control={form.control}
                name='enabled'
                render={({ field }) => (
                  <FormItem className='flex items-center justify-between gap-4 rounded-lg border p-3'>
                    <div className='grid gap-1'>
                      <FormLabel>{t('Automatic checks')}</FormLabel>
                      <FormDescription>
                        {t('Run this monitor on its configured schedule.')}
                      </FormDescription>
                    </div>
                    <FormControl>
                      <Switch
                        checked={field.value}
                        onCheckedChange={field.onChange}
                      />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />
              <FormField
                control={form.control}
                name='public_enabled'
                render={({ field }) => (
                  <FormItem className='flex items-center justify-between gap-4 rounded-lg border p-3'>
                    <div className='grid gap-1'>
                      <FormLabel>{t('Publish status')}</FormLabel>
                      <FormDescription>
                        {t(
                          'Show this monitor on signed-in and public status views.'
                        )}
                      </FormDescription>
                    </div>
                    <FormControl>
                      <Switch
                        checked={field.value}
                        onCheckedChange={field.onChange}
                      />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />

              {publicEnabled ? (
                <Alert>
                  <AlertTitle>{t('Public status disclosure')}</AlertTitle>
                  <AlertDescription>
                    {t(
                      'Published monitors expose their public name, provider, models, status, latency, and availability.'
                    )}
                  </AlertDescription>
                </Alert>
              ) : null}

              {props.monitor?.schedule_blocked_reason ? (
                <Alert variant='destructive'>
                  <AlertTitle>{t('Schedule blocked')}</AlertTitle>
                  <AlertDescription>
                    {props.monitor.schedule_blocked_reason}
                  </AlertDescription>
                </Alert>
              ) : null}
            </SideDrawerSection>
          </form>
        </Form>

        <SheetFooter className={sideDrawerFooterClassName()}>
          <SheetClose render={<Button variant='outline' />}>
            {t('Cancel')}
          </SheetClose>
          <Button
            form='channel-monitor-form'
            type='submit'
            disabled={props.isSubmitting}
          >
            {props.isSubmitting ? <Spinner data-icon='inline-start' /> : null}
            {isEdit ? t('Save monitor') : t('Create monitor')}
          </Button>
        </SheetFooter>
      </SheetContent>
    </Sheet>
  )
}
