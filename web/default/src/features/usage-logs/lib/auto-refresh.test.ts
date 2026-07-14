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
import assert from 'node:assert/strict'
import { describe, test } from 'node:test'

import {
  AUTO_REFRESH_MAX_ROUNDS,
  AUTO_REFRESH_SECONDS,
  INITIAL_AUTO_REFRESH_STATE,
  getUsageLogRefreshQueryKeys,
  getUsageLogRefreshScopeKey,
  reduceAutoRefreshState,
  type AutoRefreshState,
} from './auto-refresh.ts'

function dispatchTicks(
  state: AutoRefreshState,
  count: number
): AutoRefreshState {
  let nextState = state

  for (let index = 0; index < count; index += 1) {
    nextState = reduceAutoRefreshState(nextState, {
      type: 'tick',
      sessionId: state.sessionId,
    })
  }

  return nextState
}

describe('usage log automatic refresh state', () => {
  test('starts idle and waits for all 30 ticks before the first refresh', () => {
    assert.equal(AUTO_REFRESH_SECONDS, 30)
    assert.equal(AUTO_REFRESH_MAX_ROUNDS, 4)
    assert.deepEqual(INITIAL_AUTO_REFRESH_STATE, {
      phase: 'idle',
      secondsRemaining: 30,
      completedRounds: 0,
      sessionId: 0,
    })

    let state = reduceAutoRefreshState(INITIAL_AUTO_REFRESH_STATE, {
      type: 'start',
    })
    assert.deepEqual(state, {
      phase: 'counting',
      secondsRemaining: 30,
      completedRounds: 0,
      sessionId: 1,
    })
    assert.strictEqual(reduceAutoRefreshState(state, { type: 'start' }), state)

    state = dispatchTicks(state, 29)
    assert.equal(state.phase, 'counting')
    assert.equal(state.secondsRemaining, 1)

    state = dispatchTicks(state, 1)
    assert.deepEqual(state, {
      phase: 'refreshing',
      secondsRemaining: 0,
      completedRounds: 0,
      sessionId: 1,
    })
    assert.deepEqual(INITIAL_AUTO_REFRESH_STATE, {
      phase: 'idle',
      secondsRemaining: 30,
      completedRounds: 0,
      sessionId: 0,
    })
  })

  test('stops after exactly four settled automatic refreshes', () => {
    let state = reduceAutoRefreshState(INITIAL_AUTO_REFRESH_STATE, {
      type: 'start',
    })

    for (let round = 1; round <= AUTO_REFRESH_MAX_ROUNDS; round += 1) {
      state = dispatchTicks(state, AUTO_REFRESH_SECONDS)
      assert.equal(state.phase, 'refreshing')

      state = reduceAutoRefreshState(state, {
        type: 'refreshSettled',
        sessionId: state.sessionId,
      })

      if (round < AUTO_REFRESH_MAX_ROUNDS) {
        assert.deepEqual(state, {
          phase: 'counting',
          secondsRemaining: 30,
          completedRounds: round,
          sessionId: 1,
        })
      }
    }

    assert.deepEqual(state, {
      phase: 'idle',
      secondsRemaining: 30,
      completedRounds: 0,
      sessionId: 2,
    })
  })

  test('counts a failed automatic request through the shared settled event', () => {
    let state = reduceAutoRefreshState(INITIAL_AUTO_REFRESH_STATE, {
      type: 'start',
    })
    state = dispatchTicks(state, AUTO_REFRESH_SECONDS)

    const failedRequestSettled = {
      type: 'refreshSettled',
      sessionId: state.sessionId,
    } as const
    state = reduceAutoRefreshState(state, failedRequestSettled)

    assert.deepEqual(state, {
      phase: 'counting',
      secondsRemaining: 30,
      completedRounds: 1,
      sessionId: 1,
    })
  })

  test('ignores ticks while an automatic request is in flight', () => {
    let state = reduceAutoRefreshState(INITIAL_AUTO_REFRESH_STATE, {
      type: 'start',
    })
    state = dispatchTicks(state, AUTO_REFRESH_SECONDS)

    assert.equal(state.phase, 'refreshing')
    assert.strictEqual(
      reduceAutoRefreshState(state, {
        type: 'tick',
        sessionId: state.sessionId,
      }),
      state
    )
  })

  test('cancel invalidates countdowns and in-flight completions', () => {
    let state = reduceAutoRefreshState(INITIAL_AUTO_REFRESH_STATE, {
      type: 'start',
    })
    state = dispatchTicks(state, AUTO_REFRESH_SECONDS)
    state = reduceAutoRefreshState(state, {
      type: 'refreshSettled',
      sessionId: state.sessionId,
    })
    state = dispatchTicks(state, 5)

    const countdownSessionId = state.sessionId
    state = reduceAutoRefreshState(state, { type: 'cancel' })
    assert.deepEqual(state, {
      phase: 'idle',
      secondsRemaining: 30,
      completedRounds: 0,
      sessionId: 2,
    })
    assert.strictEqual(
      reduceAutoRefreshState(state, {
        type: 'tick',
        sessionId: countdownSessionId,
      }),
      state
    )

    state = reduceAutoRefreshState(state, { type: 'start' })
    state = dispatchTicks(state, AUTO_REFRESH_SECONDS)
    const requestSessionId = state.sessionId
    state = reduceAutoRefreshState(state, { type: 'cancel' })
    assert.strictEqual(
      reduceAutoRefreshState(state, {
        type: 'refreshSettled',
        sessionId: requestSessionId,
      }),
      state
    )

    state = reduceAutoRefreshState(state, { type: 'start' })
    state = reduceAutoRefreshState(state, {
      type: 'manualStarted',
      sessionId: state.sessionId,
    })
    const manualSessionId = state.sessionId
    state = reduceAutoRefreshState(state, { type: 'cancel' })
    assert.strictEqual(
      reduceAutoRefreshState(state, {
        type: 'manualSettled',
        sessionId: manualSessionId,
      }),
      state
    )
  })

  test('scope changes invalidate every event from the previous session', () => {
    let state = reduceAutoRefreshState(INITIAL_AUTO_REFRESH_STATE, {
      type: 'start',
    })
    const staleSessionId = state.sessionId
    state = reduceAutoRefreshState(state, { type: 'scopeChanged' })
    assert.deepEqual(state, {
      phase: 'idle',
      secondsRemaining: 30,
      completedRounds: 0,
      sessionId: 2,
    })

    state = reduceAutoRefreshState(state, { type: 'start' })
    assert.strictEqual(
      reduceAutoRefreshState(state, {
        type: 'tick',
        sessionId: staleSessionId,
      }),
      state
    )

    state = dispatchTicks(state, AUTO_REFRESH_SECONDS)
    assert.strictEqual(
      reduceAutoRefreshState(state, {
        type: 'refreshSettled',
        sessionId: staleSessionId,
      }),
      state
    )

    state = reduceAutoRefreshState(state, {
      type: 'refreshSettled',
      sessionId: state.sessionId,
    })
    state = reduceAutoRefreshState(state, {
      type: 'manualStarted',
      sessionId: state.sessionId,
    })
    assert.strictEqual(
      reduceAutoRefreshState(state, {
        type: 'manualSettled',
        sessionId: staleSessionId,
      }),
      state
    )
  })

  test('manual refresh preserves two completed rounds and restarts 30 seconds', () => {
    let state = reduceAutoRefreshState(INITIAL_AUTO_REFRESH_STATE, {
      type: 'start',
    })

    for (let round = 0; round < 2; round += 1) {
      state = dispatchTicks(state, AUTO_REFRESH_SECONDS)
      state = reduceAutoRefreshState(state, {
        type: 'refreshSettled',
        sessionId: state.sessionId,
      })
    }

    state = dispatchTicks(state, 7)
    assert.strictEqual(
      reduceAutoRefreshState(state, {
        type: 'manualStarted',
        sessionId: 0,
      }),
      state
    )
    state = reduceAutoRefreshState(state, {
      type: 'manualStarted',
      sessionId: state.sessionId,
    })
    assert.deepEqual(state, {
      phase: 'manual-refreshing',
      secondsRemaining: 23,
      completedRounds: 2,
      sessionId: 1,
    })

    assert.strictEqual(
      reduceAutoRefreshState(state, {
        type: 'manualSettled',
        sessionId: 0,
      }),
      state
    )

    state = reduceAutoRefreshState(state, {
      type: 'manualSettled',
      sessionId: state.sessionId,
    })
    assert.deepEqual(state, {
      phase: 'counting',
      secondsRemaining: 30,
      completedRounds: 2,
      sessionId: 1,
    })
  })

  test('ignores phase-specific events outside their active phase', () => {
    const idleState = INITIAL_AUTO_REFRESH_STATE
    assert.strictEqual(
      reduceAutoRefreshState(idleState, {
        type: 'tick',
        sessionId: idleState.sessionId,
      }),
      idleState
    )
    assert.strictEqual(
      reduceAutoRefreshState(idleState, {
        type: 'manualStarted',
        sessionId: idleState.sessionId,
      }),
      idleState
    )

    const countingState = reduceAutoRefreshState(idleState, { type: 'start' })
    assert.strictEqual(
      reduceAutoRefreshState(countingState, {
        type: 'refreshSettled',
        sessionId: countingState.sessionId,
      }),
      countingState
    )
    assert.strictEqual(
      reduceAutoRefreshState(countingState, {
        type: 'manualSettled',
        sessionId: countingState.sessionId,
      }),
      countingState
    )
  })
})

