# Bulk Rebate Approval, Distillation History, and Counter Optimization

## Context

The administration UI currently approves affiliate rebate transfer requests
one at a time. Distillation protection uses an exact rolling one-minute
window: Redis stores one sorted-set member per request, while the in-memory
fallback stores one timestamp per request. Active distillation state is kept
in `distillation_penalties`, but expired or administrator-cleared state is
deleted, so users cannot review when detection occurred. The API key notice is
also rendered at a compact 12px size that is difficult to read.

This change adds four related operational improvements:

1. Approve every pending rebate transfer request with one administrator
   action.
2. Give each user a permanent history of their own distillation detections.
3. Replace per-request rolling-window storage with fixed natural-minute
   counters and a single threshold-crossing transition.
4. Increase the API key notice title and body text to 14px without enlarging
   the surrounding toolbar unnecessarily.

## Goals

- Let an administrator atomically approve all pending rebate requests.
- Preserve the existing balance checks, audit records, user-visible operation
  logs, and quota cache updates for every approved request.
- Record exactly one immutable violation event for each real distillation
  state transition.
- Show users when detection occurred and what action was taken.
- Reduce Redis and process memory from O(requests per minute) to O(active
  users), with constant-time counter operations.
- Keep Redis and in-memory behavior aligned.
- Continue supporting SQLite, MySQL, and PostgreSQL.

## Non-Goals

- Selecting individual rebate requests for a batch.
- Deleting or editing violation history from the user interface.
- Changing the first-strike temporary-limit and second-strike permanent-ban
  policy.
- Eliminating the intentional fixed-window boundary burst. An account can use
  one allowance immediately before a natural-minute boundary and another
  immediately after it.
- Replacing the configured per-user or group rate limits.

## 1. Approve All Pending Rebate Requests

### API and transaction

Add an administrator-only endpoint:

```text
POST /api/user/affiliate/transfer-requests/approve-all
```

The model operation locks and loads all currently pending requests in a single
database transaction, ordered by request ID for deterministic processing. It
validates every request and applies the existing invitation-reward deduction,
user balance credit, and request status transition for each item. If any item
fails, the transaction returns an error and no request or balance change is
committed. An empty pending set succeeds with `approved_count: 0`.

The single-request approval endpoint and the batch endpoint share one
transactional approval operation so their financial invariants cannot drift.
After commit, the model updates each affected user's quota cache. Cache update
failures are logged and do not roll back committed database state, matching the
existing single-approval behavior.

The controller returns:

```json
{
  "success": true,
  "data": { "approved_count": 3 }
}
```

For every committed request, the controller writes the existing administrator
approval audit and the existing user-visible management log. Logs are never
written for a rolled-back batch.

### Administrator UI

The rebate approvals toolbar shows an `Approve All` button in the action area.
A small pending-count query is independent of the current status filter and
page, so the button always represents every pending request across pagination.
The button is disabled when the count is zero or a batch is running.

Clicking opens a destructive-impact confirmation dialog that states the exact
pending count. Confirming calls the batch endpoint once. Success displays the
approved count and invalidates both the list and pending-count queries. Failure
keeps the dialog state recoverable and shows the server error without claiming
partial success.

## 2. Permanent Distillation Violation History

### Data model

Add `distillation_violation_records` with these durable fields:

- `id`
- `user_id`, indexed for self-history queries
- `cycle_started_at`, identifying the first trigger of the penalty cycle
- `triggered_at`
- `request_count`
- `detection_threshold`
- `penalty_rpm`
- `action`, either `temporary_limit` or `permanent_ban`
- `effective_until`, set for a temporary limit and zero for a permanent ban
- `created_at`

A unique index over `user_id`, `cycle_started_at`, and `action` prevents
duplicate history rows when concurrent requests observe the same crossing.
The history row is inserted in the same transaction as the corresponding
`distillation_penalties` transition. A first strike creates one
`temporary_limit` row; a second strike in the observation period creates one
`permanent_ban` row. Requests that merely observe an already-active state do
not create records.

Clearing an active penalty deletes runtime and active penalty state but does
not delete violation history. Expired observations may continue to remove the
active penalty row without affecting history.

### Self-service API and page

Add an authenticated self endpoint:

```text
GET /api/user/distillation/violations/self?p=1&page_size=20
```

