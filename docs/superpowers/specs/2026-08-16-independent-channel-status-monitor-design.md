# Independent Channel Status Monitor Design

Date: 2026-08-16

## Context

Sub2API provides an administrator-managed active channel monitor and a
read-only user status view. Each monitor owns an upstream endpoint, credential,
protocol, primary model, optional extra models, schedule, and history. The
user-facing page exposes availability and latency without exposing credentials
or raw upstream errors.

New API already supports manual and scheduled tests for configured routing
channels, and stores the most recent test time and response time on each
channel. Those tests are coupled to real routing channels, do not keep the
history required for availability windows, and do not provide a public or
user-facing channel-status view.

This feature adds an independent active-monitoring subsystem. It does not
reuse a routing channel's credential or lifecycle and does not affect billing,
ordinary usage logs, or channel enable/disable state.

## Goals

1. Let administrators create independent health monitors for selected
   upstream endpoints and models.
2. Support OpenAI Chat Completions, OpenAI Responses, Anthropic Messages, and
   Gemini generateContent probes.
3. Publish a compact status summary above the model-square search controls.
4. Add a signed-in channel-status page with monitor timelines and 1/3/7-day
   availability.
5. Keep raw credentials, endpoint details, HTTP status codes, and raw error
   messages administrator-only.
6. Avoid duplicate probes in multi-instance deployments through database
   leases and bounded concurrency.
7. Bound database growth while preserving useful recent detail and seven days
   of availability.
8. Work with SQLite, MySQL, and PostgreSQL.

## Non-Goals

- Replacing the existing routing-channel test action or scheduled channel
  recovery behavior.
- Automatically creating a monitor for every existing channel.
- Charging monitor requests to a user, subscription, or balance.
- Writing monitor probes to normal usage logs.
- Disabling or enabling a real routing channel from monitor results.
- Supporting every New API relay channel type in the first version.
- Publicly exposing endpoint URLs, credentials, internal monitor names,
  internal groups, raw responses, or raw errors.
- Providing a public unauthenticated full-detail page. The model square exposes
  only a compact summary; full details require sign-in.

## Confirmed Product Decisions

- Administrators configure independent monitor records rather than selecting
  existing routing channels.
- A monitor has one primary model and optional extra models.
- A monitor may contain at most 20 models in total. Larger sets must be split
  into multiple monitors so one lease cannot monopolize a worker indefinitely.
- Each monitor independently configures its interval, degraded-latency
  threshold, and request timeout.
- The default interval is 60 seconds. The allowed range is 60 seconds through
  60 minutes.
- The default degraded threshold is 6 seconds and the default timeout is 30
  seconds.
- Administrators enter a separate public display name. Internal names,
  endpoints, credentials, and groups are never returned by public/user APIs.
- The channel-management page links to a dedicated monitor-management page.
- Signed-in users receive a Channel Status sidebar entry directly below
  Dashboard.
- The model-square heading, enabled-model count sentence, and descriptive
  paragraph are removed. A compact public status summary occupies that area,
  immediately above the existing search controls.
- The public summary shows overall status, last update time, status counts, and
  per-monitor public name, provider, primary model, latest latency, status, and
  current-day availability.
- The signed-in page shows 1/3/7-day availability, recent timeline data, and
  per-model details.
- Public/user failures expose only a sanitized category. Administrators can
  inspect HTTP status and a bounded raw error message.
- Raw check rows are retained for one hour. Hourly rollups cover data older
  than one hour through 24 hours. Daily rollups cover data older than 24 hours
  through seven days.
- Availability windows use natural days. One day means today's data; three
  days means today plus the previous two complete natural days; seven days
  means today plus the previous six complete natural days.

## Data Model

All monitor tables live in the main database. Time values use Unix seconds and
structured fields use TEXT plus `common.Marshal` and `common.Unmarshal` so the
schema remains portable across all supported databases.

### ChannelMonitor

