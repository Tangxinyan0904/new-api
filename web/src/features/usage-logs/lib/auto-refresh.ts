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
import type { LogsViewAccess } from '../components/usage-logs-provider'
import type { LogCategory } from '../types'

export const AUTO_REFRESH_SECONDS = 30
export const AUTO_REFRESH_MAX_ROUNDS = 4

export type AutoRefreshPhase =
  | 'idle'
  | 'counting'
  | 'refreshing'
  | 'manual-refreshing'

export interface AutoRefreshState {
  phase: AutoRefreshPhase
  secondsRemaining: number
  completedRounds: number
  sessionId: number
}

export type AutoRefreshEvent =
  | { type: 'start' }
  | { type: 'tick'; sessionId: number }
  | { type: 'refreshSettled'; sessionId: number }
  | { type: 'cancel' }
  | { type: 'scopeChanged' }
  | { type: 'manualStarted'; sessionId: number }
  | { type: 'manualSettled'; sessionId: number }

export const INITIAL_AUTO_REFRESH_STATE: AutoRefreshState = {
  phase: 'idle',
  secondsRemaining: AUTO_REFRESH_SECONDS,
  completedRounds: 0,
  sessionId: 0,
}

export function reduceAutoRefreshState(
  state: AutoRefreshState,
  event: AutoRefreshEvent
): AutoRefreshState {
  switch (event.type) {
    case 'start':
      if (state.phase !== 'idle') return state
      return {
        phase: 'counting',
        secondsRemaining: AUTO_REFRESH_SECONDS,
        completedRounds: 0,
        sessionId: state.sessionId + 1,
      }

    case 'tick':
      if (state.phase !== 'counting' || event.sessionId !== state.sessionId) {
        return state
      }
      if (state.secondsRemaining > 1) {
        return {
          ...state,
          secondsRemaining: state.secondsRemaining - 1,
        }
      }
      return {
        ...state,
        phase: 'refreshing',
        secondsRemaining: 0,
      }

    case 'refreshSettled': {
      if (state.phase !== 'refreshing' || event.sessionId !== state.sessionId) {
        return state
      }

      const completedRounds = state.completedRounds + 1
      if (completedRounds >= AUTO_REFRESH_MAX_ROUNDS) {
        return {
          ...INITIAL_AUTO_REFRESH_STATE,
          sessionId: state.sessionId + 1,
        }
      }
      return {
        ...state,
        phase: 'counting',
        secondsRemaining: AUTO_REFRESH_SECONDS,
        completedRounds,
      }
    }

    case 'manualStarted':
      if (state.phase !== 'counting' || event.sessionId !== state.sessionId) {
        return state
      }
      return {
        ...state,
        phase: 'manual-refreshing',
      }

    case 'manualSettled':
      if (
        state.phase !== 'manual-refreshing' ||
        event.sessionId !== state.sessionId
      ) {
        return state
      }
      return {
        ...state,
        phase: 'counting',
        secondsRemaining: AUTO_REFRESH_SECONDS,
      }

    case 'cancel':
    case 'scopeChanged':
      return {
        ...INITIAL_AUTO_REFRESH_STATE,
        sessionId: state.sessionId + 1,
      }
  }
}

/**
 * Prefixes of the queries a refresh reloads. They must match the keys of the
 * list query in usage-logs-table.tsx (keyed by view access) and of the stats
 * query in common-logs-stats.tsx (keyed by whether it is an admin view).
 */
export function getUsageLogRefreshQueryKeys(
  category: LogCategory,
  viewAccess: LogsViewAccess
): ReadonlyArray<readonly unknown[]> {
  const logsQueryKey = ['logs', category, viewAccess] as const
  if (category !== 'common') return [logsQueryKey] as const

  return [
    logsQueryKey,
    ['usage-logs-stats', viewAccess !== 'self'] as const,
  ] as const
}

export function getUsageLogRefreshScopeKey(
  category: LogCategory,
  viewAccess: LogsViewAccess,
  scopeKey: string
): string {
  return JSON.stringify([category, viewAccess, scopeKey])
}
