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
import { Download, Save } from 'lucide-react'
import { useEffect, useMemo, useRef, useState } from 'react'
import { useForm } from 'react-hook-form'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import * as z from 'zod'

import { MultiSelect } from '@/components/multi-select'
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
import { RadioGroup, RadioGroupItem } from '@/components/ui/radio-group'
import { Switch } from '@/components/ui/switch'
import { Textarea } from '@/components/ui/textarea'
import { DEFAULT_GEOIP_POPUP_MESSAGE } from '@/lib/constants'
import { cn } from '@/lib/utils'

import { downloadGeoIPDatabase } from '../api'
import {
  SettingsForm,
  SettingsSwitchContent,
  SettingsSwitchItem,
} from '../components/settings-form-layout'
import { SettingsPageFormActions } from '../components/settings-page-context'
import { SettingsSection } from '../components/settings-section'
import { useUpdateOption } from '../hooks/use-update-option'
import type { GeoIPMode, GeoIPSettings } from '../types'
import { countryOptions } from './countries'

const geoIPModes: Array<{
  value: GeoIPMode
  title: string
  description: string
}> = [
  {
    value: 'off',
    title: 'Off',
    description: 'Do not apply GeoIP access restrictions.',
  },
  {
    value: 'homepage_notice',
    title: 'Homepage popup notice',
    description:
      'Show a dismissible homepage popup before system announcements. API requests remain allowed.',
  },
  {
    value: 'homepage_block',
    title: 'Homepage popup block',
    description:
      'Show a non-dismissible homepage popup. API requests remain allowed.',
  },
  {
    value: 'homepage_block_api_reject',
    title: 'Homepage block + API reject',
    description:
      'Show a non-dismissible homepage popup and reject API requests from blocked countries or regions.',
  },
  {
    value: 'full_reject',
    title: 'Full reject',
    description:
      'Reject all requests from blocked countries or regions before returning frontend or API responses.',
  },
]

const geoIPSchema = z.object({
  geoip: z.object({
    mode: z.enum([
      'off',
      'homepage_notice',
      'homepage_block',
      'homepage_block_api_reject',
      'full_reject',
    ]),
    database_path: z.string().trim().min(1),
    download_url: z.string().trim(),
    maxmind_license_key: z.string().trim(),
    popup_message: z.string().trim().min(1),
    allow_private_loopback: z.boolean(),
    blocked_countries: z.array(z.string().regex(/^[A-Z]{2}$/)),
  }),
})

type GeoIPFormValues = z.output<typeof geoIPSchema>
type GeoIPFormInput = z.input<typeof geoIPSchema>

type GeoIPSectionProps = {
  defaultValues: GeoIPSettings
}

const buildFormDefaults = (
  defaults: GeoIPSectionProps['defaultValues']
): GeoIPFormInput => ({
  geoip: {
    mode: defaults['geoip.mode'],
    database_path: defaults['geoip.database_path'],
    download_url: defaults['geoip.download_url'],
    maxmind_license_key: '',
    popup_message: defaults['geoip.popup_message'],
    allow_private_loopback: defaults['geoip.allow_private_loopback'],
    blocked_countries: defaults['geoip.blocked_countries'],
  },
})

const normalizeCountryCodes = (values: string[]) => {
  const seen = new Set<string>()
  const normalized: string[] = []
  for (const value of values) {
    const code = value.trim().toUpperCase()
    if (!/^[A-Z]{2}$/.test(code) || seen.has(code)) {
      continue
    }
    seen.add(code)
    normalized.push(code)
  }
  return normalized.sort()
}

const normalizeFormValues = (values: GeoIPFormValues): GeoIPSettings => ({
  'geoip.mode': values.geoip.mode,
  'geoip.database_path': values.geoip.database_path,
  'geoip.download_url': values.geoip.download_url,
  'geoip.maxmind_license_key': values.geoip.maxmind_license_key,
  'geoip.popup_message': values.geoip.popup_message,
  'geoip.allow_private_loopback': values.geoip.allow_private_loopback,
  'geoip.blocked_countries': normalizeCountryCodes(
    values.geoip.blocked_countries
  ),
})

