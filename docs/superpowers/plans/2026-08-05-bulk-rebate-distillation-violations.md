# Bulk Rebate Approval and Distillation Violations Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add atomic approval of all pending rebate requests, permanent user-visible distillation violation history, low-cost fixed natural-minute counters with a single threshold-crossing transition, and readable API key notice text.

**Architecture:** Keep financial changes in model-layer transactions and expose thin authenticated controllers. Persist distillation history in the same transaction as active penalty transitions, while Redis and memory stores provide identical scalar fixed-minute counter results. Build focused React feature components on the existing DataTable, React Query, TanStack Router, Base UI, and i18next patterns.

**Tech Stack:** Go 1.22+, Gin, GORM v2, SQLite/MySQL/PostgreSQL, go-redis v8, testify, React 19, TypeScript, TanStack Query/Router/Table, Base UI, Tailwind CSS, Bun test, i18next.

## Global Constraints

- Execute inline in the current session and do not dispatch subagents, per the user's earlier instruction.
- Preserve all existing user changes in the dirty worktree; stage only files belonging to the current task at each commit.
- All database behavior must work with SQLite, MySQL >= 5.7.8, and PostgreSQL >= 9.6; use GORM and `lockForUpdate` instead of dialect-specific SQL.
- Batch approval is all-or-nothing and must preserve existing balance validation, audit logs, user management logs, and post-commit quota cache updates.
- A first distillation strike temporarily limits the user; a second strike during observation permanently bans the user until administrator clearing.
- Detection and penalty RPM use fixed natural-minute buckets. The threshold request passes; only that request performs the durable transition; later requests do not repeat the transition.
- Violation history is immutable and remains after active penalty expiry or administrator clearing.
- Frontend text must use `t(...)` and all seven locales: `en`, `zh`, `zh-TW`, `fr`, `ja`, `ru`, and `vi`.
- Locale JSON files may only be changed through a temporary `web/scripts/add-missing-keys.mjs`, followed by `bun run i18n:sync`, then removal of the temporary script.
- New Go tests use `testify/require` for fatal setup assertions and `testify/assert` for non-fatal value checks.
- Production code is written only after its focused test has failed for the expected missing behavior.
- Run happy-dom component test files in separate Bun processes. Existing test files each install their own global `window` and `document`; running several of them in one Bun process races those globals even though every file passes independently.

---

### Task 1: Atomic Backend Approval of All Pending Rebates

**Files:**
- Modify: `model/affiliate_transfer_request.go`
- Modify: `model/affiliate_transfer_request_test.go`
- Modify: `controller/affiliate_transfer.go`
- Modify: `controller/affiliate_transfer_test.go`
- Modify: `router/api-router.go`

**Interfaces:**
- Produces: `ApproveAllPendingAffiliateTransferRequests(reviewerID int) ([]*AffiliateTransferRequestDetail, error)`.
- Produces: `POST /api/user/affiliate/transfer-requests/approve-all` returning `{ approved_count: number }`.
- Preserves: `ApproveAffiliateTransferRequest(requestID int, reviewerID int) error` for existing callers.

- [ ] **Step 1: Write failing model tests for success, empty input, and rollback**

Add explicit fixtures containing two valid pending requests for different users, one already-approved request, and one invalid pending request. Assert literal balances and statuses:

```go
func TestApproveAllPendingAffiliateTransferRequestsApprovesEveryPendingRequest(t *testing.T) {
    setupAffiliateTransferRequestFixture(t)
    // Create users with (quota, aff_quota) of (10, 100) and (20, 200),
    // pending transfers of 150 and 250, plus an already-approved request.
    approved, err := ApproveAllPendingAffiliateTransferRequests(99)
    require.NoError(t, err)
    require.Len(t, approved, 2)
    assert.Equal(t, []int{firstRequest.Id, secondRequest.Id}, []int{approved[0].Id, approved[1].Id})
    assert.Equal(t, 160, storedFirstUser.Quota)
    assert.Equal(t, 270, storedSecondUser.Quota)
    assert.Equal(t, AffiliateTransferStatusApproved, storedFirstRequest.Status)
    assert.Equal(t, AffiliateTransferStatusApproved, storedSecondRequest.Status)
}

func TestApproveAllPendingAffiliateTransferRequestsRollsBackEntireBatch(t *testing.T) {
    setupAffiliateTransferRequestFixture(t)
    // The second request requires more invitation reward than its user owns.
    approved, err := ApproveAllPendingAffiliateTransferRequests(99)
    require.Error(t, err)
    assert.Nil(t, approved)
    assert.Equal(t, 10, storedFirstUser.Quota)
    assert.Equal(t, AffiliateTransferStatusPending, storedFirstRequest.Status)
    assert.Equal(t, AffiliateTransferStatusPending, storedSecondRequest.Status)
}
```

