# Independent Channel Status Monitor Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add an independent, multi-instance-safe upstream channel monitor with administrator management, signed-in status details, and a compact public summary above the model-square search controls.

**Architecture:** Store monitor configuration, per-model checks, and hour/day rollups in the main database. A bounded scheduler claims due monitors with conditional database leases, probes each monitor's models sequentially through one of four direct HTTP protocol adapters, and commits results without entering relay billing or logging paths. Dedicated administrator, signed-in, and public DTO/query paths prevent sensitive fields from crossing trust boundaries.

**Tech Stack:** Go 1.22+, Gin, GORM v2, AES-GCM, SQLite/MySQL/PostgreSQL, React 19, TypeScript, TanStack Router, TanStack Query, React Hook Form, Zod, Tailwind CSS, Base UI, Bun, Vitest/React Testing Library, Playwright

## Global Constraints

- Preserve all existing new-api and QuantumNous branding and attribution.
- Support SQLite, MySQL >= 5.7.8, and PostgreSQL >= 9.6 with GORM-portable schemas and queries.
- Use `common.Marshal`, `common.Unmarshal`, `common.UnmarshalJsonStr`, or `common.DecodeJson` for every JSON encode/decode operation.
- Encrypt credentials with AES-GCM key material derived from stable `CRYPTO_SECRET`; never serialize plaintext or ciphertext credentials.
- Keep monitor probes outside relay authentication, channel selection, billing, quota, rate-limit, distillation, ordinary log, and real-channel status paths.
- Support only OpenAI Chat Completions, OpenAI Responses, Anthropic Messages, and Gemini generateContent.
- Allow 1-20 models, 60-3600 second intervals, positive degraded thresholds, and 1-120 second timeouts.
- Retain raw checks for one hour, hourly rollups through 24 hours, and daily rollups through seven natural days in `time.Local`.
- Expose endpoint, internal name, creator, leases, HTTP status, and raw errors only through administrator APIs.
- Route every frontend string through i18next and synchronize `en`, `zh`, `zh-TW`, `fr`, `ru`, `ja`, and `vi`.

---

### Task 1: Portable Models and Credential Encryption

**Files:**
- Create: `model/channel_monitor.go`
- Create: `model/channel_monitor_test.go`
- Modify: `model/main.go`
- Create: `service/channel_monitor_crypto.go`
- Create: `service/channel_monitor_crypto_test.go`

**Interfaces:**
- Produces: `model.ChannelMonitor`, `model.ChannelMonitorCheck`, `model.ChannelMonitorHourlyRollup`, `model.ChannelMonitorDailyRollup`.
- Produces: `service.EncryptChannelMonitorCredential(plaintext string) (ciphertext string, suffix string, err error)`.
- Produces: `service.DecryptChannelMonitorCredential(ciphertext string) (string, error)` and `service.MaskChannelMonitorCredentialSuffix(suffix string) string`.

- [ ] **Step 1: Write failing model and encryption tests**

Create a SQLite database, auto-migrate all four structs, insert one monitor plus raw/hour/day rows, and assert duplicate `(monitor_id, model, bucket_start)` rollups fail. Temporarily set `common.CryptoSecret`, encrypt `sk-monitor-secret`, assert the value starts with `v1:`, plaintext is absent, decryption returns the original, the mask is `****cret`, and changing the secret makes decryption fail.

```go
require.NoError(t, db.AutoMigrate(&ChannelMonitor{}, &ChannelMonitorCheck{}, &ChannelMonitorHourlyRollup{}, &ChannelMonitorDailyRollup{}))
require.NoError(t, db.Create(&ChannelMonitorHourlyRollup{MonitorID: monitor.ID, Model: "gpt-test", BucketStart: 3600, TotalCount: 1}).Error)
require.Error(t, db.Create(&ChannelMonitorHourlyRollup{MonitorID: monitor.ID, Model: "gpt-test", BucketStart: 3600, TotalCount: 1}).Error)
```

- [ ] **Step 2: Run tests and verify RED**

```powershell
go test ./model ./service -run 'TestChannelMonitorModels|TestChannelMonitorCredential' -count=1
```

