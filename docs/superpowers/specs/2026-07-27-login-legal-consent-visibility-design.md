# Login Legal Consent Visibility Design

Date: 2026-07-27

## Background

When a user agreement or privacy policy is configured, password and alternative
login actions require the user to select the legal-consent checkbox. The action
buttons are correctly disabled until consent is selected, but the current shared
consent component uses a small checkbox, low-contrast translucent border, muted
background, and muted text. On a white sign-in panel, users can miss the reason
that "Continue with ..." actions are disabled.

The same `LegalConsent` component is used by sign-in and sign-up. Fixing only one
call site would leave inconsistent behavior and preserve the same discoverability
problem on the other authentication page.

## Goals

1. Make required consent immediately visible on light and dark authentication
   panels.
2. Increase the reliable pointer target without changing legal or login logic.
3. Preserve agreement/privacy links as independent navigation targets.
4. Keep the component keyboard accessible and visibly focused.
5. Apply the same presentation to sign-in and sign-up because they share the
   component.

## Confirmed Decisions

- The password, OAuth, and WeChat login gates remain unchanged.
- The shared component is updated for both sign-in and sign-up.
- The whole consent row is a pointer target except for legal-document links.
- The checkbox is larger, has a stronger unchecked border, and has an explicit
  checked fill/checkmark.
- Unchecked and checked rows have distinct high-contrast visual states.
- Existing user-agreement and privacy-policy content and routes are unchanged.

## Interaction Design

Retain the actual checkbox control and its associated label. Expand the visible
row to a stable, compact target with:

- a two-pixel high-contrast border;
- an opaque-enough warning-tinted background while unchecked;
- normal foreground text rather than muted low-contrast text;
- a checkbox target of at least 20 by 20 CSS pixels;
- pointer cursor and a clear hover state across non-link whitespace;
- a `focus-within` ring when the checkbox receives keyboard focus;
- a calmer primary/confirmed border and background when checked.

Clicking non-link space in the row toggles the checkbox once. Clicking the
checkbox itself also toggles once. Clicking the User Agreement or Privacy Policy
link opens that document and does not change consent state. The component must
avoid nested button semantics and preserve the native checkbox's Space-key
behavior.

The row remains compact and does not increase the authentication panel width.
Text may wrap naturally for longer translations; it must not overlap the
checkbox or overflow the panel on narrow mobile screens.

## Accessibility

- Keep an explicit label association with the checkbox.
- Preserve native keyboard focus and Space-key toggling.
- Ensure the focus indicator is visible in both themes.
- Maintain WCAG AA contrast for text and meaningful borders/states.
- Do not communicate checked state by color alone; the checkbox checkmark
  remains the primary state indicator.
- Agreement and privacy links retain normal keyboard navigation and visible
  link treatment.

## Scope and Compatibility

No backend, API, authentication, agreement-storage, or routing changes are
required. Existing translation strings are reused unless implementation reveals
that an accessible label is missing; any new user-facing string must then be
added through the complete frontend i18n workflow.

The component continues to render nothing when neither legal document is
configured. Existing callers continue to own the checked state and disable their
actions using the current logic.

## Testing and Verification

Focused component or browser checks cover:

- the unchecked row is visibly distinct on a white panel;
- clicking row whitespace toggles consent exactly once;
- clicking the checkbox toggles exactly once;
- clicking either legal link does not toggle consent;
- keyboard focus is visible and Space toggles the native checkbox;
- password/OAuth/WeChat actions remain disabled before consent and available
  afterward;
- sign-in and sign-up both use the enhanced shared presentation;
- long translated text wraps without clipping at desktop and narrow mobile
  widths;
- light and dark themes retain readable contrast.

Run frontend type checking, lint for the changed files, relevant tests, the
production build, and browser verification of sign-in and sign-up.

## Acceptance Criteria

1. A user can immediately identify the required legal-consent control before
   attempting an alternative login.
2. The entire non-link row reliably toggles consent, while document links do not.
3. The checkbox and focus state are clearly visible in light and dark themes.
4. Existing login gating and legal-document navigation behavior remain intact.