- [ ] **Step 2: Run the model tests and verify RED**

Run:

```powershell
go test ./model -run 'TestApproveAllPendingAffiliateTransferRequests' -count=1
```

Expected: build failure because `ApproveAllPendingAffiliateTransferRequests` does not exist.

- [ ] **Step 3: Implement one shared transactional approval operation**

Extract the existing request validation and conditional request/user updates into a transaction-scoped function used by both single and batch approval:

```go
func approveAffiliateTransferRequestWithDB(
    tx *gorm.DB,
    request *AffiliateTransferRequest,
    reviewerID int,
) error

func ApproveAllPendingAffiliateTransferRequests(
    reviewerID int,
) ([]*AffiliateTransferRequestDetail, error)
```

The batch function must:

1. call `lockForUpdate(tx).Where("status = ?", AffiliateTransferStatusPending).Order("id ASC").Find(&requests)`;
2. return an empty non-nil result for no pending rows;
3. load usernames and display names for all request user IDs inside the transaction;
4. apply `approveAffiliateTransferRequestWithDB` to every request;
5. return no result if the transaction rolls back;
6. after commit, call `cacheIncrUserQuota` once per approved request and log cache failures.

- [ ] **Step 4: Run existing and new model approval tests and verify GREEN**

Run:

```powershell
go test ./model -run 'TestApprove(AllPending)?AffiliateTransferRequest|TestAffiliateTransferRequestMultiConnectionConcurrentTerminalTransition' -count=1
```

Expected: PASS, including existing single-approval concurrency and rollback tests.

- [ ] **Step 5: Write failing controller tests for response and per-request logs**

Add a second valid request/user to the controller fixture. Call the new controller with administrator context and assert:

```go
assert.Equal(t, 2, response.Data.ApprovedCount)
require.Len(t, adminLogs, 2)
require.Len(t, userLogs, 2)
assert.ElementsMatch(t, []int{7, 8}, loggedRequestIDs)
```

Also create an invalid second request and assert the response is unsuccessful, both requests stay pending, balances stay unchanged, and no approval log exists.

- [ ] **Step 6: Run the controller tests and verify RED**

Run:

```powershell
go test ./controller -run 'TestApproveAllAffiliateTransferRequests' -count=1
```

Expected: build failure because `ApproveAllAffiliateTransferRequests` does not exist.

- [ ] **Step 7: Implement the controller, shared log writer, and route**

Add:

```go
func ApproveAllAffiliateTransferRequests(c *gin.Context)

type approveAllAffiliateTransferRequestsResult struct {
    ApprovedCount int `json:"approved_count"`
}
```

Refactor the existing administrator audit plus user management log block into `recordAffiliateTransferApproval(c, detail)` and invoke it only after model success. Register the new static POST route before the parameterized approval route.

- [ ] **Step 8: Run focused backend tests and commit**

Run:

```powershell
go test ./model ./controller -run 'AffiliateTransferRequest|AffiliateTransferRequests' -count=1
gofmt -w model/affiliate_transfer_request.go model/affiliate_transfer_request_test.go controller/affiliate_transfer.go controller/affiliate_transfer_test.go router/api-router.go
git add model/affiliate_transfer_request.go model/affiliate_transfer_request_test.go controller/affiliate_transfer.go controller/affiliate_transfer_test.go router/api-router.go
git commit -m "feat: approve all pending rebate requests"
```

Expected: tests PASS and only Task 1 files are committed.

---

### Task 2: Durable Distillation Violation Records

**Files:**
- Create: `model/distillation_violation_record.go`
- Create: `model/distillation_violation_record_test.go`
- Modify: `model/distillation_penalty.go`
- Modify: `model/distillation_penalty_test.go`
- Modify: `model/main.go`

