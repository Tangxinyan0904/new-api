# Wallet Transfer History and Usage Log Refresh Design

Date: 2026-07-14

## Goal

This design covers two user-facing changes:

1. Make affiliate reward transfers easier to understand and audit by adding user-visible transfer history, enforcing the existing minimum amount in the UI, changing the submitted state label, and permanently consuming amounts from newly rejected requests.
2. Add manual and bounded automatic refresh controls to all usage-log views.

## Confirmed Decisions

- A rejected request permanently consumes both its invitation reward and recharge rebate amounts.
- The new rejection rule applies only to requests rejected after this change is deployed. Previously rejected requests are not backfilled or deducted.
- Rejected request records and their original amounts remain available for history and audit.
- Transfer history opens in a dialog on the wallet page.
- The rejection-time forfeiture marker approach is preferred over submission-time reservation or a new accounting ledger.
- Automatic log refresh waits 30 seconds before its first refresh, executes at most four refresh rounds, and can be cancelled by clicking the control again.
- A manual refresh does not count toward the four automatic rounds. While automatic refresh is active, a manual refresh restarts the current 30-second countdown.

## Wallet Transfer Changes

### Wallet Actions

The affiliate rewards card action order becomes:

1. Refresh
2. Request Transfer
3. Transfer History

The existing two actions shift left and Transfer History occupies the rightmost position previously held by Request Transfer. The action row may wrap on narrow screens so translated labels do not overflow.

The frontend compares `total_pending_quota` with the configured `quotaPerUnit`, which represents one USD of credit. It must not hardcode the default internal value. When the available amount is below that threshold:

- the transfer dialog does not open;
- the request action is unavailable;
- a compact validation message states that the equivalent of $1 is required, formatted using the active currency display settings.

The backend keeps its authoritative `common.QuotaPerUnit` validation, including the exact-boundary behavior: an amount below one unit is rejected and an amount equal to one unit is accepted.

After a successful submission, both an active pending request and the existing one-request-per-day state render the same disabled label: `Submitted`. Once neither condition applies, the action returns to `Request Transfer` if enough rewards are available.

All new user-facing text must use the default frontend i18n system and be present in every supported locale.

### User Transfer History

Add an authenticated endpoint:

`GET /api/user/affiliate/transfer-requests/self`

The controller obtains the user ID only from the authenticated request context. It must not accept a user ID in the path, query, or request body. The model query always filters by that ID, orders by request ID descending, and uses the existing page query format.

Return a user-specific DTO containing only:

- request ID;
- invitation reward quota;
- recharge rebate quota;
- total quota;
- status;
- creation time;
- review time;
- rejection reason.

Do not expose the reviewer ID or the internal forfeiture marker.

Transfer History opens a wallet dialog and loads data on demand. It uses the existing pagination patterns and shows newest records first. Columns or responsive list fields are:

- submitted time;
- invitation reward;
- recharge rebate;
- total amount;
- status;
- reviewed time;
- rejection reason when present.

The dialog provides loading, empty, request-failure, and pagination states. Closing and reopening may reuse cached data but must revalidate it so a recently reviewed request is reflected.

### Rejection Accounting

Add an internal `int64` timestamp field named `RejectedQuotaForfeitedAt` to `AffiliateTransferRequest`, mapped to `rejected_quota_forfeited_at` and excluded from JSON serialization. A value greater than zero means the rejected amount was permanently consumed; zero or `NULL` means it was not. Do not assign a database-specific default. Existing rows therefore remain distinguishable from newly forfeited requests across SQLite, MySQL, and PostgreSQL.

Recharge rebate availability currently subtracts non-rejected requests. Update both recharge-rebate consumption queries to subtract requests matching either condition:

- status is not `rejected`; or
- the request has a positive rejection forfeiture timestamp.

The affected calculations are the current rebate summary and the recharge-source reconstruction used by transfer details. This preserves the old behavior for historical rejected rows while permanently consuming recharge rebate from newly rejected rows.

Rejecting a pending request runs in one database transaction:

1. Lock the request with `lockForUpdate(tx)` where supported.
2. Verify that its status is still `pending`.
3. Conditionally subtract the recorded invitation reward amount from `users.aff_quota` without allowing a negative balance.
4. Update the request with a status-guarded compare-and-set from `pending` to `rejected`, recording reviewer, review time, reason, and forfeiture time.
5. Do not add any amount to `users.quota`.

Any failure rolls back the complete transaction. There is no partial forfeiture. A second approve or reject operation must fail without another deduction or balance credit.

Approval must also replace the ineffective legacy GORM v1 lock call with `lockForUpdate(tx)` and use a pending-status compare-and-set before committing the balance change. Request creation locks the user row before checking pending and daily-request constraints, reducing duplicate creation races while retaining transaction-level checks for SQLite.

