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
import { useMutation, useQueryClient } from '@tanstack/react-query'
import type { ChangeEvent } from 'react'
import type { Resolver } from 'react-hook-form'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import * as z from 'zod'

import { Alert, AlertDescription } from '@/components/ui/alert'
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
import { Switch } from '@/components/ui/switch'
import { formatQuota } from '@/lib/format'

import { updateAffiliateOptions } from '../api'
import { FormDirtyIndicator } from '../components/form-dirty-indicator'
import { FormNavigationGuard } from '../components/form-navigation-guard'
import {
  SettingsForm,
  SettingsFormGrid,
  SettingsFormGridItem,
  SettingsSwitchContent,
  SettingsSwitchItem,
} from '../components/settings-form-layout'
import { SettingsPageFormActions } from '../components/settings-page-context'
import { SettingsSection } from '../components/settings-section'
import { useSettingsForm } from '../hooks/use-settings-form'
import type { AffiliateOptionsRequest } from '../types'

const hasAtMostTwoDecimals = (value: number) =>
  Math.abs(Math.round(value * 100) - value * 100) < 1e-6

const affiliateSchema = z.object({
  rebate_enabled: z.boolean(),
  rebate_rate_percent: z.coerce
    .number()
    .min(0)
    .max(100)
    .refine(hasAtMostTwoDecimals, {
      message: 'Use at most two decimal places',
    }),
  min_transfer_quota: z.coerce.number().int().min(0),
  daily_request_limit: z.coerce.number().int().min(0).max(100),
  require_approval: z.boolean(),
  forfeit_on_reject: z.boolean(),
  quota_for_inviter: z.coerce.number().int().min(0),
  quota_for_invitee: z.coerce.number().int().min(0),
})

export type AffiliateSettingsFormValues = z.infer<typeof affiliateSchema>
type NumberInputValue = number | ''

type AffiliateSettingsSectionProps = {
  defaultValues: AffiliateSettingsFormValues
  complianceConfirmed?: boolean
}