Expected: FAIL because the structs and credential functions do not exist.

- [ ] **Step 3: Add schemas and AES-GCM storage**

Use integer primary keys, Unix-second timestamps, `type:text` JSON/ciphertext fields, explicit composite indexes, and no boolean default tags. Derive a 32-byte key with SHA-256 over `new-api/channel-monitor/v1`, generate a random GCM nonce, and encode `v1:<base64 nonce+ciphertext>`. Store only the final four plaintext characters as the suffix. Add all four structs to both migration paths in `model/main.go`.

- [ ] **Step 4: Run focused tests and commit**

```powershell
go test ./model ./service -run 'TestChannelMonitorModels|TestChannelMonitorCredential' -count=1
git add model/channel_monitor.go model/channel_monitor_test.go model/main.go service/channel_monitor_crypto.go service/channel_monitor_crypto_test.go
git commit -m "feat: add channel monitor data models"
```

### Task 2: Validation and Administrator CRUD

**Files:**
- Create: `service/channel_monitor_config.go`
- Create: `service/channel_monitor_config_test.go`
- Extend: `model/channel_monitor.go`
- Create: `controller/channel_monitor.go`
- Create: `controller/channel_monitor_test.go`
- Create: `router/channel_monitor_router.go`
- Create: `router/channel_monitor_router_test.go`
- Modify: `router/api-router.go`

**Interfaces:**
- Produces: `service.ChannelMonitorInput` and `service.NormalizeChannelMonitorInput(input ChannelMonitorInput, current *model.ChannelMonitor, now int64) (*model.ChannelMonitor, error)`.
- Produces: administrator `GET/POST /api/channel-monitor`, `GET/PUT/DELETE /api/channel-monitor/:id`.
- Produces: administrator DTOs with `credential_configured` and `credential_masked`, never ciphertext.

- [ ] **Step 1: Write failing normalization tests**

Test valid pairs `openai/chat_completions`, `openai/responses`, `anthropic/messages`, and `gemini/generate_content`. Reject every other pair, query/fragment/userinfo/non-HTTP URLs, protocol endpoint paths, empty or duplicate models, more than 20 models, intervals outside `60..3600`, non-positive thresholds, and timeouts outside `1..120`. Assert an empty update credential preserves ciphertext/suffix; a replacement clears `ScheduleBlockedReason`, clears leases, and sets `NextCheckAt=now`.

- [ ] **Step 2: Run normalization tests and verify RED**

```powershell
go test ./service -run TestNormalizeChannelMonitorInput -count=1
```

- [ ] **Step 3: Implement normalization and transactional CRUD**

Trim names/models, preserve ordered unique extras using `common.Marshal`, normalize trailing slashes with `net/url`, set defaults `60`, `6000`, and `30`, and require a configured credential when enabled. Add:

```go
func ListChannelMonitors(filter ChannelMonitorListFilter) ([]*ChannelMonitor, int64, error)
func GetChannelMonitor(id int) (*ChannelMonitor, error)
func CreateChannelMonitor(monitor *ChannelMonitor) error
func UpdateChannelMonitor(monitor *ChannelMonitor) error
func DeleteChannelMonitor(id int) error
```

Delete checks, hour rollups, day rollups, and monitor explicitly in one transaction.

- [ ] **Step 4: Write failing API isolation and route tests**

Insert ciphertext containing a recognizable marker. Assert administrator list/detail contain internal/public names, endpoint, configured state, and mask but exclude `api_key_encrypted` and plaintext. Assert unauthenticated access is rejected and all routes use the existing channel-management authorization boundary.

- [ ] **Step 5: Implement controllers and routes**

Use `middleware.AdminAuth()` plus `authz.ChannelRead`, `ChannelWrite`, `ChannelSensitiveWrite`, and `ChannelOperate`. Bound pagination to page >= 1 and page size 1-100. Support provider, enabled/public state, and text filters.

- [ ] **Step 6: Run tests and commit**

