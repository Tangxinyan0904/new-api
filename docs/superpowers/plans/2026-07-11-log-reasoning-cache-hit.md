# Log Reasoning Effort and Cache Hit Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Show raw reasoning effort and provider-correct cache hit percentages in desktop, mobile, and detailed usage logs for users and administrators.

**Architecture:** Keep the log API and database unchanged. Add a pure frontend metric helper that normalizes existing cache fields and applies OpenAI or Anthropic token semantics, then reuse it across table columns, mobile cards, and the details dialog.

**Tech Stack:** React 19, TypeScript, TanStack Table, Base UI, react-i18next, Bun test runner.

---

### Task 1: Add cache metric calculation with TDD

**Files:**
- Create: `web/default/src/features/usage-logs/lib/cache-metrics.ts`
- Create: `web/default/src/features/usage-logs/lib/cache-metrics.test.ts`
- Modify: `web/default/src/features/usage-logs/types.ts`

- [ ] **Step 1: Write the failing tests**

Create table-driven `bun:test` cases that call `getCacheHitMetrics(promptTokens, other)` and assert:

```ts
expect(getCacheHitMetrics(1000, { cache_tokens: 250 })).toEqual({
  cacheReadTokens: 250,
  cacheWriteTokens: 0,
  totalInputTokens: 1000,
  percentage: 25,
  formattedPercentage: '25.000%',
})

expect(
  getCacheHitMetrics(100, {
    usage_semantic: 'anthropic',
    cache_tokens: 300,
    cache_write_tokens: 100,
  })
).toMatchObject({
  cacheReadTokens: 300,
  cacheWriteTokens: 100,
  totalInputTokens: 500,
  percentage: 60,
  formattedPercentage: '60.000%',
})
```

Also cover split `5m + 1h` cache writes, unsplit legacy creation fallback, legacy `claude: true`, zero input as `0.000%`, malformed negative/non-finite values, and a fractional result rounded to exactly three decimal places.

- [ ] **Step 2: Run the test and verify RED**

Run:

```powershell
cd web/default
bun test src/features/usage-logs/lib/cache-metrics.test.ts
```

Expected: FAIL because `cache-metrics.ts` and `getCacheHitMetrics` do not exist.

- [ ] **Step 3: Add the required log types**

Extend `LogOtherData` with:

```ts
cache_write_tokens?: number
input_tokens_total?: number
usage_semantic?: string
```

Keep the existing `claude?: boolean` legacy field.

- [ ] **Step 4: Implement the minimal pure helper**

Implement and export:

```ts
export interface CacheHitMetrics {
  cacheReadTokens: number
  cacheWriteTokens: number
  totalInputTokens: number
  percentage: number
  formattedPercentage: string
}

export function getCacheHitMetrics(
  promptTokens: number | null | undefined,
  other: LogOtherData | null | undefined
): CacheHitMetrics
```

Normalize every numeric input to a finite non-negative value. Prefer `cache_write_tokens`, then split creation totals, then the unsplit creation total. Use Anthropic semantics when `usage_semantic === 'anthropic' || claude === true`; otherwise use OpenAI semantics. Clamp percentage to `[0, 100]` and format with `toFixed(3) + '%'`.

- [ ] **Step 5: Run the focused test and verify GREEN**

Run the same `bun test` command. Expected: all cache metric cases pass with no warnings.

- [ ] **Step 6: Commit the metric helper**

```powershell
git add web/default/src/features/usage-logs/lib/cache-metrics.ts web/default/src/features/usage-logs/lib/cache-metrics.test.ts web/default/src/features/usage-logs/types.ts
git commit -m "feat(logs): calculate provider-aware cache hit rate"
```

### Task 2: Add desktop log columns and mobile fields

**Files:**
- Modify: `web/default/src/features/usage-logs/components/columns/common-logs-columns.tsx`
- Modify: `web/default/src/features/usage-logs/components/usage-logs-mobile-card.tsx`

- [ ] **Step 1: Add desktop reasoning effort column**

Add a common-log column with id `reasoning_effort`. Parse `log.other`, display its raw `reasoning_effort` value, and show `-` when absent. Do not gate the column on `isAdmin`.

- [ ] **Step 2: Add desktop cache hit column**

Add a common-log column with id `cache_hit`. Call `getCacheHitMetrics(log.prompt_tokens, parseLogOther(log.other))` and render `formattedPercentage` in tabular monospace text. Do not gate the column on `isAdmin`.

- [ ] **Step 3: Add both values to common mobile cards**

Render `SummaryField` entries backed by `cells.get('reasoning_effort')` and `cells.get('cache_hit')`. This reuses the desktop cell renderers and keeps mobile semantics identical.

- [ ] **Step 4: Run typecheck and targeted lint**

```powershell
cd web/default
bun run typecheck
bunx oxlint -c .oxlintrc.json src/features/usage-logs/components/columns/common-logs-columns.tsx src/features/usage-logs/components/usage-logs-mobile-card.tsx
```

Expected: both commands exit zero.

- [ ] **Step 5: Commit list rendering**

```powershell
git add web/default/src/features/usage-logs/components/columns/common-logs-columns.tsx web/default/src/features/usage-logs/components/usage-logs-mobile-card.tsx
git commit -m "feat(logs): show reasoning and cache hit columns"
```

