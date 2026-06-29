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
import { memo, type MouseEvent, type ReactNode } from 'react'
import { ChevronRight, Copy } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { getLobeIcon } from '@/lib/lobe-icon'
import { cn } from '@/lib/utils'
import { useCopyToClipboard } from '@/hooks/use-copy-to-clipboard'
import { DEFAULT_TOKEN_UNIT } from '../constants'
import {
  getDynamicDisplayGroupRatio,
  getDynamicPricingSummary,
} from '../lib/dynamic-price'
import { isTokenBasedModel } from '../lib/model-helpers'
import { formatPrice, formatRequestPrice } from '../lib/price'
import type { PricingModel, TokenUnit } from '../types'
import { ModelPerfBadge, type ModelPerfBadgeData } from './model-perf-badge'

export interface ModelCardProps {
  model: PricingModel
  onClick: () => void
  priceRate?: number
  usdExchangeRate?: number
  tokenUnit?: TokenUnit
  showRechargePrice?: boolean
  perf?: ModelPerfBadgeData
}

export const ModelCard = memo(function ModelCard(props: ModelCardProps) {
  const { t } = useTranslation()
  const { copyToClipboard } = useCopyToClipboard()
  const tokenUnit = props.tokenUnit ?? DEFAULT_TOKEN_UNIT
  const priceRate = props.priceRate ?? 1
  const usdExchangeRate = props.usdExchangeRate ?? 1
  const showRechargePrice = props.showRechargePrice ?? false
  const isTokenBased = isTokenBasedModel(props.model)
  const tokenUnitLabel = tokenUnit === 'K' ? '1K' : '1M'
  const groups = props.model.enable_groups || []
  const modelIconKey = props.model.icon || props.model.vendor_icon
  const modelIcon = modelIconKey ? getLobeIcon(modelIconKey, 32) : null
  const initial = props.model.model_name?.charAt(0).toUpperCase() || '?'
  const isDynamicPricing =
    props.model.billing_mode === 'tiered_expr' &&
    Boolean(props.model.billing_expr)
  const dynamicSummary = isDynamicPricing
    ? getDynamicPricingSummary(props.model, {
        tokenUnit,
        showRechargePrice,
        priceRate,
        usdExchangeRate,
        groupRatioMultiplier: getDynamicDisplayGroupRatio(props.model),
      })
    : null

  const primaryGroup = groups[0]

  const handleCopy = (e: MouseEvent<HTMLButtonElement>) => {
    e.stopPropagation()
    copyToClipboard(props.model.model_name || '')
  }

  let pricingContent: ReactNode = (
    <span className='text-[#7f8c8d] dark:text-[#94a3b8] font-medium'>
      <span className='text-[#2c3e50] dark:text-white font-mono font-black'>
        {formatRequestPrice(
          props.model,
          showRechargePrice,
          priceRate,
          usdExchangeRate
        )}
      </span>{' '}
      / {t('request')}
    </span>
  )

  if (dynamicSummary) {
    pricingContent = (
      <span className='text-muted-foreground text-xs'>
        {t('Dynamic Pricing')}
      </span>
    )
  } else if (isTokenBased) {
    pricingContent = (
      <>
        <span className='text-[#7f8c8d] dark:text-[#94a3b8] font-medium'>
          {t('Input')}{' '}
          <span className='text-[#2c3e50] dark:text-white font-mono font-black'>
            {formatPrice(
              props.model,
              'input',
              tokenUnit,
              showRechargePrice,
              priceRate,
              usdExchangeRate
            )}
          </span>
          /{tokenUnitLabel}
        </span>
        <span className='text-[#7f8c8d] dark:text-[#94a3b8] font-medium'>
          {t('Output')}{' '}
          <span className='text-[#2c3e50] dark:text-white font-mono font-black'>
            {formatPrice(
              props.model,
              'output',
              tokenUnit,
              showRechargePrice,
              priceRate,
              usdExchangeRate
            )}
          </span>
          /{tokenUnitLabel}
        </span>
      </>
    )
  }

  return (
    <div
      className={cn(
        'group relative flex flex-col rounded-[1.75rem] border-[3px] border-[#ffd1dc] bg-white p-4 transition-all duration-300 sm:p-6',
        'shadow-[3px_3px_0px_#ffd1dc] hover:-translate-y-1',
        'dark:bg-[#151d2a] dark:border-[#3b2d35] dark:shadow-[3px_3px_0px_#3b2d35]'
      )}
    >
      <div className='flex items-start justify-between gap-4'>
        <div className='flex min-w-0 items-start gap-3'>
          <div className='bg-[#f0f8ff] dark:bg-[#1a2436] flex size-11 shrink-0 items-center justify-center rounded-2xl'>
            {modelIcon || (
              <span className='text-[#64b5f6] text-lg font-black'>
                {initial}
              </span>
            )}
          </div>
          <div className='min-w-0'>
            <h3 className='text-foreground truncate font-mono text-[17px] leading-tight font-black'>
              {props.model.model_name}
            </h3>
            <div className='mt-1 flex flex-wrap items-baseline gap-x-3 gap-y-1 text-[13px]'>
              {pricingContent}
            </div>
          </div>
        </div>

        <div className='flex min-w-[5rem] shrink-0 flex-col items-stretch gap-2'>
          <button
            type='button'
            onClick={props.onClick}
            className='text-[#64b5f6] border-2 border-[#64b5f6] hover:bg-[#64b5f6] hover:text-white inline-flex items-center justify-center gap-1.5 rounded-full px-4 py-1.5 text-[13px] font-bold transition-all hover:-translate-y-0.5'
          >
            {t('Details')}
            <ChevronRight className='size-4' />
          </button>

          <button
            type='button'
            onClick={handleCopy}
            className='text-[#ff758f] border-2 border-dashed border-[#ffb3c6] dark:border-[#ff758f]/50 hover:bg-[#ffb3c6] hover:border-solid hover:border-[#ffb3c6] hover:text-white inline-flex items-center justify-center gap-1.5 rounded-full px-4 py-1.5 text-[13px] font-bold transition-all hover:-translate-y-0.5'
            title={t('Copy')}
          >
            <Copy className='size-3.5' />
            <span>{t('Copy')}</span>
          </button>
        </div>
      </div>

      <p className='text-[#7f8c8d] dark:text-[#94a3b8] mt-3 line-clamp-2 flex-1 text-[14px] leading-relaxed font-medium'>
        {props.model.description || t('No description available.')}
      </p>

      <div className='mt-4 border-t-2 border-dashed border-[#ffd1dc] dark:border-[#3b2d35] pt-3 flex items-center justify-between'>
        <div className='flex flex-wrap items-center gap-x-3 gap-y-1'>
          {primaryGroup && (
            <span className='text-[#ff758f] text-xs font-black'>
              {primaryGroup}
            </span>
          )}
          <span className='text-[#7f8c8d] text-xs font-bold'>
            {isTokenBased ? t('Token-based') : t('Per Request')}
          </span>
        </div>
        <ModelPerfBadge perf={props.perf} />
      </div>
    </div>
  )
})
