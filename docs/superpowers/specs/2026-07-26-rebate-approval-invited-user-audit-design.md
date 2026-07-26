# Rebate Approval Invited User Audit Design

Date: 2026-07-26

## Goal

Extend the admin rebate approval detail dialog so reviewers can audit every user invited by the applicant. Each invited user must show registration time and last login time, without separating users by invitation age, recharge activity, or whether they contributed to the current request.

## Confirmed Requirements

- Show all users whose `inviter_id` matches the rebate request applicant.
- Do not classify invited users as new or old.
- Do not filter the list by the transfer request creation time.
- Do not filter out users who never recharged or did not contribute to the current rebate amount.
- Include soft-deleted users and mark them as deleted.
- Show the existing `users.created_at` value as registration time.
- Show the existing `users.last_login_at` value as the last login time.
- Show `-` when a user has never logged in and `last_login_at` is zero.
- Preserve the existing recharge rebate source section; the new invited-user list supplements it.

## Chosen Approach

Extend the existing transfer-request detail response with an `invited_users` collection. The detail query already loads the applicant's invited users to calculate counts and recharge sources, so enriching that query avoids a second endpoint, client-side joins, and N+1 database access.

The response remains unpaginated because the requested behavior is to show the complete audit list in the detail dialog. The dialog body already has bounded height and scrolling, so a long list remains usable without changing the surrounding page.

## Backend Design

Add a dedicated response item representing an invited user. It contains only fields required by the admin audit UI:

- `id`
- `username`
- `display_name`
- `created_at`
- `last_login_at`
- `is_deleted`

Add `invited_users` to `AffiliateTransferRequestDetail`.

`GetAffiliateTransferRequestDetail` will query invited users with GORM `Unscoped()` so soft-deleted rows are included. The query will explicitly select only the required columns and order rows by `created_at DESC, id DESC`. This is compatible with SQLite, MySQL, and PostgreSQL and does not require a schema migration.

The same query result will continue to provide:

- `invited_count`
- invited user IDs used to find successful top-ups
- display names used by recharge source records
- the new complete `invited_users` audit collection

Soft deletion is converted to a response boolean instead of exposing GORM's internal deletion field. Passwords, access tokens, emails, OAuth identifiers, and other unrelated user fields are never selected or returned.

## Frontend Design

Extend `RebateApprovalDetail` with an invited-user item type matching the backend response.

Add an `Invited User Details` section between the existing rebate-source summary and source-record details. Each invited user row shows:

- display name, falling back to username and then a neutral placeholder
- user ID
- registration time
- last login time
- a deleted status badge when `is_deleted` is true

The list does not display new/old labels or recharge-contribution labels. Active users do not need an additional status badge. Dates use the existing `formatTimestamp` helper. A zero `last_login_at` renders as `-`.

The section uses compact bordered rows inside the existing scrolling dialog so it works on desktop and mobile without introducing a nested horizontal table.

## Internationalization

All new user-facing labels use `useTranslation()` and `t(...)`. Existing translation keys such as `Invited Users`, `User ID`, `Created At`, `Last Login`, and `Deleted` should be reused where available. Any missing keys will be added through the project's i18n maintenance script for every supported locale, followed by `bun run i18n:sync`.

## Error and Empty States

- A request with no invited users returns `invited_users: []`, not `null`.
- The UI displays a translated empty-state message when the list is empty.
- A never-logged-in user displays `-` for the last login time.
- A missing display name falls back to username; if both are empty, the UI displays `***`.
- Existing detail request loading and error behavior remains unchanged.

## Testing

Backend regression coverage will verify that the detail response:

- includes users registered before and after the transfer request
- includes users who never recharged
- includes soft-deleted users
- returns exact registration and last-login timestamps
- returns zero for users who never logged in
- orders users by registration time and then ID, both descending
- does not expose unrelated sensitive user fields

Frontend coverage will verify the display model for normal, never-logged-in, and deleted invited users. Existing rebate approval tests, frontend type checking, linting, formatting, and the production build will also be run.

## Non-Goals

- No pagination or search inside the invited-user list.
- No database migration or new user timestamp fields.
- No change to rebate calculation, approval, rejection, or recharge source attribution.
- No new/old invitation classification.
- No changes to the user-facing wallet invitation list.