### Task 3: Group reasoning effort and cache hit details

**Files:**
- Modify: `web/default/src/features/usage-logs/components/dialogs/details-dialog.tsx`

- [ ] **Step 1: Replace the ungrouped reasoning row**

Wrap the existing raw `other.reasoning_effort` badge in:

```tsx
<DetailSection label={t('Reasoning Effort')}>
  <DetailRow label={t('Level')} value={reasoningBadge} />
</DetailSection>
```

Keep the current `high` and `medium` color treatment and preserve all unknown raw values.

- [ ] **Step 2: Add the cache hit section**

For displayable logs, derive metrics with `getCacheHitMetrics(props.log.prompt_tokens, other)` and render:

```tsx
<DetailSection label={t('Cache Hit')}>
  <DetailRow label={t('Hit Rate')} value={metrics.formattedPercentage} mono />
  <DetailRow label={t('Cache Read')} value={formatTokens(metrics.cacheReadTokens)} mono />
  <DetailRow label={t('Total Input Tokens')} value={formatTokens(metrics.totalInputTokens)} mono />
</DetailSection>
```

Render the section even when cache read is zero so the agreed `0.000%` is visible.

- [ ] **Step 3: Run focused test, typecheck, and targeted lint**

```powershell
cd web/default
bun test src/features/usage-logs/lib/cache-metrics.test.ts
bun run typecheck
bunx oxlint -c .oxlintrc.json src/features/usage-logs/components/dialogs/details-dialog.tsx
```

Expected: all commands exit zero.

- [ ] **Step 4: Commit detail groups**

```powershell
git add web/default/src/features/usage-logs/components/dialogs/details-dialog.tsx
git commit -m "feat(logs): group reasoning and cache hit details"
```

### Task 4: Add all locale strings through the i18n workflow

**Files:**
- Create temporarily: `web/default/scripts/add-missing-keys.mjs`
- Modify through script: `web/default/src/i18n/locales/en.json`
- Modify through script: `web/default/src/i18n/locales/zh.json`
- Modify through script: `web/default/src/i18n/locales/fr.json`
- Modify through script: `web/default/src/i18n/locales/ja.json`
- Modify through script: `web/default/src/i18n/locales/ru.json`
- Modify through script: `web/default/src/i18n/locales/vi.json`

- [ ] **Step 1: Identify only genuinely missing keys**

Check the locale files for `Reasoning Effort`, `Cache Hit`, `Hit Rate`, `Level`, and `Total Input Tokens`. Reuse existing keys and translations where present.

- [ ] **Step 2: Add missing translations through the mandated script**

Populate `newKeys` in `scripts/add-missing-keys.mjs` for all six locales. Use concise UI translations, including Chinese `推理强度`, `缓存命中`, `命中率`, `级别`, and `总输入令牌数` where those keys are missing. Run:

```powershell
cd web/default
node scripts/add-missing-keys.mjs
bun run i18n:sync
```

- [ ] **Step 3: Remove the temporary script and verify keys**

Delete `scripts/add-missing-keys.mjs`, search every locale for each used key, and confirm `i18n:sync` reports no newly missing key.

- [ ] **Step 4: Commit translations**

```powershell
git add web/default/src/i18n/locales/en.json web/default/src/i18n/locales/zh.json web/default/src/i18n/locales/fr.json web/default/src/i18n/locales/ja.json web/default/src/i18n/locales/ru.json web/default/src/i18n/locales/vi.json
git commit -m "feat(i18n): translate log cache hit metrics"
```

### Task 5: Verify the complete feature

**Files:**
- Verify all files changed in Tasks 1-4.

- [ ] **Step 1: Run focused tests and static checks**

```powershell
cd web/default
bun test src/features/usage-logs/lib/cache-metrics.test.ts
bun run typecheck
bunx oxlint -c .oxlintrc.json src/features/usage-logs/lib/cache-metrics.ts src/features/usage-logs/lib/cache-metrics.test.ts src/features/usage-logs/types.ts src/features/usage-logs/components/columns/common-logs-columns.tsx src/features/usage-logs/components/usage-logs-mobile-card.tsx src/features/usage-logs/components/dialogs/details-dialog.tsx
bun run i18n:sync
bun run build
```

Expected: focused tests, typecheck, targeted lint, i18n synchronization, and production build all exit zero.

- [ ] **Step 2: Check repository integrity**

```powershell
cd ../..
git diff --check
git status --short
```

Expected: no whitespace errors; only intentional source changes and the pre-existing untracked `tmp/` remain.

- [ ] **Step 3: Inspect representative calculations**

Verify the existing log examples produce provider-correct values:

```text
OpenAI: prompt=4523, cache_read=4352 => 96.219%
Claude: prompt=35, cache_read=162151, cache_write=870 => 99.445%
No cache => 0.000%
```

- [ ] **Step 4: Commit any final formatting-only corrections**

Only if verification modifies intentional tracked files:

```powershell
git add web/default/src/features/usage-logs web/default/src/i18n/locales
git commit -m "chore(logs): finalize cache hit display"
```