```powershell
go test ./service ./controller ./router -run 'TestNormalizeChannelMonitorInput|TestChannelMonitorAdmin|TestChannelMonitorRoutes' -count=1
git add service/channel_monitor_config.go service/channel_monitor_config_test.go model/channel_monitor.go controller/channel_monitor.go controller/channel_monitor_test.go router/channel_monitor_router.go router/channel_monitor_router_test.go router/api-router.go
git commit -m "feat: add channel monitor administration api"
```

### Task 3: Direct Probe Protocols

**Files:**
- Create: `service/channel_monitor_probe.go`
- Create: `service/channel_monitor_probe_test.go`

**Interfaces:**
- Produces: `service.ChannelMonitorProbeResult{Status string, LatencyMs int64, ErrorCategory string, HTTPStatus int, ErrorMessage string}`.
- Produces: `service.ProbeChannelMonitorModel(ctx context.Context, monitor *model.ChannelMonitor, modelName string, credential string, client *http.Client) ChannelMonitorProbeResult`.

- [ ] **Step 1: Write failing construction tests**

Use `httptest.Server` to require these direct non-streaming requests and a low output cap:

```text
POST /v1/chat/completions                           Authorization: Bearer <key>
POST /v1/responses                                  Authorization: Bearer <key>
POST /v1/messages                                   x-api-key and anthropic-version
POST /v1beta/models/{model}:generateContent?key=... Gemini API key query
```

Return representative payloads whose extracted text is exactly `NEWAPI_MONITOR_OK`.

- [ ] **Step 2: Run tests and verify RED**

```powershell
go test ./service -run TestProbeChannelMonitorModelBuilds -count=1
```

- [ ] **Step 3: Implement bounded construction and parsing**

Serialize request DTOs with `common.Marshal`, use `http.NewRequestWithContext`, read at most 256 KiB plus a sentinel byte, and decode with `common.Unmarshal`. Extract Chat `choices[0].message.content`, Responses `output[].content[].text`, Anthropic `content[].text`, and Gemini `candidates[].content.parts[].text`. Require exact trimmed challenge equality.

Classify `timeout`, `network`, `tls`, `auth` for 401/403, `rate_limit` for 429, `upstream` for other non-2xx, and `invalid_response` for oversized/malformed/empty/mismatched bodies. Truncate messages to 512 valid UTF-8 bytes and never include the credential. Successful responses over the threshold are `degraded`.

- [ ] **Step 4: Add classification tests and commit**

Cover 401, 429, 500, deadline, TLS failure, refused connection, oversized body, malformed JSON, empty output, mismatch, operational, and delayed degraded success.

```powershell
go test ./service -run TestProbeChannelMonitorModel -count=1
git add service/channel_monitor_probe.go service/channel_monitor_probe_test.go
git commit -m "feat: add channel monitor probe protocols"
```

### Task 4: Lease Claims, Run Now, and Bounded Scheduler

**Files:**
- Extend: `model/channel_monitor.go`
- Create: `model/channel_monitor_lease_test.go`
- Create: `service/channel_monitor_scheduler.go`
- Create: `service/channel_monitor_scheduler_test.go`
- Extend: `controller/channel_monitor.go`
- Modify: `main.go`

**Interfaces:**
- Produces: `model.FindDueChannelMonitorIDs(now int64, limit int) ([]int, error)`.
- Produces: `model.ClaimChannelMonitor(id int, owner string, now int64, leaseUntil int64, manual bool) (*ChannelMonitor, bool, error)`.
- Produces: `service.RunClaimedChannelMonitor(ctx context.Context, monitor *model.ChannelMonitor, owner string) ([]model.ChannelMonitorCheck, error)`.
- Produces: `service.StartChannelMonitorScheduler()` and `service.RunChannelMonitorNow(ctx context.Context, id int) ([]model.ChannelMonitorCheck, error)`.

- [ ] **Step 1: Write failing lease tests**

Create one due monitor and call `ClaimChannelMonitor` concurrently from two owners; exactly one must succeed. A second claim fails while `lease_until >= now`, succeeds after expiry, and a manual claim may ignore future `next_check_at` but may not overlap an active lease. Disabled and blocked monitors cannot be automatically claimed.

