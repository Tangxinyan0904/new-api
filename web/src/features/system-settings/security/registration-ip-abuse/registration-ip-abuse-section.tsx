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
  Add01Icon,
  ArrowLeft01Icon,
  ArrowRight01Icon,
  InformationCircleIcon,
  Refresh01Icon,
  SearchIcon,
} from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '@/components/ui/alert-dialog'
import { Button } from '@/components/ui/button'
import {
  Empty,
  EmptyDescription,
  EmptyHeader,
  EmptyTitle,
} from '@/components/ui/empty'
import {
  InputGroup,
  InputGroupAddon,
  InputGroupInput,
} from '@/components/ui/input-group'
import { Separator } from '@/components/ui/separator'
import { Skeleton } from '@/components/ui/skeleton'
import { Spinner } from '@/components/ui/spinner'
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from '@/components/ui/tooltip'
import { useDebounce } from '@/hooks/use-debounce'

import { SettingsSection } from '../../components/settings-section'
import { RegistrationIPAllowlist } from './allowlist'
import {
  addRegistrationIPAllowlist,
  getBlockedRegistrationIPs,
  getBlockedRegistrationIPsQueryKey,
  getRegistrationIPAllowlist,
  getRegistrationIPAllowlistQueryKey,
  removeRegistrationIPAllowlist,
  unblockRegistrationIP,
} from './api'
import { BlockedIPList } from './blocked-ip-list'
import { isExactIPAddress } from './ip-validation'
import type {
  BlockedRegistrationIP,
  RegistrationIPAllowlistItem,
} from './types'

const PAGE_SIZE = 10
const REGISTRATION_IP_QUERY_PREFIX = ['registration-ip-abuse'] as const
const LOADING_ROWS = [
  'registration-ip-1',
  'registration-ip-2',
  'registration-ip-3',
]

type PageControlsProps = {
  page: number
  total: number
  onPageChange: (page: number) => void
}

function PageControls(props: PageControlsProps) {
  const { t } = useTranslation()
  const totalPages = Math.max(1, Math.ceil(props.total / PAGE_SIZE))
  if (totalPages <= 1) {
    return null
  }
  return (
    <div className='grid grid-cols-[auto_minmax(0,1fr)_auto] items-center gap-2'>
      <Tooltip>
        <TooltipTrigger
          render={
            <Button
              type='button'
              variant='outline'
              size='icon-sm'
              aria-label={t('Previous')}
              disabled={props.page <= 1}
              onClick={() => props.onPageChange(props.page - 1)}
            />
          }
        >
          <HugeiconsIcon icon={ArrowLeft01Icon} strokeWidth={2} />
        </TooltipTrigger>
        <TooltipContent>{t('Previous')}</TooltipContent>
      </Tooltip>
      <p className='text-muted-foreground min-w-0 text-center text-sm'>
        {t('Page {{current}} of {{total}}', {
          current: props.page,
          total: totalPages,
        })}
      </p>
      <Tooltip>
        <TooltipTrigger
          render={
            <Button
              type='button'
              variant='outline'
              size='icon-sm'
              aria-label={t('Next')}
              disabled={props.page >= totalPages}
              onClick={() => props.onPageChange(props.page + 1)}
            />
          }
        >
          <HugeiconsIcon icon={ArrowRight01Icon} strokeWidth={2} />
        </TooltipTrigger>
        <TooltipContent>{t('Next')}</TooltipContent>
      </Tooltip>
    </div>
  )
}

function LoadingList() {
  return (
    <div className='flex min-h-52 flex-col gap-3' aria-busy='true'>
      {LOADING_ROWS.map((row) => (
        <div
          key={row}
          className='grid min-h-16 items-center gap-3 rounded-lg border p-3 md:grid-cols-4'
        >
          <Skeleton className='h-5 w-40 max-w-full' />
          <Skeleton className='h-5 w-32 max-w-full' />
          <Skeleton className='h-5 w-20 max-w-full' />
          <Skeleton className='h-8 w-28 max-w-full md:justify-self-end' />
        </div>
      ))}
    </div>
  )
}

function ListError(props: { message: string; onRetry: () => void }) {
  const { t } = useTranslation()
  return (
    <div
      className='flex min-h-52 flex-col items-center justify-center gap-3 rounded-lg border p-4 text-center'
      role='alert'
    >
      <p className='text-muted-foreground text-sm'>{props.message}</p>
      <Button type='button' variant='outline' size='sm' onClick={props.onRetry}>
        {t('Retry')}
      </Button>
    </div>
  )
}

