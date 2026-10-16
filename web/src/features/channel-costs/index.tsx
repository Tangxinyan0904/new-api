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
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { RefreshCw, TriangleAlert, WalletCards } from 'lucide-react'
import { useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { EmptyState } from '@/components/empty-state'
import { ErrorState } from '@/components/error-state'
import { SectionPageLayout } from '@/components/layout'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { hasPermission } from '@/lib/admin-permissions'
import { handleServerError } from '@/lib/handle-server-error'
import { useAuthStore } from '@/stores/auth-store'

import {
  channelCostQueryKeys,
  getChannelCostStats,
  getChannelUpstreamCosts,
  updateChannelCostRatio,
  updateChannelUpstreamBilling,
} from './api'
import { ChannelCostGroupSection } from './components/channel-cost-group-section'
import { ChannelCostSummary } from './components/channel-cost-summary'
import { UpstreamBalanceSection } from './components/upstream-balance-section'
import { useUpstreamReport, viewerTimezone } from './hooks/use-upstream-report'
import {
  allocateUpstreamCosts,
  getChannelCostRangeStarts,
} from './lib/channel-cost-math'
import type { ApiResponse } from './types'

export function ChannelCosts() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const user = useAuthStore((state) => state.auth.user)
  const canEdit = hasPermission(user, 'channel_cost', 'write')
  const starts = getChannelCostRangeStarts(new Date())
  const query = useQuery({
    queryKey: channelCostQueryKeys.stats(starts),
    queryFn: () => getChannelCostStats(starts),
  })
  const stats = query.data
  const groups = useMemo(() => stats?.groups ?? [], [stats])
  const hasUpstreamBilling = groups.some((group) =>
    group.channels.some((channel) => channel.upstream_billing !== null)
  )
  const { query: upstreamQuery, queryKey: upstreamKey } =
    useUpstreamReport(hasUpstreamBilling)
  const [refreshingUpstream, setRefreshingUpstream] = useState(false)
  const upstreamCosts = useMemo(
    () =>
      new Map(
        (upstreamQuery.data?.channels ?? []).map((cost) => [
          cost.channel_id,
          cost,
        ])
      ),
    [upstreamQuery.data]
  )
  const shares = useMemo(
    () => allocateUpstreamCosts(groups, upstreamQuery.data?.channels ?? []),
    [groups, upstreamQuery.data]
  )
  const accounts = upstreamQuery.data?.accounts ?? []
  const lowAccounts = accounts.filter((account) => account.low).length

  const refreshUpstream = async () => {
    setRefreshingUpstream(true)
    try {
      const report = await getChannelUpstreamCosts(starts, viewerTimezone, true)
      queryClient.setQueryData(upstreamKey, report)
    } catch (error) {
      handleServerError(error)
    } finally {
      setRefreshingUpstream(false)
    }
  }

  const save = async (request: Promise<ApiResponse>): Promise<boolean> => {
    try {
      const result = await request
      if (!result.success) {
        handleServerError(result)
        return false
      }
      toast.success(t('Saved successfully'))
      await queryClient.invalidateQueries({
        queryKey: channelCostQueryKeys.all,
      })
      return true
    } catch (error) {
      handleServerError(error)
      return false
    }
  }

  return (
    <SectionPageLayout>
      <SectionPageLayout.Title>{t('Cost Statistics')}</SectionPageLayout.Title>
      {hasUpstreamBilling && (
        <SectionPageLayout.Actions>
          <Button
            type='button'
            size='sm'
            variant='outline'
            disabled={refreshingUpstream || upstreamQuery.isFetching}
            onClick={() => void refreshUpstream()}
          >
            <RefreshCw data-icon='inline-start' aria-hidden='true' />
            {t('Refresh upstream bills')}
          </Button>
        </SectionPageLayout.Actions>
      )}
      <SectionPageLayout.Content>
        {query.isError ? (
          <ErrorState
            title={t('Failed to load cost statistics')}
            onRetry={() => void query.refetch()}
          />
        ) : (
          <div className='flex flex-col gap-4'>
            {stats && !stats.log_consume_enabled && (
              <Alert>
                <TriangleAlert aria-hidden='true' />
                <AlertDescription>
                  {t(
                    'Consumption logging is turned off, so new usage is not counted. Turn it on in system settings to keep these figures up to date.'
                  )}
                </AlertDescription>
              </Alert>
            )}
            {lowAccounts > 0 && (
              <Alert variant='destructive'>
                <WalletCards aria-hidden='true' />
                <AlertDescription>
                  {t(
                    '{{count}} upstream accounts have less than a day of balance left. Top them up soon.',
                    { count: lowAccounts }
                  )}
                </AlertDescription>
              </Alert>
            )}
            {accounts.length > 0 && (
              <UpstreamBalanceSection
                accounts={accounts}
                channels={groups.flatMap((group) => group.channels)}
              />
            )}
            <ChannelCostSummary
              groups={groups}
              shares={shares}
              loading={query.isLoading}
            />
            {groups.map((group) => (
              <ChannelCostGroupSection
                key={group.name}
                group={group}
                canEdit={canEdit}
                onSaveRatio={(channelId, ratio) =>
                  save(updateChannelCostRatio(channelId, ratio))
                }
                shares={shares}
                upstreamCosts={upstreamCosts}
                upstreamLoading={upstreamQuery.isFetching}
                onSaveUpstream={(channelId, input) =>
                  save(updateChannelUpstreamBilling(channelId, input))
                }
              />
            ))}
            {stats && stats.groups.length === 0 && (
              <EmptyState title={t('No channels yet')} bordered />
            )}
            <p className='text-muted-foreground text-xs leading-relaxed'>
              {t(
                "Cost = usage at the original price × cost ratio. The original price is each request's charge divided by the group ratio it was billed at, including user group special ratios. Profit = usage amount − cost. A channel's cost ratio applies to all its groups; channels without one count their whole usage as cost. Usage is net of refunds, grouped by your local time, and updated every 5 minutes. The last 7 days include today."
              )}
            </p>
            <p className='text-muted-foreground text-xs leading-relaxed'>
              {t(
                "Channels that read their upstream bill use what the upstream actually charged, converted with the dollar rate you entered and split across the channel's groups by their share of its usage at the original price. Upstream bills are read again every 5 minutes."
              )}
            </p>
          </div>
        )}
      </SectionPageLayout.Content>
    </SectionPageLayout>
  )
}
