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
import { ChevronDown, RotateCcw } from 'lucide-react'
import { memo, useMemo, type ReactNode } from 'react'
import { useTranslation } from 'react-i18next'

import { GroupPriceRatio } from '@/components/group-price-ratio'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Collapsible,
  CollapsibleContent,
  CollapsibleTrigger,
} from '@/components/ui/collapsible'
import { categorizePricingGroups } from '@/lib/group-pricing'
import { getLobeIcon } from '@/lib/lobe-icon'
import { cn } from '@/lib/utils'

import {
  ENDPOINT_TYPES,
  FILTER_ALL,
  QUOTA_TYPES,
  getEndpointTypeLabels,
  getQuotaTypeLabels,
} from '../constants'
import { getModelQuotaType, parseTags } from '../lib/filters'
import type { PricingModel, PricingVendor } from '../types'
import { PricingGroupCategory } from './pricing-group-category'

type FilterOption = {
  value: string
  label: string
  fullWidth?: boolean
  count?: number
  suffix?: ReactNode
  icon?: ReactNode
}

type FilterSectionProps = {
  title: string
  value: string
  options: FilterOption[]
  children?: ReactNode
  onChange: (value: string) => void
}

export interface PricingSidebarProps {
  quotaTypeFilter: string
  endpointTypeFilter: string
  vendorFilter: string
  groupFilter: string
  tagFilter: string
  onQuotaTypeChange: (value: string) => void
  onEndpointTypeChange: (value: string) => void
  onVendorChange: (value: string) => void
  onGroupChange: (value: string) => void
  onTagChange: (value: string) => void
  vendors: PricingVendor[]
  groups: string[]
  groupRatios?: Record<string, number>
  originalGroupRatios?: Record<string, number>
  tags: string[]
  models: PricingModel[]
  hasActiveFilters: boolean
  onClearFilters: () => void
  className?: string
}

function FilterChip(props: {
  option: FilterOption
  active: boolean
  onClick: () => void
}) {
  const { fullWidth } = props.option
  const chipVariant = props.active ? 'secondary' : 'outline'
  return (
    <Button
      type='button'
      variant={fullWidth ? 'ghost' : chipVariant}
      size='sm'
      onClick={props.onClick}
      aria-pressed={props.active}
      className={cn(
        'max-w-full items-center px-3 py-1.5 text-xs',
        fullWidth
          ? 'border-border bg-background text-foreground hover:bg-muted aria-pressed:border-primary aria-pressed:bg-primary/10 h-auto min-h-11 w-full justify-between gap-3 rounded-lg border text-start font-semibold whitespace-normal shadow-none transition-colors'
          : cn(
              'gap-1.5 rounded-full border-2 font-bold transition-all hover:-translate-y-0.5',
              props.active
                ? 'border-[#64b5f6] bg-[#64b5f6] text-[#17354b] shadow-[0_4px_10px_rgba(100,181,246,0.3)] hover:bg-[#80c4fa]'
                : 'text-muted-foreground border-dashed border-[#ffd1dc] bg-transparent hover:border-solid hover:border-[#64b5f6] hover:bg-[#f0f8ff] hover:text-[#245a83] dark:border-[#3b2d35] dark:hover:bg-[#1a2436] dark:hover:text-[#90caf9]'
            )
      )}
      title={props.option.label}
    >
      {props.option.icon && (
        <span className='shrink-0'>{props.option.icon}</span>
      )}
      <span className={fullWidth ? 'min-w-0 flex-1 wrap-anywhere' : 'truncate'}>
        {props.option.label}
      </span>
      {(props.option.suffix || props.option.count != null) && (
        <span
          className={
            fullWidth
              ? 'max-w-1/2 min-w-0 text-end font-semibold wrap-anywhere text-[#245a83] tabular-nums dark:text-[#90caf9]'
              : 'rounded-full bg-[#f0f8ff] px-1.5 py-0.5 text-[10px] font-black text-[#245a83] dark:bg-[#1a2436] dark:text-[#90caf9]'
          }
        >
          {props.option.suffix ?? props.option.count}
        </span>
      )}
    </Button>
  )
}

