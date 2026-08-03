# API Key Notice Toolbar Position Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Place the API key notice immediately after the Status filter while preserving the shared toolbar's existing action layout.

**Architecture:** Extend `DataTableToolbar` with one optional `ReactNode` slot rendered after faceted filters in both layout branches. Pass the existing `ApiKeyNotice` through that slot only from the API key table, leaving `preActions` unchanged for every other consumer.

**Tech Stack:** React 19, TypeScript, TanStack Table, Tailwind CSS, Bun test runner

---

### Task 1: Protect Toolbar Content Order

**Files:**
- Create: `web/default/src/components/data-table/toolbar/toolbar.test.tsx`
- Modify: `web/default/src/components/data-table/toolbar/toolbar.tsx`

- [ ] **Step 1: Write the failing test**

Create a server-rendered component test with mocked table filter components. Render filter content, `afterFilters`, and `preActions`, then assert their text offsets are strictly ordered:

```tsx
expect(html.indexOf('Status filter')).toBeLessThan(
  html.indexOf('API key notice')
)
expect(html.indexOf('API key notice')).toBeLessThan(
  html.indexOf('Right action')
)
```

- [ ] **Step 2: Run test to verify it fails**

Run: `bun test src/components/data-table/toolbar/toolbar.test.tsx`

Expected: FAIL because `afterFilters` is not part of `DataTableToolbarProps` and is not rendered.

- [ ] **Step 3: Add the toolbar slot**

Add the optional property and render it after `{filterChips}` in both the `leftActions` and default branches:

```tsx
afterFilters?: ReactNode
```

```tsx
{filterChips}
{props.afterFilters}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `bun test src/components/data-table/toolbar/toolbar.test.tsx`

Expected: PASS.

### Task 2: Move the API Key Notice

**Files:**
- Modify: `web/default/src/features/keys/components/api-keys-table.tsx`

- [ ] **Step 1: Use the filter-adjacent slot**

Replace the API key toolbar configuration:

```tsx
preActions: <ApiKeyNotice notice={status?.api_key_notice} />,
```

with:

```tsx
afterFilters: <ApiKeyNotice notice={status?.api_key_notice} />,
```

- [ ] **Step 2: Run focused tests**

Run:

```bash
bun test src/components/data-table/toolbar/toolbar.test.tsx src/features/keys/components/api-key-notice.test.tsx
```

Expected: all tests pass.

### Task 3: Verify Frontend Quality and Layout

**Files:**
- Verify: `web/default/src/components/data-table/toolbar/toolbar.tsx`
- Verify: `web/default/src/components/data-table/toolbar/toolbar.test.tsx`
- Verify: `web/default/src/features/keys/components/api-keys-table.tsx`

- [ ] **Step 1: Run static checks**

Run:

```bash
bun run typecheck
bunx oxlint -c .oxlintrc.json src/components/data-table/toolbar/toolbar.tsx src/components/data-table/toolbar/toolbar.test.tsx src/features/keys/components/api-keys-table.tsx
bunx oxfmt --check src/components/data-table/toolbar/toolbar.tsx src/components/data-table/toolbar/toolbar.test.tsx src/features/keys/components/api-keys-table.tsx
bun run build
```

Expected: every command exits successfully.

- [ ] **Step 2: Inspect the running page**

Open the API key management page at desktop and mobile widths. Confirm the order is Name search, API key search, Status, notice, then right-side actions, with clean wrapping on narrow screens.

- [ ] **Step 3: Commit the verified change**

```bash
git add docs/superpowers/specs/2026-08-02-api-key-notice-toolbar-position-design.md docs/superpowers/plans/2026-08-02-api-key-notice-toolbar-position.md web/default/src/components/data-table/toolbar/toolbar.tsx web/default/src/components/data-table/toolbar/toolbar.test.tsx web/default/src/features/keys/components/api-keys-table.tsx
git commit -m "fix: position API key notice beside status filter"
```
