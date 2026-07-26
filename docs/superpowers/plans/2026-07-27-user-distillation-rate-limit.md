# User Rate Limits and Distillation Detection Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (- [ ]) syntax for tracking.

**Goal:** Add user-specific ordinary model rate limits, non-stream distillation detection with temporary and permanent punishment, and administrator configuration/penalty management.

**Architecture:** Keep ordinary limits in middleware/model-rate-limit.go and add a user-policy resolver above group/global fallback. Persist actionable distillation state in a dedicated GORM model, cache it using Redis or memory, and run non-stream detection from controller.Relay immediately after RelayInfo is available and before billing. Save the complete rate-limit form atomically through a root-only endpoint.

**Tech Stack:** Go, Gin, GORM v2, Redis, pkg/cachex, React 19, TypeScript, React Hook Form, Zod, TanStack Query, Base UI, Tailwind CSS, i18next.

---

## File Map

Backend:

- setting/rate_limit.go: typed settings, safe JSON maps, validation, precedence.
- model/option.go: option defaults and runtime option dispatch.
- controller/rate_limit.go: atomic settings save plus penalty list/clear handlers.
- middleware/model-rate-limit.go: ordinary limiter and Redis/memory success parity.
- common/rate-limit.go: explicit memory-window check, record, count, and delete.
- model/distillation_penalty.go: durable penalty state machine and list query.
- service/distillation_rate_limit.go: rolling counters, cache, and request decisions.
- controller/relay.go: pre-billing distillation call.
- router/api-router.go: root-only rate-limit routes.
- types/error.go and controller/audit.go: stable errors and audit action.
- Matching *_test.go files: deterministic regression coverage.

Frontend:

- web/default/src/features/system-settings/request-limits/rate-limit-schema.ts: schema and JSON helpers.
- web/default/src/features/system-settings/request-limits/api.ts: atomic save, user search, penalty list/clear.
- web/default/src/features/system-settings/request-limits/types.ts: DTO and form types.
- web/default/src/features/system-settings/request-limits/user-rate-limit-editor.tsx: searched-user editor.
- web/default/src/features/system-settings/request-limits/distillation-settings.tsx: switch and numeric settings.
- web/default/src/features/system-settings/request-limits/distillation-penalties-table.tsx: current penalty administration.
- Existing security settings files: new defaults and props.
- Six locale JSON files: complete translations.

### Task 1: Typed Settings and Atomic Save

**Files:**

- Create: setting/rate_limit_test.go
- Modify: setting/rate_limit.go
- Modify: model/option.go
- Create: controller/rate_limit.go
- Modify: controller/option.go
- Modify: router/api-router.go

- [ ] **Step 1: Write failing setting tests**

Cover valid and invalid user maps, safe replacement, user-over-group-over-global precedence, disabled zero distillation values, and enabled RPM below threshold.

~~~go
func TestResolveModelRequestRateLimitPrefersUserThenGroup(t *testing.T) {
    setRateLimitFixtures(t,
        map[string][2]int{"vip": {200, 100}},
        map[int][2]int{42: {20, 10}},
    )

    total, success := ResolveModelRequestRateLimit(42, "vip", 500, 300)
    assert.Equal(t, 20, total)
    assert.Equal(t, 10, success)

    total, success = ResolveModelRequestRateLimit(7, "vip", 500, 300)
    assert.Equal(t, 200, total)
    assert.Equal(t, 100, success)
}
~~~

- [ ] **Step 2: Run tests and verify failure**

Run: go test ./setting -run 'Test.*RateLimit|Test.*Distillation' -count=1

Expected: FAIL because user settings, resolver, and validation are absent.

- [ ] **Step 3: Implement typed settings**

Define DistillationSettings with Enabled, Threshold, RPM, PenaltyMinutes, and ObservationMinutes. Add ModelRequestRateLimitUser as map[int][2]int. Parse group/user JSON into temporary maps with common.Unmarshal, validate, then swap under Lock. Serialize with common.Marshal. Add ResolveModelRequestRateLimit(userId, group, globalTotal, globalSuccess).

