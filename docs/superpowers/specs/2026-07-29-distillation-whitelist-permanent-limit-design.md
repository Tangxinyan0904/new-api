# Distillation Whitelist and Permanent Rate Limit Design

Date: 2026-07-29

## Background

The current distillation protection counts validated non-stream model requests
in a rolling one-minute window. A first threshold crossing applies a temporary
non-stream RPM limit. A second threshold crossing during the observation period
permanently bans model API access until an administrator clears the penalty.

Administrators need two additional controls:

1. A user whitelist that completely exempts selected users from distillation
   detection and enforcement without changing ordinary model-request limits.
2. A global second-strike action that can permanently rate-limit non-stream
   requests instead of permanently banning model API access.

The whitelist user search must use the same endpoint, query behavior, and user
display format as the existing user-specific rate-limit editor.

## Goals

1. Let administrators search for users by username or numeric user ID and add
   them to a distillation whitelist.
2. Clear all existing distillation state when a user enters or leaves the
   whitelist so removal always resumes detection from a clean state.
3. Let administrators choose one global second-strike action: permanent ban or
   permanent non-stream rate limit.
4. Store the action and RPM selected at trigger time so later setting changes do
   not reinterpret existing permanent penalties.
5. Keep permanent penalties effective when distillation detection is disabled
   and across process restarts.
6. Preserve existing permanent-ban records and support SQLite, MySQL, and
   PostgreSQL migrations.

## Confirmed Decisions

- The whitelist is global and contains stable numeric user IDs.
- Adding a user to the whitelist clears temporary punishment, observation,
  permanent ban, permanent limit, and all related runtime counters.
- Removing a user from the whitelist also clears related state and resumes
  detection from a clean state.
- Whitelisted users still use ordinary global, group, and user-specific model
  request rate limits.
- The second-strike action is one global setting, not a per-user setting.
- Permanent limiting reuses the configured penalty RPM at the moment of the
  second trigger.
- The persisted permanent RPM does not change when the administrator later
  edits the global penalty RPM.
- Permanent limiting applies only to non-stream requests. Streaming requests
  remain unaffected by that state.
- A previously triggered permanent state retains its original action when the
  global second-strike action changes.
- Existing permanent bans remain permanent bans after migration.
- The request that reaches the first or second detection threshold continues;
  the new penalty starts with the next request.
- Permanent bans and permanent limits remain enforced when distillation
  detection is disabled and can only be removed by an administrator clear or
  by a whitelist membership change.

## Configuration

Add these system options:

- `ModelRequestRateLimitDistillationWhitelist`
- `ModelRequestRateLimitDistillationSecondStrikeAction`

The whitelist is serialized as a JSON array of positive numeric user IDs:

```json
[123, 456]
```

Parsing accepts duplicate IDs but normalizes them into an ascending, unique
list. Non-integers, zero, negative values, values above `math.MaxInt32`,
non-array JSON, and malformed JSON are rejected. The setting package
keeps an ID set behind the existing model-request rate-limit mutex so request
checks are constant time and race-safe. JSON serialization and parsing use the
project `common` wrappers.

The second-strike action accepts exactly:

- `ban`
- `permanent_limit`

Its default is `ban`, preserving current behavior when upgrading an existing
installation.

The existing bulk rate-limit settings endpoint validates and persists both
options with the rest of the rate-limit form. A shared rate-limit settings
service owns whitelist membership changes and their state cleanup. Every write
path, including generic option updates, must call that service or reject the
membership change; no path may update the whitelist without performing the
same cleanup invariant.

## Durable Penalty Model

Keep `PermanentlyBannedAt` unchanged and add two zero-compatible columns to
`DistillationPenalty`:

- `PermanentlyLimitedAt int64`
- `PermanentLimitRPM int`

The existing `permanent` phase continues to represent a permanent ban so API
consumers and existing rows remain compatible. Add a `permanent_limited` phase
for a permanent non-stream rate limit.

