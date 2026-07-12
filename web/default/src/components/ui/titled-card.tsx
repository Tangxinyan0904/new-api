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

import { cn } from '@/lib/utils'

import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from './card'
import { IconBadge, type IconBadgeTone } from './icon-badge'

type TitledCardProps = {
  title: ReactNode
  description?: ReactNode
  icon?: ReactNode
  action?: ReactNode
  children?: ReactNode
  disableHoverEffect?: boolean
  className?: string
  headerClassName?: string
  contentClassName?: string
  iconClassName?: string
  iconTone?: IconBadgeTone
  titleClassName?: string
  descriptionClassName?: string
}

export function TitledCard({
  title,
  description,
  icon,
  action,
  children,
  disableHoverEffect,
  className,
  headerClassName,
  contentClassName,
  iconClassName,
  iconTone,
  titleClassName,
  descriptionClassName,
}: TitledCardProps) {
  return (
    <Card
      data-card-hover={disableHoverEffect ? 'false' : undefined}
      className={cn(
        'relative gap-0 overflow-hidden rounded-[1.75rem] border-[3px] border-[#ffd1dc] bg-white py-0 shadow-[3px_3px_0px_#ffd1dc] dark:border-[#3b2d35] dark:bg-[#151d2a] dark:shadow-[3px_3px_0px_#3b2d35]',
        !disableHoverEffect &&
          'transition-transform duration-200 hover:-translate-y-1',
        className
      )}
    >
      <div
        aria-hidden='true'
        className='pointer-events-none absolute -top-8 -right-8 z-0 h-24 w-24 rounded-full bg-[#ffb3c6]/25'
      />
      <div
        aria-hidden='true'
        className='pointer-events-none absolute -bottom-8 -left-8 z-0 h-24 w-24 rounded-full bg-[#64b5f6]/20'
      />
      <CardHeader
        className={cn(
          'relative z-10 border-b-2 border-dashed border-[#ffd1dc] p-4 dark:border-[#3b2d35] sm:p-5',
          headerClassName
        )}
      >
        <div className='flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between'>
          <div className='flex min-w-0 items-center gap-3'>
            {icon != null && (
              <IconBadge
                size='title'
                tone={iconTone}
                className={cn(
                  'size-10 rounded-full sm:size-10',
                  iconTone == null &&
                    'bg-gradient-to-br from-[#ffb3c6] to-[#ff758f] text-white shadow-[0_4px_10px_rgba(255,117,143,0.4)]',
                  iconClassName
                )}
              >
                {icon}
              </IconBadge>
            )}
            <div className='min-w-0'>
              <CardTitle
                className={cn(
                  'text-lg font-black tracking-tight text-[#2c3e50] dark:text-[#e2e8f0] sm:text-xl',
                  titleClassName
                )}
              >
                {title}
              </CardTitle>
              {description != null && (
                <CardDescription
                  className={cn(
                    'mt-1 text-sm font-bold text-[#7f8c8d] dark:text-[#94a3b8]',
                    descriptionClassName
                  )}
                >
                  {description}
                </CardDescription>
              )}
            </div>
          </div>
          {action != null && (
            <div className='mt-2 w-full shrink-0 sm:mt-0 sm:w-auto'>
              {action}
            </div>
          )}
        </div>
      </CardHeader>
      <CardContent
        className={cn(
          'relative z-10 p-4 text-[#2c3e50] dark:text-[#e2e8f0] sm:p-5',
          contentClassName
        )}
      >
        {children}
      </CardContent>
    </Card>
  )
}