- [ ] **Step 4: Register options and atomic endpoint**

Add all new defaults and updateOptionMap cases in model/option.go. Add PUT /api/rate-limit under RootAuth. Its typed request contains every ordinary and distillation field, validates the complete state, and writes one map through model.UpdateOptionsBulk. Keep candidate validation in the generic option handler for direct API callers.

- [ ] **Step 5: Run and commit**

Run: go test ./setting ./controller -run 'RateLimit|Distillation' -count=1

Expected: PASS.

Commit message: feat(rate-limit): add typed user and distillation settings

### Task 2: Ordinary User Policy and Success-Count Parity

**Files:**

- Create: common/rate-limit_test.go
- Modify: common/rate-limit.go
- Create: middleware/model-rate-limit_test.go
- Modify: middleware/model-rate-limit.go

- [ ] **Step 1: Write failing window tests**

~~~go
func TestInMemoryRateLimiterCheckDoesNotRecord(t *testing.T) {
    var limiter InMemoryRateLimiter
    limiter.Init(0)

    assert.True(t, limiter.Check("success:1", 1, 60))
    assert.True(t, limiter.Check("success:1", 1, 60))
    limiter.Record("success:1", 60)
    assert.False(t, limiter.Check("success:1", 1, 60))
}
~~~

Also test RequestWithCount and Delete, plus middleware user precedence.

- [ ] **Step 2: Run and verify failure**

Run: go test ./common ./middleware -run 'RateLimit' -count=1

Expected: FAIL because explicit window operations and user resolution are absent.

- [ ] **Step 3: Implement memory operations**

Expose Check, Record, RequestWithCount, and Delete on InMemoryRateLimiter. Keep Request as a compatibility wrapper. Prune expired timestamps under the existing mutex; do not add sleeps to tests.

- [ ] **Step 4: Fix middleware behavior**

Resolve the selected pair through setting.ResolveModelRequestRateLimit. In memory mode, check the real success key before c.Next and record that same key only for status below 400. Remove the temporary _check key. Preserve Redis check-then-record behavior.

- [ ] **Step 5: Run and commit**

Run: go test ./common ./middleware -run 'RateLimit' -count=1

Expected: PASS.

Commit message: fix(rate-limit): align user policy and success counters

### Task 3: Durable Penalty State Machine

**Files:**

- Create: model/distillation_penalty.go
- Create: model/distillation_penalty_test.go
- Modify: model/main.go

- [ ] **Step 1: Write failing model tests**

Use explicit Unix timestamps and SQLite fixtures for first trigger, temporary phase, second trigger, observation expiry becoming a new first strike, permanent immutability, idempotent clear, list filtering, and concurrent transitions.

~~~go
func TestAdvanceDistillationPenaltyBansSecondTrigger(t *testing.T) {
    setupDistillationPenaltyFixture(t)

    first, err := AdvanceDistillationPenalty(7, 1000, 600, 3600)
    require.NoError(t, err)
    assert.Equal(t, DistillationPhaseTemporary, first.Phase(1000))

    second, err := AdvanceDistillationPenalty(7, 1700, 600, 3600)
    require.NoError(t, err)
    assert.Equal(t, int64(1700), second.PermanentlyBannedAt)
}
~~~

- [ ] **Step 2: Run and verify failure**

Run: go test ./model -run 'DistillationPenalty' -count=1

Expected: FAIL because the model does not exist.

- [ ] **Step 3: Implement model operations**

Create DistillationPenalty with unique indexed UserId; FirstTriggeredAt, TemporaryLimitedUntil, ObservationUntil, PermanentlyBannedAt; CreatedAt and UpdatedAt Unix timestamps. Implement:

- GetDistillationPenalty
- AdvanceDistillationPenalty
- ClearDistillationPenalty
- ListDistillationPenalties

Use lockForUpdate for existing rows, a unique user constraint for creation races, a guarded delete for expired observation rows, and no dialect-specific SQL.

- [ ] **Step 4: Register migrations**

