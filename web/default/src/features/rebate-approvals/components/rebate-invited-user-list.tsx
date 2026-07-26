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

import { Badge } from '@/components/ui/badge'
import { Empty, EmptyHeader, EmptyTitle } from '@/components/ui/empty'
import { Label } from '@/components/ui/label'
import { cn } from '@/lib/utils'

import { getInvitedUserAuditPresentation } from '../lib/invited-user-display'
import type { RebateApprovalInvitedUser } from '../types'

function AuditField(props: {
  label: React.ReactNode
  value: React.ReactNode
  mono?: boolean
}): React.ReactElement {
  return (
    <div className='grid min-w-0 grid-cols-[6rem_minmax(0,1fr)] gap-3 text-xs'>
      <span className='text-muted-foreground min-w-0'>{props.label}</span>
      <span className={cn('min-w-0 break-all', props.mono && 'font-mono')}>
        {props.value}
      </span>
    </div>
  )
}

export function RebateInvitedUserList(props: {
  users: RebateApprovalInvitedUser[]
}): React.ReactElement {
  const { t } = useTranslation()
  let content = (
    <Empty className='rounded-md border py-4'>
      <EmptyHeader>
        <EmptyTitle>{t('No invited users found.')}</EmptyTitle>
      </EmptyHeader>
    </Empty>
  )

  if (props.users.length > 0) {
    content = (
      <div className='flex flex-col gap-2'>
        {props.users.map((user) => {
          const presentation = getInvitedUserAuditPresentation(user)
          return (
            <div
              key={user.id}
              className='bg-background min-w-0 rounded-md border p-3 [contain-intrinsic-size:auto_112px] [content-visibility:auto]'
            >
              <div className='mb-2 flex min-w-0 items-center justify-between gap-2'>
                <div className='min-w-0 truncate text-sm font-medium'>
                  {presentation.displayName}
                </div>
                {presentation.isDeleted && (
                  <Badge variant='destructive'>{t('Deleted')}</Badge>
                )}
              </div>
              <div className='grid gap-1.5 sm:grid-cols-2'>
                <AuditField label={t('User ID')} value={user.id} mono />
                <AuditField
                  label={t('Created At')}
                  value={presentation.createdAt}
                  mono
                />
                <AuditField
                  label={t('Last Login')}
                  value={presentation.lastLoginAt}
                  mono
                />
              </div>
            </div>
          )
        })}
      </div>
    )
  }

  return (
    <div className='flex min-w-0 flex-col gap-2'>
      <Label className='text-xs font-semibold'>
        {t('Invited User Details')}
      </Label>
      {content}
    </div>
  )
}