function FilterSection(props: FilterSectionProps) {
  return (
    <Collapsible
      defaultOpen
      className='border-b-2 border-dashed border-[#ffd1dc] py-3 first:pt-0 last:border-b-0 dark:border-[#3b2d35]'
    >
      <CollapsibleTrigger className='group flex w-full items-center justify-between py-2.5 text-left'>
        <span className='text-[13.5px] font-black tracking-wide text-[#2c3e50] dark:text-[#e2e8f0]'>
          {props.title}
        </span>
        <ChevronDown className='size-4 text-[#7f8c8d] transition-transform group-data-[panel-open]:rotate-180 dark:text-[#94a3b8]' />
      </CollapsibleTrigger>
      <CollapsibleContent>
        <div className='flex flex-wrap gap-2 pt-1 pb-2'>
          {props.options.map((option) => (
            <FilterChip
              key={option.value}
              option={option}
              active={props.value === option.value}
              onClick={() => props.onChange(option.value)}
            />
          ))}
        </div>
        {props.children}
      </CollapsibleContent>
    </Collapsible>
  )
}

export const PricingSidebar = memo(function PricingSidebar(
  props: PricingSidebarProps
) {
  const { t } = useTranslation()
  const counts = useMemo(() => {
    const vendors = new Map<string, number>()
    const tags = new Map<string, number>()
    const endpoints = new Map<string, number>()
    const quotas = { token: 0, request: 0, task: 0 }
    for (const model of props.models) {
      if (model.vendor_name) {
        vendors.set(
          model.vendor_name,
          (vendors.get(model.vendor_name) ?? 0) + 1
        )
      }
      for (const tag of new Set(
        parseTags(model.tags).map((tag) => tag.toLowerCase())
      )) {
        tags.set(tag, (tags.get(tag) ?? 0) + 1)
      }
      for (const endpoint of new Set(model.supported_endpoint_types ?? [])) {
        endpoints.set(endpoint, (endpoints.get(endpoint) ?? 0) + 1)
      }
      const quotaType = getModelQuotaType(model)
      if (quotaType) quotas[quotaType]++
    }
    return { vendors, tags, endpoints, quotas }
  }, [props.models])
  const quotaTypeLabels = getQuotaTypeLabels(t)
  const endpointTypeLabels = getEndpointTypeLabels(t)

  const vendorOptions: FilterOption[] = [
    {
      value: FILTER_ALL,
      label: t('All Vendors'),
      count: props.models.length,
    },
    ...props.vendors
      .map((vendor) => ({
        value: vendor.name,
        label: vendor.name,
        count: counts.vendors.get(vendor.name) ?? 0,
        icon: vendor.icon ? getLobeIcon(vendor.icon, 14) : undefined,
      }))
      .filter((vendor) => vendor.count > 0),
  ]

  const groupCategories = useMemo(
    () =>
      categorizePricingGroups(
        props.groups,
        (name) => name,
        (name) => props.groupRatios?.[name]
      ),
    [props.groups, props.groupRatios]
  )

  const quotaOptions: FilterOption[] = [
    {
      value: QUOTA_TYPES.ALL,
      label: quotaTypeLabels[QUOTA_TYPES.ALL],
      count: props.models.length,
    },
    {
      value: QUOTA_TYPES.TOKEN,
      label: quotaTypeLabels[QUOTA_TYPES.TOKEN],
      count: counts.quotas.token,
    },
    {
      value: QUOTA_TYPES.REQUEST,
      label: quotaTypeLabels[QUOTA_TYPES.REQUEST],
      count: counts.quotas.request,
    },
    {
      value: QUOTA_TYPES.TASK,
      label: quotaTypeLabels[QUOTA_TYPES.TASK],
      count: counts.quotas.task,
    },
  ]

  const tagOptions: FilterOption[] = [
    {
      value: FILTER_ALL,
      label: t('All Tags'),
      count: props.models.length,
    },
    ...props.tags.map((tag) => ({
      value: tag,
      label: tag,
      count: counts.tags.get(tag.toLowerCase()) ?? 0,
    })),
  ]

  const endpointOptions: FilterOption[] = [
    {
      value: ENDPOINT_TYPES.ALL,
      label: endpointTypeLabels[ENDPOINT_TYPES.ALL],
      count: props.models.length,
    },
    ...Object.entries(endpointTypeLabels)
      .filter(([value]) => value !== ENDPOINT_TYPES.ALL)
      .map(([value, label]) => ({
        value,
        label,
        count: counts.endpoints.get(value) ?? 0,
      })),
  ]

  return (
    <aside
      className={cn(
        'rounded-[1.75rem] border-[3px] border-[#ffd1dc] bg-white p-4 shadow-[3px_3px_0px_#ffd1dc] transition-all sm:p-5',
        'dark:border-[#3b2d35] dark:bg-[#151d2a] dark:shadow-[3px_3px_0px_#3b2d35]',
        props.className
      )}
    >
      <div className='mb-4 flex items-start justify-between gap-2'>
        <div>
          <h2 className='text-[16px] font-black tracking-tight text-[#2c3e50] dark:text-[#e2e8f0]'>
            {t('Filter')}
          </h2>
          <p className='mt-1 text-[12px] leading-relaxed font-bold text-[#7f8c8d] dark:text-[#94a3b8]'>
            {t('Refine models by provider, group, type, and tags.')}
          </p>
        </div>
        <Button
          type='button'
          variant='ghost'
          size='sm'
          onClick={props.onClearFilters}
          disabled={!props.hasActiveFilters}
          className='h-8 gap-1.5 rounded-full px-3 text-[12px] font-bold text-[#64b5f6] hover:bg-[#f0f8ff] hover:text-[#2196f3] dark:hover:bg-[#1a2436] dark:hover:text-[#42a5f5]'
        >
          <RotateCcw className='size-3.5' />
          {t('Reset')}
        </Button>
      </div>

      {props.hasActiveFilters && (
        <Badge
          variant='secondary'
          className='mb-4 rounded-md border-none bg-[#ffb3c6]/20 font-bold text-[#ff758f]'
        >
          {t('Filters active')}
        </Badge>
      )}

      <div className='space-y-1'>
        <FilterSection
          title={t('Groups')}
          value={props.groupFilter}
          options={[
            { value: FILTER_ALL, label: t('All Groups'), fullWidth: true },
          ]}
          onChange={props.onGroupChange}
        >
          {groupCategories.map((category) => (
            <PricingGroupCategory key={category.label} label={category.label}>
              <div className='grid grid-cols-1 gap-1.5'>
                {category.items.map((group) => {
                  const ratio = props.groupRatios?.[group]
                  return (
                    <FilterChip
                      key={group}
                      active={props.groupFilter === group}
                      onClick={() => props.onGroupChange(group)}
                      option={{
                        value: group,
                        label: group,
                        fullWidth: true,
                        suffix:
                          typeof ratio === 'number' ? (
                            <GroupPriceRatio
                              className='items-end'
                              ratio={ratio}
                              originalRatio={props.originalGroupRatios?.[group]}
                            />
                          ) : undefined,
                      }}
                    />
                  )
                })}
              </div>
            </PricingGroupCategory>
          ))}
        </FilterSection>
        <FilterSection
          title={t('All Vendors')}
          value={props.vendorFilter}
          options={vendorOptions}
          onChange={props.onVendorChange}
        />
        <FilterSection
          title={t('Model Tags')}
          value={props.tagFilter}
          options={tagOptions}
          onChange={props.onTagChange}
        />
        <FilterSection
          title={t('Pricing Type')}
          value={props.quotaTypeFilter}
          options={quotaOptions}
          onChange={props.onQuotaTypeChange}
        />
        <FilterSection
          title={t('Endpoint Type')}
          value={props.endpointTypeFilter}
          options={endpointOptions}
          onChange={props.onEndpointTypeChange}
        />
      </div>
    </aside>
  )
})
