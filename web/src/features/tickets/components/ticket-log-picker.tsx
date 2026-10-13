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
import { keepPreviousData, useQuery } from '@tanstack/react-query'
import { ChevronLeft, ChevronRight } from 'lucide-react'
import { useId, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import { Input } from '@/components/ui/input'
import { useDebounce } from '@/hooks/use-debounce'

import { getTicketLogCandidates } from '../api'
import { TICKET_LIMITS, ticketQueryKeys } from '../constants'
import { TicketLogFacts } from './ticket-log-list'

const PAGE_SIZE = 5

interface TicketLogPickerProps {
  /** Selected log ids. */
  value: number[]
  onChange: (logIds: number[]) => void
  disabled?: boolean
}

/** Lets the user tick up to TICKET_LIMITS.MAX_LOGS of their recent call logs. */
export function TicketLogPicker(props: TicketLogPickerProps) {
  const { t } = useTranslation()
  const labelId = useId()
  const [page, setPage] = useState(1)
  const [modelFilter, setModelFilter] = useState('')
  const modelName = useDebounce(modelFilter.trim(), 300)
  const query = useQuery({
    queryKey: ticketQueryKeys.logCandidates({ p: page, model_name: modelName }),
    queryFn: () =>
      getTicketLogCandidates({
        p: page,
        page_size: PAGE_SIZE,
        model_name: modelName,
      }),
    placeholderData: keepPreviousData,
  })
  const logs = query.data?.items ?? []
  const pageCount = Math.max(1, Math.ceil((query.data?.total ?? 0) / PAGE_SIZE))
  const full = props.value.length >= TICKET_LIMITS.MAX_LOGS

  const toggle = (logId: number, checked: boolean) => {
    if (!checked) {
      props.onChange(props.value.filter((id) => id !== logId))
      return
    }
    if (!full && !props.value.includes(logId)) {
      props.onChange([...props.value, logId])
    }
  }

  let list: React.ReactNode
  if (query.isPending) {
    list = (
      <p className='text-muted-foreground px-3 py-6 text-center text-sm'>
        {t('Loading...')}
      </p>
    )
  } else if (query.isError) {
    list = (
      <p className='text-destructive px-3 py-6 text-center text-sm'>
        {t('Failed to load call logs')}
      </p>
    )
  } else if (logs.length === 0) {
    list = (
      <p className='text-muted-foreground px-3 py-6 text-center text-sm'>
        {t('No call logs yet')}
      </p>
    )
  } else {
    list = (
      <ul className='divide-y'>
        {logs.map((log) => {
          const checked = props.value.includes(log.log_id)
          return (
            <li key={log.log_id}>
              <label className='hover:bg-muted/50 flex cursor-pointer items-start gap-3 px-3 py-2'>
                <Checkbox
                  className='mt-0.5'
                  checked={checked}
                  disabled={props.disabled || (!checked && full)}
                  onCheckedChange={(next) => toggle(log.log_id, next)}
                />
                <TicketLogFacts log={log} />
              </label>
            </li>
          )
        })}
      </ul>
    )
  }

  return (
    <div role='group' aria-labelledby={labelId} className='flex flex-col gap-2'>
      <div className='flex flex-wrap items-baseline justify-between gap-x-3'>
        <span id={labelId} className='text-sm font-medium'>
          {t('Related call logs')}
        </span>
        <span className='text-muted-foreground text-xs tabular-nums'>
          {t('{{count}} of {{max}} selected', {
            count: props.value.length,
            max: TICKET_LIMITS.MAX_LOGS,
          })}
        </span>
      </div>
      <p className='text-muted-foreground text-xs'>
        {t(
          'Optional. Tick the requests that went wrong so support can look them up directly.'
        )}
      </p>
      <Input
        value={modelFilter}
        placeholder={t('Filter by model')}
        aria-label={t('Filter by model')}
        disabled={props.disabled}
        onChange={(event) => {
          setModelFilter(event.target.value)
          setPage(1)
        }}
      />
      <div className='rounded-lg border'>{list}</div>
      {pageCount > 1 && (
        <div className='flex items-center justify-end gap-1'>
          <span className='text-muted-foreground me-1 text-xs tabular-nums'>
            {page} / {pageCount}
          </span>
          <Button
            type='button'
            variant='outline'
            size='icon'
            className='size-7'
            aria-label={t('Previous')}
            disabled={page <= 1}
            onClick={() => setPage(page - 1)}
          >
            <ChevronLeft aria-hidden='true' />
          </Button>
          <Button
            type='button'
            variant='outline'
            size='icon'
            className='size-7'
            aria-label={t('Next')}
            disabled={page >= pageCount}
            onClick={() => setPage(page + 1)}
          >
            <ChevronRight aria-hidden='true' />
          </Button>
        </div>
      )}
    </div>
  )
}