Add `channel_monitors` with these durable fields:

- `id`
- `internal_name`
- `public_name`
- `provider`: `openai`, `anthropic`, or `gemini`
- `api_mode`: `chat_completions`, `responses`, `messages`, or
  `generate_content`
- `endpoint`: normalized base URL, administrator-only
- `api_key_encrypted`: AES-GCM ciphertext, never serialized directly
- `primary_model`
- `extra_models`: ordered JSON array stored as TEXT
- `interval_seconds`
- `degraded_threshold_ms`
- `timeout_seconds`
- `enabled`
- `public_enabled`
- `last_checked_at`
- `next_check_at`
- `schedule_blocked_reason`
- `lease_owner`
- `lease_until`
- `created_by`
- `created_at`
- `updated_at`

Indexes support due-monitor scans, public lists, and administrator filtering:

- `(enabled, next_check_at, lease_until)`
- `(public_enabled, enabled)`
- `provider`

`public_enabled` defaults to false in service normalization. A newly created
monitor is not exposed until an administrator explicitly enables publication.
Changing it to false removes the monitor from public and user APIs
immediately.

### ChannelMonitorCheck

Add `channel_monitor_checks`, one row per checked model per probe run:

- `id`
- `monitor_id`
- `model`
- `status`: `operational`, `degraded`, or `failed`
- `latency_ms`
- `error_category`
- `http_status`
- `error_message`
- `checked_at`

`error_message` is bounded and administrator-only. Public/user queries select
only safe fields. Indexes cover `(monitor_id, model, checked_at)` and
`checked_at` cleanup.

### ChannelMonitorHourlyRollup

Add `channel_monitor_hourly_rollups`, unique by monitor, model, and natural
hour. It stores:

- total, operational, degraded, and failed counts;
- summed latency and latency sample count;
- bucket start time and computed time.

Hourly rollups are retained until their data is older than 24 hours.

### ChannelMonitorDailyRollup

Add `channel_monitor_daily_rollups`, unique by monitor, model, and natural-day
bucket start. It stores the same weighted counts and latency accumulators as
the hourly table. Daily rollups are retained for seven natural days.

Natural-hour and natural-day boundaries use the deployment's Go process local
timezone (`time.Local`, normally configured through `TZ`). All application
nodes must use the same timezone. Bucket starts are persisted as Unix seconds,
and tests inject a fixed location instead of depending on the test machine.
Changing the deployment timezone changes future bucket boundaries but does
not rewrite historical buckets.

Deleting a monitor deletes its checks and rollups through explicit model-layer
cleanup in one transaction. The implementation does not depend on
dialect-specific cascade behavior.

## Credential Storage

Monitor credentials are encrypted with AES-GCM using key material derived from
the existing `CRYPTO_SECRET`. Each ciphertext includes a random nonce and a
version prefix so the storage format can evolve.

Deployments using channel monitors must keep `CRYPTO_SECRET` stable across
restarts and application nodes. Changing it intentionally requires
administrators to replace stored monitor credentials.

Create and update responses expose only whether a credential is configured and
an optional masked suffix. An empty credential on update means keep the
existing credential. Clearing a credential is not allowed while the monitor is
enabled.

If decryption fails, the scheduler does not send a request. It writes one
failed configuration result for each configured model, sets
`schedule_blocked_reason`, clears `next_check_at` and the active lease, and
leaves the monitor enabled but unscheduled. Supplying a replacement credential
clears the block and schedules an immediate check. Public/user views show the
monitor as failed without revealing the decryption reason.

## Scheduler and Leases

Eligible application nodes run a lightweight due-monitor scanner. The scanner
does not create one timer or goroutine per monitor.

1. Every few seconds, query a bounded batch of enabled, unblocked monitors
   whose `next_check_at` is due and whose lease is absent or expired.
2. Claim each monitor in a short transaction. The claim updates
   `lease_owner` and `lease_until` only if the prior lease is still available.