describe('getUsageLogRefreshQueryKeys', () => {
  test('returns the common, drawing, and task query prefixes', () => {
    assert.deepEqual(getUsageLogRefreshQueryKeys('common', false), [
      ['logs', 'common', false],
      ['usage-logs-stats', false],
    ])
    assert.deepEqual(getUsageLogRefreshQueryKeys('common', true), [
      ['logs', 'common', true],
      ['usage-logs-stats', true],
    ])
    assert.deepEqual(getUsageLogRefreshQueryKeys('drawing', true), [
      ['logs', 'drawing', true],
    ])
    assert.deepEqual(getUsageLogRefreshQueryKeys('task', false), [
      ['logs', 'task', false],
    ])
  })
})

describe('getUsageLogRefreshScopeKey', () => {
  test('changes when any applied query identity changes', () => {
    const baseScope = getUsageLogRefreshScopeKey(
      'common',
      false,
      '{"page":1,"pageSize":20}'
    )

    assert.equal(
      baseScope,
      getUsageLogRefreshScopeKey('common', false, '{"page":1,"pageSize":20}')
    )
    assert.notEqual(
      baseScope,
      getUsageLogRefreshScopeKey('common', true, '{"page":1,"pageSize":20}')
    )
    assert.notEqual(
      baseScope,
      getUsageLogRefreshScopeKey('common', false, '{"page":2,"pageSize":20}')
    )
    assert.notEqual(
      baseScope,
      getUsageLogRefreshScopeKey('drawing', false, '{"page":1,"pageSize":20}')
    )
  })
})
