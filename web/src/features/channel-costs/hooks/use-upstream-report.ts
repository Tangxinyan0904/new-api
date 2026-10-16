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

import { hasPermission } from '@/lib/admin-permissions'
import { ROLE } from '@/lib/roles'
import { useAuthStore } from '@/stores/auth-store'

import { channelCostQueryKeys, getChannelUpstreamCosts } from '../api'
import { getChannelCostRangeStarts } from '../lib/channel-cost-math'

/** Matches the server's five-minute cache of upstream answers. */
export const UPSTREAM_REPORT_REFETCH_MS = 5 * 60_000

export const viewerTimezone = Intl.DateTimeFormat().resolvedOptions().timeZone

/**
 * Upstream costs and balance forecasts for staff who can read cost
 * statistics. The cost page and the sidebar badge share this query.
 */
export function useUpstreamReport(enabled = true) {
  const user = useAuthStore((state) => state.auth.user)
  const canRead =
    !!user &&
    user.role >= ROLE.ADMIN &&
    hasPermission(user, 'channel_cost', 'read')
  const starts = getChannelCostRangeStarts(new Date())
  const queryKey = channelCostQueryKeys.upstream(starts, viewerTimezone)
  const query = useQuery({
    queryKey,
    queryFn: () => getChannelUpstreamCosts(starts, viewerTimezone),
    enabled: canRead && enabled,
    refetchInterval: UPSTREAM_REPORT_REFETCH_MS,
    retry: false,
    meta: { errorToast: false },
  })
  return { query, queryKey, starts }
}

/** How many upstream accounts have less than a day of balance left. */
export function useLowUpstreamBalanceCount(): number {
  const { query } = useUpstreamReport()
  return query.data?.accounts.filter((account) => account.low).length ?? 0
}
