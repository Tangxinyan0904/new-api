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
import type { Control } from 'react-hook-form'
import { useTranslation } from 'react-i18next'

import { FieldDescription, FieldLegend, FieldSet } from '@/components/ui/field'
import {
  FormControl,
  FormDescription,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from '@/components/ui/form'
import { Input } from '@/components/ui/input'
import { Switch } from '@/components/ui/switch'

import {
  SettingsSwitchContent,
  SettingsSwitchItem,
} from '../components/settings-form-layout'
import type { RateLimitFormValues } from './types'

type DistillationSettingsProps = {
  control: Control<RateLimitFormValues>
  enabled: boolean
}

function parsePositiveSetting(value: string): number {
  const parsed = Number.parseInt(value, 10)
  return Number.isNaN(parsed) ? 0 : parsed
}

export function DistillationSettings(props: DistillationSettingsProps) {
  const { t } = useTranslation()

  return (
    <FieldSet
      data-settings-form-span='full'
      className='bg-muted/20 rounded-xl border p-4'
    >
      <FieldLegend>{t('Distillation protection')}</FieldLegend>
      <FieldDescription>
        {t(
          'First trigger applies a temporary limit. A second trigger during the observation period permanently blocks the user until an administrator clears it.'
        )}
      </FieldDescription>

      <FormField
        control={props.control}
        name='ModelRequestRateLimitDistillationEnabled'
        render={({ field }) => (
          <SettingsSwitchItem className='py-0'>
            <SettingsSwitchContent>
              <FormLabel>{t('Enable distillation detection')}</FormLabel>
              <FormDescription>
                {t(
                  'Count non-stream model requests in a rolling one-minute window.'
                )}
              </FormDescription>
            </SettingsSwitchContent>
            <FormControl>
              <Switch checked={field.value} onCheckedChange={field.onChange} />
            </FormControl>
          </SettingsSwitchItem>
        )}
      />

      <div className='grid grid-cols-1 gap-4 sm:grid-cols-2 xl:grid-cols-4'>
        <FormField
          control={props.control}
          name='ModelRequestRateLimitDistillationThreshold'
          render={({ field }) => (
            <FormItem>
              <FormLabel>{t('Detection threshold')}</FormLabel>
              <FormControl>
                <Input
                  type='number'
                  min={1}
                  max={2147483647}
                  step={1}
                  disabled={!props.enabled}
                  {...field}
                  onChange={(event) =>
                    field.onChange(parsePositiveSetting(event.target.value))
                  }
                />
              </FormControl>
              <FormDescription>
                {t('non-stream requests / minute')}
              </FormDescription>
              <FormMessage />
            </FormItem>
          )}
        />

        <FormField
          control={props.control}
          name='ModelRequestRateLimitDistillationRPM'
          render={({ field }) => (
            <FormItem>
              <FormLabel>{t('Penalty RPM')}</FormLabel>
              <FormControl>
                <Input
                  type='number'
                  min={1}
                  max={2147483647}
                  step={1}
                  disabled={!props.enabled}
                  {...field}
                  onChange={(event) =>
                    field.onChange(parsePositiveSetting(event.target.value))
                  }
                />
              </FormControl>
              <FormDescription>{t('requests / minute')}</FormDescription>
              <FormMessage />
            </FormItem>
          )}
        />

        <FormField
          control={props.control}
          name='ModelRequestRateLimitDistillationPenaltyMinutes'
          render={({ field }) => (
            <FormItem>
              <FormLabel>{t('Penalty duration')}</FormLabel>
              <FormControl>
                <Input
                  type='number'
                  min={1}
                  max={2147483647}
                  step={1}
                  disabled={!props.enabled}
                  {...field}
                  onChange={(event) =>
                    field.onChange(parsePositiveSetting(event.target.value))
                  }
                />
              </FormControl>
              <FormDescription>{t('minutes')}</FormDescription>
              <FormMessage />
            </FormItem>
          )}
        />

        <FormField
          control={props.control}
          name='ModelRequestRateLimitDistillationObservationMinutes'
          render={({ field }) => (
            <FormItem>
              <FormLabel>{t('Observation period')}</FormLabel>
              <FormControl>
                <Input
                  type='number'
                  min={1}
                  max={2147483647}
                  step={1}
                  disabled={!props.enabled}
                  {...field}
                  onChange={(event) =>
                    field.onChange(parsePositiveSetting(event.target.value))
                  }
                />
              </FormControl>
              <FormDescription>{t('minutes')}</FormDescription>
              <FormMessage />
            </FormItem>
          )}
        />
      </div>
    </FieldSet>
  )
}