3. Submit claimed monitors to a fixed worker pool with default concurrency 5.
4. Run all models for one monitor sequentially so one monitor cannot consume
   the entire pool through a large extra-model list.
5. Insert check rows and update `last_checked_at` and `next_check_at` in one
   transaction.
6. Clear the lease after completion. A crashed worker becomes eligible after
   lease expiry.

The lease duration is derived from the monitor timeout, model count, and a
bounded safety margin. Claim/update operations use GORM and conditional
updates. When a transaction requires a row lock, it uses the project's
`lockForUpdate` helper so SQLite does not receive unsupported SQL.

Manual Run Now uses the same lease. If the monitor is already running, the API
returns a conflict instead of starting an overlapping probe. Automatic failure
does not trigger rapid retries; the next run uses the configured interval.

## Probe Execution

The probe service sends a small non-streaming request directly to the configured
endpoint. It does not enter token authentication, channel selection, quota
pre-consumption, settlement, user rate limits, distillation detection, or
ordinary log generation.

Protocol adapters build and parse:

- OpenAI Chat Completions;
- OpenAI Responses;
- Anthropic Messages;
- Gemini generateContent.

Each request asks for a small deterministic challenge response and uses a low
output-token cap. A 2xx status alone is insufficient: the adapter must find
usable text and verify the expected challenge answer. Empty output, malformed
payloads, or mismatched challenge answers are failed checks.

The checker limits response-body reads and never logs credentials or complete
response bodies. Endpoint configuration accepts the same administrator trust
boundary as routing-channel base URLs, but requires a normalized base URL with
no query, fragment, or embedded user information. Plain HTTP remains available
for administrator-controlled internal testing with an explicit warning;
HTTPS is the recommended default.

### Status Classification

- `operational`: valid challenge response at or below the monitor's degraded
  threshold.
- `degraded`: valid challenge response above the degraded threshold but before
  the request timeout.
- `failed`: timeout, connection failure, TLS failure, authentication failure,
  rate limit, upstream non-2xx response, malformed response, empty response, or
  challenge mismatch.

The stored error category is one of:

- `network`
- `tls`
- `auth`
- `rate_limit`
- `timeout`
- `upstream`
- `invalid_response`
- `config`

The public/user payload exposes only the category. HTTP status and a truncated
error message are administrator-only.

`unknown` is a view-only state for a published monitor that has never produced
a result. It is not stored in check rows. Overall status is unknown when every
monitor is unknown, failed when any monitor is failed, degraded when no monitor
is failed but at least one is degraded or unknown, and operational only when
every published monitor is operational.

## Aggregation and Retention

A maintenance worker aggregates and deletes in small batches:

1. Checks older than one hour are grouped into natural-hour rollups, then
   deleted after the rollup commit.
2. Hourly rollups older than 24 hours are grouped into natural-day rollups,
   then deleted after the daily rollup commit.
3. Daily rollups older than seven natural days are deleted.

Unique bucket keys make reprocessing idempotent. The rollup stores counts and
latency sums instead of precomputed percentages so multiple sources can be
combined with correct weighting.

Availability treats both `operational` and `degraded` as available. A window
with no samples returns `null`, not zero percent.

- 1 day combines all raw, hourly, and daily buckets belonging to the current
  natural day.
- 3 days combines buckets belonging to the current natural day and the
  previous two complete natural days.
- 7 days combines buckets belonging to the current natural day and the
  previous six complete natural days.

Each source covers a mutually exclusive age tier, so a natural day that is
partly hourly and partly daily is combined without double counting. This keeps
the natural-day result exact even while an older hourly bucket is incrementally
rolled into its daily bucket.

The recent timeline returns at most 60 primary-model check points from the last
hour, newest first. The frontend reverses them for left-to-right display and
pads missing points without inventing failures.

## API Design

### Administrator APIs

Add administrator-authorized endpoints under `/api/channel-monitor`:

- `GET /api/channel-monitor`
- `POST /api/channel-monitor`
- `GET /api/channel-monitor/:id`
- `PUT /api/channel-monitor/:id`
- `DELETE /api/channel-monitor/:id`
- `POST /api/channel-monitor/:id/run`
- `GET /api/channel-monitor/:id/history`

The list supports pagination, provider, enabled/public state, and text search.
Run Now returns the newly committed model results. History supports model,
status/category, and time filters, bounded pagination, and newest-first order.

The endpoints use the existing channel-management authorization boundary.
Credential viewing is never added; authorized users can replace a credential
but cannot retrieve it.

### Public Summary API

Add an anonymous read-only endpoint:

```text
GET /api/channel-monitor/public-summary
```

It returns only enabled, public monitors and includes:

- overall status;
- last data update;
- operational/degraded/failed/unknown counts;
- public name, provider, primary model, latest status, latest latency, current
  natural-day availability, and latest safe check time per monitor.

The response is cached briefly and protected by the existing anonymous API
rate limit. The cache is invalidated after monitor publication changes or new
check commits.

### Signed-In Status APIs

Add authenticated read-only endpoints:

- `GET /api/channel-monitor/status`
- `GET /api/channel-monitor/status/:id`

They return only enabled, public monitors. The list contains primary-model
timeline and summary fields. The detail endpoint contains all configured models
with latest state, 1/3/7-day availability, and weighted average latency. It
does not expose endpoint, credential metadata, internal name, creator, lease,
HTTP status, or raw error message.

## Administrator Interface

The Channels page receives a `Channel Monitor` action beside its existing
primary controls. It navigates to a dedicated administrator page rather than
placing monitor forms inside the channel table.

The monitor table shows:

- internal and public names;
- provider and API mode;
- primary model and total model count;
- latest status and latency;
- last check time and next scheduled time;
- interval;
- enabled and public states;
- edit, Run Now, history, and delete actions.

Create/edit uses a focused drawer with:

- internal name and public name;
- provider and protocol mode;
- endpoint and credential;
- primary model and extra-model editor;
- interval, degraded threshold, and timeout;
- enabled and public switches.

The endpoint and credential fields include clear security guidance. The public
switch states exactly which fields become visible. Deletion requires
confirmation and states that checks and rollups will be removed.

The history dialog shows administrator-only status/category, HTTP status,
bounded message, model, latency, and time. It never offers the stored
credential.

## Signed-In Channel Status Page

Add `/channel-status` and a `Channel Status` item under the General sidebar
group directly below Dashboard. It is available to normal users and
administrators.

The page contains:

- 1/3/7-day segmented controls;
- overall operational/degraded/failed status;
- manual refresh;
- 60-second auto refresh with a visible countdown;
- a responsive monitor grid;
- monitor details on card activation.

Each monitor card displays public name, provider, primary model, latest status,
latest latency, selected-window availability, and up to 60 recent timeline
points. The details view lists primary and extra models with latest state,
1/3/7-day availability, weighted average latency, and sanitized category.

Status always uses text and an icon in addition to color. Light and dark theme
colors must meet the project's contrast expectations. Card dimensions remain
stable across loading, missing history, long model names, and translated text.

## Model Square Summary

Remove the current model-square heading block:

- `Model Square`;
- the enabled-model count sentence;
- the descriptive paragraph.

In the same location, immediately above the existing search controls, render a
compact full-width status region rather than a nested decorative card. It
shows:

- overall state and last update time;
- operational/degraded/failed counts;
- a responsive compact list of public monitor summaries;
- a `View details` command.

The summary is public even when the model-square module itself does not require
authentication. An unauthenticated user who activates View details is sent to
sign-in and then can continue to `/channel-status`; an authenticated user
navigates directly.

Loading uses stable placeholders. With no published monitors, show a compact
localized empty state and leave the search controls and model list usable. A
summary API failure shows an inline unavailable state without blocking model
pricing data.

