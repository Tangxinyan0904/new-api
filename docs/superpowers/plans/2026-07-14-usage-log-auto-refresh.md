# Usage Log Auto Refresh Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a manual refresh control and a cancellable 30-second automatic refresh cycle that runs at most four times for every usage-log section.

**Architecture:** Model the bounded countdown as a pure reducer with session IDs so cancellation and stale async completions are deterministic and directly testable. A reusable `LogRefreshActions` component owns timers and category-scoped React Query refetches, while `LogsFilterToolbar` only controls placement.

**Tech Stack:** React 19, TypeScript, TanStack Query/Router/Table, Base UI, Lucide React, Tailwind CSS, Bun test runner, Rsbuild.

---

## File Map

- Create `web/default/src/features/usage-logs/lib/auto-refresh.ts`: pure state machine, constants, and query-key selection.
- Create `web/default/src/features/usage-logs/lib/auto-refresh.test.ts`: deterministic round, cancellation, manual reset, and query-scope tests.
- Create `web/default/src/features/usage-logs/components/log-refresh-actions.tsx`: own timers, session-aware async refresh, and the two controls.
- Modify `web/default/src/features/usage-logs/components/logs-filter-toolbar.tsx`: add a dedicated refresh slot before Search on desktop and mobile.
- Modify `web/default/src/features/usage-logs/components/common-logs-filter-bar.tsx`: mount scoped refresh actions for common logs.
- Modify `web/default/src/features/usage-logs/components/task-logs-filter-bar.tsx`: mount scoped refresh actions for drawing/task logs.
- Modify `web/default/src/i18n/locales/{en,zh,fr,ja,ru,vi}.json`: translate the active cancellation label if missing.

### Task 1: Build the Automatic Refresh State Machine With TDD

**Files:**
- Create: `web/default/src/features/usage-logs/lib/auto-refresh.ts`
- Create: `web/default/src/features/usage-logs/lib/auto-refresh.test.ts`

- [ ] **Step 1: Add the failing state-machine tests**

Create `auto-refresh.test.ts` with `node:test` and `node:assert/strict`:

```ts
import assert from 'node:assert/strict'
import { describe, test } from 'node:test'

import {
  AUTO_REFRESH_MAX_ROUNDS,
  AUTO_REFRESH_SECONDS,
  INITIAL_AUTO_REFRESH_STATE,
  getUsageLogRefreshQueryKeys,
  reduceAutoRefreshState,
} from './auto-refresh'

describe('usage log automatic refresh', () => {
  test('waits thirty ticks before requesting the first refresh', () => {
    let state = reduceAutoRefreshState(INITIAL_AUTO_REFRESH_STATE, {
      type: 'start',
    })
    assert.equal(state.secondsRemaining, AUTO_REFRESH_SECONDS)
    const sessionId = state.sessionId

    for (let second = 1; second < AUTO_REFRESH_SECONDS; second += 1) {
      state = reduceAutoRefreshState(state, { type: 'tick', sessionId })
      assert.equal(state.phase, 'counting')
    }
    state = reduceAutoRefreshState(state, { type: 'tick', sessionId })
    assert.equal(state.phase, 'refreshing')
  })

  test('stops after exactly four settled refresh rounds', () => {
    let state = reduceAutoRefreshState(INITIAL_AUTO_REFRESH_STATE, {
      type: 'start',
    })
    const sessionId = state.sessionId

    for (let round = 1; round <= AUTO_REFRESH_MAX_ROUNDS; round += 1) {
      for (let second = 0; second < AUTO_REFRESH_SECONDS; second += 1) {
        state = reduceAutoRefreshState(state, { type: 'tick', sessionId })
      }
      assert.equal(state.phase, 'refreshing')
      state = reduceAutoRefreshState(state, {
        type: 'refreshSettled',
        sessionId,
      })
      assert.equal(state.completedRounds, round === AUTO_REFRESH_MAX_ROUNDS ? 0 : round)
    }
    assert.equal(state.phase, 'idle')
  })

  test('cancellation invalidates an in-flight completion', () => {
    let state = reduceAutoRefreshState(INITIAL_AUTO_REFRESH_STATE, {
      type: 'start',
    })
    const staleSessionId = state.sessionId
    for (let second = 0; second < AUTO_REFRESH_SECONDS; second += 1) {
      state = reduceAutoRefreshState(state, {
        type: 'tick',
        sessionId: staleSessionId,
      })
    }
    state = reduceAutoRefreshState(state, { type: 'cancel' })
    state = reduceAutoRefreshState(state, {
      type: 'refreshSettled',
      sessionId: staleSessionId,
    })
    assert.equal(state.phase, 'idle')
  })

  test('manual refresh pauses and resets without consuming a round', () => {
    let state = reduceAutoRefreshState(INITIAL_AUTO_REFRESH_STATE, {
      type: 'start',
    })
    const sessionId = state.sessionId
    state = reduceAutoRefreshState(state, {
      type: 'manualStarted',
      sessionId,
    })
    assert.equal(state.phase, 'manual-refreshing')
    state = reduceAutoRefreshState(state, {
      type: 'manualSettled',
      sessionId,
    })
    assert.equal(state.phase, 'counting')
    assert.equal(state.secondsRemaining, AUTO_REFRESH_SECONDS)
    assert.equal(state.completedRounds, 0)
  })

  test('selects only active category query prefixes', () => {
    assert.deepEqual(getUsageLogRefreshQueryKeys('common', true), [
      ['logs', 'common', true],
      ['usage-logs-stats', true],
    ])
    assert.deepEqual(getUsageLogRefreshQueryKeys('drawing', false), [
      ['logs', 'drawing', false],
    ])
    assert.deepEqual(getUsageLogRefreshQueryKeys('task', false), [
      ['logs', 'task', false],
    ])
  })
})
```