export function AffiliateSettingsSection({
  defaultValues,
  complianceConfirmed = true,
}: AffiliateSettingsSectionProps) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const saveOptions = useMutation({ mutationFn: updateAffiliateOptions })

  const handleNumberChange =
    (onChange: (value: NumberInputValue) => void) =>
    (event: ChangeEvent<HTMLInputElement>) => {
      const value = event.currentTarget.valueAsNumber
      onChange(Number.isNaN(value) ? '' : value)
    }

  const { form, handleSubmit, isDirty, isSubmitting } =
    useSettingsForm<AffiliateSettingsFormValues>({
      resolver: zodResolver(affiliateSchema) as Resolver<
        AffiliateSettingsFormValues,
        unknown,
        AffiliateSettingsFormValues
      >,
      defaultValues,
      onSubmit: async (data) => {
        const request: AffiliateOptionsRequest = { ...data }
        const response = await saveOptions
          .mutateAsync(request)
          .catch((error: unknown) => {
            toast.error(
              error instanceof Error
                ? error.message
                : t('Failed to update setting')
            )
            throw error
          })
        if (!response.success) {
          const message = response.message || t('Failed to update setting')
          toast.error(message)
          // Keep the form dirty so the administrator can correct and retry.
          throw new Error(message)
        }
        await queryClient.invalidateQueries({ queryKey: ['system-options'] })
        toast.success(t('Setting updated successfully'))
      },
    })

  const rebateEnabled = form.watch('rebate_enabled')

  const renderNumberField = (
    name:
      | 'rebate_rate_percent'
      | 'min_transfer_quota'
      | 'daily_request_limit'
      | 'quota_for_inviter'
      | 'quota_for_invitee',
    label: string,
    describe: (value: NumberInputValue) => string,
    step?: string
  ) => (
    <FormField
      control={form.control}
      name={name}
      render={({ field }) => (
        <FormItem>
          <FormLabel>{label}</FormLabel>
          <FormControl>
            <Input
              type='number'
              inputMode='decimal'
              step={step}
              min={0}
              value={field.value ?? ''}
              onChange={handleNumberChange(field.onChange)}
              name={field.name}
              onBlur={field.onBlur}
              ref={field.ref}
            />
          </FormControl>
          <FormDescription>{describe(field.value)}</FormDescription>
          <FormMessage />
        </FormItem>
      )}
    />
  )

  const renderSwitchField = (
    name: 'rebate_enabled' | 'require_approval' | 'forfeit_on_reject',
    label: string,
    description: string
  ) => (
    <SettingsFormGridItem span='full'>
      <FormField
        control={form.control}
        name={name}
        render={({ field }) => (
          <SettingsSwitchItem>
            <SettingsSwitchContent>
              <FormLabel>{label}</FormLabel>
              <FormDescription>{description}</FormDescription>
            </SettingsSwitchContent>
            <FormControl>
              <Switch
                checked={field.value}
                onCheckedChange={field.onChange}
                disabled={saveOptions.isPending}
              />
            </FormControl>
          </SettingsSwitchItem>
        )}
      />
    </SettingsFormGridItem>
  )

  const quotaPreview = (value: NumberInputValue) =>
    formatQuota(value === '' ? 0 : value)

  return (
    <SettingsSection title={t('Invitation Rebate')}>
      <FormNavigationGuard when={isDirty} />

      {!complianceConfirmed ? (
        <Alert variant='destructive'>
          <AlertDescription>
            {t(
              'Non-zero invitation rewards require compliance confirmation in Payment Gateway settings.'
            )}
          </AlertDescription>
        </Alert>
      ) : null}

      <Form {...form}>
        <SettingsForm onSubmit={handleSubmit}>
          <SettingsPageFormActions
            onSave={handleSubmit}
            isSaving={saveOptions.isPending || isSubmitting}
          />
          <FormDirtyIndicator isDirty={isDirty} />
          <SettingsFormGrid>
            {renderSwitchField(
              'rebate_enabled',
              t('Enable Recharge Rebate'),
              t(
                'When disabled, new top-ups earn no rebate and earned rebate is frozen and hidden from wallets until it is enabled again.'
              )
            )}

            {rebateEnabled
              ? renderNumberField(
                  'rebate_rate_percent',
                  t('Rebate Rate (%)'),
                  () =>
                    t(
                      'Percentage of invited users credited recharge paid to the inviter. A new rate only applies to top-ups completed after saving.'
                    ),
                  '0.01'
                )
              : null}

            {renderNumberField(
              'min_transfer_quota',
              t('Minimum Transfer Amount'),
              (value) =>
                t(
                  'Smallest amount per transfer request ({{formattedQuota}}). 0 means no minimum.',
                  { formattedQuota: quotaPreview(value) }
                )
            )}

            {renderNumberField(
              'daily_request_limit',
              t('Daily Transfer Requests'),
              () =>
                t(
                  'Maximum transfer requests per user per day. 0 means unlimited.'
                )
            )}

            {renderSwitchField(
              'require_approval',
              t('Require Administrator Approval'),
              t(
                'When disabled, transfer requests are credited to the user balance immediately.'
              )
            )}

            {renderSwitchField(
              'forfeit_on_reject',
              t('Forfeit Rewards on Rejection'),
              t(
                'When enabled, a rejected request permanently consumes its amount. When disabled, the amount becomes available to request again.'
              )
            )}

            {renderNumberField(
              'quota_for_inviter',
              t('Inviter Reward'),
              (value) =>
                t(
                  'Quota given to users who invite others ({{formattedQuota}})',
                  { formattedQuota: quotaPreview(value) }
                )
            )}

            {renderNumberField(
              'quota_for_invitee',
              t('Invitee Reward'),
              (value) =>
                t('Quota given to invited users ({{formattedQuota}})', {
                  formattedQuota: quotaPreview(value),
                })
            )}
          </SettingsFormGrid>
        </SettingsForm>
      </Form>
    </SettingsSection>
  )
}