const normalizeDefaults = (defaults: GeoIPSettings): GeoIPSettings => ({
  ...defaults,
  'geoip.maxmind_license_key': '',
  'geoip.blocked_countries': normalizeCountryCodes(
    defaults['geoip.blocked_countries']
  ),
})

const valuesEqual = (a: unknown, b: unknown) => {
  if (Array.isArray(a) && Array.isArray(b)) {
    return JSON.stringify(a) === JSON.stringify(b)
  }
  return a === b
}

export function GeoIPSection({ defaultValues }: GeoIPSectionProps) {
  const { t } = useTranslation()
  const updateOption = useUpdateOption()
  const [isDownloading, setIsDownloading] = useState(false)
  const displayDefaults = useMemo<GeoIPSettings>(() => {
    const popupMessage = defaultValues['geoip.popup_message']
    return {
      ...defaultValues,
      'geoip.popup_message':
        popupMessage === DEFAULT_GEOIP_POPUP_MESSAGE
          ? t(DEFAULT_GEOIP_POPUP_MESSAGE)
          : popupMessage,
    }
  }, [defaultValues, t])
  const baselineRef = useRef<GeoIPSettings>(normalizeDefaults(displayDefaults))

  const formDefaults = useMemo(
    () => buildFormDefaults(displayDefaults),
    [displayDefaults]
  )

  const form = useForm<GeoIPFormInput, unknown, GeoIPFormValues>({
    resolver: zodResolver(geoIPSchema),
    defaultValues: formDefaults,
  })

  useEffect(() => {
    baselineRef.current = normalizeDefaults(displayDefaults)
    form.reset(buildFormDefaults(displayDefaults))
  }, [displayDefaults, form])

  const onSubmit = async (values: GeoIPFormValues) => {
    const normalized = normalizeFormValues(values)
    const updates = (Object.keys(normalized) as Array<keyof GeoIPSettings>)
      .filter(
        (key) =>
          key !== 'geoip.maxmind_license_key' || normalized[key] !== ''
      )
      .filter((key) => !valuesEqual(normalized[key], baselineRef.current[key]))

    if (updates.length === 0) {
      toast.info(t('No changes to save'))
      return
    }

    for (const key of updates) {
      const value = normalized[key]
      await updateOption.mutateAsync({
        key,
        value: Array.isArray(value) ? JSON.stringify(value) : value,
      })
    }

    baselineRef.current = normalizeDefaults(normalized)
    form.setValue('geoip.maxmind_license_key', '')
  }

  const handleDownload = async () => {
    setIsDownloading(true)
    try {
      const response = await downloadGeoIPDatabase()
      if (response.success) {
        toast.success(t('GeoIP database downloaded'))
      } else {
        toast.error(response.message || t('Download failed'))
      }
    } catch (error) {
      toast.error(error instanceof Error ? error.message : t('Download failed'))
    } finally {
      setIsDownloading(false)
    }
  }

  return (
    <SettingsSection title={t('GeoIP Access Restriction')}>
      <Form {...form}>
        <SettingsForm onSubmit={form.handleSubmit(onSubmit)}>
          <SettingsPageFormActions
            onSave={form.handleSubmit(onSubmit)}
            isSaving={updateOption.isPending}
            saveLabel='Save GeoIP settings'
          />

          <FormField
            control={form.control}
            name='geoip.mode'
            render={({ field }) => (
              <FormItem data-settings-form-span='full'>
                <FormLabel>{t('GeoIP access mode')}</FormLabel>
                <FormControl>
                  <RadioGroup
                    className='grid gap-2 lg:grid-cols-2'
                    value={field.value}
                    onValueChange={field.onChange}
                  >
                    {geoIPModes.map((mode) => (
                      <label
                        key={mode.value}
                        className={cn(
                          'border-input hover:bg-muted/40 flex min-h-12 cursor-pointer items-start gap-3 rounded-xl border px-3 py-3 text-sm transition-colors',
                          field.value === mode.value &&
                            'border-primary ring-primary/20 ring-1'
                        )}
                      >
                        <RadioGroupItem value={mode.value} className='mt-0.5' />
                        <span className='min-w-0'>
                          <span className='block font-medium'>
                            {t(mode.title)}
                          </span>
                          <span className='text-muted-foreground block text-xs leading-5'>
                            {t(mode.description)}
                          </span>
                        </span>
                      </label>
                    ))}
                  </RadioGroup>
                </FormControl>
                <FormMessage />
              </FormItem>
            )}
          />

          <FormField
            control={form.control}
            name='geoip.database_path'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('GeoIP database path')}</FormLabel>
                <FormControl>
                  <Input placeholder='Country.mmdb' {...field} />
                </FormControl>
                <FormDescription>
                  {t('File path for the downloaded GeoIP2 country database.')}
                </FormDescription>
                <FormMessage />
              </FormItem>
            )}
          />

          <FormField
            control={form.control}
            name='geoip.download_url'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('Download source URL')}</FormLabel>
                <FormControl>
                  <Input
                    placeholder='https://example.com/Country.mmdb'
                    {...field}
                  />
                </FormControl>
                <FormDescription>
                  {t('Direct download URL for .mmdb, .mmdb.gz, .tar.gz or .zip.')}
                </FormDescription>
                <FormMessage />
              </FormItem>
            )}
          />

          <FormField
            control={form.control}
            name='geoip.maxmind_license_key'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('MaxMind License Key')}</FormLabel>
                <FormControl>
                  <Input
                    type='password'
                    autoComplete='off'
                    placeholder={t('Paste MaxMind License Key')}
                    {...field}
                  />
                </FormControl>
                <FormDescription>
                  {t(
                    'Used when no direct download URL is set. It is saved on the backend and hidden after refresh.'
                  )}
                </FormDescription>
                <FormMessage />
              </FormItem>
            )}
          />

          <div data-settings-form-span='full'>
            <Button
              type='button'
              variant='outline'
              className='w-full'
              disabled={isDownloading}
              onClick={handleDownload}
            >
              {isDownloading ? (
                <Save className='size-4 animate-pulse' />
              ) : (
                <Download className='size-4' />
              )}
              {t('Download database now')}
            </Button>
          </div>

          <FormField
            control={form.control}
            name='geoip.popup_message'
            render={({ field }) => (
              <FormItem data-settings-form-span='full'>
                <FormLabel>{t('GeoIP popup message')}</FormLabel>
                <FormControl>
                  <Textarea rows={3} {...field} />
                </FormControl>
                <FormDescription>
                  {t('Message shown to visitors from blocked countries or regions.')}
                </FormDescription>
                <FormMessage />
              </FormItem>
            )}
          />

          <FormField
            control={form.control}
            name='geoip.allow_private_loopback'
            render={({ field }) => (
              <SettingsSwitchItem>
                <SettingsSwitchContent>
                  <FormLabel>
                    {t('Allow private and local loopback IPs to skip GeoIP checks')}
                  </FormLabel>
                  <FormDescription>
                    {t(
                      'Applies to localhost, private networks, and reverse proxies passing internal client addresses.'
                    )}
                  </FormDescription>
                </SettingsSwitchContent>
                <FormControl>
                  <Switch
                    checked={field.value}
                    onCheckedChange={field.onChange}
                  />
                </FormControl>
              </SettingsSwitchItem>
            )}
          />

          <FormField
            control={form.control}
            name='geoip.blocked_countries'
            render={({ field }) => (
              <FormItem data-settings-form-span='full'>
                <FormLabel>{t('Blocked countries or regions')}</FormLabel>
                <FormControl>
                  <MultiSelect
                    options={countryOptions}
                    selected={field.value}
                    onChange={(values) =>
                      field.onChange(normalizeCountryCodes(values))
                    }
                    allowCreate
                    placeholder={t('Search country or enter ISO code')}
                    createLabel='Add "{{value}}"'
                    emptyText={t('No matching countries')}
                  />
                </FormControl>
                <FormDescription>
                  {t(
                    'Search by country or region name, or enter an ISO 3166-1 alpha-2 country code.'
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
