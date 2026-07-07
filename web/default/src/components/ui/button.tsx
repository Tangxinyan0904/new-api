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
import { Button as ButtonPrimitive } from '@base-ui/react/button'
import { cva, type VariantProps } from 'class-variance-authority'
import { isValidElement } from 'react'

import { cn } from '@/lib/utils'

const buttonVariants = cva(
  "group/button inline-flex shrink-0 items-center justify-center rounded-full border border-transparent bg-clip-padding font-bold whitespace-nowrap outline-none select-none focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/50 active:not-aria-[haspopup]:translate-y-px disabled:pointer-events-none disabled:opacity-50 transition-all duration-200 ease-[cubic-bezier(0.175,0.885,0.32,1.275)] [&_svg]:pointer-events-none [&_svg]:shrink-0 [&_svg:not([class*='size-'])]:size-4",
  {
    variants: {
      variant: {
        default: 
          'bg-gradient-to-br from-[#64b5f6] to-[#2196f3] text-white shadow-[0_6px_16px_-4px_rgba(100,181,246,0.5)] hover:-translate-y-[3px] hover:scale-[1.03] hover:shadow-[0_8px_20px_-2px_rgba(100,181,246,0.6)]',
        outline:
          'border-2 border-dashed border-[#64b5f6] bg-[#f0f8ff] text-[#2196f3] shadow-none hover:bg-[#64b5f6] hover:text-white hover:-translate-y-[3px] hover:scale-[1.03] dark:bg-[#1a2436] dark:border-[#42a5f5] dark:text-[#42a5f5]',
        pay: 
          'bg-gradient-to-br from-[#ffb3c6] to-[#ff758f] text-white shadow-[0_6px_16px_-4px_rgba(255,117,143,0.5)] hover:-translate-y-[3px] hover:scale-[1.03] hover:shadow-[0_8px_20px_-2px_rgba(255,117,143,0.6)]',
        image: 
          'bg-gradient-to-br from-[#a855f7] to-[#b388ff] text-white shadow-[0_6px_16px_-4px_rgba(179,136,255,0.5)] hover:-translate-y-[3px] hover:scale-[1.03]',
        secondary:
          'bg-secondary text-secondary-foreground hover:bg-secondary/80 aria-expanded:bg-secondary aria-expanded:text-secondary-foreground',
        ghost:
          'hover:bg-muted hover:text-foreground aria-expanded:bg-muted aria-expanded:text-foreground dark:hover:bg-muted/50',
        destructive:
          'bg-destructive/10 text-destructive hover:bg-destructive/20 focus-visible:border-destructive/40 focus-visible:ring-destructive/20 dark:bg-destructive/20 dark:hover:bg-destructive/30 dark:focus-visible:ring-destructive/40',
        link: 'text-primary underline-offset-4 hover:underline',
      },
      size: {
        default:
          'h-[2.8rem] px-6 text-[0.95rem] gap-2 has-data-[icon=inline-end]:pr-4 has-data-[icon=inline-start]:pl-4',
        xs: "h-6 gap-1 px-2 text-xs in-data-[slot=button-group]:rounded-full has-data-[icon=inline-end]:pr-1.5 has-data-[icon=inline-start]:pl-1.5 [&_svg:not([class*='size-'])]:size-3",
        sm: "h-8 gap-1.5 px-3 text-[0.85rem] in-data-[slot=button-group]:rounded-full has-data-[icon=inline-end]:pr-2 has-data-[icon=inline-start]:pl-2 [&_svg:not([class*='size-'])]:size-3.5",
        lg: 'h-12 gap-2 px-8 text-base has-data-[icon=inline-end]:pr-6 has-data-[icon=inline-start]:pl-6',
        icon: 'size-[2.8rem]',
        'icon-xs':
          "size-6 in-data-[slot=button-group]:rounded-full [&_svg:not([class*='size-'])]:size-3",
        'icon-sm': 'size-8 in-data-[slot=button-group]:rounded-full',
        'icon-lg': 'size-12',
      },
    },
    defaultVariants: {
      variant: 'default',
      size: 'default',
    },
  }
)

function isNativeButtonRender(render: ButtonPrimitive.Props['render']) {
  if (!render || !isValidElement(render)) {
    return true
  }

  return render.type === 'button'
}

function Button({
  className,
  variant = 'default',
  size = 'default',
  nativeButton,
  render,
  ...props
}: ButtonPrimitive.Props & VariantProps<typeof buttonVariants>) {
  return (
    <ButtonPrimitive
      data-slot='button'
      className={cn(buttonVariants({ variant, size, className }))}
      nativeButton={nativeButton ?? isNativeButtonRender(render)}
      render={render}
      {...props}
    />
  )
}

export { Button, buttonVariants }