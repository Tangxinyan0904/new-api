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

import { cn } from '@/lib/utils'

export function GroupPriceRatio(props: {
  ratio: number
  originalRatio?: number
  className?: string
  children?: ReactNode
}) {
  const { t } = useTranslation()
  const actual = props.children ?? `${props.ratio}x`
  if (
    typeof props.originalRatio !== 'number' ||
    props.originalRatio === props.ratio
  ) {
    return <span className={props.className}>{actual}</span>
  }

  return (
    <span
      className={cn('inline-flex min-w-0 flex-col gap-0.5', props.className)}
    >
      <span className='inline-flex flex-wrap items-baseline gap-x-1.5 tabular-nums'>
        <del className='text-muted-foreground'>
          <span className='sr-only'>{t('Base')} </span>
          {props.originalRatio}x
        </del>
        <span className='sr-only'>{t('Current')} </span>
        {actual}
      </span>
      <small className='text-muted-foreground text-[11px] leading-tight font-normal whitespace-normal'>
        {t('Your special group rate applies')}
      </small>
    </span>
  )
}