**Interfaces:**
- Produces: `DistillationTrigger` carrying the trigger-time settings snapshot.
- Produces: `DistillationViolationRecord` and action constants `temporary_limit` and `permanent_ban`.
- Produces: `ListUserDistillationViolationRecords(userID int, pageInfo *common.PageInfo) ([]*DistillationViolationRecord, int64, error)`.
- Changes: `AdvanceDistillationPenalty(trigger DistillationTrigger) (*DistillationPenalty, error)`.

- [ ] **Step 1: Write failing transition-history tests**

Use explicit first and second triggers:

```go
firstTrigger := DistillationTrigger{
    UserID: 7, TriggeredAt: 1000, PenaltySeconds: 600,
    ObservationSeconds: 3600, RequestCount: 200,
    DetectionThreshold: 200, PenaltyRPM: 10,
}
_, err := AdvanceDistillationPenalty(firstTrigger)
require.NoError(t, err)

secondTrigger := firstTrigger
secondTrigger.TriggeredAt = 1700
_, err = AdvanceDistillationPenalty(secondTrigger)
require.NoError(t, err)

var records []DistillationViolationRecord
require.NoError(t, DB.Order("id ASC").Find(&records).Error)
require.Len(t, records, 2)
assert.Equal(t, DistillationViolationActionTemporaryLimit, records[0].Action)
assert.Equal(t, int64(1600), records[0].EffectiveUntil)
assert.Equal(t, DistillationViolationActionPermanentBan, records[1].Action)
assert.Zero(t, records[1].EffectiveUntil)
```

Add tests proving a repeated call during the temporary phase creates no row, concurrent second triggers create exactly one permanent row, and `ClearDistillationPenalty` leaves both history rows intact.

- [ ] **Step 2: Run the history tests and verify RED**

Run:

```powershell
go test ./model -run 'TestAdvanceDistillationPenalty.*History|TestDistillationViolation' -count=1
```

Expected: build failure because the record and trigger types do not exist.

- [ ] **Step 3: Define the portable history model and pagination query**

Create the model with GORM-compatible scalar columns and a composite unique index:

```go
type DistillationViolationRecord struct {
    Id                 int                         `json:"id"`
    UserId             int                         `json:"-" gorm:"not null;index;uniqueIndex:idx_distillation_violation_transition,priority:1"`
    CycleStartedAt     int64                       `json:"cycle_started_at" gorm:"not null;uniqueIndex:idx_distillation_violation_transition,priority:2"`
    TriggeredAt        int64                       `json:"triggered_at" gorm:"not null;index"`
    RequestCount       int                         `json:"request_count" gorm:"not null"`
    DetectionThreshold int                         `json:"detection_threshold" gorm:"not null"`
    PenaltyRPM         int                         `json:"penalty_rpm" gorm:"not null"`
    Action             DistillationViolationAction `json:"action" gorm:"type:varchar(32);not null;uniqueIndex:idx_distillation_violation_transition,priority:3"`
    EffectiveUntil     int64                       `json:"effective_until" gorm:"not null"`
    CreatedAt          int64                       `json:"created_at" gorm:"autoCreateTime;column:created_at"`
}
```

`ListUserDistillationViolationRecords` must enforce positive user IDs, clamp page size to 100, filter by authenticated user ID, order by `triggered_at DESC, id DESC`, and return an empty slice instead of null.

- [ ] **Step 4: Insert history in the penalty transaction**

Replace positional transition arguments with:

```go
type DistillationTrigger struct {
    UserID              int
    TriggeredAt         int64
    PenaltySeconds      int64
    ObservationSeconds  int64
    RequestCount        int
    DetectionThreshold  int
    PenaltyRPM          int
}
```

Validate all fields before opening the transaction. Insert the temporary record only when a new penalty cycle is created. Insert the permanent record only when the observation state actually updates to permanent. Use `clause.OnConflict{DoNothing: true}` with the composite unique index as concurrency defense, but keep the penalty transition and record write in the same transaction.

- [ ] **Step 5: Add both migration paths**

Add `&DistillationViolationRecord{}` to `migrateDB` and the named `migrateDBFast` list next to `DistillationPenalty`.