Add DistillationPenalty to both migrateDB and migrateDBFast.

- [ ] **Step 5: Run and commit**

Run: go test ./model -run 'DistillationPenalty|LockForUpdate' -count=1

Expected: PASS.

Commit message: feat(rate-limit): persist distillation penalties

### Task 4: Rolling Detection Engine and Relay Integration

**Files:**

- Create: service/distillation_rate_limit.go
- Create: service/distillation_rate_limit_test.go
- Modify: controller/relay.go
- Modify: types/error.go

- [ ] **Step 1: Write failing engine tests**

Use a small runtime-store interface and fakes. Prove that streams skip detection but honor permanent bans; request X passes; request X+1 sees temporary Y RPM; temporary traffic does not advance detection; disabled detection bypasses temporary enforcement; observation expiry resets; and the second threshold persists a ban after the current request.

- [ ] **Step 2: Run and verify failure**

Run: go test ./service -run 'DistillationRateLimit' -count=1

Expected: FAIL because the engine and error codes are absent.

- [ ] **Step 3: Implement Redis and memory rolling windows**

Use one Redis Lua sorted-set operation to prune entries older than the window, atomically check/add, and return the post-add count. Use InMemoryRateLimiter.RequestWithCount without Redis. Keep separate detection and temporary-RPM keys. Redis errors return an error and never silently fall back.

- [ ] **Step 4: Implement cached durable decisions**

Use cachex.HybridCache[string] with StringCodec. Marshal penalty values through common.Marshal/common.Unmarshal and support a negative-cache sentinel. Cache for at most 60 seconds and invalidate after transition or administrator clear.

Add error codes distillation_rate_limited and distillation_banned. Return HTTP 429 for temporary denial and HTTP 403 with a contact-administrator message for permanent denial.

- [ ] **Step 5: Call before billing**

Immediately after successful GenRelayInfo:

~~~go
newAPIError = service.CheckDistillationRateLimit(c, relayInfo)
if newAPIError != nil {
    return
}
~~~

This ensures validation failures do not count and upstream failures do count.

- [ ] **Step 6: Run and commit**

Run: go test ./service ./controller -run 'Distillation|Relay' -count=1

Expected: PASS.

Commit message: feat(rate-limit): enforce distillation detection

### Task 5: Penalty Administration API and Audit

**Files:**

- Modify: controller/rate_limit.go
- Create: controller/rate_limit_test.go
- Modify: router/api-router.go
- Modify: controller/audit.go
- Modify: web/default/src/features/usage-logs/lib/format.ts

- [ ] **Step 1: Write failing handler tests**

Cover pagination/search response, expired-row exclusion, idempotent clear, target-user audit parameters, and service cache/runtime invalidation.

- [ ] **Step 2: Run and verify failure**

Run: go test ./controller -run 'DistillationPenalty|RateLimitSettings' -count=1

Expected: FAIL because list/clear handlers are absent.

- [ ] **Step 3: Implement root routes**

Register:

- PUT /api/rate-limit
- GET /api/rate-limit/distillation/penalties
- DELETE /api/rate-limit/distillation/penalties/:user_id

The clear handler requires a positive user ID, clears durable and runtime state, and records rate_limit.distillation_clear with target_user_id.

- [ ] **Step 4: Add audit display mapping and commit**

Run: go test ./controller ./model ./service -run 'Distillation|RateLimit' -count=1

Expected: PASS.

Commit message: feat(rate-limit): add distillation penalty administration

### Task 6: Frontend Schema, API, and User Editor

**Files:**

- Create: web/default/src/features/system-settings/request-limits/types.ts
- Create: web/default/src/features/system-settings/request-limits/api.ts
- Create: web/default/src/features/system-settings/request-limits/rate-limit-schema.ts
- Create: web/default/src/features/system-settings/request-limits/rate-limit-schema.test.ts
- Create: web/default/src/features/system-settings/request-limits/user-rate-limit-editor.tsx
- Modify: web/default/src/features/system-settings/request-limits/rate-limit-section.tsx
- Modify: web/default/src/features/system-settings/types.ts
- Modify: web/default/src/features/system-settings/security/index.tsx
- Modify: web/default/src/features/system-settings/security/section-registry.tsx

