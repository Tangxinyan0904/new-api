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
import { Activity } from 'lucide-react'
import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { getRelayActivity, getUserQuotaDates } from '@/features/dashboard/api'
import {
  StatStrip,
  type StatStripItem,
} from '@/features/dashboard/components/ui/stat-strip'
import { RELAY_ACTIVITY_REFRESH_MS } from '@/features/dashboard/constants'
import { useModelStatCardsConfig } from '@/features/dashboard/hooks/use-dashboard-config'
import {
  buildQueryParams,
  calculateDashboardStats,
  formatStatNumber,
  getDefaultDays,
} from '@/features/dashboard/lib'
import type {
  QuotaDataItem,
  DashboardFilters,
} from '@/features/dashboard/types'
import { toIntlLocale } from '@/i18n/languages'
import { formatNumber, formatQuota } from '@/lib/format'
import { computeTimeRange } from '@/lib/time'
import { useAuthStore } from '@/stores/auth-store'

interface LogStatCardsProps {
  filters?: DashboardFilters
  onDataUpdate?: (data: QuotaDataItem[], loading: boolean) => void
}

export function LogStatCards(props: LogStatCardsProps) {
  const { t, i18n } = useTranslation()
  const statCardsConfig = useModelStatCardsConfig()
  const user = useAuthStore((state) => state.auth.user)
  const isAdmin = !!(user?.role && user.role >= 10)
  const [stats, setStats] = useState<{
    totalQuota: number
    totalCount: number
    totalTokens: number
  } | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState(false)

  const [timeRangeMinutes, setTimeRangeMinutes] = useState(0)

  const { filters, onDataUpdate } = props

  // Live figures ignore the time filter: admins see the whole site, users
  // their own account.
  const activity = useQuery({
    queryKey: ['dashboard', 'relay-activity', isAdmin],
    queryFn: () => getRelayActivity(isAdmin),
    refetchInterval: RELAY_ACTIVITY_REFRESH_MS,
    meta: { errorToast: false },
  })

  useEffect(() => {
    const abortController = new AbortController()
    // eslint-disable-next-line react-hooks/set-state-in-effect
    setLoading(true)

    setError(false)
    onDataUpdate?.([], true)

    const timeRange = computeTimeRange(
      getDefaultDays(filters?.time_granularity),
      filters?.start_timestamp,
      filters?.end_timestamp
    )
    const timeDiff = (timeRange.end_timestamp - timeRange.start_timestamp) / 60
    setTimeRangeMinutes(timeDiff)

    void getUserQuotaDates(buildQueryParams(timeRange, filters), isAdmin)
      .then((res) => {
        if (abortController.signal.aborted) return
        const data = res?.data || []
        setStats(calculateDashboardStats(data))
        onDataUpdate?.(data, false)
      })
      .catch(() => {
        if (abortController.signal.aborted) return
        setStats(null)
        setError(true)
        onDataUpdate?.([], false)
      })
      .finally(() => {
        if (!abortController.signal.aborted) {
          setLoading(false)
        }
      })

    return () => {
      abortController.abort()
    }
  }, [filters, isAdmin, onDataUpdate])

  const adaptedStats = {
    rpm: stats?.totalCount ?? 0,
    quota: stats?.totalQuota ?? 0,
    tpm: stats?.totalTokens ?? 0,
  }

  const locale = toIntlLocale(i18n.resolvedLanguage || i18n.language)
  const items: StatStripItem[] = statCardsConfig.map((config) => {
    const rawValue = config.getValue(adaptedStats, timeRangeMinutes)
    const formatted =
      config.key === 'quota'
        ? {
            displayValue: formatQuota(rawValue),
            fullValue: formatQuota(rawValue),
          }
        : formatStatNumber(rawValue, locale)

    return {
      key: config.key,
      title: config.title,
      value: formatted.displayValue,
      fullValue: formatted.fullValue,
      description: config.description,
      icon: config.icon,
      iconTone: config.iconTone,
      loading,
      error,
    }
  })
  const concurrencyRpm = activity.data
    ? `${formatNumber(activity.data.concurrency, locale)} / ${formatNumber(activity.data.rpm, locale)}`
    : ''
  items.push({
    key: 'concurrency-rpm',
    title: t('Concurrency / RPM'),
    value: concurrencyRpm,
    fullValue: concurrencyRpm,
    description: isAdmin
      ? t('Whole site, live, refreshed every minute')
      : t('Your account, live, refreshed every minute'),
    icon: Activity,
    iconTone: 'chart-1',
    loading: activity.isPending,
    error: activity.isError,
  })

  return (
    <div className='overflow-hidden rounded-lg border'>
      <StatStrip
        items={items}
        columnsClassName='grid-cols-2 sm:grid-cols-3 2xl:grid-cols-6'
      />
    </div>
  )
}