- [ ] **Step 6: Run model tests and commit**

Run:

```powershell
gofmt -w model/distillation_violation_record.go model/distillation_violation_record_test.go model/distillation_penalty.go model/distillation_penalty_test.go model/main.go
go test ./model -run 'Distillation' -count=1
git add model/distillation_violation_record.go model/distillation_violation_record_test.go model/distillation_penalty.go model/distillation_penalty_test.go model/main.go
git commit -m "feat: persist distillation violation history"
```

Expected: PASS with exactly two records across a first/second strike cycle and history retained after clearing.

---

### Task 3: Fixed Natural-Minute Counters and Single Crossing

**Files:**
- Modify: `service/distillation_rate_limit_runtime.go`
- Create: `service/distillation_rate_limit_runtime_test.go`
- Modify: `service/distillation_rate_limit.go`
- Modify: `service/distillation_rate_limit_test.go`
- Modify: `service/distillation_penalty_cache.go`
- Modify: `controller/rate_limit_test.go`

**Interfaces:**
- Produces: `distillationCounterResult{Allowed bool, Count int, Crossed bool}`.
- Changes runtime contract to `Take(ctx context.Context, key string, maximum int, now time.Time) (distillationCounterResult, error)`.
- Consumes: `model.DistillationTrigger` from Task 2.

- [ ] **Step 1: Write failing Redis and memory counter tests**

Add deterministic natural-minute cases using `time.Unix(125, 0)` and `time.Unix(180, 0)`:

```go
first, err := store.Take(ctx, "detection:7", 2, time.Unix(125, 0))
require.NoError(t, err)
assert.Equal(t, distillationCounterResult{Allowed: true, Count: 1}, first)

crossing, err := store.Take(ctx, "detection:7", 2, time.Unix(179, 0))
require.NoError(t, err)
assert.Equal(t, distillationCounterResult{Allowed: true, Count: 2, Crossed: true}, crossing)

over, err := store.Take(ctx, "detection:7", 2, time.Unix(179, 0))
require.NoError(t, err)
assert.Equal(t, distillationCounterResult{Allowed: false, Count: 3}, over)

reset, err := store.Take(ctx, "detection:7", 2, time.Unix(180, 0))
require.NoError(t, err)
assert.Equal(t, distillationCounterResult{Allowed: true, Count: 1}, reset)
```

Run the same contract against memory and miniredis. Add a Redis concurrency test asserting exactly one of `maximum` accepted calls returns `Crossed == true`, and confirm only scalar string keys exist, with no ZSET or sequence key.

- [ ] **Step 2: Run runtime tests and verify RED**

Run:

```powershell
go test ./service -run 'Test.*DistillationFixedMinute' -count=1
```

Expected: build failure because `Take` and `distillationCounterResult` do not exist.

- [ ] **Step 3: Implement scalar Redis and partitioned memory stores**

Use a minute-suffixed key and a Lua script that performs `INCR`, sets expiry only when count is one, and returns `{allowed, count, crossed}`. The expiry must be at least 120 seconds so the current bucket cannot disappear early.

For memory mode, store one `{minute, count}` scalar per base key in a fixed number of mutex-protected partitions selected from a stable hash of the key. Reset the scalar when `now.Unix()/60` changes. A single process-level cleanup loop removes records not touched for more than two minutes; do not store timestamp slices.

- [ ] **Step 4: Write failing service tests for single transition and beyond-threshold behavior**

Change the fake runtime to queue literal `distillationCounterResult` values. Assert:

```go
runtimeStore.results = []distillationCounterResult{
    {Allowed: true, Count: 1},
    {Allowed: true, Count: 2, Crossed: true},
    {Allowed: false, Count: 3},
}
```

The first crossing calls `penalties.Advance` exactly once and passes a `model.DistillationTrigger` containing threshold `2`, request count `2`, and RPM `1`. The over-threshold request uses the temporary counter path without a second `Advance` call. A transition error deletes only the current detection bucket and allows a later crossing to retry.

- [ ] **Step 5: Run service tests and verify RED**

Run:

```powershell
go test ./service -run 'TestDistillation' -count=1
```

