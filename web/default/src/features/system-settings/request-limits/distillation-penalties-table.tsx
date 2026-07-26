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
  ArrowLeft01Icon,
  ArrowRight01Icon,
  Refresh01Icon,
  SearchIcon,
} from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

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
import { Skeleton } from '@/components/ui/skeleton'
import { Spinner } from '@/components/ui/spinner'
import { useDebounce } from '@/hooks/use-debounce'

import { clearDistillationPenalty, getDistillationPenalties } from './api'
import { getDistillationPenaltiesQueryKey } from './distillation-penalties'
import { DistillationPenaltyList } from './distillation-penalty-list'
import type { DistillationPenalty } from './types'

const PAGE_SIZE = 10
const DISTILLATION_PENALTIES_QUERY_PREFIX = ['distillation-penalties'] as const
const LOADING_ROWS = ['penalty-1', 'penalty-2', 'penalty-3']

export function DistillationPenaltiesTable() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [page, setPage] = useState(1)
  const [keyword, setKeyword] = useState('')
  const [selectedPenalty, setSelectedPenalty] =
    useState<DistillationPenalty | null>(null)
  const debouncedKeyword = useDebounce(keyword.trim(), 350)

  const penaltiesQuery = useQuery({
    queryKey: getDistillationPenaltiesQueryKey(
      page,
      PAGE_SIZE,
      debouncedKeyword
    ),
    queryFn: () =>
      getDistillationPenalties({
        page,
        pageSize: PAGE_SIZE,
        keyword: debouncedKeyword,
      }),
    placeholderData: (previousData) => previousData,
    staleTime: 30_000,
  })

  const clearMutation = useMutation({
    mutationFn: clearDistillationPenalty,
    onSuccess: async (_data, userId) => {
      const remainingTotal = Math.max(0, (penaltiesQuery.data?.total ?? 0) - 1)
      const lastPage = Math.max(1, Math.ceil(remainingTotal / PAGE_SIZE))
      setPage((currentPage) => Math.min(currentPage, lastPage))
      setSelectedPenalty(null)
      await queryClient.invalidateQueries({
        queryKey: DISTILLATION_PENALTIES_QUERY_PREFIX,
      })
      toast.success(
        t('Distillation penalty cleared for user {{id}}', { id: userId })
      )
    },
    onError: (error: Error) => {
      toast.error(
        error.message
          ? t(error.message)
          : t('Failed to clear distillation penalty')
      )
    },
  })

  const penalties = penaltiesQuery.data?.items ?? []
  const total = penaltiesQuery.data?.total ?? 0
  const totalPages = Math.max(1, Math.ceil(total / PAGE_SIZE))
  const clearingUserId = clearMutation.isPending
    ? clearMutation.variables
    : undefined

  const handleKeywordChange = (value: string) => {
    setKeyword(value)
    setPage(1)
  }

  return (
    <section className='flex min-w-0 flex-col gap-4'>
      <div>
        <h3 className='text-base font-semibold'>
          {t('Distillation penalties')}
        </h3>
        <p className='text-muted-foreground mt-1 text-sm'>
          {t(
            'Review temporary limits, observation periods, and permanent bans.'
          )}
        </p>
      </div>

      <div className='flex flex-col gap-2 sm:flex-row sm:items-center'>
        <InputGroup className='min-w-0 flex-1 sm:max-w-sm'>
          <InputGroupAddon>
            <HugeiconsIcon icon={SearchIcon} strokeWidth={2} />
          </InputGroupAddon>
          <InputGroupInput
            value={keyword}
            onChange={(event) => handleKeywordChange(event.target.value)}
            placeholder={t('Search by username or user ID')}
          />
        </InputGroup>
        <Button
          type='button'
          variant='outline'
          size='sm'
          disabled={penaltiesQuery.isFetching}
          onClick={() => void penaltiesQuery.refetch()}
        >
          {penaltiesQuery.isFetching ? (
            <Spinner data-icon='inline-start' />
          ) : (
            <HugeiconsIcon
              icon={Refresh01Icon}
              strokeWidth={2}
              data-icon='inline-start'
            />
          )}
          {t('Refresh')}
        </Button>
      </div>

      {penaltiesQuery.isLoading ? (
        <div className='flex flex-col gap-3' aria-busy='true'>
          {LOADING_ROWS.map((row) => (
            <div
              key={row}
              className='grid gap-3 rounded-xl border p-4 md:grid-cols-4'
            >
              <Skeleton className='h-5 w-32' />
              <Skeleton className='h-5 w-24' />
              <Skeleton className='h-5 w-40' />
              <Skeleton className='h-8 w-28 md:justify-self-end' />
            </div>
          ))}
        </div>
      ) : null}

      {penaltiesQuery.isError ? (
        <div
          className='flex min-h-40 flex-col items-center justify-center gap-3 rounded-xl border p-4 text-center'
          role='alert'
        >
          <p className='text-muted-foreground text-sm'>
            {t('Failed to load distillation penalties')}
          </p>
          <Button
            type='button'
            variant='outline'
            size='sm'
            onClick={() => void penaltiesQuery.refetch()}
          >
            {t('Retry')}
          </Button>
        </div>
      ) : null}

      {!penaltiesQuery.isLoading &&
      !penaltiesQuery.isError &&
      penalties.length === 0 ? (
        <Empty className='border'>
          <EmptyHeader>
            <EmptyTitle>{t('No active distillation penalties')}</EmptyTitle>
            <EmptyDescription>
              {t(
                'Temporary limits, observation periods, and permanent bans will appear here.'
              )}
            </EmptyDescription>
          </EmptyHeader>
        </Empty>
      ) : null}

      {!penaltiesQuery.isLoading &&
      !penaltiesQuery.isError &&
      penalties.length > 0 ? (
        <DistillationPenaltyList
          penalties={penalties}
          clearingUserId={clearingUserId}
          onClear={setSelectedPenalty}
        />
      ) : null}

      {!penaltiesQuery.isLoading &&
      !penaltiesQuery.isError &&
      totalPages > 1 ? (
        <div className='grid grid-cols-[auto_minmax(0,1fr)_auto] items-center gap-2'>
          <Button
            type='button'
            variant='outline'
            size='sm'
            disabled={page <= 1}
            onClick={() => setPage((currentPage) => currentPage - 1)}
          >
            <HugeiconsIcon
              icon={ArrowLeft01Icon}
              strokeWidth={2}
              data-icon='inline-start'
            />
            {t('Previous')}
          </Button>
          <p className='text-muted-foreground min-w-0 text-center text-sm'>
            {t('Page {{current}} of {{total}}', {
              current: page,
              total: totalPages,
            })}
          </p>
          <Button
            type='button'
            variant='outline'
            size='sm'
            disabled={page >= totalPages}
            onClick={() => setPage((currentPage) => currentPage + 1)}
          >
            {t('Next')}
            <HugeiconsIcon
              icon={ArrowRight01Icon}
              strokeWidth={2}
              data-icon='inline-end'
            />
          </Button>
        </div>
      ) : null}

      <AlertDialog
        open={selectedPenalty !== null}
        onOpenChange={(open) => {
          if (!open && !clearMutation.isPending) {
            setSelectedPenalty(null)
          }
        }}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>
              {t('Clear distillation penalty?')}
            </AlertDialogTitle>
            <AlertDialogDescription>
              {t(
                'This removes the current penalty and resets detection history for {{user}}.',
                {
                  user:
                    selectedPenalty?.username ||
                    t('User #{{id}}', { id: selectedPenalty?.user_id ?? 0 }),
                }
              )}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel disabled={clearMutation.isPending}>
              {t('Cancel')}
            </AlertDialogCancel>
            <AlertDialogAction
              type='button'
              variant='destructive'
              disabled={clearMutation.isPending || selectedPenalty === null}
              onClick={() => {
                if (selectedPenalty) {
                  clearMutation.mutate(selectedPenalty.user_id)
                }
              }}
            >
              {clearMutation.isPending ? (
                <Spinner data-icon='inline-start' />
              ) : null}
              {t('Clear penalty')}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </section>
  )
}
