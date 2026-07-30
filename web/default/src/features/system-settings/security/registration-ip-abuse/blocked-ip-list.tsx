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
import {
  ArrowDown01Icon,
  ArrowUp01Icon,
  UserUnlock01Icon,
} from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
import { Fragment, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Separator } from '@/components/ui/separator'
import { Spinner } from '@/components/ui/spinner'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from '@/components/ui/tooltip'
import { formatTimestampToDate } from '@/lib/format'

import type { BlockedRegistrationIP, RegistrationIPAccount } from './types'

type BlockedIPListProps = {
  items: BlockedRegistrationIP[]
  unblockingIP?: string
  onUnblock: (item: BlockedRegistrationIP) => void
}

function AccountStatus(props: { account: RegistrationIPAccount }) {
  const { t } = useTranslation()
  if (props.account.deleted) {
    return <Badge variant='destructive'>{t('Deleted')}</Badge>
  }
  if (props.account.restore_eligible) {
    return <Badge variant='secondary'>{t('Restores on unblock')}</Badge>
  }
  if (props.account.status === 1) {
    return <Badge variant='outline'>{t('Enabled')}</Badge>
  }
  return <Badge variant='outline'>{t('Manual status preserved')}</Badge>
}

function AccountDetails(props: { accounts: RegistrationIPAccount[] }) {
  const { t } = useTranslation()
  return (
    <div className='grid gap-2 py-3 sm:grid-cols-2 xl:grid-cols-3'>
      {props.accounts.map((account) => (
        <article
          key={account.user_id}
          className='bg-muted/40 min-w-0 rounded-md p-3'
        >
          <div className='flex min-w-0 items-start justify-between gap-2'>
            <div className='min-w-0'>
              <p className='truncate font-medium' title={account.username}>
                {account.username || t('Deleted user')}
              </p>
              <p
                className='text-muted-foreground truncate text-xs'
                title={account.display_name}
              >
                {account.display_name || t('No display name')}
              </p>
            </div>
            <AccountStatus account={account} />
          </div>
          <dl className='mt-3 grid gap-2 text-xs'>
            <div className='flex items-start justify-between gap-3'>
              <dt className='text-muted-foreground'>{t('User ID')}</dt>
              <dd className='font-mono'>{account.user_id}</dd>
            </div>
            <div className='flex items-start justify-between gap-3'>
              <dt className='text-muted-foreground'>{t('Registered')}</dt>
              <dd className='text-right tabular-nums'>
                {formatTimestampToDate(account.registration_at)}
              </dd>
            </div>
          </dl>
        </article>
      ))}
    </div>
  )
}

function DetailsButton(props: {
  expanded: boolean
  item: BlockedRegistrationIP
  onToggle: (ip: string) => void
}) {
  const { t } = useTranslation()
  const label = props.expanded
    ? t('Hide account details')
    : t('Show account details')
  return (
    <Tooltip>
      <TooltipTrigger
        render={
          <Button
            type='button'
            variant='ghost'
            size='icon-sm'
            aria-label={label}
            aria-expanded={props.expanded}
            onClick={() => props.onToggle(props.item.ip)}
          />
        }
      >
        <HugeiconsIcon
          icon={props.expanded ? ArrowUp01Icon : ArrowDown01Icon}
          strokeWidth={2}
        />
      </TooltipTrigger>
      <TooltipContent>{label}</TooltipContent>
    </Tooltip>
  )
}

function UnblockButton(props: {
  item: BlockedRegistrationIP
  pending: boolean
  onUnblock: (item: BlockedRegistrationIP) => void
}) {
  const { t } = useTranslation()
  return (
    <Button
      type='button'
      variant='outline'
      size='sm'
      disabled={props.pending}
      onClick={() => props.onUnblock(props.item)}
    >
      {props.pending ? (
        <Spinner data-icon='inline-start' />
      ) : (
        <HugeiconsIcon
          icon={UserUnlock01Icon}
          strokeWidth={2}
          data-icon='inline-start'
        />
      )}
      {t('Unblock IP')}
    </Button>
  )
}

