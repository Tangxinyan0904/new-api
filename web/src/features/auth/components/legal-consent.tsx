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
import { useTranslation } from 'react-i18next'

import { Checkbox } from '@/components/ui/checkbox'
import { Label } from '@/components/ui/label'
import { cn } from '@/lib/utils'

import type { SystemStatus } from '../types'

interface LegalConsentProps {
  status: SystemStatus | null
  checked: boolean
  onCheckedChange: (nextValue: boolean) => void
  className?: string
}

export function LegalConsent(props: LegalConsentProps) {
  const { t } = useTranslation()
  const hasUserAgreement = Boolean(props.status?.user_agreement_enabled)
  const hasPrivacyPolicy = Boolean(props.status?.privacy_policy_enabled)

  if (!hasUserAgreement && !hasPrivacyPolicy) {
    return null
  }

  const handleChange = (value: boolean) => {
    props.onCheckedChange(value === true)
  }

  return (
    <Label
      htmlFor='legal-consent'
      className={cn(
        'focus-within:ring-ring/60 flex cursor-pointer items-start gap-3 rounded-lg border-2 px-3 py-2.5 text-left text-sm leading-5 font-medium transition-colors focus-within:ring-3 focus-within:ring-offset-2',
        props.checked
          ? 'border-primary/70 bg-primary/10 text-foreground hover:bg-primary/15'
          : 'border-destructive/70 bg-destructive/10 text-foreground hover:bg-destructive/15',
        props.className
      )}
    >
      <Checkbox
        id='legal-consent'
        checked={props.checked}
        onCheckedChange={handleChange}
        className='border-foreground/70 bg-background mt-0.5 size-5 border-2 shadow-sm'
      />
      <span className='min-w-0 flex-1'>
        {t('I have read and agree to the')}{' '}
        {hasUserAgreement && (
          <a
            href='/user-agreement'
            target='_blank'
            rel='noopener noreferrer'
            className='text-primary focus-visible:ring-ring/60 rounded-sm font-semibold underline underline-offset-4 focus-visible:ring-2 focus-visible:outline-none'
          >
            {t('User Agreement')}
          </a>
        )}
        {hasUserAgreement && hasPrivacyPolicy && <> {t('and')} </>}
        {hasPrivacyPolicy && (
          <a
            href='/privacy-policy'
            target='_blank'
            rel='noopener noreferrer'
            className='text-primary focus-visible:ring-ring/60 rounded-sm font-semibold underline underline-offset-4 focus-visible:ring-2 focus-visible:outline-none'
          >
            {t('Privacy Policy')}
          </a>
        )}
        .
      </span>
    </Label>
  )
}
