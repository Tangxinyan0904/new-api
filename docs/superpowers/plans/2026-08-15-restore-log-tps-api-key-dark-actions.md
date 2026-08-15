# Restore Log TPS, API Key Dark Actions, and First-Visit Theme Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Restore desktop log throughput, give three API key actions a dark-mode-specific high-contrast treatment, and default new browser profiles to the existing `simple-large` preset without overriding saved choices.

**Architecture:** Reuse one pure throughput helper in desktop and mobile log views, add a semantic Button variant backed by light/dark CSS variables, and separate the first-visit preset default from the literal base palette in the existing theme customization provider. Existing cookies remain the persistence boundary and every preset selection is stored explicitly.

**Tech Stack:** React 19, TypeScript, Base UI, Tailwind CSS v4, class-variance-authority, Bun test runner, happy-dom, Playwright

## Global Constraints

- Preserve all existing new-api and QuantumNous branding and attribution.
- Add no new user-facing copy or i18n keys.
- Keep non-API-key outline buttons unchanged.
- Keep theme mode, font, radius, density, and content-layout defaults unchanged.
- Use the established `completion_tokens / use_time` throughput rule only when both values are positive.

---

### Task 1: Restore Desktop Throughput

**Files:**
- Modify: `web/src/features/usage-logs/lib/format.ts`
- Test: `web/src/features/usage-logs/lib/__tests__/format.test.ts`
- Modify: `web/src/features/usage-logs/components/usage-logs-mobile-card.tsx`
- Modify: `web/src/features/usage-logs/components/columns/common-logs-columns.tsx`
- Modify: `web/src/features/usage-logs/components/__tests__/stream-tps-cell.test.tsx`

**Interfaces:**
- Produces: `getTokensPerSecond(completionTokens: number, useTimeSec: number): number | null`
- Consumes: `StreamTpsCell`'s existing `tokensPerSecond` property.

- [ ] **Step 1: Write failing throughput tests**

Add literal expectations for valid, zero-duration, zero-output, and non-finite inputs:

```ts
expect(getTokensPerSecond(120, 2)).toBe(60)
expect(getTokensPerSecond(120, 0)).toBeNull()
expect(getTokensPerSecond(0, 2)).toBeNull()
expect(getTokensPerSecond(Number.POSITIVE_INFINITY, 2)).toBeNull()
```

Update `stream-tps-cell.test.tsx` so the default rendering must include `42 t/s`, and add an unavailable case that includes the em dash.

- [ ] **Step 2: Run tests and verify RED**

Run:

```powershell
bun test src/features/usage-logs/lib/__tests__/format.test.ts src/features/usage-logs/components/__tests__/stream-tps-cell.test.tsx
```

Expected: FAIL because `getTokensPerSecond` does not exist and the current stream-cell regression expects throughput to be hidden.

- [ ] **Step 3: Implement the shared calculation and desktop wiring**

Add:

```ts
export function getTokensPerSecond(
  completionTokens: number,
  useTimeSec: number
): number | null {
  if (!Number.isFinite(completionTokens) || completionTokens <= 0) return null
  if (!Number.isFinite(useTimeSec) || useTimeSec <= 0) return null
  return completionTokens / useTimeSec
}
```

Use it in both mobile and desktop. Pass the result to `StreamTpsCell`, remove `showTokensPerSecond={false}`, and set the stream column `size` to `72`.

- [ ] **Step 4: Run focused tests and verify GREEN**

Run the two files from Step 2 and confirm they pass.

### Task 2: Add the API Key Semantic Action Variant

**Files:**
- Modify: `web/src/styles/theme.css`
- Modify: `web/src/components/ui/button.tsx`
- Modify: `web/src/features/keys/components/api-keys-cells.tsx`
- Modify: `web/src/features/keys/components/api-key-group-combobox.tsx`
- Modify: `web/src/features/keys/components/data-table-row-actions.tsx`
- Test: `web/src/features/keys/components/__tests__/editable-api-key-group-cell.test.tsx`
- Test: `web/src/features/keys/components/__tests__/api-key-group-combobox.test.tsx`
- Create: `web/src/features/keys/components/__tests__/api-key-row-actions.test.tsx`

**Interfaces:**
- Produces: `Button` variant `apiKeyAction`.
- Produces: Tailwind colors `api-key-action`, `api-key-action-surface`, and `api-key-action-foreground`.

- [ ] **Step 1: Update component tests to require the dedicated treatment**

Change the Copy and compact group assertions from `outline-action` classes to:

```ts
assert.equal(button.classList.contains('border-api-key-action'), true)
assert.equal(button.classList.contains('bg-api-key-action-surface'), true)
assert.equal(button.classList.contains('text-api-key-action'), true)
```

Add a focused row-actions render test that locates the One-click import to CC Switch button by accessible text and requires the same three semantic classes.

- [ ] **Step 2: Run API key tests and verify RED**

Run:

```powershell
bun test src/features/keys/components/__tests__/editable-api-key-group-cell.test.tsx src/features/keys/components/__tests__/api-key-group-combobox.test.tsx src/features/keys/components/__tests__/api-key-row-actions.test.tsx
```

Expected: FAIL because the semantic classes and variant do not exist.

- [ ] **Step 3: Add tokens, variant, and scoped usages**