## Error Handling

- Validation rejects missing names, unsupported provider/mode combinations,
  empty primary models, duplicate extra models, more than 20 total models,
  invalid intervals, invalid threshold/timeout values, and malformed base
  URLs.
- Update operations preserve the stored credential when the credential field
  is omitted or empty.
- A configured monitor with an undecryptable credential is blocked from
  scheduling, receives one failed configuration result, and resumes only after
  a credential replacement clears the block.
- A lease conflict returns a stable conflict response for Run Now.
- Probe failure writes a failed check and schedules the normal next interval;
  it does not retry in a tight loop.
- Rollup writes and source deletion occur transactionally. Failed aggregation
  leaves source rows for a later idempotent retry.
- Public and signed-in APIs never fall back to administrator DTOs.
- Public summary failure never prevents the model square from loading.

## Testing

Implementation follows test-driven development and protects observable
contracts rather than implementation details.

Backend coverage includes:

- GORM model migration and behavior on SQLite plus configured MySQL and
  PostgreSQL test paths;
- credential encryption, masking, keep-on-empty update, and decryption failure;
- provider/mode validation and normalized endpoint behavior;
- exact request construction and response parsing for all four protocol modes;
- valid, degraded, auth, rate-limit, timeout, upstream, empty, malformed, and
  challenge-mismatch classifications;
- response-size and error-message bounds;
- lease claims under concurrent workers, lease expiry recovery, and Run Now
  conflicts;
- no billing, quota, channel-status, or ordinary-log side effects;
- raw-to-hourly and hourly-to-daily idempotent aggregation;
- one-hour, 24-hour, and seven-day retention boundaries;
- natural-day 1/3/7-day weighted availability and no-sample `null` behavior;
- administrator authorization and credential non-disclosure;
- public/user DTO field isolation and publication filtering;
- summary cache invalidation after configuration and result changes.

New or substantially rewritten Go tests use `testify/require` for fatal/setup
assertions and `testify/assert` for non-fatal comparisons.

Frontend coverage includes:

- Channels-page monitor entry and administrator route;
- create/edit validation, masked credential behavior, Run Now, history, and
  delete confirmation;
- sidebar placement and normal-user access;
- model-square heading removal and public summary placement above search;
- public summary loading, empty, partial failure, degraded, and failed states;
- unauthenticated and authenticated View details navigation;
- 1/3/7-day switching, manual refresh, auto-refresh countdown, and cleanup;
- long public/model names, missing history, and fixed layout behavior;
- responsive desktop/mobile presentation and light/dark contrast;
- all new copy through the project's i18n workflow for every supported locale.

Verification includes focused Go packages, broader affected Go tests, frontend
unit tests, type checking, changed-file lint, formatting checks, production
frontend build, production Go build with embedded frontend, and Playwright
screenshots of the administrator page, signed-in status page, and public model
square at desktop and mobile widths.

## Acceptance Criteria

1. An authorized administrator can create an independent monitor with a public
   name, endpoint, encrypted credential, primary model, extra models, schedule,
   degraded threshold, and timeout.
2. Multiple application instances cannot normally probe the same monitor at
   the same time, and an expired lease is recoverable.
3. Four supported protocol modes validate actual response content and classify
   operational, degraded, and failed results.
4. Probe traffic does not charge users, appear in ordinary usage logs, or
   change real routing-channel state.
5. Raw checks are kept for one hour, hourly rollups through 24 hours, and daily
   rollups through seven natural days.
6. Signed-in users can view published monitors, recent timeline data, all
   configured models, and natural-day 1/3/7-day availability without seeing
   sensitive fields.
7. The model square no longer shows its former heading/count/description block
   and instead shows a public compact channel-status summary above search.
8. Public APIs expose only explicitly published monitor data and sanitized
   failure categories.
9. Monitor configuration, aggregation, and queries work on SQLite, MySQL, and
   PostgreSQL.