- [ ] **Step 2: Run lease tests and verify RED**

```powershell
go test ./model -run TestClaimChannelMonitor -count=1
```

- [ ] **Step 3: Implement conditional leases**

Use a GORM conditional update over ID, enabled state, empty blocked reason, due time for automatic claims, and `(lease_until = 0 OR lease_until < now)`. Reload only when `RowsAffected == 1`. Compute lease seconds as `modelCount * timeoutSeconds + 30`, clamped to `60..2460`.

- [ ] **Step 4: Write failing scheduler tests**

Inject clock, scanner cadence, HTTP client, and worker count. Assert at most five monitors run concurrently; models within one monitor run sequentially in configured order; committed checks share a completion timestamp; `NextCheckAt = completedAt + interval`; and leases clear. Probe failures still create failed checks and use the normal interval. Decryption failure writes one `config` result per model, sets `ScheduleBlockedReason`, clears `NextCheckAt`, and sends no HTTP request.

- [ ] **Step 5: Implement scanner, workers, and Run Now**

Start only on master nodes. Poll every five seconds, query at most 50 due IDs, claim before enqueueing, and use one fixed buffered worker pool of size five. Decrypt once, probe models sequentially, and commit checks plus monitor timestamps/lease fields in one transaction. Export `service.ErrChannelMonitorBusy`; make Run Now use the same claim/run path and map a conflict to HTTP 409.

Call `service.StartChannelMonitorScheduler()` after database/HTTP-client initialization and before serving requests.

- [ ] **Step 6: Run tests and commit**

```powershell
go test ./model ./service ./controller -run 'TestClaimChannelMonitor|TestChannelMonitorScheduler|TestRunChannelMonitorNow' -count=1
git add model/channel_monitor.go model/channel_monitor_lease_test.go service/channel_monitor_scheduler.go service/channel_monitor_scheduler_test.go controller/channel_monitor.go main.go
git commit -m "feat: schedule independent channel monitors"
```

### Task 5: Rollups, Availability, and DTO Isolation

**Files:**
- Create: `model/channel_monitor_rollup.go`
- Create: `model/channel_monitor_rollup_test.go`
- Create: `service/channel_monitor_query.go`
- Create: `service/channel_monitor_query_test.go`
- Extend: `service/channel_monitor_scheduler.go`
- Extend: `controller/channel_monitor.go`
- Extend: `router/channel_monitor_router.go`

**Interfaces:**
- Produces: `model.AggregateChannelMonitorHistory(now time.Time, location *time.Location, batchSize int) error`.
- Produces: `service.GetChannelMonitorPublicSummary() (ChannelMonitorPublicSummary, error)`.
- Produces: `service.ListPublishedChannelMonitorStatus(days int) ([]ChannelMonitorStatusItem, error)`.
- Produces: `service.GetPublishedChannelMonitorDetail(id int) (*ChannelMonitorStatusDetail, error)`.

- [ ] **Step 1: Write failing aggregation tests**

Use fixed `Asia/Shanghai` time and records on exact one-hour, 24-hour, and seven-day boundaries. Raw rows strictly older than one hour become natural-hour rollups; hourly rows strictly older than 24 hours become natural-day rollups; daily rows before the seven-day natural-day cutoff are deleted. Run twice and assert counts do not double.

Create mixed raw/hour/day samples across today and six prior days. Assert availability is `(operational + degraded) / total`, average latency is weighted by sample count, natural 1/3/7-day windows are exact, and no samples serialize availability as `null`.

- [ ] **Step 2: Run tests and verify RED**

```powershell
go test ./model ./service -run 'TestAggregateChannelMonitorHistory|TestChannelMonitorAvailability' -count=1
```

- [ ] **Step 3: Implement idempotent transactional aggregation**

Select bounded oldest source rows, group in Go by monitor/model/bucket, and upsert with `clause.OnConflict` adding counts and latency sums. Delete exact source IDs only in the same transaction. Run maintenance after scheduler scans but at most once per minute.

- [ ] **Step 4: Write failing public/user isolation tests**

