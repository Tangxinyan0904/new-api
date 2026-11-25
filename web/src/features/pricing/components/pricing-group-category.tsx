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
import type { ReactNode } from 'react'
import { useTranslation } from 'react-i18next'

export function PricingGroupCategory(props: {
  label: 'Claude' | 'GPT' | 'Other'
  children: ReactNode
}) {
  const { t } = useTranslation()
  const title = props.label === 'Other' ? t('Other') : props.label
  return (
    <div role='group' aria-label={title} className='space-y-2 py-1'>
      <h3 className='text-muted-foreground text-xs font-medium'>{title}</h3>
      {props.children}
    </div>
  )
}
