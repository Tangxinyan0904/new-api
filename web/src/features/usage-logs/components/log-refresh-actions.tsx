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
import { useIsFetching, useQueryClient } from '@tanstack/react-query'
import { Loader2, RefreshCw, Timer } from 'lucide-react'
import { useCallback, useEffect, useReducer, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from '@/components/ui/tooltip'

import {
  INITIAL_AUTO_REFRESH_STATE,
  getUsageLogRefreshQueryKeys,
  getUsageLogRefreshScopeKey,
  reduceAutoRefreshState,
} from '../lib/auto-refresh'
import type { LogCategory } from '../types'

interface LogRefreshActionsProps {
  logCategory: LogCategory
  isAdminView: boolean
  scopeKey: string
}

function LogRefreshActionsSession(props: LogRefreshActionsProps) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [state, dispatch] = useReducer(
    reduceAutoRefreshState,
    INITIAL_AUTO_REFRESH_STATE
  )
  const [manualRefreshing, setManualRefreshing] = useState(false)
  const mountedRef = useRef(true)

  const fetchingLogs = useIsFetching({
    queryKey: ['logs', props.logCategory, props.isAdminView],
  })
  const fetchingStats = useIsFetching({
    queryKey: ['usage-logs-stats', props.isAdminView],
    predicate: () => props.logCategory === 'common',
  })
  const relevantFetching = fetchingLogs + fetchingStats

  const runRefresh = useCallback(async (): Promise<void> => {
    const queryKeys = getUsageLogRefreshQueryKeys(
      props.logCategory,
      props.isAdminView
    )
    await Promise.allSettled(
      queryKeys.map((queryKey) =>
        queryClient.refetchQueries({ queryKey, type: 'active' })
      )
    )
  }, [props.isAdminView, props.logCategory, queryClient])

  useEffect(() => {
    mountedRef.current = true
    return () => {
      mountedRef.current = false
    }
  }, [])

  useEffect(() => {
    if (state.phase !== 'counting') return

    const sessionId = state.sessionId
    const intervalId = window.setInterval(() => {
      dispatch({ type: 'tick', sessionId })
    }, 1000)

    return () => window.clearInterval(intervalId)
  }, [state.phase, state.sessionId])

  useEffect(() => {
    if (state.phase !== 'refreshing') return

    const sessionId = state.sessionId
    let active = true
    void runRefresh().finally(() => {
      if (active) {
        dispatch({ type: 'refreshSettled', sessionId })
      }
    })

    return () => {
      active = false
    }
  }, [runRefresh, state.phase, state.sessionId])

  const handleManualRefresh = useCallback(async (): Promise<void> => {
    const automaticCounting = state.phase === 'counting'
    const sessionId = state.sessionId

    if (automaticCounting) {
      dispatch({ type: 'manualStarted', sessionId })
    }
    setManualRefreshing(true)

    try {
      await runRefresh()
    } finally {
      if (mountedRef.current) {
        setManualRefreshing(false)
        if (automaticCounting) {
          dispatch({ type: 'manualSettled', sessionId })
        }
      }
    }
  }, [runRefresh, state.phase, state.sessionId])

  const handleAutoRefresh = useCallback((): void => {
    dispatch({ type: state.phase === 'idle' ? 'start' : 'cancel' })
  }, [state.phase])

  const automaticActive = state.phase !== 'idle'
  const automaticRefreshing =
    state.phase === 'refreshing' || state.phase === 'manual-refreshing'
  const manualButtonDisabled =
    manualRefreshing ||
    relevantFetching > 0 ||
    state.phase === 'refreshing' ||
    state.phase === 'manual-refreshing'
  const autoButtonDisabled = !automaticActive && manualRefreshing
  const showManualSpinner = manualRefreshing || relevantFetching > 0

  let autoLabel = t('Auto refresh')
  if (state.phase === 'counting') {
    autoLabel = `${state.secondsRemaining}s`
  } else if (automaticRefreshing) {
    autoLabel = t('Refreshing...')
  }

  return (
    <div className='flex shrink-0 items-center gap-1.5'>
      <Tooltip>
        <TooltipTrigger
          render={
            <Button
              type='button'
              variant='outline'
              size='icon'
              onClick={() => void handleManualRefresh()}
              disabled={manualButtonDisabled}
              aria-label={t('Refresh')}
            />
          }
        >
          {showManualSpinner ? (
            <Loader2
              data-icon='inline-start'
              className='animate-spin'
              aria-hidden='true'
            />
          ) : (
            <RefreshCw data-icon='inline-start' aria-hidden='true' />
          )}
        </TooltipTrigger>
        <TooltipContent>{t('Refresh')}</TooltipContent>
      </Tooltip>

      <Button
        type='button'
        variant={automaticActive ? 'secondary' : 'outline'}
        onClick={handleAutoRefresh}
        disabled={autoButtonDisabled}
        aria-pressed={automaticActive}
        aria-label={
          automaticActive ? t('Cancel auto refresh') : t('Auto refresh')
        }
        className='min-w-[6.5rem] tabular-nums'
      >
        {automaticRefreshing ? (
          <Loader2
            data-icon='inline-start'
            className='animate-spin'
            aria-hidden='true'
          />
        ) : (
          <Timer data-icon='inline-start' aria-hidden='true' />
        )}
        {autoLabel}
      </Button>
    </div>
  )
}

export function LogRefreshActions(props: LogRefreshActionsProps) {
  const sessionKey = getUsageLogRefreshScopeKey(
    props.logCategory,
    props.isAdminView,
    props.scopeKey
  )

  // A scope change remounts the timer owner before any effect can refresh the new scope.
  return <LogRefreshActionsSession key={sessionKey} {...props} />
}
