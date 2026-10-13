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
import type { LucideIcon } from 'lucide-react'

import { IconBadge, type IconBadgeTone } from '@/components/ui/icon-badge'
import { Skeleton } from '@/components/ui/skeleton'
import { cn } from '@/lib/utils'

export interface StatStripItem {
  key: string
  title: string
  value: string
  /** Shown as a tooltip when value is abbreviated. */
  fullValue?: string
  description: string
  icon: LucideIcon
  iconTone: IconBadgeTone
  loading?: boolean
  error?: boolean
}

interface StatStripProps {
  items: StatStripItem[]
  /** Grid column classes for each breakpoint. */
  columnsClassName: string
  /** Keep descriptions on small screens, where they are hidden by default. */
  alwaysShowDescription?: boolean
  /** Show titles as written, e.g. model names, instead of upper case. */
  preserveTitleCase?: boolean
}

/** A row of compact stats divided by vertical rules, as on the dashboard. */
export function StatStrip(props: StatStripProps) {
  const descriptionClassName = props.alwaysShowDescription
    ? 'mt-1 text-xs'
    : 'mt-1 hidden text-xs md:block'

  return (
    <div
      className={cn(
        'divide-border/60 grid min-w-0 divide-x',
        props.columnsClassName
      )}
    >
      {props.items.map((item, idx) => {
        const Icon = item.icon
        let valueContent
        if (item.loading) {
          valueContent = (
            <div className='mt-1 flex flex-col gap-1 sm:mt-2 sm:gap-1.5'>
              <Skeleton className='h-5 w-16 sm:h-7 sm:w-20' />
              <Skeleton className='hidden h-3.5 w-28 md:block' />
            </div>
          )
        } else if (item.error) {
          valueContent = (
            <>
              <div className='text-muted-foreground mt-1 font-mono text-base leading-tight font-bold tracking-tight tabular-nums sm:mt-2 sm:text-2xl sm:leading-normal'>
                --
              </div>
              <div
                className={cn('text-muted-foreground/40', descriptionClassName)}
              >
                {item.description}
              </div>
            </>
          )
        } else {
          valueContent = (
            <>
              <div
                className='text-foreground mt-1 max-w-full truncate font-mono text-base leading-tight font-bold tracking-tight tabular-nums sm:mt-2 sm:text-2xl sm:leading-normal'
                title={item.fullValue}
              >
                {item.value}
              </div>
              <div
                className={cn(
                  'text-muted-foreground/60 truncate',
                  descriptionClassName
                )}
                title={item.description}
              >
                {item.description}
              </div>
            </>
          )
        }

        return (
          <div
            key={item.key}
            className={cn(
              'min-w-0 px-2.5 py-1.5 sm:px-5 sm:py-4',
              idx === props.items.length - 1 &&
                props.items.length % 2 !== 0 &&
                'col-span-2 sm:col-span-1'
            )}
          >
            <div className='flex min-w-0 items-center gap-1.5 sm:gap-2'>
              <IconBadge
                tone={item.iconTone}
                size='stat'
                className='size-4 rounded-sm sm:size-7 sm:rounded-md [&>svg]:size-2.5 sm:[&>svg]:size-3.5'
              >
                <Icon />
              </IconBadge>
              <div
                className={cn(
                  'text-muted-foreground truncate text-[11px] leading-4 font-medium',
                  !props.preserveTitleCase &&
                    'tracking-wide uppercase sm:tracking-wider',
                  'sm:text-xs'
                )}
                title={item.title}
              >
                {item.title}
              </div>
            </div>

            {valueContent}
          </div>
        )
      })}
    </div>
  )
}