export function RegistrationIPAbuseSection() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [blockedPage, setBlockedPage] = useState(1)
  const [allowlistPage, setAllowlistPage] = useState(1)
  const [keyword, setKeyword] = useState('')
  const [allowlistInput, setAllowlistInput] = useState('')
  const [allowlistError, setAllowlistError] = useState('')
  const [selectedBlockedIP, setSelectedBlockedIP] =
    useState<BlockedRegistrationIP | null>(null)
  const [selectedAllowlistIP, setSelectedAllowlistIP] = useState<{
    ip: string
    isBlocked: boolean
  } | null>(null)
  const [selectedRemoval, setSelectedRemoval] =
    useState<RegistrationIPAllowlistItem | null>(null)
  const debouncedKeyword = useDebounce(keyword.trim(), 350)

  const blockedQuery = useQuery({
    queryKey: getBlockedRegistrationIPsQueryKey(
      blockedPage,
      PAGE_SIZE,
      debouncedKeyword
    ),
    queryFn: () =>
      getBlockedRegistrationIPs({
        page: blockedPage,
        pageSize: PAGE_SIZE,
        keyword: debouncedKeyword,
      }),
    placeholderData: (previousData) => previousData,
    staleTime: 30_000,
  })
  const allowlistQuery = useQuery({
    queryKey: getRegistrationIPAllowlistQueryKey(allowlistPage, PAGE_SIZE),
    queryFn: () =>
      getRegistrationIPAllowlist({
        page: allowlistPage,
        pageSize: PAGE_SIZE,
      }),
    placeholderData: (previousData) => previousData,
    staleTime: 30_000,
  })

  const refreshLists = async () => {
    await Promise.all([blockedQuery.refetch(), allowlistQuery.refetch()])
  }

  const invalidateLists = async () => {
    await queryClient.invalidateQueries({
      queryKey: REGISTRATION_IP_QUERY_PREFIX,
    })
  }

  const unblockMutation = useMutation({
    mutationFn: unblockRegistrationIP,
    onSuccess: async (result) => {
      const remaining = Math.max(0, (blockedQuery.data?.total ?? 0) - 1)
      setBlockedPage((current) =>
        Math.min(current, Math.max(1, Math.ceil(remaining / PAGE_SIZE)))
      )
      setSelectedBlockedIP(null)
      await invalidateLists()
      toast.success(
        t('Registration IP {{ip}} was unblocked', { ip: result.ip })
      )
    },
    onError: (error: Error) => {
      toast.error(error.message || t('Failed to unblock registration IP'))
    },
  })

  const addAllowlistMutation = useMutation({
    mutationFn: addRegistrationIPAllowlist,
    onSuccess: async (result) => {
      setSelectedAllowlistIP(null)
      setAllowlistInput('')
      setAllowlistError('')
      await invalidateLists()
      toast.success(
        t('Registration IP {{ip}} was added to the allowlist', {
          ip: result.ip,
        })
      )
    },
    onError: (error: Error) => {
      toast.error(error.message || t('Failed to add registration IP allowlist'))
    },
  })

  const removeAllowlistMutation = useMutation({
    mutationFn: removeRegistrationIPAllowlist,
    onSuccess: async (result) => {
      const remaining = Math.max(0, (allowlistQuery.data?.total ?? 0) - 1)
      setAllowlistPage((current) =>
        Math.min(current, Math.max(1, Math.ceil(remaining / PAGE_SIZE)))
      )
      setSelectedRemoval(null)
      await invalidateLists()
      toast.success(
        t('Registration IP {{ip}} was removed from the allowlist', {
          ip: result.ip,
        })
      )
    },
    onError: (error: Error) => {
      toast.error(
        error.message || t('Failed to remove registration IP allowlist')
      )
    },
  })

  const blockedItems = blockedQuery.data?.items ?? []
  const allowlistItems = allowlistQuery.data?.items ?? []
  const refreshing = blockedQuery.isFetching || allowlistQuery.isFetching
  const handleAllowlistRequest = () => {
    const ip = allowlistInput.trim()
    if (!isExactIPAddress(ip)) {
      setAllowlistError(t('Enter one exact IPv4 or IPv6 address'))
      return
    }
    setAllowlistError('')
    setSelectedAllowlistIP({
      ip,
      isBlocked: blockedItems.some((item) => item.ip === ip),
    })
  }

  return (
    <SettingsSection
      title={t('Registration Abuse Protection')}
      className='min-w-0 gap-6'
    >
      <Alert>
        <HugeiconsIcon icon={InformationCircleIcon} strokeWidth={2} />
        <AlertTitle>{t('Fixed rule: 3 accounts per IP')}</AlertTitle>
        <AlertDescription>
          {t(
            'The fourth self-service registration disables all accounts in the current IP cycle and blocks later registrations.'
          )}
        </AlertDescription>
      </Alert>

      <section className='flex min-w-0 flex-col gap-4'>
        <div className='flex items-start justify-between gap-3'>
          <div>
            <h4 className='text-sm font-semibold'>{t('Blocked IPs')}</h4>
            <p className='text-muted-foreground mt-1 text-sm'>
              {t('Search blocked IPs and review every associated account.')}
            </p>
          </div>
          <Tooltip>
            <TooltipTrigger
              render={
                <Button
                  type='button'
                  variant='outline'
                  size='icon-sm'
                  aria-label={t('Refresh')}
                  disabled={refreshing}
                  onClick={() => void refreshLists()}
                />
              }
            >
              {refreshing ? (
                <Spinner />
              ) : (
                <HugeiconsIcon icon={Refresh01Icon} strokeWidth={2} />
              )}
            </TooltipTrigger>
            <TooltipContent>{t('Refresh')}</TooltipContent>
          </Tooltip>
        </div>

        <InputGroup className='max-w-lg'>
          <InputGroupAddon>
            <HugeiconsIcon icon={SearchIcon} strokeWidth={2} />
          </InputGroupAddon>
          <InputGroupInput
            value={keyword}
            onChange={(event) => {
              setKeyword(event.target.value)
              setBlockedPage(1)
            }}
            placeholder={t('Search by IP, user ID, username, or display name')}
            aria-label={t('Search by IP, user ID, username, or display name')}
          />
        </InputGroup>

        {blockedQuery.isLoading ? <LoadingList /> : null}
        {blockedQuery.isError ? (
          <ListError
            message={t('Failed to load blocked registration IPs')}
            onRetry={() => void blockedQuery.refetch()}
          />
        ) : null}
        {!blockedQuery.isLoading &&
        !blockedQuery.isError &&
        blockedItems.length === 0 ? (
          <Empty className='min-h-52 border'>
            <EmptyHeader>
              <EmptyTitle>{t('No blocked registration IPs')}</EmptyTitle>
              <EmptyDescription>
                {t('Blocked IPs will appear here after the limit is exceeded.')}
              </EmptyDescription>
            </EmptyHeader>
          </Empty>
        ) : null}
        {!blockedQuery.isLoading &&
        !blockedQuery.isError &&
        blockedItems.length > 0 ? (
          <BlockedIPList
            items={blockedItems}
            unblockingIP={
              unblockMutation.isPending ? unblockMutation.variables : undefined
            }
            onUnblock={setSelectedBlockedIP}
          />
        ) : null}
        {!blockedQuery.isLoading && !blockedQuery.isError ? (
          <PageControls
            page={blockedPage}
            total={blockedQuery.data?.total ?? 0}
            onPageChange={setBlockedPage}
          />
        ) : null}
      </section>

      <Separator />

      <section className='flex min-w-0 flex-col gap-4'>
        <div>
          <h4 className='text-sm font-semibold'>
            {t('Registration IP allowlist')}
          </h4>
          <p className='text-muted-foreground mt-1 text-sm'>
            {t('Allowlisted exact IPs bypass registration counting.')}
          </p>
        </div>

        <div className='flex flex-col gap-2 sm:flex-row sm:items-start'>
          <div className='min-w-0 flex-1 sm:max-w-lg'>
            <label htmlFor='registration-ip-allowlist' className='sr-only'>
              {t('Exact IPv4 or IPv6 address')}
            </label>
            <InputGroup>
              <InputGroupInput
                id='registration-ip-allowlist'
                value={allowlistInput}
                onChange={(event) => {
                  setAllowlistInput(event.target.value)
                  if (allowlistError) {
                    setAllowlistError('')
                  }
                }}
                placeholder={t('Exact IPv4 or IPv6 address')}
                aria-invalid={allowlistError ? true : undefined}
                aria-describedby={
                  allowlistError ? 'registration-ip-allowlist-error' : undefined
                }
              />
            </InputGroup>
            <p
              id='registration-ip-allowlist-error'
              className='text-destructive mt-1 min-h-4 text-xs'
            >
              {allowlistError}
            </p>
          </div>
          <Button
            type='button'
            size='sm'
            disabled={addAllowlistMutation.isPending}
            onClick={handleAllowlistRequest}
          >
            {addAllowlistMutation.isPending ? (
              <Spinner data-icon='inline-start' />
            ) : (
              <HugeiconsIcon
                icon={Add01Icon}
                strokeWidth={2}
                data-icon='inline-start'
              />
            )}
            {t('Add to allowlist')}
          </Button>
        </div>

        {allowlistQuery.isLoading ? <LoadingList /> : null}
        {allowlistQuery.isError ? (
          <ListError
            message={t('Failed to load registration IP allowlist')}
            onRetry={() => void allowlistQuery.refetch()}
          />
        ) : null}
        {!allowlistQuery.isLoading &&
        !allowlistQuery.isError &&
        allowlistItems.length === 0 ? (
          <Empty className='min-h-52 border'>
            <EmptyHeader>
              <EmptyTitle>{t('No allowlisted registration IPs')}</EmptyTitle>
              <EmptyDescription>
                {t('Exact IPs added to the allowlist will appear here.')}
              </EmptyDescription>
            </EmptyHeader>
          </Empty>
        ) : null}
        {!allowlistQuery.isLoading &&
        !allowlistQuery.isError &&
        allowlistItems.length > 0 ? (
          <RegistrationIPAllowlist
            items={allowlistItems}
            removingIP={
              removeAllowlistMutation.isPending
                ? removeAllowlistMutation.variables
                : undefined
            }
            onRemove={setSelectedRemoval}
          />
        ) : null}
        {!allowlistQuery.isLoading && !allowlistQuery.isError ? (
          <PageControls
            page={allowlistPage}
            total={allowlistQuery.data?.total ?? 0}
            onPageChange={setAllowlistPage}
          />
        ) : null}
      </section>

      <AlertDialog
        open={selectedBlockedIP !== null}
        onOpenChange={(open) => {
          if (!open && !unblockMutation.isPending) {
            setSelectedBlockedIP(null)
          }
        }}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t('Unblock this IP?')}</AlertDialogTitle>
            <AlertDialogDescription>
              {t(
                'Only eligible automatically disabled accounts will be restored. The registration count for {{ip}} will reset.',
                { ip: selectedBlockedIP?.ip ?? '' }
              )}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel disabled={unblockMutation.isPending}>
              {t('Cancel')}
            </AlertDialogCancel>
            <AlertDialogAction
              type='button'
              disabled={unblockMutation.isPending || selectedBlockedIP === null}
              onClick={() => {
                if (selectedBlockedIP) {
                  unblockMutation.mutate(selectedBlockedIP.ip)
                }
              }}
            >
              {unblockMutation.isPending ? (
                <Spinner data-icon='inline-start' />
              ) : null}
              {t('Unblock IP')}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>

      <AlertDialog
        open={selectedAllowlistIP !== null}
        onOpenChange={(open) => {
          if (!open && !addAllowlistMutation.isPending) {
            setSelectedAllowlistIP(null)
          }
        }}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>
              {t('Add this IP to the allowlist?')}
            </AlertDialogTitle>
            <AlertDialogDescription>
              {selectedAllowlistIP?.isBlocked
                ? t(
                    'This IP is blocked. Adding it will restore eligible accounts, reset its count, and bypass future registration counting.'
                  )
                : t(
                    'This IP will bypass registration counting. If it is blocked, eligible accounts will also be restored.'
                  )}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel disabled={addAllowlistMutation.isPending}>
              {t('Cancel')}
            </AlertDialogCancel>
            <AlertDialogAction
              type='button'
              disabled={
                addAllowlistMutation.isPending || selectedAllowlistIP === null
              }
              onClick={() => {
                if (selectedAllowlistIP) {
                  addAllowlistMutation.mutate(selectedAllowlistIP.ip)
                }
              }}
            >
              {addAllowlistMutation.isPending ? (
                <Spinner data-icon='inline-start' />
              ) : null}
              {t('Add to allowlist')}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>

      <AlertDialog
        open={selectedRemoval !== null}
        onOpenChange={(open) => {
          if (!open && !removeAllowlistMutation.isPending) {
            setSelectedRemoval(null)
          }
        }}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>
              {t('Remove this IP from the allowlist?')}
            </AlertDialogTitle>
            <AlertDialogDescription>
              {t(
                'Registration counting for {{ip}} will restart from zero in a new cycle.',
                { ip: selectedRemoval?.ip ?? '' }
              )}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel disabled={removeAllowlistMutation.isPending}>
              {t('Cancel')}
            </AlertDialogCancel>
            <AlertDialogAction
              type='button'
              variant='destructive'
              disabled={
                removeAllowlistMutation.isPending || selectedRemoval === null
              }
              onClick={() => {
                if (selectedRemoval) {
                  removeAllowlistMutation.mutate(selectedRemoval.ip)
                }
              }}
            >
              {removeAllowlistMutation.isPending ? (
                <Spinner data-icon='inline-start' />
              ) : null}
              {t('Remove')}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </SettingsSection>
  )
}
