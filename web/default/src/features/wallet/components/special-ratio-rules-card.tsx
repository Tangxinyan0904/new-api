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
import { useQuery } from '@tanstack/react-query'
import { Percent, RefreshCw, TriangleAlert } from 'lucide-react'
import { Fragment, useEffect } from 'react'
import { useTranslation } from 'react-i18next'

import { Alert, AlertAction, AlertTitle } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { Separator } from '@/components/ui/separator'
import { Skeleton } from '@/components/ui/skeleton'
import { TitledCard } from '@/components/ui/titled-card'
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from '@/components/ui/tooltip'

import { getWalletSpecialRatioRules } from '../api'
import {
  getSpecialRatioCardState,
  getSpecialRatioSummary,
} from '../lib/special-ratios'

type SpecialRatioRulesCardProps = {
  onAvailabilityChange?: (available: boolean) => void
}

const loadingRows = ['first', 'second', 'third']

export function SpecialRatioRulesCard(props: SpecialRatioRulesCardProps) {
  const { t } = useTranslation()
  const onAvailabilityChange = props.onAvailabilityChange
  const query = useQuery({
    queryKey: ['wallet-special-ratios'],
    queryFn: getWalletSpecialRatioRules,
  })
  const state = getSpecialRatioCardState({
    isPending: query.isPending,
    isError: query.isError,
    count: query.data?.length ?? 0,
  })

  useEffect(() => {
    onAvailabilityChange?.(state.available)
  }, [onAvailabilityChange, state.available])

  if (state.display === 'hidden') return null

  return (
    <TitledCard
      title={t('Special ratios')}
      icon={<Percent className='size-4' />}
      iconTone='info'
      disableHoverEffect
      className='h-fit'
      contentClassName='py-3 sm:py-4'
    >
      {state.display === 'loading' && (
        <div className='space-y-3' aria-busy='true'>
          {loadingRows.map((key) => (
            <div key={key} className='flex items-center justify-between gap-4'>
              <div className='min-w-0 flex-1 space-y-2'>
                <Skeleton className='h-4 w-3/4' />
                <Skeleton className='h-3 w-1/2' />
              </div>
              <Skeleton className='h-6 w-14' />
            </div>
          ))}
        </div>
      )}

      {state.display === 'error' && (
        <Alert variant='destructive'>
          <TriangleAlert />
          <AlertTitle>{t('Failed to load special ratios')}</AlertTitle>
          <AlertAction>
            <Button
              type='button'
              size='sm'
              variant='outline'
              disabled={query.isFetching}
              onClick={() => void query.refetch()}
            >
              <RefreshCw
                className={query.isFetching ? 'animate-spin' : undefined}
              />
              {t('Retry')}
            </Button>
          </AlertAction>
        </Alert>
      )}

      {state.display === 'rules' && (
        <TooltipProvider delay={200}>
          <div>
            {query.data?.map((rule, index) => {
              const label = `${rule.user_group} -> ${rule.billing_group}`
              const summary = getSpecialRatioSummary({
                billingGroup: rule.billing_group,
                baseRatio: rule.base_ratio,
                specialRatio: rule.special_ratio,
              })
              return (
                <Fragment key={`${rule.user_group}:${rule.billing_group}`}>
                  {index > 0 && <Separator />}
                  <div className='flex min-w-0 items-center justify-between gap-4 py-3 first:pt-0 last:pb-0'>
                    <div className='min-w-0'>
                      <Tooltip>
                        <TooltipTrigger
                          render={
                            <span className='block truncate text-sm font-semibold' />
                          }
                        >
                          {label}
                        </TooltipTrigger>
                        <TooltipContent>{label}</TooltipContent>
                      </Tooltip>
                      <p className='text-muted-foreground mt-1 text-xs leading-5'>
                        {t(
                          'Current ratio: {{currentRatio}}x, ratio after upgrading to {{group}}: {{upgradeRatio}}x',
                          {
                            currentRatio: summary.currentRatio,
                            group: summary.upgradeGroup,
                            upgradeRatio: summary.upgradeRatio,
                          }
                        )}
                      </p>
                    </div>
                  </div>
                </Fragment>
              )
            })}
          </div>
        </TooltipProvider>
      )}
    </TitledCard>
  )
}
