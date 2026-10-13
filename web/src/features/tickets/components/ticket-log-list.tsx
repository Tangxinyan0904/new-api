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
import { useTranslation } from 'react-i18next'

import { CopyButton } from '@/components/copy-button'
import { StatusBadge } from '@/components/status-badge'
import { toIntlLocale } from '@/i18n/languages'
import { formatNumber, formatQuota, formatTimestamp } from '@/lib/format'

import { TICKET_LOG_TYPE_ERROR } from '../constants'
import type { TicketAudience, TicketLog } from '../types'

/** What happened in one call: model, outcome, time, tokens and cost. */
export function TicketLogFacts(props: {
  log: TicketLog
  showChannel?: boolean
}) {
  const { t, i18n } = useTranslation()
  const locale = toIntlLocale(i18n.resolvedLanguage || i18n.language)
  const log = props.log
  const failed = log.type === TICKET_LOG_TYPE_ERROR

  return (
    <div className='flex min-w-0 flex-1 flex-col gap-1'>
      <div className='flex flex-wrap items-center gap-x-2 gap-y-1'>
        <span className='text-sm font-medium break-all'>
          {log.model_name || t('Unknown model')}
        </span>
        <StatusBadge
          label={failed ? t('Failed') : t('Success')}
          variant={failed ? 'danger' : 'success'}
          copyable={false}
        />
        <span className='text-muted-foreground ms-auto text-xs tabular-nums'>
          {formatTimestamp(log.created_at)}
        </span>
      </div>
      <div className='text-muted-foreground flex flex-wrap gap-x-3 gap-y-0.5 text-xs tabular-nums'>
        {log.token_name && (
          <span>
            {t('Token')}: {log.token_name}
          </span>
        )}
        {!failed && (
          <span>
            {t('Tokens')}: {formatNumber(log.prompt_tokens, locale)} /{' '}
            {formatNumber(log.completion_tokens, locale)}
          </span>
        )}
        {!failed && (
          <span>
            {t('Cost')}: {formatQuota(log.quota)}
          </span>
        )}
        {log.use_time > 0 && (
          <span>{t('{{seconds}} s', { seconds: log.use_time })}</span>
        )}
        {log.is_stream && <span>{t('Stream')}</span>}
        {props.showChannel && !!log.channel_id && (
          <span>
            {t('Channel')} #{log.channel_id}
          </span>
        )}
      </div>
      {failed && log.content && (
        <p
          className='text-destructive line-clamp-2 text-xs break-all'
          title={log.content}
        >
          {log.content}
        </p>
      )}
    </div>
  )
}

/** The call logs attached to an API ticket, for its owner and for staff. */
export function TicketLogList(props: {
  logs: TicketLog[]
  audience: TicketAudience
}) {
  const { t } = useTranslation()
  if (props.logs.length === 0) return null

  return (
    <section
      aria-label={t('Attached call logs')}
      className='bg-card flex flex-col gap-1 rounded-lg border p-3 sm:p-4'
    >
      <h3 className='text-sm font-semibold'>{t('Attached call logs')}</h3>
      <ul className='divide-y'>
        {props.logs.map((log) => (
          <li key={log.log_id} className='flex flex-col gap-1 py-2'>
            <TicketLogFacts
              log={log}
              showChannel={props.audience === 'staff'}
            />
            {log.request_id && (
              <div className='text-muted-foreground flex items-center gap-1 text-xs'>
                <span>{t('Request ID')}:</span>
                <span className='font-mono break-all'>{log.request_id}</span>
                <CopyButton
                  value={log.request_id}
                  aria-label={t('Copy request ID')}
                  className='size-6'
                />
              </div>
            )}
          </li>
        ))}
      </ul>
    </section>
  )
}