Create published, unpublished, disabled, and never-checked monitors. Assert public and signed-in JSON contain only public name, provider, models, safe statuses/categories, latency, availability, check times, and timeline. Assert `internal_name`, `endpoint`, `credential`, `api_key`, `lease`, `http_status`, and `error_message` are absent. Unpublished/disabled monitors are absent; a never-checked published monitor is `unknown`.

- [ ] **Step 5: Implement dedicated query DTOs and caching**

Use narrow `Select` query structs rather than administrator models. Overall state is unknown when all items are unknown, failed when any item failed, degraded when none failed and any item is degraded/unknown, otherwise operational. Return at most 60 primary-model raw points from the last hour newest-first. Add a mutex-protected 15-second public summary cache and invalidate it after config changes and committed checks.

- [ ] **Step 6: Add history, public, and signed-in endpoints**

Implement administrator `GET /api/channel-monitor/:id/history` with bounded filters. Register anonymous `GET /api/channel-monitor/public-summary` before parameter routes. Register authenticated `GET /api/channel-monitor/status` and `GET /api/channel-monitor/status/:id`; return only enabled, published monitors.

- [ ] **Step 7: Run tests and commit**

```powershell
go test ./model ./service ./controller ./router -run 'TestAggregateChannelMonitorHistory|TestChannelMonitorAvailability|TestChannelMonitorPublic|TestChannelMonitorStatus|TestChannelMonitorHistory' -count=1
git add model/channel_monitor_rollup.go model/channel_monitor_rollup_test.go service/channel_monitor_query.go service/channel_monitor_query_test.go service/channel_monitor_scheduler.go controller/channel_monitor.go router/channel_monitor_router.go
git commit -m "feat: publish channel monitor status data"
```

### Task 6: Administrator Management Page

**Files:**
- Create: `web/src/features/channel-monitors/types.ts`
- Create: `web/src/features/channel-monitors/api.ts`
- Create: `web/src/features/channel-monitors/index.tsx`
- Create: `web/src/features/channel-monitors/components/channel-monitor-table.tsx`
- Create: `web/src/features/channel-monitors/components/channel-monitor-form-drawer.tsx`
- Create: `web/src/features/channel-monitors/components/channel-monitor-history-dialog.tsx`
- Create: `web/src/features/channel-monitors/components/__tests__/management.test.tsx`
- Create: `web/src/features/channel-monitors/lib/schema.ts`
- Create: `web/src/features/channel-monitors/lib/__tests__/schema.test.ts`
- Create: `web/src/routes/_authenticated/channel-monitors/index.tsx`
- Modify: `web/src/features/channels/components/channels-primary-buttons.tsx`

**Interfaces:**
- Consumes: administrator APIs from Tasks 2, 4, and 5.
- Produces: administrator route `/channel-monitors` guarded by `ROLE.ADMIN`.
- Produces: frontend form schema matching backend constraints.

- [ ] **Step 1: Write failing validation and navigation tests**

Assert exact provider/mode pairs, deduplication, 20-model limit, interval `60..3600`, timeout `1..120`, and required create credential. Render channel primary controls, activate Channel Monitor, and assert navigation to `/channel-monitors`.

- [ ] **Step 2: Run tests and verify RED**

```powershell
cd web
bun test src/features/channel-monitors/lib/__tests__/schema.test.ts src/features/channel-monitors/components/__tests__/management.test.tsx
```

- [ ] **Step 3: Implement typed API and management UI**

Use Zod response schemas and React Query keys rooted at `['channel-monitors']`. Render a stable responsive table with names, provider/mode, model count, latest status/latency, last/next check, interval, enabled/public flags, and icon actions for edit, Run Now, history, and delete. Use React Hook Form + Zod in a drawer, tag-style extra models, masked credential state, endpoint HTTP warning, publication disclosure, and delete confirmation. Invalidate related queries after mutations and show the localized busy message on HTTP 409.

- [ ] **Step 4: Run tests and commit**

```powershell
bun test src/features/channel-monitors/lib/__tests__/schema.test.ts src/features/channel-monitors/components/__tests__/management.test.tsx
cd ..
git add web/src/features/channel-monitors web/src/routes/_authenticated/channel-monitors/index.tsx web/src/features/channels/components/channels-primary-buttons.tsx
git commit -m "feat: add channel monitor administration ui"
```

