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
  Delete02Icon,
  UserAdd01Icon,
  UserSearch01Icon,
} from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
import { useQuery } from '@tanstack/react-query'
import { useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'
import {
  Combobox,
  ComboboxCollection,
  ComboboxContent,
  ComboboxEmpty,
  ComboboxGroup,
  ComboboxInput,
  ComboboxItem,
  ComboboxList,
} from '@/components/ui/combobox'
import {
  Empty,
  EmptyDescription,
  EmptyHeader,
  EmptyMedia,
  EmptyTitle,
} from '@/components/ui/empty'
import {
  Field,
  FieldDescription,
  FieldGroup,
  FieldLegend,
  FieldSet,
} from '@/components/ui/field'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { useDebounce } from '@/hooks/use-debounce'

import { searchRateLimitUsers } from './api'
import {
  addUserRateLimitRule,
  hydrateUserRateLimitRules,
  parseUserRateLimitRules,
  serializeUserRateLimitRules,
} from './rate-limit-schema'
import type {
  HydratedUserRateLimitRule,
  RateLimitUserSummary,
  UserRateLimitRule,
} from './types'

type UserRateLimitEditorProps = {
  value: string
  onChange: (value: string) => void
  defaultMaxRequests: number
  defaultMaxSuccess: number
}

type UserRateLimitRowProps = {
  rule: HydratedUserRateLimitRule
  onUpdate: (rule: UserRateLimitRule) => void
  onRemove: (userId: number) => void
}

function userLabel(user: RateLimitUserSummary): string {
  const displayName = user.displayName ? ` · ${user.displayName}` : ''
  return `${user.username}${displayName} · ID ${user.id}`
}

function parseInputInteger(value: string, fallback: number): number {
  const parsed = Number.parseInt(value, 10)
  return Number.isNaN(parsed) ? fallback : parsed
}

function UserRateLimitRow(props: UserRateLimitRowProps) {
  const { t } = useTranslation()
  const userName =
    props.rule.username || t('User #{{id}}', { id: props.rule.userId })

  return (
    <Field className='rounded-lg border p-3'>
      <div className='grid min-w-0 gap-3 sm:grid-cols-[minmax(0,1fr)_minmax(8rem,0.45fr)_minmax(8rem,0.45fr)_auto] sm:items-end'>
        <div className='min-w-0 self-center'>
          <p className='truncate text-sm font-medium'>{userName}</p>
          <p className='text-muted-foreground truncate text-xs'>
            {props.rule.displayName
              ? `${props.rule.displayName} · ID ${props.rule.userId}`
              : `ID ${props.rule.userId}`}
          </p>
        </div>
        <div className='grid gap-1.5'>
          <Label htmlFor={`user-rate-total-${props.rule.userId}`}>
            {t('Max requests')}
          </Label>
          <Input
            id={`user-rate-total-${props.rule.userId}`}
            type='number'
            min={0}
            max={2147483647}
            value={props.rule.maxRequests}
            onChange={(event) =>
              props.onUpdate({
                userId: props.rule.userId,
                maxRequests: parseInputInteger(event.target.value, 0),
                maxSuccess: props.rule.maxSuccess,
              })
            }
          />
        </div>
        <div className='grid gap-1.5'>
          <Label htmlFor={`user-rate-success-${props.rule.userId}`}>
            {t('Max successful requests')}
          </Label>
          <Input
            id={`user-rate-success-${props.rule.userId}`}
            type='number'
            min={1}
            max={2147483647}
            value={props.rule.maxSuccess}
            onChange={(event) =>
              props.onUpdate({
                userId: props.rule.userId,
                maxRequests: props.rule.maxRequests,
                maxSuccess: parseInputInteger(event.target.value, 1),
              })
            }
          />
        </div>
        <Button
          type='button'
          variant='ghost'
          size='icon'
          aria-label={t('Remove user rate limit')}
          onClick={() => props.onRemove(props.rule.userId)}
        >
          <HugeiconsIcon icon={Delete02Icon} strokeWidth={2} />
        </Button>
      </div>
    </Field>
  )
}

export function UserRateLimitEditor(props: UserRateLimitEditorProps) {
  const { t } = useTranslation()
  const [search, setSearch] = useState('')
  const [knownUsers, setKnownUsers] = useState(
    () => new Map<number, RateLimitUserSummary>()
  )
  const debouncedSearch = useDebounce(search.trim(), 350)

  const rules = useMemo(() => {
    try {
      return parseUserRateLimitRules(props.value)
    } catch {
      return []
    }
  }, [props.value])
  const configuredUserIds = useMemo(
    () => new Set(rules.map((rule) => rule.userId)),
    [rules]
  )

  const usersQuery = useQuery({
    queryKey: ['rate-limit-user-search', debouncedSearch],
    queryFn: () => searchRateLimitUsers(debouncedSearch),
    enabled: debouncedSearch.length > 0,
  })
  const availableUsers = useMemo(
    () =>
      (usersQuery.data ?? []).filter((user) => !configuredUserIds.has(user.id)),
    [configuredUserIds, usersQuery.data]
  )
  const hydratedRules = useMemo(() => {
    const users = new Map(knownUsers)
    for (const user of usersQuery.data ?? []) users.set(user.id, user)
    return hydrateUserRateLimitRules(rules, [...users.values()])
  }, [knownUsers, rules, usersQuery.data])

  const handleAdd = (user: RateLimitUserSummary | null) => {
    if (!user) return
    const result = addUserRateLimitRule(rules, {
      userId: user.id,
      maxRequests: props.defaultMaxRequests,
      maxSuccess: props.defaultMaxSuccess,
    })
    if (!result.added) return

    setKnownUsers((current) => {
      const next = new Map(current)
      next.set(user.id, user)
      return next
    })
    props.onChange(serializeUserRateLimitRules(result.rules))
    setSearch('')
  }

  const handleUpdate = (nextRule: UserRateLimitRule) => {
    const nextRules = rules.map((rule) =>
      rule.userId === nextRule.userId ? nextRule : rule
    )
    props.onChange(serializeUserRateLimitRules(nextRules))
  }

  const handleRemove = (userId: number) => {
    props.onChange(
      serializeUserRateLimitRules(
        rules.filter((rule) => rule.userId !== userId)
      )
    )
  }

  let emptySearchText = t('Type a username or user ID to search')
  if (debouncedSearch && usersQuery.isFetching) {
    emptySearchText = t('Searching users...')
  } else if (debouncedSearch) {
    emptySearchText = t('No matching users found')
  }

  return (
    <FieldSet>
      <FieldLegend variant='label'>
        {t('User-specific rate limits')}
      </FieldLegend>
      <FieldDescription>
        {t('User rules override group and global rate limits.')}
      </FieldDescription>
      <Combobox<RateLimitUserSummary>
        items={availableUsers}
        filteredItems={availableUsers}
        filter={null}
        value={null}
        inputValue={search}
        onInputValueChange={setSearch}
        onValueChange={handleAdd}
        itemToStringLabel={userLabel}
        itemToStringValue={(user) => String(user.id)}
        autoComplete='none'
      >
        <ComboboxInput
          className='w-full'
          placeholder={t('Search users by username or ID')}
          showClear
        />
        <ComboboxContent>
          <ComboboxList>
            <ComboboxGroup>
              <ComboboxCollection>
                {(user: RateLimitUserSummary) => (
                  <ComboboxItem key={user.id} value={user}>
                    <HugeiconsIcon icon={UserSearch01Icon} strokeWidth={2} />
                    <span className='min-w-0 truncate'>{userLabel(user)}</span>
                  </ComboboxItem>
                )}
              </ComboboxCollection>
            </ComboboxGroup>
          </ComboboxList>
          <ComboboxEmpty>{emptySearchText}</ComboboxEmpty>
        </ComboboxContent>
      </Combobox>

      {hydratedRules.length === 0 ? (
        <Empty className='border'>
          <EmptyHeader>
            <EmptyMedia variant='icon'>
              <HugeiconsIcon icon={UserAdd01Icon} strokeWidth={2} />
            </EmptyMedia>
            <EmptyTitle>{t('No user-specific rules')}</EmptyTitle>
            <EmptyDescription>
              {t('Search for a user above to add an override.')}
            </EmptyDescription>
          </EmptyHeader>
        </Empty>
      ) : (
        <FieldGroup className='gap-3'>
          {hydratedRules.map((rule) => (
            <UserRateLimitRow
              key={rule.userId}
              rule={rule}
              onUpdate={handleUpdate}
              onRemove={handleRemove}
            />
          ))}
        </FieldGroup>
      )}
    </FieldSet>
  )
}