Phase precedence is:

1. `permanent` when `PermanentlyBannedAt` is positive.
2. `permanent_limited` when `PermanentlyLimitedAt` and `PermanentLimitRPM` are
   positive.
3. `temporary` while the temporary deadline is active.
4. `observation` while the observation deadline is active.
5. `clean` otherwise.

A record with a positive permanent-limit timestamp and a non-positive RPM is
invalid and must fail closed rather than silently bypass enforcement.

On the second threshold crossing, the transactional state transition receives
the validated action and current penalty RPM. It writes exactly one permanent
state:

- `ban` sets `PermanentlyBannedAt`.
- `permanent_limit` sets `PermanentlyLimitedAt` and snapshots
  `PermanentLimitRPM`.

Concurrent second crossings may produce only one durable transition. Existing
row-locking and unique-conflict handling remain in place and continue to use
`lockForUpdate` for cross-database compatibility.

GORM auto-migration adds the two columns with zero values. No existing row
rewrite is required, and no dialect-specific column type or default is used.

## Request Enforcement

Distillation enforcement runs in this order after relay request validation and
stream detection:

1. Validate the authenticated user ID.
2. Read the in-memory settings snapshot.
3. If the user is whitelisted, return immediately without consulting the
   penalty database, cache, Redis, or in-memory distillation counters.
4. Load the durable penalty state.
5. Reject a permanent ban with the existing OpenAI-compatible HTTP 403 error.
6. For a permanent limit:
   - allow streaming requests without consuming its bucket;
   - apply the snapshotted RPM to non-stream requests through a dedicated
     rolling one-minute bucket;
   - return an OpenAI-compatible HTTP 429 response when that bucket is full.
7. For all non-permanent states, retain the existing stream, feature-switch,
   temporary-punishment, detection, and transition behavior.

The permanent bucket uses a distinct runtime key from the temporary bucket.
Redis remains the runtime backend when configured; otherwise the existing
in-memory limiter is used. Runtime bucket loss can reset only the current
one-minute count. The permanent state and RPM are restored from the database.

The distillation enabled switch controls new detection and temporary
punishment exactly as it does today. It does not bypass either permanent state.

Ordinary request limiting remains an earlier, independent middleware layer.
Whitelist membership never bypasses global, group, or user-specific ordinary
limits.

## Whitelist State Cleanup

When saving the rate-limit form, the controller compares the old and new
whitelists. Every user in the membership symmetric difference is reset before
the new options are persisted. Resetting a user deletes:

- the durable penalty row;
- the hybrid penalty cache entry;
- the detection counter;
- the temporary RPM bucket;
- the permanent RPM bucket.

Clearing both newly added and newly removed users guarantees that removal
always starts clean, including after an earlier partial runtime cleanup.

If any cleanup fails, the settings update stops and returns an error. Users
already cleared remain clean; this is safer than retaining or restoring a
hidden permanent penalty. Each successfully cleared user receives an
administrator audit record, including when a later cleanup or option write
fails. The normal rate-limit settings update audit remains unchanged.

The explicit administrator clear endpoint remains idempotent and performs the
same complete cleanup. It works for temporary, observation, permanently banned,
and permanently limited states.

## Administrator API and UI

The existing `PUT /api/rate-limit` request and form values add:

- `ModelRequestRateLimitDistillationWhitelist` as a JSON string;
- `ModelRequestRateLimitDistillationSecondStrikeAction` as an enum string.

No new whitelist endpoint is needed. The editor calls the existing
`/api/user/search` route through the same `searchRateLimitUsers` frontend
function used by the user-specific rate-limit editor. It uses the same
debounced username or user-ID search, result mapping, and user label format.

The distillation settings panel adds:

1. A segmented action control labeled for permanent ban and permanent limit.
2. A whitelist combobox followed by removable selected-user rows.
3. Supporting text explaining that permanent limiting snapshots the current
   penalty RPM at the second trigger.