The existing admin list, detail, approve, and reject routes remain compatible. Rejection audit records continue to include both component amounts and the total amount.

## Usage Log Refresh Changes

### Control Placement

Extend `LogsFilterToolbar` with a dedicated refresh-action slot rather than overloading the existing leading action slot.

Desktop action order is:

`Reset -> Refresh -> Auto refresh -> Search -> Column settings`

The existing common-log sensitive-value toggle remains in its current leading slot. On mobile, Refresh and Auto refresh appear immediately before Search in the compact action area. Controls may wrap as a group at narrow widths, and the countdown control keeps a stable width so values from `30s` to `1s` do not shift surrounding actions.

Use familiar Lucide icons, accessible labels/tooltips for icon-only controls, and `aria-pressed` or equivalent state on the automatic-refresh control.

### Refresh Scope

Implement one reusable usage-log refresh action component/hook shared by common, drawing, and task filter bars.

Manual and automatic refresh operate on the currently active React Query identity. They preserve:

- the applied section;
- user or administrator view;
- applied URL filters;
- current page and page size;
- active table filters.

They do not apply unsubmitted draft filter input and do not navigate or reset pagination.

Use active, category-scoped query prefixes instead of broadly invalidating every cached log page:

- all sections refetch `['logs', logCategory, isAdminView]`;
- common logs additionally refetch `['usage-logs-stats', isAdminView]`;
- drawing and task logs do not refetch common-log statistics.

Manual refresh starts immediately and is disabled while the same refresh action is in flight.

### Automatic Refresh State Machine

The automatic refresh lifecycle is:

1. `idle`: show `Auto refresh`.
2. On click, enter `counting` with `30s`; do not issue an immediate request.
3. Count down once per second.
4. At zero, enter `refreshing` and run the scoped refresh once.
5. When all relevant queries settle, count the round whether it succeeded or failed.
6. If fewer than four rounds have completed, restart at `30s`.
7. After the fourth round settles, return to `idle`.

The next countdown begins after the preceding request settles, so slow requests cannot overlap. The four-round limit applies to scheduled refresh rounds, not internal HTTP retry attempts.

Clicking Auto refresh in either `counting` or `refreshing` cancels all future rounds immediately. An already-started query may finish, but its completion cannot restart the timer. Use a session or generation token together with timer cleanup to prevent stale asynchronous completions from reactivating a cancelled session.

A manual refresh while automatic refresh is active does not increment the automatic round count. It resets the current countdown to 30 seconds after the manual request settles, preventing an immediate duplicate automatic request.

Automatic refresh stops and resets when any part of the applied query identity changes, including:

- common, drawing, or task section;
- user or administrator view;
- applied search or reset parameters;
- page or page size;
- table filter identity;
- component unmount or navigation away from usage logs.

Editing a draft field without applying it does not change the active query and does not stop the session.

### Failure Handling

Refresh failures use the existing query error/toast behavior. A failed automatic round still consumes one of the four rounds, keeping the traffic limit deterministic. If common-log list and statistics refreshes have different outcomes, wait for both to settle before advancing the state machine.

Timer cleanup must occur on cancellation, identity changes, and unmount. No background timer may update state after the component is gone.

## Testing

### Backend

Add deterministic tests for these contracts:

- transfer requests below one configured quota unit fail and an exact one-unit request succeeds;
- a new rejection deducts invitation reward, permanently consumes recharge rebate, and does not increase main quota;
- a historical rejected record without a forfeiture marker remains available and does not consume current rebate;
- a repeated or competing rejection performs at most one deduction;
- competing approve/reject operations produce exactly one terminal transition and never duplicate a balance credit;
- the self-history endpoint returns only the authenticated user's records, in newest-first paginated order;
- the self-history response omits reviewer and internal forfeiture fields.

New or substantially rewritten Go tests use `testify/require` for fatal assertions and `testify/assert` for non-fatal comparisons.

### Frontend

Add focused deterministic tests for the refresh state transitions and query scope:

- no request before the first 30 seconds;
- exactly four completed rounds before stopping;
- cancellation during countdown and during an in-flight request;
- failed and slow requests still obey the round and overlap rules;
- manual refresh does not consume a round and resets the countdown;
- common logs refresh list and statistics, while drawing/task logs refresh only the list;
- query identity changes and unmount clean up the session.

Cover wallet behavior for the minimum amount, the `Submitted` state, user-history isolation at the API boundary, and the history dialog's loading, empty, status, rejection-reason, and pagination states.

Run focused Go tests, frontend type checking, lint/format checks for changed files, the production frontend build, and browser verification at desktop and narrow mobile widths.

## Out of Scope

- Retroactively deducting previously rejected transfer requests.
- Deleting or zeroing historical request amounts.
- Partial transfer amounts.
- Replacing the transfer-request model with a general accounting ledger.
- Unbounded or persistent usage-log polling beyond four rounds.
