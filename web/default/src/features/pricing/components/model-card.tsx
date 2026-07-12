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
import { ChevronRight, Copy } from 'lucide-react'
import { memo, type ReactNode } from 'react'
import { useTranslation } from 'react-i18next'

import { useCopyToClipboard } from '@/hooks/use-copy-to-clipboard'
import { getLobeIcon } from '@/lib/lobe-icon'
import { cn } from '@/lib/utils'

import { DEFAULT_TOKEN_UNIT } from '../constants'
import {
  getDynamicDisplayGroupRatio,
  getDynamicPricingSummary,
} from '../lib/dynamic-price'
import { parseTags } from '../lib/filters'
import { isTokenBasedModel } from '../lib/model-helpers'
import { formatPrice, formatRequestPrice } from '../lib/price'
import type { PricingModel, TokenUnit } from '../types'
import { ModelBillingModeBadge } from './model-billing-mode-badge'
import { ModelPerfBadge, type ModelPerfBadgeData } from './model-perf-badge'

export interface ModelCardProps {
  model: PricingModel
  onClick: () => void
  priceRate?: number
  usdExchangeRate?: number
  tokenUnit?: TokenUnit
  showRechargePrice?: boolean
  selectedGroup?: string
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
  const tags = parseTags(props.model.tags)
  const groups = props.model.enable_groups || []
  const endpoints = props.model.supported_endpoint_types || []
  const modelIconKey = props.model.icon || props.model.vendor_icon
  const modelIcon = modelIconKey ? getLobeIcon(modelIconKey, 32) : null
  const initial = props.model.model_name?.charAt(0).toUpperCase() || '?'
  const isDynamicPricing =
    props.model.billing_mode === 'tiered_expr' &&
    Boolean(props.model.billing_expr)
  const hasCachedPrice = isTokenBased && props.model.cache_ratio != null
  const dynamicSummary = isDynamicPricing
    ? getDynamicPricingSummary(props.model, {
        tokenUnit,
        showRechargePrice,
        priceRate,
        usdExchangeRate,
        groupRatioMultiplier: getDynamicDisplayGroupRatio(
          props.model,
          props.selectedGroup
        ),
      })
    : null

  const primaryGroup = groups[0]
  const bottomTags = [...endpoints.slice(0, 2), ...tags.slice(0, 2)]
  const hiddenCount =
    Math.max(groups.length - 1, 0) +
    Math.max(endpoints.length - 2, 0) +
    Math.max(tags.length - 2, 0)

  const handleCopy = (e: React.MouseEvent) => {
    e.stopPropagation()
    copyToClipboard(props.model.model_name || '')
  }

  let priceSummary: ReactNode
  if (dynamicSummary) {
    if (dynamicSummary.isSpecialExpression) {
      priceSummary = (
        <span className='min-w-0'>
          <span className='text-amber-700 dark:text-amber-300'>
            {t('Special billing expression')}
          </span>
          <code className='text-muted-foreground/70 mt-0.5 line-clamp-1 block font-mono text-[11px] break-all'>
            {dynamicSummary.rawExpression}
          </code>
        </span>
      )
    } else if (dynamicSummary.primaryEntries.length > 0) {
      priceSummary = (
        <>
          {dynamicSummary.primaryEntries.map((entry) => (
            <span
              key={entry.key}
              className='font-medium whitespace-nowrap text-[#7f8c8d] dark:text-[#94a3b8]'
            >
              {t(entry.shortLabel)}{' '}
              <span className='font-mono font-black text-[#2c3e50] dark:text-white'>
                {entry.formatted}
              </span>
            </span>
          ))}
        </>
      )
    } else {
      priceSummary = (
        <span className='text-sm font-medium text-[#7f8c8d] dark:text-[#94a3b8]'>
          {t('Dynamic Pricing')}
        </span>
      )
    }
  } else if (isTokenBased) {
    priceSummary = (
      <>
        <span className='font-medium whitespace-nowrap text-[#7f8c8d] dark:text-[#94a3b8]'>
          {t('Input')}{' '}
          <span className='font-mono font-black text-[#2c3e50] dark:text-white'>
            {formatPrice(
              props.model,
              'input',
              tokenUnit,
              showRechargePrice,
              priceRate,
              usdExchangeRate,
              props.selectedGroup
            )}
          </span>
        </span>
        <span className='font-medium whitespace-nowrap text-[#7f8c8d] dark:text-[#94a3b8]'>
          {t('Output')}{' '}
          <span className='font-mono font-black text-[#2c3e50] dark:text-white'>
            {formatPrice(
              props.model,
              'output',
              tokenUnit,
              showRechargePrice,
              priceRate,
              usdExchangeRate,
              props.selectedGroup
            )}
          </span>
        </span>
        {hasCachedPrice && (
          <span className='font-medium whitespace-nowrap text-[#7f8c8d] dark:text-[#94a3b8]'>
            {t('Cached')}{' '}
            <span className='font-mono font-black text-[#2c3e50] dark:text-white'>
              {formatPrice(
                props.model,
                'cache',
                tokenUnit,
                showRechargePrice,
                priceRate,
                usdExchangeRate,
                props.selectedGroup
              )}
            </span>
          </span>
        )}
      </>
    )
  } else {
    priceSummary = (
      <span className='font-medium whitespace-nowrap text-[#7f8c8d] dark:text-[#94a3b8]'>
        <span className='font-mono font-black text-[#2c3e50] dark:text-white'>
          {formatRequestPrice(
            props.model,
            showRechargePrice,
            priceRate,
            usdExchangeRate,
            props.selectedGroup
          )}
        </span>{' '}
        / {t('request')}
      </span>
    )
  }

