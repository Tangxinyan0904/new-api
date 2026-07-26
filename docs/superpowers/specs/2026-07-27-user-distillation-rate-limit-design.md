# User Rate Limits and Distillation Detection Design

Date: 2026-07-27

## Background

Model-request rate limiting currently runs after token authentication for the
protected model routes. The configured limits have three global values: an
enabled flag, a period in minutes, and total/successful request limits. A JSON
map can override the two request limits by group.

Although configuration is group-based, the Redis and in-memory counter keys
already include the authenticated user ID. The missing capability is therefore
policy selection for a specific user, not per-user counter isolation.

The existing middleware runs before relay request parsing. At that point it
cannot reliably distinguish streaming from non-streaming requests. The stream
flag is established only after request validation and relay-info generation, so
distillation detection must run later in the relay flow.

The current Redis and in-memory implementations also disagree on successful
request counting. Redis records only successful downstream responses, while the
in-memory check key currently advances for every request. This discrepancy must
be corrected while the policy resolver is extended.

## Goals

1. Allow an administrator to configure model-request limits for selected users.
2. Resolve ordinary limits in the order user override, group override, global
   default.
3. Detect a configurable burst of validated non-stream requests within one
   minute and temporarily reduce that user's non-stream request rate.
4. Treat a second trigger during a configurable observation period as a
   permanent model-API ban that only an administrator can clear.
5. Keep permanent bans durable across restarts while retaining the current
   Redis/in-memory runtime counter architecture.
6. Provide an administrator-facing list of active distillation penalties and a
   safe clear action.
7. Preserve SQLite, MySQL, and PostgreSQL support.

## Confirmed Decisions

- User rules are selected by searching username or user ID, but are stored by
  stable numeric user ID.
- A user rule replaces the matching group/global ordinary rule. It does not run
  as a second independent ordinary limiter.
- A user rule has the same two limits as a group rule: total requests and
  successful requests. It shares the global rate-limit period.
- Distillation detection counts every validated non-stream request that is
  ready to be sent upstream. An upstream failure still counts.
- Streaming requests neither contribute to distillation detection nor consume
  the temporary distillation RPM allowance.
- The request that reaches the detection threshold is allowed to continue. A
  resulting temporary limit or permanent ban starts with the next request.
- A temporary distillation limit is additional to the ordinary limit, so the
  more restrictive effective result wins for non-stream requests.
- Temporary punishment duration is administrator-configurable.
- Detection is paused during temporary punishment.
- The first-strike observation clock starts when temporary punishment ends.
- A second trigger during observation permanently bans model API requests.
- If observation expires without a second trigger, the first strike is cleared.
- A permanent ban does not disable the user account. The user can still sign in
  to the console.
- Runtime counters use Redis when configured and in-memory storage otherwise.
  Counter loss on a restart without Redis is acceptable.
- Turning off distillation detection stops new detection and stops enforcing a
  temporary RPM limit. Permanent bans remain effective until explicitly
  cleared. Existing timestamps continue to elapse while detection is off.
- Administrators view and clear penalties from the Rate Limiting settings page.
- Clearing a penalty also clears the strike count and all related runtime keys.

## Non-Goals

- Per-model, per-token, or per-channel user-rate policies.
- Distillation detection for streaming requests.
- Disabling console login or changing the user's normal enabled/disabled status.
- A permanent historical event ledger. The administrator list represents
  current actionable penalty state.
- Retrofitting the generic web/API route throttles configured by environment
  variables.
- Changing the meaning of the existing global or group rate-limit period.

## Configuration Design

### User Overrides

Add a system option named `ModelRequestRateLimitUser`. Its serialized form is a
JSON object keyed by positive decimal user ID:

```json
{
  "123": [200, 100],
  "456": [0, 1000]
}
```

The first value is the total request limit and may be zero for unlimited. The
second value is the successful request limit and must be at least one. Both are
bounded by `math.MaxInt32`, matching existing group validation.

The frontend owns username lookup and display. The backend policy resolver uses
only the authenticated numeric user ID. A rule for a deleted user is harmless
and can still be removed from the editor; it never applies to another user.

All serialization and parsing in the setting package must use `common.Marshal`
and `common.Unmarshal`. Updating either the group or user map requires the write
lock. Read access returns copied values while holding the read lock.

### Distillation Options

Add these system options:

- `ModelRequestRateLimitDistillationEnabled`
- `ModelRequestRateLimitDistillationThreshold`
- `ModelRequestRateLimitDistillationRPM`
- `ModelRequestRateLimitDistillationPenaltyMinutes`
- `ModelRequestRateLimitDistillationObservationMinutes`

The feature defaults to disabled. Numeric options may remain zero while it is
disabled. Enabling or saving an enabled configuration requires all four numeric
values to be positive integers and requires the punishment RPM to be lower than
the detection threshold.

The detection interval is always one minute. Punishment and observation values
are entered and stored as minutes to match the existing rate-limit page.