Duplicate users are filtered from search results. Persisted IDs without loaded
user metadata use the existing `User #<id>` fallback.

The active-penalties response adds:

- phase `permanent_limited`;
- `permanently_limited_at`;
- `permanent_limit_rpm`.

The penalties table renders a distinct permanent-limit status and its RPM. Its
existing clear confirmation and action apply to both permanent outcomes.

All new visible text uses literal `t('English source')` keys and is translated
for every supported frontend locale through the project i18n workflow.

## Errors and Fail-Closed Behavior

- A malformed whitelist or unsupported action is rejected before persistence.
- A configured Redis command failure rejects the affected request rather than
  bypassing either temporary or permanent rate limiting.
- A database or cache failure while resolving a non-whitelisted user's durable
  state rejects the request.
- An invalid permanent-limit row rejects the request with a storage error and
  is visible to administrators; it does not become unlimited access.
- Permanent ban responses remain HTTP 403.
- Temporary RPM exhaustion retains `distillation_rate_limited`. Permanent RPM
  exhaustion returns HTTP 429 with the distinct stable error code
  `distillation_permanent_rate_limited`.
- Administrator clear remains idempotent for missing and expired records.

## Testing

Backend tests protect these contracts:

- whitelist JSON parsing, normalization, validation, and concurrent reads;
- whitelist precedence over temporary, observation, permanent-ban, and
  permanent-limit states;
- whitelisted users remain subject to ordinary request limits;
- membership changes clear durable state, cache, and all runtime keys;
- removal resumes detection from a clean state;
- `ban` preserves current second-strike behavior;
- `permanent_limit` snapshots the current RPM and starts after the threshold
  request;
- later action or RPM setting changes do not alter existing permanent states;
- permanent limiting affects non-stream requests only;
- permanent states remain active while detection is disabled;
- concurrent second crossings write one permanent outcome;
- invalid persisted permanent-limit state fails closed;
- administrator listing and clear support both permanent outcomes;
- schema migration and model behavior work through the project's SQLite,
  MySQL, and PostgreSQL paths without dialect-specific SQL.

Frontend tests protect:

- the whitelist uses the existing debounced user search contract;
- duplicate selection prevention, serialization, hydration fallback, and
  removal;
- action selection serialization and validation;
- permanent-limit explanatory copy and penalty RPM display;
- permanent-limit phase rendering and clear behavior;
- all locale files remain complete and synchronized.

Verification includes focused Go tests, affected package tests, `go test ./...`,
frontend component tests, `bun run typecheck`, lint on every changed frontend
file, i18n checks, `bun run build`, and browser checks at desktop and narrow
mobile widths.

## Non-Goals

- Per-user second-strike actions or per-user permanent RPM configuration.
- Whitelisting by group, token, model, or channel.
- Applying permanent distillation RPM limits to streaming requests.
- Reinterpreting existing permanent states when global settings change.
- Bypassing ordinary model-request, web-route, or API-route rate limits.

## Acceptance Criteria

1. An administrator can find users with the existing user-rate search behavior,
   add them to the whitelist, and remove them without duplicate IDs.
2. A whitelisted user bypasses every distillation state while ordinary rate
   limits continue to apply.
3. Whitelist membership changes clear all durable and runtime distillation
   state, and removal starts detection from a clean state.
4. A second observation-period trigger uses the configured global action and
   preserves the threshold request.
5. A permanent limit stores the trigger-time RPM, affects only non-stream
   requests, survives detection disable and restarts, and requires an
   administrator clear or whitelist change to remove.
6. Existing permanent bans remain bans after deployment.
7. Administrators can distinguish, inspect, and clear both permanent outcomes.
8. Backend behavior, migrations, frontend validation, responsive UI, and i18n
   pass the required verification across supported environments.