### Task 7: Signed-In Channel Status Page

**Files:**
- Create: `web/src/features/channel-status/types.ts`
- Create: `web/src/features/channel-status/api.ts`
- Create: `web/src/features/channel-status/index.tsx`
- Create: `web/src/features/channel-status/components/status-summary.tsx`
- Create: `web/src/features/channel-status/components/monitor-status-card.tsx`
- Create: `web/src/features/channel-status/components/monitor-status-details.tsx`
- Create: `web/src/features/channel-status/components/__tests__/status-page.test.tsx`
- Create: `web/src/features/channel-status/hooks/use-channel-status-refresh.ts`
- Create: `web/src/features/channel-status/hooks/__tests__/refresh.test.ts`
- Create: `web/src/routes/_authenticated/channel-status/index.tsx`
- Modify: `web/src/hooks/use-sidebar-data.ts`

**Interfaces:**
- Consumes: signed-in status APIs from Task 5.
- Produces: normal-user route `/channel-status` and General sidebar item directly below Dashboard.
- Produces: 60-second auto-refresh countdown with manual refresh and cleanup.

- [ ] **Step 1: Write failing refresh and page tests**

With fake timers, assert countdown starts at 60, decrements each second, refetches and resets at zero, stops when toggled, and clears timers on unmount. Assert 1/3/7 selection changes availability, cards are keyboard-activatable, unknown uses text/icon, long names stay bounded, and details include primary plus extra models.

- [ ] **Step 2: Run tests and verify RED**

```powershell
cd web
bun test src/features/channel-status/hooks/__tests__/refresh.test.ts src/features/channel-status/components/__tests__/status-page.test.tsx
```

- [ ] **Step 3: Implement the signed-in page**

Use segmented 1/3/7-day controls, icon buttons with tooltips for manual/automatic refresh, visible countdown, a full-width summary, and responsive monitor grid. Timeline cells have stable dimensions, accessible labels, and neutral blanks for missing samples. Open details in a sheet/dialog listing model state, 1/3/7 availability, weighted latency, and sanitized category. Add Channel Status after Dashboard without an admin role.

- [ ] **Step 4: Run tests and commit**

```powershell
bun test src/features/channel-status/hooks/__tests__/refresh.test.ts src/features/channel-status/components/__tests__/status-page.test.tsx
cd ..
git add web/src/features/channel-status web/src/routes/_authenticated/channel-status/index.tsx web/src/hooks/use-sidebar-data.ts
git commit -m "feat: add signed-in channel status page"
```

### Task 8: Public Model-Square Summary

**Files:**
- Create: `web/src/features/channel-status/components/public-status-summary.tsx`
- Create: `web/src/features/channel-status/components/__tests__/public-summary.test.tsx`
- Modify: `web/src/features/pricing/index.tsx`

**Interfaces:**
- Consumes: anonymous `GET /api/channel-monitor/public-summary`.
- Removes: Model Square heading, enabled-model sentence, and descriptive paragraph.
- Produces: compact public status region immediately above the existing search control.

- [ ] **Step 1: Write failing public summary tests**

Render loading, empty, operational, degraded, failed, and request-error fixtures. Assert stable placeholders, overall state/update time, counts, monitor summaries, and that API failure leaves search usable. Authenticated View details navigates to `/channel-status`; unauthenticated navigation goes through `/sign-in` with `/channel-status` as redirect. Assert all three former heading texts are absent.

- [ ] **Step 2: Run tests and verify RED**

```powershell
cd web
bun test src/features/channel-status/components/__tests__/public-summary.test.tsx
```

- [ ] **Step 3: Replace pricing hero copy**

Keep the unframed full-width header area. Place `PublicStatusSummary` before `SearchBar`, showing overall state, last update, operational/degraded/failed/unknown counts, and responsive monitor rows with public name, provider, primary model, latency, status, and today's availability. Keep pricing queries independent so summary failure never blocks the model list.

