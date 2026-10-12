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
import { TriangleAlert } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { EmptyState } from '@/components/empty-state'
import { ErrorState } from '@/components/error-state'
import { SectionPageLayout } from '@/components/layout'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { hasPermission } from '@/lib/admin-permissions'
import { handleServerError } from '@/lib/handle-server-error'
import { useAuthStore } from '@/stores/auth-store'

import {
  channelCostQueryKeys,
  getChannelCostStats,
  updateChannelCostRatio,
} from './api'
import { ChannelCostGroupSection } from './components/channel-cost-group-section'
import { ChannelCostSummary } from './components/channel-cost-summary'
import { getChannelCostRangeStarts } from './lib/channel-cost-math'
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
  const channels = stats?.groups.flatMap((group) => group.channels) ?? []

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
            <ChannelCostSummary channels={channels} loading={query.isLoading} />
            {stats?.groups.map((group) => (
              <ChannelCostGroupSection
                key={group.name}
                group={group}
                canEdit={canEdit}
                onSaveRatio={(channelId, ratio) =>
                  save(updateChannelCostRatio(channelId, ratio))
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
          </div>
        )}
      </SectionPageLayout.Content>
    </SectionPageLayout>
  )
}