Expected: compile or assertion failure because the service still uses rolling-window `RequestWithCount` and positional `Advance` arguments.

- [ ] **Step 6: Integrate fixed counters with penalty state**

Pass the injected `dependencies.now()` value into every `Take` call so tests, Redis, and memory share the same bucket. Only `Crossed == true` constructs and submits `model.DistillationTrigger`. Keep the threshold request successful. For `Allowed == false` without a crossing, apply the configured temporary RPM counter and never call `Advance` again.

Update `cachedDistillationPenaltyStore.Advance` to accept the trigger struct and cache or invalidate the returned state consistently. Update clearing to delete the current-minute detection and temporary keys. Preserve permanent bans when detection is disabled.

- [ ] **Step 7: Run service and clearing regression tests and commit**

Run:

```powershell
gofmt -w service/distillation_rate_limit_runtime.go service/distillation_rate_limit_runtime_test.go service/distillation_rate_limit.go service/distillation_rate_limit_test.go service/distillation_penalty_cache.go controller/rate_limit_test.go
go test ./service ./controller -run 'Distillation' -count=1
git add service/distillation_rate_limit_runtime.go service/distillation_rate_limit_runtime_test.go service/distillation_rate_limit.go service/distillation_rate_limit_test.go service/distillation_penalty_cache.go controller/rate_limit_test.go
git commit -m "perf: use fixed-minute distillation counters"
```

Expected: PASS, with one durable transition at the threshold and natural-minute reset behavior.

---

### Task 4: Authenticated Self Violation History API

**Files:**
- Modify: `controller/rate_limit.go`
- Modify: `controller/rate_limit_test.go`
- Modify: `router/api-router.go`

**Interfaces:**
- Produces: `GET /api/user/distillation/violations/self?p=<page>&page_size=<size>`.
- Consumes: `ListUserDistillationViolationRecords` from Task 2.

- [ ] **Step 1: Write a failing controller isolation and pagination test**

Create records for user IDs 301 and 999. Put `id=301` in Gin context while also supplying `user_id=999` in the query string. Assert total two, newest-first IDs for user 301 only, and no serialized `user_id` field:

```go
assert.Equal(t, 2, response.Data.Total)
require.Len(t, response.Data.Items, 1)
assert.Equal(t, secondRecord.Id, response.Data.Items[0].Id)
assert.NotContains(t, recorder.Body.String(), "user_id")
```

- [ ] **Step 2: Run the test and verify RED**

Run:

```powershell
go test ./controller -run 'TestListSelfDistillationViolationRecords' -count=1
```

Expected: build failure because the controller does not exist.

- [ ] **Step 3: Implement the thin controller and authenticated route**

Add:

```go
func ListSelfDistillationViolationRecords(c *gin.Context) {
    pageInfo := common.GetPageQuery(c)
    items, total, err := model.ListUserDistillationViolationRecords(c.GetInt("id"), pageInfo)
    // ApiError on error; otherwise populate standard PageInfo and ApiSuccess.
}
```

Register the GET route in `selfRoute`, protected by `middleware.UserAuth()`.

- [ ] **Step 4: Run tests and commit**

Run:

```powershell
gofmt -w controller/rate_limit.go controller/rate_limit_test.go router/api-router.go
go test ./controller -run 'DistillationViolation|DistillationPenalty' -count=1
git add controller/rate_limit.go controller/rate_limit_test.go router/api-router.go
git commit -m "feat: expose self distillation violation history"
```

Expected: PASS and the query-supplied user ID has no effect.

---

### Task 5: Rebate Approve-All Administrator UI

**Files:**
- Modify: `web/src/features/rebate-approvals/api.ts`
- Modify: `web/src/features/rebate-approvals/types.ts`
- Create: `web/src/features/rebate-approvals/components/rebate-approve-all-action.tsx`
- Create: `web/src/features/rebate-approvals/components/__tests__/rebate-approve-all-action.test.tsx`
- Modify: `web/src/features/rebate-approvals/components/rebate-approvals-table.tsx`

**Interfaces:**
- Produces: `approveAllPendingRebateTransferRequests(): Promise<ApiResponse<RebateApproveAllResult>>`.
- Produces: `RebateApproveAllAction` with `pendingCount`, `isCountLoading`, and `onApproveAll` props.