  return (
    <div
      className={cn(
        'group relative flex flex-col rounded-[1.75rem] border-[3px] border-[#ffd1dc] bg-white p-4 transition-all duration-300 sm:p-6',
        'shadow-[3px_3px_0px_#ffd1dc] hover:-translate-y-1',
        'dark:border-[#3b2d35] dark:bg-[#151d2a] dark:shadow-[3px_3px_0px_#3b2d35]'
      )}
    >
      {/* Header: icon + name + price + actions */}
      <div className='flex items-start justify-between gap-4'>
        <div className='flex min-w-0 items-start gap-3'>
          <div className='flex size-11 shrink-0 items-center justify-center rounded-2xl bg-[#f0f8ff] dark:bg-[#1a2436]'>
            {modelIcon || (
              <span className='text-lg font-black text-[#64b5f6]'>
                {initial}
              </span>
            )}
          </div>
          <div className='min-w-0'>
            <h3 className='text-foreground truncate font-mono text-[17px] leading-tight font-black'>
              {props.model.model_name}
            </h3>
            <div className='mt-1 flex flex-wrap items-baseline gap-x-3 gap-y-1 text-[13px]'>
              {priceSummary}
            </div>
          </div>
        </div>

        <div className='flex min-w-[5rem] shrink-0 flex-col items-stretch gap-2'>
          <button
            type='button'
            onClick={props.onClick}
            className='inline-flex items-center justify-center gap-1.5 rounded-full border-2 border-[#64b5f6] px-4 py-1.5 text-[13px] font-bold text-[#64b5f6] transition-all hover:-translate-y-0.5 hover:bg-[#64b5f6] hover:text-white'
          >
            {t('Details')}
            <ChevronRight className='size-4' />
          </button>
          <button
            type='button'
            onClick={handleCopy}
            className='inline-flex items-center justify-center gap-1.5 rounded-full border-2 border-dashed border-[#ffb3c6] px-4 py-1.5 text-[13px] font-bold text-[#ff758f] transition-all hover:-translate-y-0.5 hover:border-solid hover:border-[#ffb3c6] hover:bg-[#ffb3c6] hover:text-white dark:border-[#ff758f]/50'
            title={t('Copy')}
          >
            <Copy className='size-3.5' />
            <span>{t('Copy')}</span>
          </button>
        </div>
      </div>

      {/* Description */}
      <p className='mt-3 line-clamp-2 flex-1 text-[14px] leading-relaxed font-medium text-[#7f8c8d] dark:text-[#94a3b8]'>
        {props.model.description || t('No description available.')}
      </p>

      {/* Footer: left metadata and right performance summary share row alignment */}
      <div className='mt-4 grid grid-cols-[minmax(0,1fr)_auto] items-start gap-x-2 gap-y-1 border-t-2 border-dashed border-[#ffd1dc] pt-3 dark:border-[#3b2d35]'>
        <div className='flex min-w-0 flex-wrap items-center gap-x-2 gap-y-1'>
          {primaryGroup && (
            <span className='text-sm font-black text-[#ff758f]'>
              {primaryGroup}
            </span>
          )}
          <ModelBillingModeBadge model={props.model} />
        </div>
        <ModelPerfBadge perf={props.perf} className='row-span-2 self-start' />

        <div className='flex min-w-0 flex-wrap items-center gap-x-2.5 gap-y-0.5 sm:gap-x-3 sm:gap-y-1'>
          {bottomTags.map((item) => (
            <span key={item} className='text-muted-foreground/70 text-xs'>
              {item}
            </span>
          ))}
          <span className='text-muted-foreground/50 text-xs'>
            {tokenUnitLabel}
          </span>
          {hiddenCount > 0 && (
            <span className='text-muted-foreground/40 text-xs'>
              +{hiddenCount}
            </span>
          )}
        </div>
      </div>
    </div>
  )
})
