/*
Copyright (C) 2023-2026 QuantumNous
*/
import * as React from 'react'
import { cn } from '@/lib/utils'

function Card({
  className,
  size = 'default',
  ...props
}: React.ComponentProps<'div'> & { size?: 'default' | 'sm' }) {
  return (
    <div
      data-slot='card'
      data-size={size}
      className={cn(
        // 核心：复用全局定义的 card 基础样式，只覆盖外观样式
        'group/card flex flex-col gap-4 overflow-hidden rounded-[1.75rem] py-4 text-sm',
        // 风格化：3px 边框 + 右下角硬阴影
        'bg-white border-[3px] border-[#ffd1dc] shadow-[3px_3px_0px_#ffd1dc]',
        'dark:bg-[#151d2a] dark:border-[#3b2d35] dark:shadow-[3px_3px_0px_#3b2d35]',
        // 确保覆盖 shadcn 默认的 ring 样式
        'ring-0',
        className
      )}
      {...props}
    />
  )
}

function CardHeader({ className, ...props }: React.ComponentProps<'div'>) {
  return (
    <div
      data-slot='card-header'
      className={cn(
        'group/card-header flex flex-col gap-1 px-5 pt-5 pb-4',
        // 内部虚线分割
        'border-b-2 border-dashed border-[#ffd1dc] dark:border-[#3b2d35]',
        className
      )}
      {...props}
    />
  )
}

function CardTitle({ className, ...props }: React.ComponentProps<'div'>) {
  return (
    <div
      data-slot='card-title'
      className={cn(
        'text-base leading-snug font-black text-[#2c3e50] dark:text-[#e2e8f0]',
        className
      )}
      {...props}
    />
  )
}

function CardDescription({ className, ...props }: React.ComponentProps<'div'>) {
  return (
    <div
      data-slot='card-description'
      className={cn('text-sm font-bold text-[#7f8c8d] dark:text-[#94a3b8]', className)}
      {...props}
    />
  )
}

function CardAction({ className, ...props }: React.ComponentProps<'div'>) {
  return (
    <div
      data-slot='card-action'
      className={cn('flex items-center gap-2', className)}
      {...props}
    />
  )
}

function CardContent({ className, ...props }: React.ComponentProps<'div'>) {
  return (
    <div
      data-slot='card-content'
      className={cn('px-5 py-4 text-[#2c3e50] dark:text-[#e2e8f0]', className)}
      {...props}
    />
  )
}

function CardFooter({ className, ...props }: React.ComponentProps<'div'>) {
  return (
    <div
      data-slot='card-footer'
      className={cn(
        'flex items-center p-5 pt-0',
        // 底部背景与分割线
        'bg-[#f0f8ff] dark:bg-[#1a2436] border-t-2 border-dashed border-[#ffd1dc] dark:border-[#3b2d35]',
        className
      )}
      {...props}
    />
  )
}

export {
  Card,
  CardHeader,
  CardFooter,
  CardTitle,
  CardAction,
  CardDescription,
  CardContent,
}