Register the three Tailwind color mappings in `@theme inline`. Define light values equal to the current outline action. Define dark values with a bright cyan-blue action color, darker blue-neutral surface, and dark hover foreground. Add:

```ts
apiKeyAction:
  'border-api-key-action bg-api-key-action-surface text-api-key-action hover:bg-api-key-action hover:text-api-key-action-foreground border-2 border-dashed shadow-none hover:-translate-y-[3px] hover:scale-[1.03]',
```

Use `variant='apiKeyAction'` on the Copy and CC Switch buttons. Use `compact ? 'apiKeyAction' : 'outline'` for the group combobox.

- [ ] **Step 4: Run API key tests and verify GREEN**

Run the three files from Step 2 and confirm they pass.

### Task 3: Default New Browsers to Simple Large-font

**Files:**
- Modify: `web/src/lib/theme-customization.ts`
- Modify: `web/src/context/theme-customization-provider.tsx`
- Create: `web/src/context/__tests__/theme-customization-provider.test.tsx`

**Interfaces:**
- Changes: `DEFAULT_THEME_CUSTOMIZATION.preset` from `default` to `simple-large`.
- Preserves: `theme_preset` cookie as the browser persistence key.

- [ ] **Step 1: Write failing provider behavior tests**

With a clean cookie jar, render the provider and assert:

```ts
assert.equal(document.body.dataset.themePreset, 'simple-large')
assert.match(document.cookie, /theme_preset=simple-large/)
```

Then select `default`, assert the data attribute is removed but `theme_preset=default` is stored, remount, and assert `default` remains selected.

- [ ] **Step 2: Run the provider test and verify RED**

Run:

```powershell
bun test src/context/__tests__/theme-customization-provider.test.tsx
```

Expected: FAIL because a clean browser currently selects the base `default` palette and removes the preset cookie.

- [ ] **Step 3: Implement first-visit default and explicit persistence**

Set the customization default preset to `simple-large`. In the provider, remove `data-theme-preset` only when the selected value is the literal `default` palette. Persist the current preset from the preset effect on mount and every preset change, and make `setPreset` update state without deleting the cookie.

- [ ] **Step 4: Run the provider test and verify GREEN**

Run the file from Step 2 and confirm it passes.

### Task 4: Verify the Complete Frontend Change

**Files:**
- Verify all files changed in Tasks 1-3.

- [ ] **Step 1: Run focused and related tests**

Run:

```powershell
bun test src/features/usage-logs/lib/__tests__/format.test.ts src/features/usage-logs/components/__tests__/stream-tps-cell.test.tsx src/features/keys/components/__tests__/editable-api-key-group-cell.test.tsx src/features/keys/components/__tests__/api-key-group-combobox.test.tsx src/features/keys/components/__tests__/api-key-row-actions.test.tsx src/context/__tests__/theme-customization-provider.test.tsx
```

- [ ] **Step 2: Run static and build checks**

Run:

```powershell
bun run typecheck
bunx oxlint -c .oxlintrc.json src/features/usage-logs/lib/format.ts src/features/usage-logs/lib/__tests__/format.test.ts src/features/usage-logs/components/usage-logs-mobile-card.tsx src/features/usage-logs/components/columns/common-logs-columns.tsx src/features/usage-logs/components/__tests__/stream-tps-cell.test.tsx src/components/ui/button.tsx src/features/keys/components/api-keys-cells.tsx src/features/keys/components/api-key-group-combobox.tsx src/features/keys/components/data-table-row-actions.tsx src/features/keys/components/__tests__/editable-api-key-group-cell.test.tsx src/features/keys/components/__tests__/api-key-group-combobox.test.tsx src/features/keys/components/__tests__/api-key-row-actions.test.tsx src/lib/theme-customization.ts src/context/theme-customization-provider.tsx src/context/__tests__/theme-customization-provider.test.tsx
bunx oxfmt --check src/features/usage-logs/lib/format.ts src/features/usage-logs/lib/__tests__/format.test.ts src/features/usage-logs/components/usage-logs-mobile-card.tsx src/features/usage-logs/components/columns/common-logs-columns.tsx src/features/usage-logs/components/__tests__/stream-tps-cell.test.tsx src/components/ui/button.tsx src/features/keys/components/api-keys-cells.tsx src/features/keys/components/api-key-group-combobox.tsx src/features/keys/components/data-table-row-actions.tsx src/features/keys/components/__tests__/editable-api-key-group-cell.test.tsx src/features/keys/components/__tests__/api-key-group-combobox.test.tsx src/features/keys/components/__tests__/api-key-row-actions.test.tsx src/lib/theme-customization.ts src/context/theme-customization-provider.tsx src/context/__tests__/theme-customization-provider.test.tsx
bun run build
```

Expected: every command exits successfully.

- [ ] **Step 3: Verify in a real browser**

Start the local frontend and inspect desktop usage logs plus the API key page in light and dark modes. Confirm `t/s` is visible, the stream column remains compact, the three API key actions share the dedicated treatment, the dark surface/border contrast is clear, and a clean browser profile starts on `simple-large` while a changed preset survives reload.

- [ ] **Step 4: Review and commit**

Run `git diff --check`, inspect the complete diff, and commit the verified implementation without touching unrelated worktrees or root-directory changes.