- [ ] **Step 1: Write failing component interaction tests**

Use happy-dom, a real React Query client, and a controlled `onApproveAll` promise. Protect these behaviors:

```tsx
expect(button.disabled).toBe(true) // pendingCount = 0
button.click()                     // pendingCount = 3
expect(dialog.textContent).toContain('3')
confirmButton.click()
expect(callCount).toBe(1)
resolveApproval({ success: true, data: { approved_count: 3 } })
expect(queryClient.getQueryState(['rebate-approvals'])?.isInvalidated).toBe(true)
```

Add a business-failure case that keeps the action usable and renders one error toast without a success toast.

- [ ] **Step 2: Run the component test and verify RED**

Run:

```powershell
bun test src/features/rebate-approvals/components/__tests__/rebate-approve-all-action.test.tsx
```

Working directory: `web`.

Expected: module-not-found failure for `rebate-approve-all-action`.

- [ ] **Step 3: Implement API types and the focused action component**

Add:

```ts
export interface RebateApproveAllResult {
  approved_count: number
}

export async function approveAllPendingRebateTransferRequests(): Promise<
  ApiResponse<RebateApproveAllResult>
>
```

The component uses one `useMutation`, an `AlertDialog`, a `CheckCheck` icon, pending/disabled states, business-error handling, success count toast, and prefix invalidation for `['rebate-approvals']`.

- [ ] **Step 4: Add the independent pending-count query to the table**

Fetch `getRebateTransferRequests({ p: 1, page_size: 1, status: 'pending' })` under `['rebate-approvals', 'pending-count']`. Pass its total to `RebateApproveAllAction` through `toolbarProps.preActions`. The current status filter and page must not affect this query.

- [ ] **Step 5: Run focused frontend tests and commit**

Run from `web`:

```powershell
bun test src/features/rebate-approvals/components/__tests__/rebate-approve-all-action.test.tsx
bun test src/features/rebate-approvals/components/__tests__/rebate-approval-row-actions.test.tsx
bun run typecheck
```

Then:

```powershell
git add web/src/features/rebate-approvals/api.ts web/src/features/rebate-approvals/types.ts web/src/features/rebate-approvals/components/rebate-approve-all-action.tsx web/src/features/rebate-approvals/components/__tests__/rebate-approve-all-action.test.tsx web/src/features/rebate-approvals/components/rebate-approvals-table.tsx
git commit -m "feat(web): approve all pending rebates"
```

Expected: PASS and no changes to the existing row action behavior.

---

### Task 6: Personal Violation Records Page and Sidebar Entry

**Files:**
- Create: `web/src/features/violation-records/types.ts`
- Create: `web/src/features/violation-records/api.ts`
- Create: `web/src/features/violation-records/lib/violation-display.ts`
- Create: `web/src/features/violation-records/lib/__tests__/violation-display.test.ts`
- Create: `web/src/features/violation-records/components/violation-records-columns.tsx`
- Create: `web/src/features/violation-records/components/violation-records-table.tsx`
- Create: `web/src/features/violation-records/components/__tests__/violation-records-table.test.tsx`
- Create: `web/src/features/violation-records/index.tsx`
- Create: `web/src/routes/_authenticated/violations/index.tsx`
- Modify generated: `web/src/routeTree.gen.ts`
- Modify: `web/src/hooks/use-sidebar-data.ts`
- Modify: `web/src/hooks/use-sidebar-config.ts`
- Modify: `web/src/features/system-settings/maintenance/config.ts`
- Modify: `web/src/features/system-settings/maintenance/sidebar-modules-section.tsx`

**Interfaces:**
- Produces: `getViolationRecords(page: number, pageSize: number)`.
- Produces: authenticated `/violations` route with `{ page, pageSize }` search state.
- Produces: Personal navigation order Wallet, Profile, Violation Records.

- [ ] **Step 1: Write failing display and table tests**

Protect literal action mapping and effective-time behavior:

```ts
expect(getViolationActionLabel('temporary_limit')).toBe('Temporary limit')
expect(getViolationActionLabel('permanent_ban')).toBe('Permanent ban')
expect(getViolationEffectiveLabel(permanentRecord)).toBe('Permanent')
```

