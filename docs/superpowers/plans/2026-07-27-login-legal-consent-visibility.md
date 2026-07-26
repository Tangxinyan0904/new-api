# Login Legal Consent Visibility Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make the shared sign-in/sign-up legal-consent control highly visible and make its whole non-link row clickable without changing authentication gates.

**Architecture:** Keep the existing controlled `LegalConsent` API and replace its split container/label structure with one semantic label row containing the existing Base UI checkbox and legal links. Use checked-state-dependent Tailwind classes for contrast and a focused server-rendered regression test for the visible markup contract; verify pointer and keyboard behavior in a real browser.

**Tech Stack:** React 19, TypeScript, Base UI Checkbox, Tailwind CSS, react-i18next, Bun test, React DOM server rendering.

---

## File Map

- `web/default/src/features/auth/components/legal-consent.tsx`: shared consent row used by sign-in and sign-up.
- `web/default/src/features/auth/components/legal-consent.test.tsx`: server-rendered contract tests for visibility, checked state, links, and conditional rendering.

### Task 1: Add the Failing Visibility Contract

**Files:**

- Create: `web/default/src/features/auth/components/legal-consent.test.tsx`

- [ ] **Step 1: Write the failing test**

Create a Bun test that mocks only translation, renders the real component, and checks the user-visible markup contract:

```tsx
import { describe, expect, mock, test } from 'bun:test'
import { renderToStaticMarkup } from 'react-dom/server'

import type { SystemStatus } from '../types'

mock.module('react-i18next', () => ({
  useTranslation: () => ({ t: (key: string) => key }),
}))

const { LegalConsent } = await import('./legal-consent')

const legalStatus = {
  user_agreement_enabled: true,
  privacy_policy_enabled: true,
} as SystemStatus

describe('LegalConsent', () => {
  test('renders a high-contrast unchecked row with a larger checkbox', () => {
    const html = renderToStaticMarkup(
      <LegalConsent
        status={legalStatus}
        checked={false}
        onCheckedChange={() => undefined}
      />
    )

    expect(html).toContain('border-destructive/70')
    expect(html).toContain('bg-destructive/10')
    expect(html).toContain('focus-within:ring-3')
    expect(html).toContain('size-5')
    expect(html).toContain('href="/user-agreement"')
    expect(html).toContain('href="/privacy-policy"')
    expect(html).toContain('and')
  })

  test('renders a distinct confirmed state', () => {
    const html = renderToStaticMarkup(
      <LegalConsent
        status={legalStatus}
        checked
        onCheckedChange={() => undefined}
      />
    )

    expect(html).toContain('border-primary/70')
    expect(html).toContain('bg-primary/10')
    expect(html).not.toContain('border-destructive/70')
  })

  test('renders nothing when no legal document is enabled', () => {
    const html = renderToStaticMarkup(
      <LegalConsent
        status={{} as SystemStatus}
        checked={false}
        onCheckedChange={() => undefined}
      />
    )

    expect(html).toBe('')
  })
})
```

- [ ] **Step 2: Run the test and verify RED**

Run from `web/default`:

```bash
bun test src/features/auth/components/legal-consent.test.tsx
```

Expected: the unchecked/checked visibility assertions fail because the current component still uses translucent muted styling and a `size-4` checkbox.

### Task 2: Implement the Shared High-Contrast Row

**Files:**

- Modify: `web/default/src/features/auth/components/legal-consent.tsx`

- [ ] **Step 1: Replace the split container with one semantic label row**

Keep the existing props and early return. Render `Label` as the outer row with `htmlFor='legal-consent'`, place `Checkbox` and the copy inside it, and select row colors from `checked`:

```tsx
<Label
  htmlFor='legal-consent'
  className={cn(
    'focus-within:ring-ring/60 flex cursor-pointer items-start gap-3 rounded-lg border-2 px-3 py-2.5 text-left text-sm leading-5 font-medium transition-colors focus-within:ring-3 focus-within:ring-offset-2',
    checked
      ? 'border-primary/70 bg-primary/10 text-foreground hover:bg-primary/15'
      : 'border-destructive/70 bg-destructive/10 text-foreground hover:bg-destructive/15 dark:border-destructive/80 dark:bg-destructive/15',
    className
  )}
>
  <Checkbox
    id='legal-consent'
    checked={checked}
    onCheckedChange={handleChange}
    className='border-foreground/70 bg-background mt-0.5 size-5 border-2 shadow-sm'
  />
  <span className='min-w-0 flex-1'>...</span>
</Label>
```

Keep the agreement/privacy anchors as links with `font-semibold`, `underline`, and a visible focus style. Use the existing `t('and')` key instead of the current untranslated literal conjunction. Do not add click handlers: native label activation toggles non-link row space once, while interactive anchor descendants retain their own action.

- [ ] **Step 2: Run the focused test and verify GREEN**

Run from `web/default`:

```bash
bun test src/features/auth/components/legal-consent.test.tsx
```

Expected: all three tests pass.

- [ ] **Step 3: Run static checks**

Run from `web/default`:

```bash
bun run typecheck
bunx oxlint -c .oxlintrc.json src/features/auth/components/legal-consent.tsx src/features/auth/components/legal-consent.test.tsx
bun run format:check
```

Expected: all commands exit successfully.

- [ ] **Step 4: Commit**

```bash
git add web/default/src/features/auth/components/legal-consent.tsx web/default/src/features/auth/components/legal-consent.test.tsx
git commit -m "fix(auth): make legal consent prominent"
```

### Task 3: Browser Interaction and Layout Verification

**Files:**

- Verify only; fix `web/default/src/features/auth/components/legal-consent.tsx` if a check fails.

- [ ] **Step 1: Verify sign-in and sign-up behavior**

With both legal documents enabled, check `/sign-in` and `/sign-up` at desktop and narrow mobile widths:

- the unchecked row is immediately visible on a white panel;
- clicking row whitespace toggles consent once;
- clicking the checkbox toggles consent once;
- clicking either legal link opens the document without changing consent;
- keyboard Tab shows the focus ring and Space toggles the checkbox;
- password, OAuth, and WeChat actions remain disabled before consent and enabled afterward;
- long translated text wraps inside the panel without clipping;
- checked and unchecked states remain readable in light and dark themes.

- [ ] **Step 2: Run the production build**

Run from `web/default`:

```bash
bun run build
```

Expected: production build succeeds.

- [ ] **Step 3: Commit browser-driven fixes if needed**

```bash
git add web/default/src/features/auth/components/legal-consent.tsx web/default/src/features/auth/components/legal-consent.test.tsx
git commit -m "fix(auth): refine consent interaction"
```
