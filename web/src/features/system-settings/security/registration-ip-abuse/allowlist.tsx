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

import type { RegistrationIPAllowlistItem } from './types'

type RegistrationIPAllowlistProps = {
  items: RegistrationIPAllowlistItem[]
  removingIP?: string
  onRemove: (item: RegistrationIPAllowlistItem) => void
}

function RemoveButton(props: {
  item: RegistrationIPAllowlistItem
  pending: boolean
  onRemove: (item: RegistrationIPAllowlistItem) => void
}) {
  const { t } = useTranslation()
  return (
    <Button
      type='button'
      variant='destructive'
      size='sm'
      disabled={props.pending}
      onClick={() => props.onRemove(props.item)}
    >
      {props.pending ? (
        <Spinner data-icon='inline-start' />
      ) : (
        <HugeiconsIcon
          icon={Delete02Icon}
          strokeWidth={2}
          data-icon='inline-start'
        />
      )}
      {t('Remove')}
    </Button>
  )
}

export function RegistrationIPAllowlist(props: RegistrationIPAllowlistProps) {
  const { t } = useTranslation()
  return (
    <>
      <div className='flex flex-col gap-3 md:hidden'>
        {props.items.map((item) => (
          <article
            key={item.ip}
            className='flex min-w-0 flex-col gap-3 rounded-lg border p-3'
          >
            <code className='text-sm font-semibold break-all'>{item.ip}</code>
            <dl className='grid grid-cols-2 gap-3 text-xs'>
              <div>
                <dt className='text-muted-foreground'>{t('Added')}</dt>
                <dd className='mt-1 tabular-nums'>
                  {formatTimestampToDate(item.created_at)}
                </dd>
              </div>
              <div>
                <dt className='text-muted-foreground'>{t('Updated')}</dt>
                <dd className='mt-1 tabular-nums'>
                  {formatTimestampToDate(item.updated_at)}
                </dd>
              </div>
            </dl>
            <RemoveButton
              item={item}
              pending={props.removingIP === item.ip}
              onRemove={props.onRemove}
            />
          </article>
        ))}
      </div>

      <div className='hidden md:block'>
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>{t('Registration IP')}</TableHead>
              <TableHead>{t('Added')}</TableHead>
              <TableHead>{t('Updated')}</TableHead>
              <TableHead className='text-right'>{t('Actions')}</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {props.items.map((item) => (
              <TableRow key={item.ip}>
                <TableCell className='max-w-80 whitespace-normal'>
                  <code className='break-all'>{item.ip}</code>
                </TableCell>
                <TableCell>{formatTimestampToDate(item.created_at)}</TableCell>
                <TableCell>{formatTimestampToDate(item.updated_at)}</TableCell>
                <TableCell className='text-right'>
                  <RemoveButton
                    item={item}
                    pending={props.removingIP === item.ip}
                    onRemove={props.onRemove}
                  />
                </TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      </div>
    </>
  )
}
