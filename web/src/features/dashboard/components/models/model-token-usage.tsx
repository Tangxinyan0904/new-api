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
import { Boxes } from 'lucide-react'
import { useId, useMemo } from 'react'
import { useTranslation } from 'react-i18next'

import {
  StaticDataTable,
  type StaticDataTableColumn,
} from '@/components/data-table'
import { Dialog } from '@/components/dialog'
import { Button } from '@/components/ui/button'
import type { IconBadgeTone } from '@/components/ui/icon-badge'
import {
  StatStrip,
  type StatStripItem,
} from '@/features/dashboard/components/ui/stat-strip'
import {
  aggregateModelTokenUsage,
  formatStatNumber,
} from '@/features/dashboard/lib'
import type { ModelTokenUsage, QuotaDataItem } from '@/features/dashboard/types'
import { toIntlLocale } from '@/i18n/languages'
import { formatNumber } from '@/lib/format'

const TOP_MODEL_COUNT = 5

/** The same tones, in order, as the stat cards above. */
const MODEL_TONES: IconBadgeTone[] = [
  'info',
  'success',
  'chart-4',
  'chart-2',
  'warning',
]

interface ModelTokenUsageCardProps {
  data: QuotaDataItem[]
  loading: boolean
}

function AllModelTokenUsageDialog(props: {
  usage: ModelTokenUsage[]
  disabled: boolean
}) {
  const { t, i18n } = useTranslation()
  const locale = toIntlLocale(i18n.resolvedLanguage || i18n.language)
  const columns: StaticDataTableColumn<ModelTokenUsage>[] = [
    {
      id: 'model',
      header: t('Model'),
      cell: (row) => (
        <span className='font-medium break-all'>
          {row.model || t('Unknown model')}
        </span>
      ),
    },
    {
      id: 'tokens',
      header: t('Tokens'),
      className: 'text-end',
      cellClassName: 'text-end tabular-nums',
      cell: (row) => formatNumber(row.tokens, locale),
    },
    {
      id: 'cache_tokens',
      header: t('Cache tokens'),
      className: 'text-end',
      cellClassName: 'text-end tabular-nums',
      cell: (row) => formatNumber(row.cacheTokens, locale),
    },
  ]

  return (
    <Dialog
      title={t('Model token usage')}
      description={t('Every model used in the selected time range.')}
      trigger={
        <Button variant='ghost' size='sm' disabled={props.disabled}>
          {t('View all')}
        </Button>
      }
    >
      <StaticDataTable
        columns={columns}
        data={props.usage}
        getRowKey={(row) => row.model}
      />
    </Dialog>
  )
}

/** Tokens and cached tokens of the busiest models, like the stat cards. */
export function ModelTokenUsageCard(props: ModelTokenUsageCardProps) {
  const { t, i18n } = useTranslation()
  const headingId = useId()
  const locale = toIntlLocale(i18n.resolvedLanguage || i18n.language)
  const usage = useMemo(
    () => aggregateModelTokenUsage(props.data),
    [props.data]
  )

  const items: StatStripItem[] = props.loading
    ? MODEL_TONES.map((tone, index) => ({
        key: `loading-${index}`,
        title: '',
        value: '',
        description: '',
        icon: Boxes,
        iconTone: tone,
        loading: true,
      }))
    : usage.slice(0, TOP_MODEL_COUNT).map((model, index) => {
        const tokens = formatStatNumber(model.tokens, locale)
        return {
          key: model.model,
          title: model.model || t('Unknown model'),
          value: tokens.displayValue,
          fullValue: tokens.fullValue,
          description: t('Cached {{count}}', {
            count: formatStatNumber(model.cacheTokens, locale).displayValue,
          }),
          icon: Boxes,
          iconTone: MODEL_TONES[index],
        }
      })

  return (
    <section
      aria-labelledby={headingId}
      className='overflow-hidden rounded-lg border'
    >
      <div className='flex items-center justify-between gap-2 border-b px-2.5 py-1 sm:px-5 sm:py-1.5'>
        <h2 id={headingId} className='text-sm font-semibold'>
          {t('Model token usage')}
        </h2>
        <AllModelTokenUsageDialog
          usage={usage}
          disabled={props.loading || usage.length === 0}
        />
      </div>
      {!props.loading && usage.length === 0 ? (
        <p className='text-muted-foreground px-5 py-6 text-center text-sm'>
          {t('No data')}
        </p>
      ) : (
        <StatStrip
          items={items}
          columnsClassName='grid-cols-2 sm:grid-cols-3 lg:grid-cols-5'
          alwaysShowDescription
          preserveTitleCase
        />
      )}
    </section>
  )
}