## Ordinary Rate-Limit Resolution

The existing `ModelRequestRateLimitEnabled` switch continues to control the
global, group, and user ordinary limit layers together. When that switch is on,
every authenticated model request resolves its limits as follows:

1. Start with the global total and successful limits.
2. If the request's token/user group has an override, replace both limits with
   that group rule.
3. If the authenticated user ID has an override, replace both limits with that
   user rule.
4. Execute exactly one ordinary total limiter and one ordinary successful
   request limiter using the selected pair.

Counter keys remain scoped by user ID. Changing the selected policy does not
merge old group and user limits. Existing counter state may remain until its
normal expiry, but the new maximum is used immediately.

The Redis and memory implementations must expose the same observable successful
request behavior: previous successful responses are checked before dispatch,
and a response with status below 400 is recorded after dispatch. Failed
responses do not advance the successful counter in either backend.

The distillation switch and punishment state are independent of this ordinary
master switch. Disabling ordinary rate limiting does not disable enabled
distillation detection, temporary punishment, or permanent-ban enforcement.

## Distillation State Machine

The durable state for one user has four derived phases:

| Phase | Condition | Behavior |
| --- | --- | --- |
| Clean | No active record, or observation has expired | Detection is active when enabled |
| Temporary | Current time is before `temporary_limited_until` | Apply extra non-stream RPM; pause detection |
| Observation | Temporary limit ended and observation has not expired | Detection is active; next trigger is the second strike |
| Permanently banned | `permanently_banned_at` is positive | Reject all model API requests until administrator clear |

Transitions are:

1. Clean plus threshold reached becomes Temporary.
2. Temporary becomes Observation when its timestamp elapses.
3. Observation plus threshold reached becomes Permanently banned.
4. Observation becomes Clean when its timestamp elapses without a trigger.
5. Administrator clear moves any phase to Clean.

The threshold-reaching request is allowed for both the first and second
transition. Runtime enforcement observes the newly written state on the next
request.

## Relay Request Flow

Permanent-ban enforcement belongs in the authenticated model-request path and
does not depend on the ordinary rate-limit enabled flag or the distillation
detection enabled flag. A permanently banned user is rejected before upstream
work or billing begins.

For a request that passes the permanent-ban and ordinary-limit checks:

1. Parse and validate the relay request using the existing format-specific
   validation path.
2. Generate relay information and establish the stream flag.
3. Skip distillation handling for streaming requests.
4. If detection is disabled, skip temporary enforcement and detection. Do not
   delete a durable first-strike or permanent-ban record.
5. Load the current durable penalty state through its cache.
6. If temporary punishment is active, apply the additional `Y RPM` limiter and
   skip detection.
7. Otherwise, record this validated non-stream request in the one-minute
   detection counter.
8. When that request first reaches `X`, execute the corresponding durable state
   transition in a transaction, clear/reset relevant runtime keys, and allow the
   current request to continue.
9. Continue into existing billing, channel selection, and upstream relay logic.

Because detection occurs before the upstream call, upstream successes and
failures both count. Authentication failures and request-validation failures do
not count.

Detection uses the same continuous-window semantics as the existing request
limiter rather than a wall-clock minute boundary. Redis threshold crossing must
be atomic; the in-memory implementation must perform the equivalent operation
under a lock. Concurrent requests may produce only one first- or second-strike
transition.

## Persistence and Cache

Add a dedicated GORM model/table for current distillation penalty state. The
record is unique by user ID and contains:

- primary key;
- unique indexed user ID;
- first-triggered Unix timestamp;
- temporary-limit-until Unix timestamp;
- observation-until Unix timestamp;
- permanently-banned-at Unix timestamp;
- `CreatedAt` and `UpdatedAt` Unix timestamps using GORM's integer auto-create
  and auto-update tags.

No database-specific default or date type is required. A zero Unix timestamp
means the corresponding transition has not occurred. GORM performs the schema
migration on SQLite, MySQL, and PostgreSQL.

State transitions use a transaction. Existing rows are loaded through
`lockForUpdate(tx)`, which emits a row lock only on dialects that support it.
Creation races are resolved through the unique user ID constraint. After a
unique-conflict result, the transaction reloads the winning row once and applies
the transition against that row. A concurrent threshold crossing cannot create
two records or downgrade a permanent ban.

The database is authoritative for first strikes and permanent bans. Redis or an
in-memory cache avoids a database read on every request. Administrator clear and
automatic state transitions invalidate that cache. On a cache miss after a
restart, the database state is reloaded.

Detection counters and temporary RPM buckets are runtime state only. They use
Redis when configured and memory otherwise. Losing those keys resets the current
counter/bucket but does not erase the durable penalty phase or permanent ban.

