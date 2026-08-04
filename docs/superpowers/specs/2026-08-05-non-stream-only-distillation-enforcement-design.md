# Non-Stream-Only Distillation Enforcement

## Context

Distillation protection is intended to detect bursts of non-stream requests.
The current service already excludes stream requests from threshold counting
and temporary RPM enforcement, but it loads the runtime counter store and the
user's durable penalty before applying that exclusion. As a result, a user
with a permanent distillation penalty receives a 403 response for both stream
and non-stream requests.

This change makes the boundary explicit: distillation detection and every
penalty created by it apply only to non-stream requests. Stream requests remain
available even when a user has an existing permanent penalty.

## Goals

- Bypass the complete distillation path for stream requests.
- Avoid runtime-counter, cache, and database work for stream requests.
- Keep the existing fixed-minute threshold and temporary RPM behavior for
  non-stream requests.
- Keep the existing second-trigger permanent punishment for non-stream
  requests.
- Describe permanent punishment accurately in API errors and administrator UI
  text.
- Preserve all existing penalty data without a migration.

## Non-Goals

- Adding an administrator setting or a stream/non-stream mode selector.
- Changing how a request is classified as stream or non-stream.
- Changing thresholds, penalty RPM, penalty duration, observation duration,
  whitelist behavior, or administrator clearing behavior.
- Changing fixed natural-minute counter semantics.
- Removing or rewriting existing penalty or violation-history records.
- Applying distillation logic to individual chunks in a streaming response.

## Request Flow

`controller/relay.go` continues to call `CheckDistillationRateLimit` once per
model request, before the request is relayed upstream. The service handles the
request in this order:

1. Read the already-normalized `RelayInfo.IsStream` value.
2. If the request is stream, return success immediately.
3. For a non-stream request, initialize the configured distillation runtime
   store.
4. Load the user's active penalty state.
5. If the penalty is permanent, reject the non-stream request with HTTP 403.
6. Otherwise preserve the existing enablement, temporary RPM, detection
   threshold, observation, and penalty-transition logic.

The public entry point performs the stream return before
`currentDistillationRuntimeStore`. The internal testable service function also
performs the stream return before validating or reading its runtime and penalty
dependencies. Keeping both guards prevents a future caller from accidentally
reintroducing storage access for streams.

There is no work per server-sent-event chunk. A stream request pays only for
the existing request classification and one boolean branch in this service.

## Non-Stream Behavior

All current non-stream behavior remains unchanged:

- Detection and temporary enforcement use fixed natural-minute counters.
- The request that first reaches the configured threshold is allowed and
  performs one durable state transition.
- Later requests in the active temporary phase use the configured penalty RPM.
- A second threshold transition during the observation period creates the
  permanent penalty.
- The transition request remains allowed; subsequent non-stream requests
  receive the permanent-penalty response.
- Clearing a penalty removes its active state and current runtime counters but
  retains durable violation history.
- A runtime or penalty-store failure for a non-stream request continues to fail
  closed with the existing storage error.

Disabling distillation continues to bypass detection and temporary enforcement
for non-stream requests. An already-created permanent penalty continues to be
enforced for non-stream requests until an administrator clears it, matching the
current policy.

## Existing Data and Compatibility

No model, schema, migration, option, or API response shape changes are needed.
Existing `distillation_penalties` rows and violation-history rows remain valid.
Their meaning is narrowed at enforcement time: they restrict only non-stream
requests. Existing permanent records are not deleted or converted. They remain
effective for non-stream requests after deployment while stream requests
bypass them.

The change is independent of the runtime implementation. Redis and the
in-memory fallback retain identical fixed-minute behavior because neither is
entered for a stream request. Database behavior remains compatible with
SQLite, MySQL, and PostgreSQL because no database query or schema is changed.

Whitelist matching and user-specific rate limits remain separate controls.
This design bypasses only distillation enforcement for streams; other existing
rate limits continue to apply according to their own rules.

## Error and UI Wording

The permanent-penalty backend error explicitly states that non-stream model
API access is permanently suspended after repeated distillation detection and
that the user must contact an administrator to restore it. It must not imply
that streaming access is suspended.

User- and administrator-facing labels use non-stream-specific wording:

- `Permanent ban` becomes `Permanent non-stream ban`.
- `Permanent ban time` becomes `Permanent non-stream ban time`.
- Descriptions that list permanent distillation penalties refer to permanent
  non-stream bans.

The Chinese translation must explicitly say that non-stream requests are
permanently forbidden, and use equivalent wording for the timestamp and
explanatory descriptions. Every supported frontend locale receives the same
semantic clarification through the existing i18n workflow. The penalty phase
value, API fields, badge severity, and clear action remain unchanged.

## Testing

Implementation follows test-driven development. Backend regression coverage
must prove:

- A stream request succeeds when the user has a permanent penalty.
- A stream request does not call the runtime counter store or penalty store.
- A stream request still succeeds when distillation dependencies are
  unavailable, demonstrating that the bypass precedes their initialization.
- A non-stream request with a permanent penalty still returns HTTP 403 and
  `distillation_banned`.
- The permanent error text identifies non-stream access.
- Existing non-stream threshold crossing, temporary RPM enforcement, second
  observation trigger, fixed-minute reset, and transition-failure behavior
  continue to pass unchanged.

Frontend coverage updates the administrator penalty display and personal
violation-history tests to assert the non-stream-specific permanent label,
timestamp heading, and descriptions. Locale synchronization must report no
missing, extra, or untranslated keys in any supported locale.

Verification includes the affected Go service and controller tests, affected
frontend tests, frontend type-checking and production build, i18n
synchronization, formatting, and `git diff --check`.

## Acceptance Criteria

- Stream requests never read or mutate distillation runtime counters,
  penalties, or violation history.
- Stream requests are not rejected by temporary or permanent distillation
  penalties.
- Non-stream requests retain the existing detection and punishment policy.
- Existing permanent penalties remain administrator-clearable and continue to
  block only non-stream requests.
- User UI, administrator UI, and backend errors clearly describe the permanent
  action as a non-stream restriction.
- No new setting, migration, or database-specific behavior is introduced.