- [ ] **Step 1: Write failing pure tests**

Test ID-keyed parsing, stable sorted serialization, duplicate prevention, unknown user preservation, disabled zero values, and enabled RPM below threshold.

Run from web/default: bun test src/features/system-settings/request-limits/rate-limit-schema.test.ts

Expected: FAIL because the schema module is absent.

- [ ] **Step 2: Implement schema and APIs**

Define RateLimitFormValues and the Zod schema. Add typed calls for atomic save, /api/user/search, penalty list, and clear. The atomic save returns one business response and produces one success/error toast.

- [ ] **Step 3: Build the searched-user editor**

Use the existing combobox/command pattern and useDebounce. Search non-empty username/ID text, show username/display name/ID, exclude selected users, and append total/success inputs. Preserve deleted configured IDs as User #ID until removed.

- [ ] **Step 4: Compose atomic submission**

Move schema/parsing out of rate-limit-section.tsx, replace per-option mutations with one mutation against PUT /api/rate-limit, and pass all defaults through SecuritySettings and section-registry.

- [ ] **Step 5: Run and commit**

From web/default:

- bun test src/features/system-settings/request-limits/rate-limit-schema.test.ts
- bun run typecheck

Expected: PASS.

Commit message: feat(rate-limit): add user policy editor

### Task 7: Distillation Settings and Penalty Table

**Files:**

- Create: web/default/src/features/system-settings/request-limits/distillation-settings.tsx
- Create: web/default/src/features/system-settings/request-limits/distillation-penalties-table.tsx
- Modify: web/default/src/features/system-settings/request-limits/rate-limit-section.tsx

- [ ] **Step 1: Add settings fields**

Render the switch plus threshold, punishment RPM, punishment minutes, and observation minutes through the parent form. Keep values visible but disabled while off. Show explicit units.

- [ ] **Step 2: Add penalty table**

Use query key ['distillation-penalties', page, pageSize, keyword]. Provide debounced username/ID search, refresh, loading, empty, error, phase badges, timestamps, pagination, confirmation, and clear. Invalidate only this query family.

- [ ] **Step 3: Verify responsive composition**

Keep global inputs at three columns, distillation inputs in a responsive grid, and allow user/penalty rows to stack without clipped translations.

- [ ] **Step 4: Run and commit**

Run from web/default: bun run typecheck

Expected: PASS.

Commit message: feat(rate-limit): add distillation administration UI

### Task 8: i18n and Full Verification

**Files:**

- Modify: web/default/src/i18n/locales/en.json
- Modify: web/default/src/i18n/locales/zh.json
- Modify: web/default/src/i18n/locales/fr.json
- Modify: web/default/src/i18n/locales/ru.json
- Modify: web/default/src/i18n/locales/ja.json
- Modify: web/default/src/i18n/locales/vi.json

- [ ] **Step 1: Complete translations**

Follow the i18n-translate skill, run bun run i18n:sync, and translate only feature/audit keys introduced here in all six locales.

- [ ] **Step 2: Run backend verification**

Run:

- go test ./setting ./common ./middleware ./model ./service ./controller -count=1
- go test ./... -count=1

Expected: all tests PASS.

- [ ] **Step 3: Run frontend verification**

From web/default run:

- bun test src/features/system-settings/request-limits/rate-limit-schema.test.ts
- bun run typecheck
- bun run lint
- bun run format:check
- bun run i18n:sync
- bun run build

Expected: all commands succeed and no feature key remains untranslated.

- [ ] **Step 4: Browser verification**

Verify user-rule precedence; invalid RPM/threshold rejection; first temporary trigger; stream exclusion; observation reset; second permanent trigger; disable semantics; administrator clear; and desktop/narrow layouts.

- [ ] **Step 5: Commit final fixes**

Commit message: chore(rate-limit): complete translations and verification fixes