Loading an expired observation record treats it as Clean. Cleanup uses a
status-guarded delete (`permanently_banned_at = 0` and
`observation_until <= now`) before a new transition is written, so it cannot
delete a concurrently renewed or permanently banned row. The administrator list
excludes expired observation rows even if cleanup has not completed. Expired
rows are not retained as an audit history.

## Administrator API

Add administrator-only endpoints under the existing API admin routing group:

- `GET /api/rate-limit/distillation/penalties`
- `DELETE /api/rate-limit/distillation/penalties/:user_id`

The list endpoint supports the project's normal page/page-size parameters and
an optional username or numeric user-ID search. It returns only current
actionable rows, newest first, with:

- user ID and username;
- derived phase;
- first-triggered time;
- temporary-limit-until time;
- observation-until time;
- permanently-banned-at time.

The clear endpoint is idempotent. It deletes the durable state if present,
clears all distillation cache/counter/bucket keys for the user, and returns
success even if the record has already expired or been removed. The action is
recorded through the existing administrator audit mechanism without exposing
sensitive token data.

## Rate-Limiting Page

Keep the current global fields and group editor. Add three focused components so
the existing section does not grow into one large form:

1. A user-rate editor with an administrator user search, selected-user rows,
   total/successful numeric inputs, and a remove action.
2. A distillation settings block with a switch and the four numeric fields. The
   fields are visible when enabled and show explicit request/RPM/minute units.
3. A current-penalties table with username/ID search, pagination, a manual
   refresh icon button, phase labels, relevant timestamps, and a clear action.

The user-rate editor serializes the ID-keyed map into the existing system-option
form submission. The penalty table uses React Query independently of the
settings form, so refreshing or clearing a penalty cannot discard unsaved rate
configuration.

Clearing a penalty requires a confirmation dialog naming the affected user.
After success, invalidate the penalty-list query and show the existing localized
success-toast pattern.

All new user-facing strings must use literal `t('English source')` keys and be
translated in all supported locales through the project's i18n synchronization
workflow.

## Error Handling

- Invalid enabled distillation settings or malformed user-limit JSON are
  rejected by the option update endpoint and shown as field/form errors.
- A temporary distillation limit returns an OpenAI-compatible HTTP 429 response.
- A permanent distillation ban returns an OpenAI-compatible HTTP 403 response
  stating that model API access is suspended and an administrator must clear it.
- A configured Redis command failure follows the existing conservative limiter
  behavior: log the error and reject the request rather than silently bypassing
  enforcement. In-memory fallback is selected when Redis is not configured, not
  after an arbitrary Redis runtime failure.
- A database/cache-miss failure while determining durable ban state rejects the
  request and records a server error so a persistent ban cannot be bypassed by a
  storage outage.
- Clearing a missing/expired penalty succeeds idempotently.

## Testing and Verification

Backend tests protect these observable contracts:

- user rules override group and global pairs;
- group rules still override global pairs when no user rule exists;
- malformed IDs/limits are rejected and setting-map updates are race-safe;
- Redis and memory backends count successful requests identically;
- only validated non-stream requests contribute to detection;
- upstream failures still count, while validation failures and streams do not;
- the threshold request passes and the following request sees temporary
  enforcement;
- detection is paused during temporary punishment;
- the observation timer begins after temporary punishment;
- observation expiry clears the first strike;
- a second observation-period trigger writes one permanent ban under concurrent
  requests;
- permanent bans survive feature disable and process/cache restart;
- disabling detection bypasses temporary limits and new detection;
- administrator clear is authorized, idempotent, resets state, and invalidates
  runtime keys;
- permanent bans affect model API access but not normal console sign-in;
- the new migration and model behavior avoid dialect-specific SQL and work with
  the project's SQLite, MySQL, and PostgreSQL paths.

New or substantially changed Go tests use `testify/require` for fatal/setup
assertions and `testify/assert` for non-fatal comparisons.

Frontend verification covers:

- searching/selecting a user and serializing a stable ID-keyed rule;
- adding, editing, and removing user overrides;
- enabled/disabled validation and the `Y < X` invariant;
- penalty loading, empty, error, pagination, refresh, confirmation, and clear
  states;
- responsive layout without clipped translated labels.

Run focused Go tests, broader affected-package tests, frontend type checking,
lint on every changed frontend file, i18n checks, the production frontend build,
and browser verification at desktop and narrow mobile viewports.

## Acceptance Criteria

1. An administrator can assign a rate pair to a searched user, and it takes
   precedence over that user's group/global pair.
2. A configured number of validated non-stream requests triggers temporary
   non-stream RPM punishment without blocking the threshold request.
3. A second trigger during observation permanently blocks only model API access.
4. A clean observation period resets the first strike.
5. Permanent bans remain in force after disabling detection or restarting the
   service and are removed only by the administrator clear action.
6. Streaming requests remain unaffected by detection and temporary punishment.
7. Redis and memory ordinary success counters produce the same behavior.
8. The Rate Limiting page can configure the feature and manage current penalty
   state without database-dialect-specific behavior.