export function BlockedIPList(props: BlockedIPListProps) {
  const { t } = useTranslation()
  const [expandedIPs, setExpandedIPs] = useState<Set<string>>(() => new Set())

  const toggleIP = (ip: string) => {
    setExpandedIPs((current) => {
      const next = new Set(current)
      if (next.has(ip)) {
        next.delete(ip)
      } else {
        next.add(ip)
      }
      return next
    })
  }

  return (
    <>
      <div className='flex flex-col gap-3 md:hidden'>
        {props.items.map((item) => {
          const expanded = expandedIPs.has(item.ip)
          return (
            <article
              key={item.ip}
              className='flex min-w-0 flex-col gap-3 rounded-lg border p-3'
            >
              <div className='flex min-w-0 items-start justify-between gap-2'>
                <code className='min-w-0 text-sm font-semibold break-all'>
                  {item.ip}
                </code>
                <DetailsButton
                  expanded={expanded}
                  item={item}
                  onToggle={toggleIP}
                />
              </div>
              <dl className='grid grid-cols-2 gap-3 text-xs'>
                <div>
                  <dt className='text-muted-foreground'>{t('Blocked at')}</dt>
                  <dd className='mt-1 tabular-nums'>
                    {formatTimestampToDate(item.blocked_at)}
                  </dd>
                </div>
                <div>
                  <dt className='text-muted-foreground'>{t('Accounts')}</dt>
                  <dd className='mt-1'>
                    {t('{{count}} accounts', {
                      count: item.associated_account_count,
                    })}
                  </dd>
                </div>
                <div>
                  <dt className='text-muted-foreground'>
                    {t('Cycle registrations')}
                  </dt>
                  <dd className='mt-1'>{item.registration_count}</dd>
                </div>
                <div>
                  <dt className='text-muted-foreground'>{t('Cycle')}</dt>
                  <dd className='mt-1'>{item.current_cycle}</dd>
                </div>
              </dl>
              {expanded ? (
                <>
                  <Separator />
                  <AccountDetails accounts={item.accounts} />
                </>
              ) : null}
              <UnblockButton
                item={item}
                pending={props.unblockingIP === item.ip}
                onUnblock={props.onUnblock}
              />
            </article>
          )
        })}
      </div>

      <div className='hidden md:block'>
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead className='w-10'>
                <span className='sr-only'>{t('Account details')}</span>
              </TableHead>
              <TableHead>{t('Registration IP')}</TableHead>
              <TableHead>{t('Blocked at')}</TableHead>
              <TableHead>{t('Cycle registrations')}</TableHead>
              <TableHead>{t('Accounts')}</TableHead>
              <TableHead className='text-right'>{t('Actions')}</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {props.items.map((item) => {
              const expanded = expandedIPs.has(item.ip)
              return (
                <Fragment key={item.ip}>
                  <TableRow>
                    <TableCell>
                      <DetailsButton
                        expanded={expanded}
                        item={item}
                        onToggle={toggleIP}
                      />
                    </TableCell>
                    <TableCell className='max-w-64 whitespace-normal'>
                      <code className='break-all'>{item.ip}</code>
                    </TableCell>
                    <TableCell>
                      {formatTimestampToDate(item.blocked_at)}
                    </TableCell>
                    <TableCell>{item.registration_count}</TableCell>
                    <TableCell>
                      {t('{{count}} accounts', {
                        count: item.associated_account_count,
                      })}
                    </TableCell>
                    <TableCell className='text-right'>
                      <UnblockButton
                        item={item}
                        pending={props.unblockingIP === item.ip}
                        onUnblock={props.onUnblock}
                      />
                    </TableCell>
                  </TableRow>
                  {expanded ? (
                    <TableRow>
                      <TableCell colSpan={6} className='whitespace-normal'>
                        <AccountDetails accounts={item.accounts} />
                      </TableCell>
                    </TableRow>
                  ) : null}
                </Fragment>
              )
            })}
          </TableBody>
        </Table>
      </div>
    </>
  )
}