- [ ] **Step 4: Run tests and commit**

```powershell
bun test src/features/channel-status/components/__tests__/public-summary.test.tsx
cd ..
git add web/src/features/channel-status/components/public-status-summary.tsx web/src/features/channel-status/components/__tests__/public-summary.test.tsx web/src/features/pricing/index.tsx
git commit -m "feat: show public channel status on model square"
```

### Task 9: Internationalization and Frontend Static Checks

**Files:**
- Modify: `web/src/i18n/locales/en.json`
- Modify: `web/src/i18n/locales/zh.json`
- Modify: `web/src/i18n/locales/zh-TW.json`
- Modify: `web/src/i18n/locales/fr.json`
- Modify: `web/src/i18n/locales/ru.json`
- Modify: `web/src/i18n/locales/ja.json`
- Modify: `web/src/i18n/locales/vi.json`
- Update: `web/src/routeTree.gen.ts` through normal TanStack tooling.

**Interfaces:**
- Consumes: every literal `t('...')` key introduced in Tasks 6-8.
- Produces: complete translations for every supported locale and a valid generated route tree.

- [ ] **Step 1: Synchronize and translate keys**

Run `bun run i18n:sync`, then translate every introduced key in all seven locale files. Keep protocol/provider identifiers unchanged. No non-English locale may retain an English fallback except technical proper nouns.

- [ ] **Step 2: Verify i18n, types, lint, and formatting**

```powershell
cd web
bun run i18n:sync
bun run typecheck
bunx oxlint -c .oxlintrc.json src/features/channel-monitors src/features/channel-status src/features/channels/components/channels-primary-buttons.tsx src/features/pricing/index.tsx src/hooks/use-sidebar-data.ts src/routes/_authenticated/channel-monitors/index.tsx src/routes/_authenticated/channel-status/index.tsx
bunx oxfmt --check src/features/channel-monitors src/features/channel-status src/features/channels/components/channels-primary-buttons.tsx src/features/pricing/index.tsx src/hooks/use-sidebar-data.ts src/routes/_authenticated/channel-monitors/index.tsx src/routes/_authenticated/channel-status/index.tsx src/i18n/locales
```

- [ ] **Step 3: Commit localization and route generation**

```powershell
cd ..
git add web/src/i18n web/src/routeTree.gen.ts
git commit -m "feat: localize channel monitor interfaces"
```

### Task 10: End-to-End Verification and Final Review

**Files:**
- Verify all files changed in Tasks 1-9.

- [ ] **Step 1: Run backend focused and broad tests**

```powershell
go test ./model ./service ./controller ./router -run ChannelMonitor -count=1
go test ./model ./service ./controller ./router -count=1
```

If configured MySQL/PostgreSQL test DSNs exist, repeat the model/rollup behavior tests against them; otherwise record that local database verification covered SQLite only.

- [ ] **Step 2: Run complete frontend checks**

```powershell
cd web
bun test src/features/channel-monitors src/features/channel-status
bun run typecheck
bun run lint
bun run format:check
bun run build
cd ..
```

- [ ] **Step 3: Run production Go build**

```powershell
go build ./...
```

- [ ] **Step 4: Verify the real UI**

Start the backend with built frontend and use Playwright at desktop/mobile widths in light/dark modes. Verify `/channels` navigation, `/channel-monitors` CRUD/run/history, `/channel-status` windows/refresh/details, and `/pricing` public summary. Capture screenshots and confirm no overflow, overlap, blank content, color-only status, unstable cards, or leaked sensitive fields. Verify unauthenticated View details redirects through sign-in.

- [ ] **Step 5: Review security and side effects**

Search the diff and HTTP payloads for `api_key_encrypted`, raw credentials, endpoint leakage in public/user DTOs, direct `encoding/json` calls, and any quota/log/channel mutation invoked from probe code. Confirm conditional lease ownership and cache invalidation after both configuration and result changes.

- [ ] **Step 6: Final diff checks**

```powershell
git diff --check
git status --short
git log --oneline --decorate -10
```

Fix every issue, rerun its affected test and the final verification commands, then create a cleanup commit only when verification required changes.