The user ID comes only from the authenticated context. The endpoint never
accepts another user ID and returns records newest first through the standard
paginated response.

Add `/violations` under the Personal sidebar group after Wallet and Profile.
The page title is `Violation Records`. Its table shows:

- detection time;
- request count and the snapshotted threshold;
- action taken;
- effective-until time for a temporary limit, or `Permanent` for a permanent
  ban.

The empty state explicitly states that no distillation violations were found.
All text uses the existing frontend i18n system in every supported locale.

## 3. Fixed Natural-Minute Counters With Single Crossing

### Counter contract

Both distillation detection and temporary penalty RPM enforcement use fixed
natural-minute buckets. A bucket is identified by the Unix minute containing
the request. The counter operation atomically returns:

- whether the request is within the configured maximum;
- the current count;
- whether this request is the one request that first reached the threshold.

The threshold request remains allowed, preserving current behavior. Only the
request with `crossed == true` advances durable penalty state and writes a
violation record. Requests beyond the threshold do not repeat that database
transition. They immediately use the temporary penalty RPM path while the
crossing request finishes. If the durable transition fails, the current
detection bucket is deleted so a later request can retry instead of leaving an
unrecorded penalty state.

Sequentially, a second-strike crossing commits the permanent ban before its
threshold request returns, so the next request receives the permanent-ban
response as it does today.

### Redis implementation

Redis uses one scalar key per user, purpose, and Unix minute. One Lua script
per request atomically increments the count, assigns an expiry on the first
increment, and reports the limit and first-crossing result. Keys live long
enough to cover the active minute and then expire automatically. No sorted
sets, per-request members, sequence keys, scans, or background jobs are used.

This changes Redis storage from O(request volume) to O(active users) and each
check from sorted-set maintenance to constant-time scalar operations. The
minute suffix makes buckets align across application instances.

### In-memory implementation

The fallback uses scalar counts keyed by the same purpose, user, and Unix
minute. Counters are partitioned so unrelated users do not contend on one
global timestamp queue. Expired buckets are removed outside the request hot
path. It stores no per-request timestamps and returns the same count and
crossing semantics as Redis.

Administrator penalty clearing removes the current detection and temporary
RPM buckets as well as active penalty state, while historical violation rows
remain intact.

## 4. API Key Notice Readability

The existing notice position, width, wrapping, and bounded scrolling remain
unchanged. The title and body change from `text-xs` to `text-sm` (14px), with a
compact line height. This improves readability without allowing long notices
to expand the API key toolbar indefinitely.

## Error Handling and Concurrency

- Batch approval is all-or-nothing. The response never reports a partial
  count.
- Row locks use the project's `lockForUpdate` helper, which skips unsupported
  SQLite locking syntax and emits the correct locking clause for MySQL and
  PostgreSQL.
- Conditional status updates remain the final defense against concurrent
  single approvals or rejections.
- The fixed-window counter is atomic in Redis and in memory.
- Only a real penalty transition inserts a history row; the unique transition
  index provides a database-level duplicate guard.
- Runtime-store failures continue to fail closed with the existing
  distillation storage error.
- A failed history insert rolls back its penalty transition so active state
  and visible history cannot disagree.

## Testing

Implementation follows test-driven development.

Backend coverage includes:

- successful multi-request batch approval;
- no-op approval with no pending requests;
- full rollback when any request is invalid;
- racing single and batch decisions without double credit;
- cache updates only after commit;
- one first-strike and one second-strike history row;
- no duplicate rows under concurrent crossings;
- history persistence after active-state clearing and observation expiry;
- self-history isolation and pagination;
- fixed-minute count, first-crossing, natural-minute reset, and concurrent
  atomicity for Redis and in-memory stores;
- identical service behavior at and after the threshold.

Frontend coverage includes:

- pending count and disabled batch button state;
- confirmation count, single API call, loading state, success invalidation,
  and failure recovery;
- violation table rows, empty state, pagination, action labels, and timestamps;
- Personal sidebar placement and route access;
- 14px API key notice title and body.

Verification includes affected Go test packages, frontend unit tests,
type-checking, linting of changed files, i18n synchronization for all seven
locales, production build, and browser checks of the administrator and user
pages at desktop and mobile widths.
