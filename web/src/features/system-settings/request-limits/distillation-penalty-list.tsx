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
import { Delete02Icon } from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
import { useTranslation } from 'react-i18next'

import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Spinner } from '@/components/ui/spinner'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { formatTimestampToDate } from '@/lib/format'

import { getDistillationPenaltyPhaseConfig } from './distillation-penalties'
import type { DistillationPenalty, DistillationPenaltyPhase } from './types'

type PenaltyRowProps = {
  penalty: DistillationPenalty
  clearing: boolean
  onClear: (penalty: DistillationPenalty) => void
}

type DistillationPenaltyListProps = {
  penalties: DistillationPenalty[]
  clearingUserId?: number
  onClear: (penalty: DistillationPenalty) => void
}

function PenaltyPhaseBadge(props: { phase: DistillationPenaltyPhase }) {
  const { t } = useTranslation()
  const config = getDistillationPenaltyPhaseConfig(props.phase)
  return <Badge variant={config.variant}>{t(config.labelKey)}</Badge>
}

function ClearPenaltyButton(props: PenaltyRowProps) {
  const { t } = useTranslation()
  return (
    <Button
      type='button'
      variant='destructive'
      size='sm'
      disabled={props.clearing}
      onClick={() => props.onClear(props.penalty)}
    >
      {props.clearing ? (
        <Spinner data-icon='inline-start' />
      ) : (
        <HugeiconsIcon
          icon={Delete02Icon}
          strokeWidth={2}
          data-icon='inline-start'
        />
      )}
      {t('Clear penalty')}
    </Button>
  )
}

function PenaltyMobileCard(props: PenaltyRowProps) {
  const { t } = useTranslation()
  return (
    <article className='flex flex-col gap-4 rounded-xl border p-4 md:hidden'>
      <div className='flex min-w-0 items-start justify-between gap-3'>
        <div className='min-w-0'>
          <p className='truncate font-medium'>{props.penalty.username}</p>
          <p className='text-muted-foreground text-xs'>
            {t('User ID {{id}}', { id: props.penalty.user_id })}
          </p>
        </div>
        <PenaltyPhaseBadge phase={props.penalty.phase} />
      </div>

      <dl className='grid grid-cols-1 gap-3 text-sm sm:grid-cols-2'>
        <div>
          <dt className='text-muted-foreground text-xs'>
            {t('First trigger')}
          </dt>
          <dd className='mt-1 break-words tabular-nums'>
            {formatTimestampToDate(props.penalty.first_triggered_at)}
          </dd>
        </div>
        <div>
          <dt className='text-muted-foreground text-xs'>
            {t('Temporary limit ends')}
          </dt>
          <dd className='mt-1 break-words tabular-nums'>
            {formatTimestampToDate(props.penalty.temporary_limited_until)}
          </dd>
        </div>
        <div>
          <dt className='text-muted-foreground text-xs'>
            {t('Observation ends')}
          </dt>
          <dd className='mt-1 break-words tabular-nums'>
            {formatTimestampToDate(props.penalty.observation_until)}
          </dd>
        </div>
        <div>
          <dt className='text-muted-foreground text-xs'>
            {t('Permanent ban time')}
          </dt>
          <dd className='mt-1 break-words tabular-nums'>
            {formatTimestampToDate(props.penalty.permanently_banned_at)}
          </dd>
        </div>
        <div>
          <dt className='text-muted-foreground text-xs'>{t('Updated')}</dt>
          <dd className='mt-1 break-words tabular-nums'>
            {formatTimestampToDate(props.penalty.updated_at)}
          </dd>
        </div>
      </dl>

      <ClearPenaltyButton {...props} />
    </article>
  )
}

function PenaltiesDesktopTable(props: DistillationPenaltyListProps) {
  const { t } = useTranslation()
  return (
    <div className='hidden md:block'>
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead>{t('User')}</TableHead>
            <TableHead>{t('Phase')}</TableHead>
            <TableHead>{t('First trigger')}</TableHead>
            <TableHead>{t('Temporary limit ends')}</TableHead>
            <TableHead>{t('Observation ends')}</TableHead>
            <TableHead>{t('Permanent ban time')}</TableHead>
            <TableHead>{t('Updated')}</TableHead>
            <TableHead className='text-right'>{t('Actions')}</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {props.penalties.map((penalty) => (
            <TableRow key={penalty.user_id}>
              <TableCell className='whitespace-normal'>
                <p className='font-medium'>{penalty.username}</p>
                <p className='text-muted-foreground text-xs'>
                  {t('User ID {{id}}', { id: penalty.user_id })}
                </p>
              </TableCell>
              <TableCell>
                <PenaltyPhaseBadge phase={penalty.phase} />
              </TableCell>
              <TableCell>
                {formatTimestampToDate(penalty.first_triggered_at)}
              </TableCell>
              <TableCell>
                {formatTimestampToDate(penalty.temporary_limited_until)}
              </TableCell>
              <TableCell>
                {formatTimestampToDate(penalty.observation_until)}
              </TableCell>
              <TableCell>
                {formatTimestampToDate(penalty.permanently_banned_at)}
              </TableCell>
              <TableCell>{formatTimestampToDate(penalty.updated_at)}</TableCell>
              <TableCell className='text-right'>
                <ClearPenaltyButton
                  penalty={penalty}
                  clearing={props.clearingUserId === penalty.user_id}
                  onClear={props.onClear}
                />
              </TableCell>
            </TableRow>
          ))}
        </TableBody>
      </Table>
    </div>
  )
}

export function DistillationPenaltyList(props: DistillationPenaltyListProps) {
  return (
    <>
      <div className='flex flex-col gap-3 md:hidden'>
        {props.penalties.map((penalty) => (
          <PenaltyMobileCard
            key={penalty.user_id}
            penalty={penalty}
            clearing={props.clearingUserId === penalty.user_id}
            onClear={props.onClear}
          />
        ))}
      </div>
      <PenaltiesDesktopTable {...props} />
    </>
  )
}