- [ ] **Step 2: Run the focused test and verify RED**

```powershell
Set-Location web/default
bun test src/features/usage-logs/lib/auto-refresh.test.ts
```

Expected: fail because `auto-refresh.ts` does not exist.

- [ ] **Step 3: Implement the reducer and query-key helper**

Create `auto-refresh.ts` with these public contracts:

```ts
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

export const INITIAL_AUTO_REFRESH_STATE: AutoRefreshState = {
  phase: 'idle',
  secondsRemaining: AUTO_REFRESH_SECONDS,
  completedRounds: 0,
  sessionId: 0,
}

export type AutoRefreshEvent =
  | { type: 'start' }
  | { type: 'cancel' }
  | { type: 'scopeChanged' }
  | { type: 'tick'; sessionId: number }
  | { type: 'refreshSettled'; sessionId: number }
  | { type: 'manualStarted'; sessionId: number }
  | { type: 'manualSettled'; sessionId: number }

function resetState(state: AutoRefreshState): AutoRefreshState {
  return {
    ...INITIAL_AUTO_REFRESH_STATE,
    sessionId: state.sessionId + 1,
  }
}

export function reduceAutoRefreshState(
  state: AutoRefreshState,
  event: AutoRefreshEvent
): AutoRefreshState {
  if (event.type === 'start') {
    if (state.phase !== 'idle') return state
    return {
      phase: 'counting',
      secondsRemaining: AUTO_REFRESH_SECONDS,
      completedRounds: 0,
      sessionId: state.sessionId + 1,
    }
  }
  if (event.type === 'cancel' || event.type === 'scopeChanged') {
    return resetState(state)
  }
  if (event.sessionId !== state.sessionId) return state

  if (event.type === 'tick') {
    if (state.phase !== 'counting') return state
    if (state.secondsRemaining > 1) {
      return { ...state, secondsRemaining: state.secondsRemaining - 1 }
    }
    return { ...state, phase: 'refreshing', secondsRemaining: 0 }
  }
  if (event.type === 'refreshSettled') {
    if (state.phase !== 'refreshing') return state
    const completedRounds = state.completedRounds + 1
    if (completedRounds >= AUTO_REFRESH_MAX_ROUNDS) {
      return resetState(state)
    }
    return {
      ...state,
      phase: 'counting',
      secondsRemaining: AUTO_REFRESH_SECONDS,
      completedRounds,
    }
  }
  if (event.type === 'manualStarted') {
    if (state.phase !== 'counting') return state
    return { ...state, phase: 'manual-refreshing' }
  }
  if (state.phase !== 'manual-refreshing') return state
  return {
    ...state,
    phase: 'counting',
    secondsRemaining: AUTO_REFRESH_SECONDS,
  }
}

export function getUsageLogRefreshQueryKeys(
  logCategory: LogCategory,
  isAdminView: boolean
): ReadonlyArray<readonly unknown[]> {
  const keys: Array<readonly unknown[]> = [
    ['logs', logCategory, isAdminView],
  ]
  if (logCategory === 'common') {
    keys.push(['usage-logs-stats', isAdminView])
  }
  return keys
}
```

- [ ] **Step 4: Run the focused test and verify GREEN**