Render the table with a mocked network boundary returning one temporary and one permanent record. Assert the table exposes detection time, `200 / 200`, translated action labels, permanent text, and a next-page control driven by server total. Add an empty response assertion for the dedicated empty state.

- [ ] **Step 2: Run tests and verify RED**

Run from `web`:

```powershell
bun test src/features/violation-records/lib/__tests__/violation-display.test.ts src/features/violation-records/components/__tests__/violation-records-table.test.tsx
```

Expected: module-not-found failures because the feature does not exist.

- [ ] **Step 3: Implement types, API, display mapping, and columns**

Define the wire type with numeric timestamps and the two action literals. Use `formatTimestamp` for detection/effective times, `StatusBadge` for action, and a compact tabular `request_count / detection_threshold` cell. Unknown action values fall back to a neutral `Unknown` label rather than throwing.

- [ ] **Step 4: Implement the paginated page and authenticated route**

Use `useTableUrlState`, `useQuery`, `useDataTable`, and `DataTablePage` with manual pagination. The route search schema is:

```ts
const violationsSearchSchema = z.object({
  page: z.number().optional().catch(1),
  pageSize: z.number().optional().catch(20),
})
```

Authentication only requires an existing user; redirect unauthenticated access to `/sign-in` through the established authenticated layout behavior rather than adding an administrator check.

- [ ] **Step 5: Add sidebar configuration and generate the route tree**

Add `ShieldAlert` after Profile, map `/violations` to `personal.violations`, and set `violations: true` in both sidebar default definitions. Add matching title/description metadata in the sidebar settings page. Run `bun run build` once to regenerate `routeTree.gen.ts`; do not hand-edit generated route entries.

- [ ] **Step 6: Run feature tests, typecheck, and commit**

Run from `web`:

```powershell
bun test src/features/violation-records
bun run typecheck
```

Then stage only Task 6 files and commit:

```powershell
git commit -m "feat(web): show personal violation records"
```

Expected: PASS, route tree includes `/violations`, and the Personal menu order is stable.

---

### Task 7: API Key Notice Size and Complete i18n

**Files:**
- Modify: `web/src/features/keys/components/api-key-notice.tsx`
- Modify: `web/src/features/keys/components/__tests__/api-key-notice.test.tsx`
- Temporarily create then delete: `web/scripts/add-missing-keys.mjs`
- Modify via script only: `web/src/i18n/locales/en.json`
- Modify via script only: `web/src/i18n/locales/zh.json`
- Modify via script only: `web/src/i18n/locales/zh-TW.json`
- Modify via script only: `web/src/i18n/locales/fr.json`
- Modify via script only: `web/src/i18n/locales/ja.json`
- Modify via script only: `web/src/i18n/locales/ru.json`
- Modify via script only: `web/src/i18n/locales/vi.json`
- Update generated reports only if `bun run i18n:sync` changes tracked report files.

**Interfaces:**
- Preserves existing notice props and layout.
- Adds translations for every new button, dialog, toast, page, table, empty-state, action, and sidebar-settings string.

- [ ] **Step 1: Write the failing notice typography test**

Extend the existing static markup assertion:

```ts
expect(html).toMatch(/<h[^>]*class="[^"]*text-sm/)
expect(html).toMatch(/class="[^"]*text-sm[^"]*break-words/)
expect(html).not.toContain('text-xs')
```

- [ ] **Step 2: Run the notice test and verify RED**

Run from `web`:

```powershell
bun test src/features/keys/components/__tests__/api-key-notice.test.tsx
```

Expected: FAIL because title and description still contain `text-xs`.

- [ ] **Step 3: Change title and body to compact 14px text**

Replace `text-xs` with `text-sm leading-5` on both title and description. Keep width, padding, wrapping, `max-h-12`, and overflow behavior unchanged.

- [ ] **Step 4: Add every new translation through the required script**

Create `web/scripts/add-missing-keys.mjs` with a `newKeys` object containing exact values for all seven locales. It must load each locale JSON, update only `translation`, alphabetically sort keys, and write valid UTF-8 JSON. Include at least these English keys after confirming exact call sites:

