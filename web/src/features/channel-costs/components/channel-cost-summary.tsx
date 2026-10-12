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
import { CalendarDays, CalendarRange, Sun } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { StatCard } from '@/features/dashboard/components/ui/stat-card'
import { formatQuota } from '@/lib/format'

import {
  CHANNEL_COST_RANGES,
  sumChannelCostFigures,
} from '../lib/channel-cost-math'
import type { ChannelCostChannel, ChannelCostRange } from '../types'

const RANGE_CONFIG = {
  today: { labelKey: 'Today', icon: Sun, tone: 'accent-1' },
  yesterday: { labelKey: 'Yesterday', icon: CalendarDays, tone: 'accent-2' },
  week: { labelKey: 'Last 7 days', icon: CalendarRange, tone: 'accent-3' },
} as const satisfies Record<ChannelCostRange, object>

interface ChannelCostSummaryProps {
  channels: ChannelCostChannel[]
  defaultRatio: number
  loading: boolean
}

export function ChannelCostSummary(props: ChannelCostSummaryProps) {
  const { t } = useTranslation()

  return (
    <section
      aria-label={t('Cost summary')}
      className='bg-card grid grid-cols-1 gap-2 rounded-2xl border p-3 shadow-xs sm:grid-cols-3 sm:gap-3 sm:p-5'
    >
      {CHANNEL_COST_RANGES.map((range) => {
        const config = RANGE_CONFIG[range]
        const total = sumChannelCostFigures(
          props.channels,
          range,
          props.defaultRatio
        )
        return (
          <div
            key={range}
            className='bg-background/60 rounded-xl border p-3'
            data-testid={`cost-summary-${range}`}
          >
            <StatCard
              title={t(config.labelKey)}
              value={formatQuota(total.used)}
              description={t('Usage amount')}
              icon={config.icon}
              tone={config.tone}
              loading={props.loading}
              details={[
                { label: t('Upstream cost'), value: formatQuota(total.cost) },
                {
                  label: t('Profit'),
                  value: formatQuota(total.profit),
                  tone: total.profit < 0 ? 'destructive' : 'success',
                },
              ]}
            />
          </div>
        )
      })}
    </section>
  )
}