Run the command from Step 2. Expected: all state and query-scope tests pass.

- [ ] **Step 5: Commit the state machine**

```powershell
Set-Location ../..
git add web/default/src/features/usage-logs/lib/auto-refresh.ts web/default/src/features/usage-logs/lib/auto-refresh.test.ts
git commit -m "feat(logs): model bounded automatic refresh"
```

### Task 2: Build Reusable Refresh Controls

**Files:**
- Create: `web/default/src/features/usage-logs/components/log-refresh-actions.tsx`

- [ ] **Step 1: Implement scoped query refetching**

Create `LogRefreshActions` with props:

```ts
interface LogRefreshActionsProps {
  logCategory: LogCategory
  isAdminView: boolean
  scopeKey: string
}
```

Use `useQueryClient`, the reducer from Task 1, and a memoized refresh function:

```ts
const runRefresh = useCallback(async () => {
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
```

Do not call the filter bars' `onSearch`: refresh must preserve the current page and applied URL state.

- [ ] **Step 2: Implement timer and stale-session cleanup**

Use one interval only while `state.phase === 'counting'`:

```ts
useEffect(() => {
  if (state.phase !== 'counting') return
  const sessionId = state.sessionId
  const intervalId = window.setInterval(() => {
    dispatch({ type: 'tick', sessionId })
  }, 1000)
  return () => window.clearInterval(intervalId)
}, [state.phase, state.sessionId])
```

Use a separate effect for automatic requests. The captured session ID makes completion after cancellation harmless:

```ts
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
```

Dispatch `scopeChanged` whenever `logCategory`, `isAdminView`, or `scopeKey` changes. The effect cleanup must clear the interval on unmount. Track mounted state with a ref so a standalone manual request also skips `setManualRefreshing` and reducer dispatch after unmount.

- [ ] **Step 3: Implement manual refresh and cancellation**

Keep local `manualRefreshing` state. When automatic counting is active, dispatch `manualStarted`, await `runRefresh`, then dispatch `manualSettled` with the captured session ID. When automatic mode is idle, manual refresh only executes the query. If Auto refresh is clicked in any non-idle phase, dispatch `cancel`; otherwise dispatch `start`.

Include `useIsFetching` for the current `['logs', category, isAdmin]` prefix and common statistics prefix. Disable the manual button while relevant queries are in flight. Keep the automatic button clickable whenever automatic mode is active so it can cancel an in-flight round; disable it only when a standalone manual refresh is in flight and automatic mode is idle, preventing a new countdown from starting over that request.

- [ ] **Step 4: Render stable accessible controls**

Render a familiar icon-only Refresh button with a tooltip and a fixed-width Auto refresh button. Use `RefreshCw`, `Timer`, and `Loader2` from Lucide. The automatic label rules are:

```ts
let autoLabel = t('Auto refresh')
if (state.phase === 'counting') autoLabel = `${state.secondsRemaining}s`
if (
  state.phase === 'refreshing' ||
  state.phase === 'manual-refreshing'
) {
  autoLabel = t('Refreshing...')
}
```

Set `aria-pressed={state.phase !== 'idle'}`, use `t('Cancel auto refresh')` as the active accessible label, and apply a stable minimum width such as `min-w-[6.5rem]` so the toolbar does not shift each second.

- [ ] **Step 5: Run focused static checks**

```powershell
Set-Location web/default
bun test src/features/usage-logs/lib/auto-refresh.test.ts
bun run typecheck
bunx oxlint -c .oxlintrc.json src/features/usage-logs/lib/auto-refresh.ts src/features/usage-logs/lib/auto-refresh.test.ts src/features/usage-logs/components/log-refresh-actions.tsx
```

Expected: all commands exit zero.

- [ ] **Step 6: Commit reusable controls**

```powershell
Set-Location ../..
git add web/default/src/features/usage-logs/components/log-refresh-actions.tsx
git commit -m "feat(logs): add manual and automatic refresh controls"
```

### Task 3: Place and Wire Refresh Controls

**Files:**
- Modify: `web/default/src/features/usage-logs/components/logs-filter-toolbar.tsx`
- Modify: `web/default/src/features/usage-logs/components/common-logs-filter-bar.tsx`
- Modify: `web/default/src/features/usage-logs/components/task-logs-filter-bar.tsx`

- [ ] **Step 1: Add a dedicated toolbar slot**

Extend `LogsFilterToolbarProps`:

```ts
refreshActions?: ReactNode
```

On desktop, render it after Reset and before Search. Keep `actionStart` before Reset so the sensitive-value toggle remains unchanged:

```tsx
{props.actionStart}
<Button ...>{t('Reset')}</Button>
{props.refreshActions}
<Button ...>{t('Search')}</Button>
<DataTableViewOptions table={props.table} />
```

In the mobile summary row, add `flex-wrap`, then render the slot after Filter and before Search. Do not duplicate it in the filter drawer footer, whose buttons only reset/apply draft filters.

- [ ] **Step 2: Wire common logs with list and statistics scope**

In `CommonLogsFilterBar`, import `LogRefreshActions` and pass:

```tsx
refreshActions={
  <LogRefreshActions
    logCategory='common'
    isAdminView={isAdmin}
    scopeKey={JSON.stringify(searchParams)}
  />
}
```

This category causes the component to refetch both the active log list and `usage-logs-stats`.

- [ ] **Step 3: Wire drawing and task logs with list-only scope**

In `TaskLogsFilterBar`, pass `props.logCategory`, the derived `isAdmin`, and `JSON.stringify(searchParams)` to the same component. The query-key helper must return only the active log list prefix for both categories.

- [ ] **Step 4: Run test, typecheck, and targeted lint**

```powershell
Set-Location web/default
bun test src/features/usage-logs/lib/auto-refresh.test.ts
bun run typecheck
bunx oxlint -c .oxlintrc.json src/features/usage-logs/components/logs-filter-toolbar.tsx src/features/usage-logs/components/common-logs-filter-bar.tsx src/features/usage-logs/components/task-logs-filter-bar.tsx src/features/usage-logs/components/log-refresh-actions.tsx
```

Expected: all commands exit zero.

- [ ] **Step 5: Commit toolbar integration**

```powershell
Set-Location ../..
git add web/default/src/features/usage-logs/components/logs-filter-toolbar.tsx web/default/src/features/usage-logs/components/common-logs-filter-bar.tsx web/default/src/features/usage-logs/components/task-logs-filter-bar.tsx
git commit -m "feat(logs): place refresh actions before search"
```

### Task 4: Complete i18n and Browser Verification

**Files:**
- Modify: `web/default/src/i18n/locales/en.json`
- Modify: `web/default/src/i18n/locales/zh.json`
- Modify: `web/default/src/i18n/locales/fr.json`
- Modify: `web/default/src/i18n/locales/ja.json`
- Modify: `web/default/src/i18n/locales/ru.json`
- Modify: `web/default/src/i18n/locales/vi.json`
- Verify: all files from Tasks 1-3

- [ ] **Step 1: Translate the active cancellation label**

Read `.agents/skills/i18n-translate/SKILL.md` completely and follow its synchronization workflow. Reuse existing `Refresh`, `Auto refresh`, and `Refreshing...` keys. Add `Cancel auto refresh` to every supported locale with a natural translation.

- [ ] **Step 2: Synchronize, format, and run full frontend checks**

```powershell
Set-Location web/default
bun run i18n:sync
bun run format
bun run format:check
bun test src/features/usage-logs/lib/auto-refresh.test.ts
bun test src/features/usage-logs/lib/cache-metrics.test.ts
bun run typecheck
bunx oxlint -c .oxlintrc.json src/features/usage-logs
bun run build
```

Expected: locale synchronization, formatting, both focused test suites, typecheck, lint, and production build all exit zero.

- [ ] **Step 3: Verify behavior with a real browser**

Start the existing local server and use the authenticated local session. Verify common, drawing, and task sections in user and administrator views:

1. Control order is Reset, Refresh, Auto refresh, Search on desktop; mobile places Refresh and Auto refresh immediately before Search.
2. Manual refresh preserves applied filters, page, page size, and view.
3. Auto refresh displays 30s immediately, makes no immediate request, and refreshes four times before returning to idle.
4. Clicking Auto refresh again cancels during both countdown and an active request.
5. A manual refresh during the cycle restarts at 30s without consuming a round.
6. Common refresh updates list and statistics; drawing/task refresh only their list.
7. Changing section, view, filter, page, or page size stops the cycle.
8. At 320px and 390px widths, controls wrap without overlap or clipped text.

- [ ] **Step 4: Commit translations and final formatting**

```powershell
Set-Location ../..
git add web/default/src/i18n/locales web/default/src/features/usage-logs
git commit -m "feat(i18n): translate log refresh controls"
git diff --check
git status --short
```

Expected: no whitespace errors and no uncommitted implementation files; the user's pre-existing `.gitignore` change remains untouched in the original worktree.