```text
Approve All
Approve all pending rebate requests?
Approve all {{count}} pending rebate requests and credit every user balance? This action is atomic and cannot be partially completed.
Approved {{count}} rebate requests
No pending rebate requests
Violation Records
Detection Time
Request Count / Threshold
Action Taken
Effective Until
Temporary limit
Permanent ban
Permanent
No Violation Records Found
No distillation violations have been recorded for this account.
Distillation violation history
Review when distillation protection was triggered for your account.
Control whether users can review their distillation violation history.
```

Run:

```powershell
node scripts/add-missing-keys.mjs
bun run i18n:sync
Remove-Item -LiteralPath scripts/add-missing-keys.mjs
```

Read `_reports/_sync-report.json` and require `missingCount: 0` for every locale. Do not accept untranslated new UI strings.

- [ ] **Step 5: Run focused tests, typecheck, lint, and commit**

Run from `web`:

```powershell
bun test src/features/keys/components/__tests__/api-key-notice.test.tsx
bun test src/features/rebate-approvals/components/__tests__/rebate-approve-all-action.test.tsx
bun test src/features/rebate-approvals/components/__tests__/rebate-approval-row-actions.test.tsx
bun test src/features/violation-records/components/__tests__/violation-records-table.test.tsx
bun test src/features/violation-records/lib/__tests__/violation-display.test.ts
bun run typecheck
bun run lint
bun run format:check
```

Stage the notice files and only the new i18n hunks, then verify the temporary
script is absent. The locale files already contain preserved uncommitted user
work; if the new hunks cannot be separated non-interactively, leave those
locale files unstaged for the final combined review rather than attributing or
reverting older changes. Commit the safely isolated files:

```powershell
git commit -m "feat(web): translate approvals and violation history"
```

Expected: all commands PASS and all seven locales contain every new key.

---

### Task 8: Full Regression and Browser Verification

**Files:**
- Modify only files required to fix failures caused by Tasks 1-7.

**Interfaces:**
- Verifies every contract delivered by the prior tasks.

- [ ] **Step 1: Run complete affected backend packages**

Run:

```powershell
go test ./model ./service ./controller ./router -count=1
go test ./middleware -count=1
go vet ./model ./service ./controller ./router
```

Expected: PASS without race, migration, or serialization failures.

- [ ] **Step 2: Run frontend regression and production build**

Run from `web`:

```powershell
bun test src/features/rebate-approvals/components/__tests__/rebate-approve-all-action.test.tsx
bun test src/features/rebate-approvals/components/__tests__/rebate-approval-row-actions.test.tsx
bun test src/features/rebate-approvals/components/__tests__/rebate-approval-detail-dialog.test.tsx
bun test src/features/violation-records/components/__tests__/violation-records-table.test.tsx
bun test src/features/violation-records/lib/__tests__/violation-display.test.ts
bun test src/features/keys/components/__tests__/api-key-notice.test.tsx
bun test src/components/data-table/toolbar/__tests__/toolbar.test.tsx
bun run typecheck
bun run lint
bun run format:check
bun run build
```

Expected: PASS and the route tree/build include `/violations`.

- [ ] **Step 3: Start the local service and verify administrator workflow**

Start the existing backend/frontend development configuration without exposing credentials. In a signed-in administrator browser session:

1. open `/rebate-approvals`;
2. verify `Approve All` is disabled at zero and shows the cross-page pending count otherwise;
3. open the confirmation and verify the count and atomicity warning;
4. approve test fixtures and verify the table/count refresh once;
5. verify the 14px API key notice on `/keys` in light and dark themes.

- [ ] **Step 4: Verify personal violation history at desktop and mobile widths**

In a common-user session:

1. verify Personal navigation order Wallet, Profile, Violation Records;
2. open `/violations` at 1440x900 and 390x844;
3. verify no overlap, clipped text, or horizontal toolbar overflow;
4. verify temporary and permanent records show correct times and labels;
5. verify the user cannot request another user's records by changing URL parameters.

- [ ] **Step 5: Inspect the final diff and commit verification fixes**

Run:

```powershell
git diff --check
git status --short
git diff --stat 2f09512c2..HEAD
```

Confirm unrelated pre-existing wallet, API key group, button, theme, and locale changes remain preserved. Commit only any verification fixes with a scoped message. Do not push or merge unless the user explicitly requests it.
