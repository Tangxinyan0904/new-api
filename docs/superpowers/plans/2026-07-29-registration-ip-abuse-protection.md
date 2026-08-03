# Registration IP Abuse Protection Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Enforce a durable three-account-per-exact-IP self-service registration limit, disable all four current-cycle accounts on the fourth registration, and give root administrators searchable unblock and exact-IP allowlist controls.

**Architecture:** Keep registration ownership in `model/` so password, OAuth, WeChat, and Telegram all use one main-database transaction for user creation, provider binding, IP association, counting, and threshold enforcement. Store IP state and per-user associations in portable GORM models, preserve administrator status decisions with an explicit restoration marker, expose root-only controller endpoints, and render the management workflow as a dedicated Security Settings section.

**Tech Stack:** Go 1.22+, Gin, GORM v2, `net/netip`, SQLite/MySQL/PostgreSQL, React 19, TypeScript, TanStack Query, Base UI/shadcn components, i18next, Bun/Vitest, testify.

---

## File Structure

- Create `model/registration_ip_abuse.go`: canonicalization, persistence models, atomic self-service registration, unblock/allowlist transitions, manual-status protection, and administrator queries.
- Create `model/registration_ip_abuse_test.go`: model behavior, concurrency, restoration, allowlist, and search regressions.
- Modify `model/main.go`: migrate both new tables in normal and fast migration modes.
- Create `controller/registration_ip_abuse.go`: shared registration-error response and root administration handlers.
- Create `controller/registration_ip_abuse_test.go`: endpoint validation, pagination, mutations, and audit behavior.
- Modify `controller/user.go`, `controller/oauth.go`, `controller/wechat.go`, and `controller/telegram.go`: route every self-service creation path through the shared transaction.
- Create `controller/registration_ip_entrypoints_test.go`: password, OAuth, WeChat, and Telegram integration behavior.
- Modify `router/api-router.go`: register root-only administration routes.
- Modify `i18n/keys.go` and `i18n/locales/{en,zh-CN}.yaml`: stable registration-protection responses.
- Create `web/default/src/features/system-settings/security/registration-ip-abuse/{api.ts,types.ts,registration-ip-abuse-section.tsx,registration-ip-abuse-section.test.tsx}`: data contract and complete administrator workflow.
- Modify `web/default/src/features/system-settings/security/section-registry.tsx`: add the Security navigation entry.
- Create then remove `web/default/scripts/add-missing-keys.mjs`: apply all locale copy through the required script workflow.
- Modify through script `web/default/src/i18n/locales/{en,zh,zh-TW,fr,ja,ru,vi}.json`.

## Task 1: Add Portable IP State and Canonicalization

**Files:**

- Create: `model/registration_ip_abuse_test.go`
- Create: `model/registration_ip_abuse.go`
- Modify: `model/main.go`

- [ ] **Step 1: Write normalization and migration tests**

Add a table test that requires `NormalizeRegistrationIP` to return `203.0.113.7`, unmap `::ffff:203.0.113.7`, canonicalize IPv6, strip a zone from `fe80::1%eth0`, and reject empty values, CIDR, host/port pairs, and hostnames. Add an AutoMigrate test that migrates `RegistrationIPState` and `RegistrationIPAccount`, inserts one row of each, and verifies their unique IP and user constraints.

~~~go
func TestNormalizeRegistrationIP(t *testing.T) {
    tests := []struct {
        name string
        raw string
        want string
        wantErr bool
    }{
        {name: "IPv4", raw: "203.0.113.7", want: "203.0.113.7"},
        {name: "mapped IPv4", raw: "::ffff:203.0.113.7", want: "203.0.113.7"},
        {name: "IPv6", raw: "2001:0db8::1", want: "2001:db8::1"},
        {name: "zone", raw: "fe80::1%eth0", want: "fe80::1"},
        {name: "CIDR", raw: "203.0.113.0/24", wantErr: true},
        {name: "port", raw: "203.0.113.7:443", wantErr: true},
        {name: "host", raw: "example.com", wantErr: true},
        {name: "empty", raw: "", wantErr: true},
    }
    // Run each case with require.Error/NoError and assert.Equal.
}
~~~

- [ ] **Step 2: Run the focused model test and verify RED**

Run: `go test ./model -run 'TestNormalizeRegistrationIP|TestRegistrationIPModelsMigrate' -count=1`

Expected: FAIL because the models and normalizer do not exist.

- [ ] **Step 3: Define the portable models and exact-IP parser**

Implement these contracts without boolean default tags or dialect-specific SQL:

~~~go
const RegistrationIPAccountLimit = 3

var (
    ErrRegistrationIPBlocked = errors.New("registration IP is blocked")
    ErrRegistrationIPLimitExceeded = errors.New("registration IP account limit exceeded")
)

type RegistrationIPState struct {
    Id int `json:"id"`
    IP string `json:"ip" gorm:"type:varchar(45);not null;uniqueIndex"`
    CurrentCycle int `json:"current_cycle" gorm:"not null"`
    RegistrationCount int `json:"registration_count" gorm:"not null"`
    BlockedAt int64 `json:"blocked_at" gorm:"not null;index"`
    Allowlisted bool `json:"allowlisted" gorm:"not null;index"`
    CreatedAt int64 `json:"created_at" gorm:"autoCreateTime;column:created_at"`
    UpdatedAt int64 `json:"updated_at" gorm:"autoUpdateTime;column:updated_at"`
}

type RegistrationIPAccount struct {
    Id int `json:"id"`
    UserId int `json:"user_id" gorm:"not null;uniqueIndex"`
    RegistrationIP string `json:"registration_ip" gorm:"type:varchar(45);not null;index"`
    RegistrationCycle int `json:"registration_cycle" gorm:"not null;index"`
    AutoDisabledAt int64 `json:"auto_disabled_at" gorm:"not null"`
    RestoreEligible bool `json:"restore_eligible" gorm:"not null"`
    CreatedAt int64 `json:"created_at" gorm:"autoCreateTime;column:created_at"`
    UpdatedAt int64 `json:"updated_at" gorm:"autoUpdateTime;column:updated_at"`
}
~~~

`NormalizeRegistrationIP` trims input, removes a trailing IPv6 zone before `netip.ParseAddr`, calls `Unmap()`, and returns `Addr.String()`.

- [ ] **Step 4: Register both models in normal and fast migrations**

Add `&RegistrationIPState{}` and `&RegistrationIPAccount{}` to `migrateDB` and `migrateDBFast`.

- [ ] **Step 5: Run tests and commit**

Run: `go test ./model -run 'TestNormalizeRegistrationIP|TestRegistrationIPModelsMigrate' -count=1`

Expected: PASS.

Commit: `git add model/registration_ip_abuse.go model/registration_ip_abuse_test.go model/main.go && git commit -m "feat(registration): add IP abuse state models"`

## Task 2: Make Self-Service Registration Atomic

**Files:**

- Modify: `model/registration_ip_abuse_test.go`
- Modify: `model/registration_ip_abuse.go`

- [ ] **Step 1: Write threshold, rollback, and concurrency tests**

Use a shared in-memory SQLite database and explicit fixtures. Protect these observable contracts: registrations one through three remain enabled; registration four is persisted and disables exactly the four current-cycle users; a fifth attempt returns `ErrRegistrationIPBlocked` without a user; provider callback failure rolls back all writes; and concurrent attempts cannot leave more than three current-cycle accounts enabled.

~~~go
type SelfServiceRegistrationResult struct {
    CanonicalIP string
    TriggeredBlock bool
    AffectedUserIDs []int
}

result, err := RegisterSelfServiceUser(
    user,
    0,
    "203.0.113.10",
    func(tx *gorm.DB) error { return nil },
)
~~~

- [ ] **Step 2: Run the behavior tests and verify RED**

Run: `go test ./model -run 'TestRegisterSelfServiceUser' -count=1`

Expected: FAIL because shared registration has not been implemented.

- [ ] **Step 3: Implement the single registration transaction**

Canonicalize first. Inside one main-database transaction create the state with `clause.OnConflict{DoNothing: true}`, load it with `lockForUpdate(tx)`, reject a blocked non-allowlisted state, insert the user and provider callback, create the association, skip counting for allowlisted state, otherwise increment count, and on count four disable still-enabled current-cycle users and mark only them restore-eligible before blocking the IP. After commit run `FinishInsert`, invalidate user/token caches for affected IDs, and emit a warning containing only canonical IP and IDs.

- [ ] **Step 4: Run focused and package tests**

Run:

~~~powershell
go test ./model -run 'TestRegisterSelfServiceUser' -count=1
go test ./model -count=1
~~~

Expected: PASS.

- [ ] **Step 5: Commit the atomic registration service**

Commit: `git add model/registration_ip_abuse.go model/registration_ip_abuse_test.go && git commit -m "feat(registration): enforce per-IP account limit"`

## Task 3: Add Safe Unblock, Allowlist, Search, and Manual Status Semantics

**Files:**

- Modify: `model/registration_ip_abuse_test.go`
- Modify: `model/registration_ip_abuse.go`

- [ ] **Step 1: Write administrator behavior tests**

Add deterministic tests for eligible-only non-deleted restoration; administrator disable/enable clearing restoration markers; cycle reset; blocked-IP allowlisting; idempotent duplicate add/remove; removal starting a new zero-count cycle; blocked search by exact IP, numeric ID, username, and display name; deleted-account display; and paginated allowlist ordering.

~~~go
type RegistrationIPMutationResult struct {
    CanonicalIP string `json:"ip"`
    AffectedUserIDs []int `json:"affected_user_ids"`
    AffectedAccountCount int `json:"affected_account_count"`
    Allowlisted bool `json:"allowlisted"`
}

func UnblockRegistrationIP(rawIP string) (*RegistrationIPMutationResult, error)
func AddRegistrationIPAllowlist(rawIP string) (*RegistrationIPMutationResult, error)
func RemoveRegistrationIPAllowlist(rawIP string) (*RegistrationIPMutationResult, error)
func SetUserStatusByAdmin(userID int, status int) error
func ListBlockedRegistrationIPs(keyword string, pageInfo *common.PageInfo) ([]*BlockedRegistrationIPListItem, int64, error)
func ListRegistrationIPAllowlist(pageInfo *common.PageInfo) ([]*RegistrationIPAllowlistItem, int64, error)
~~~

- [ ] **Step 2: Run administrator model tests and verify RED**

Run: `go test ./model -run 'Test(Unblock|RegistrationIPAllowlist|SetUserStatusByAdmin|ListBlockedRegistrationIPs)' -count=1`

Expected: FAIL because the administration functions do not exist.

- [ ] **Step 3: Implement locked transitions and safe DTOs**

Use `lockForUpdate(tx)` for state mutation. Restore only scoped users with disabled status, clear old-cycle restoration markers even for deleted users, commit before cache invalidation, and never unblock through `SetUserStatusByAdmin`. Search association/user data with portable GORM joins and return no credentials or provider identifiers.

- [ ] **Step 4: Run model tests and commit**

Run: `go test ./model -run 'RegistrationIP|SetUserStatusByAdmin' -count=1`

Expected: PASS.

Commit: `git add model/registration_ip_abuse.go model/registration_ip_abuse_test.go && git commit -m "feat(registration): manage blocked IP recovery"`

## Task 4: Route Every Self-Service Entry Point Through the Shared Service

**Files:**

- Create: `controller/registration_ip_entrypoints_test.go`
- Create: `controller/registration_ip_abuse.go`
- Modify: `controller/user.go`
- Modify: `controller/oauth.go`
- Modify: `controller/wechat.go`
- Modify: `controller/telegram.go`
- Modify: `i18n/keys.go`
- Modify: `i18n/locales/en.yaml`
- Modify: `i18n/locales/zh-CN.yaml`

- [ ] **Step 1: Write entry-point integration tests**

Exercise real behavior for password, built-in/custom OAuth, WeChat, and Telegram. Require the fourth account and existing side effects to persist without login, later blocked registrations to create nothing, existing bound users to continue logging in, and administrator/root creation to remain untracked.

- [ ] **Step 2: Run entry-point tests and verify RED**

Run: `go test ./controller -run 'TestRegistrationIP.*(Password|OAuth|WeChat|Telegram|Admin)' -count=1`

Expected: FAIL because current entry points call `Insert`/`InsertWithTx` directly and Telegram does not self-create.

- [ ] **Step 3: Add stable localized backend errors**

Add `registration_ip.blocked`, `registration_ip.limit_exceeded`, and `registration_ip.invalid` constants and English/Chinese translations. The threshold message must explicitly state that the account was created but disabled; the blocked message must state that an administrator must unblock the IP.

- [ ] **Step 4: Integrate password and OAuth creation**

Password registration calls `RegisterSelfServiceUser`, preserves default-token creation, and emits the threshold response only afterward. OAuth supplies either custom binding creation or built-in provider ID updates as the transaction callback and maps the threshold result before login.

- [ ] **Step 5: Integrate WeChat and add Telegram self-registration**

WeChat sets `WeChatId` before the shared call. Telegram derives a bounded username/display name from verified widget fields, sets `TelegramId`, self-creates when registration is enabled, maps threshold/blocked errors, checks status, and otherwise preserves existing-user login.

- [ ] **Step 6: Protect manual administrator status changes**

In `ManageUser`, use `model.SetUserStatusByAdmin` for both `disable` and `enable`; keep role checks, audits, and non-status actions unchanged.

- [ ] **Step 7: Run controller/model tests and commit**

Run:

~~~powershell
go test ./controller -run 'RegistrationIP|ManageUser' -count=1
go test ./controller ./model -count=1
~~~

Expected: PASS.

Commit: `git add controller/registration_ip_abuse.go controller/registration_ip_entrypoints_test.go controller/user.go controller/oauth.go controller/wechat.go controller/telegram.go i18n && git commit -m "feat(registration): protect all self-service entry points"`

## Task 5: Expose Root-Only Administration APIs and Audits

**Files:**

- Create: `controller/registration_ip_abuse_test.go`
- Modify: `controller/registration_ip_abuse.go`
- Modify: `controller/audit.go`
- Modify: `router/api-router.go`

- [ ] **Step 1: Write endpoint and audit tests**

Cover paginated blocked search/allowlist, invalid/CIDR rejection, idempotent mutations, unblock result data, and structured actions `registration_ip.unblock`, `registration_ip.allowlist_add`, and `registration_ip.allowlist_remove` with canonical IP, affected count/IDs, and resulting state.

- [ ] **Step 2: Run controller tests and verify RED**

Run: `go test ./controller -run 'TestRegistrationIPAbuse' -count=1`

Expected: FAIL because handlers and templates do not exist.

- [ ] **Step 3: Implement handlers and root routes**

Register `/api/registration-ip-abuse/blocked`, `/:ip/unblock`, `/allowlist` GET/POST, and `/allowlist/:ip` DELETE behind `middleware.RootAuth()`. Use `common.DecodeJson`, `common.GetPageQuery`, `common.ApiSuccess`, and existing audit helpers.

- [ ] **Step 4: Run controller/router tests and commit**

Run: `go test ./controller ./router -run 'RegistrationIP' -count=1`

Expected: PASS.

Commit: `git add controller/registration_ip_abuse.go controller/registration_ip_abuse_test.go controller/audit.go router/api-router.go && git commit -m "feat(registration): add blocked IP admin API"`

## Task 6: Build the Security Settings Management Section

**Files:**

- Create: `web/default/src/features/system-settings/security/registration-ip-abuse/types.ts`
- Create: `web/default/src/features/system-settings/security/registration-ip-abuse/api.ts`
- Create: `web/default/src/features/system-settings/security/registration-ip-abuse/registration-ip-abuse-section.test.tsx`
- Create: `web/default/src/features/system-settings/security/registration-ip-abuse/registration-ip-abuse-section.tsx`
- Modify: `web/default/src/features/system-settings/security/section-registry.tsx`

- [ ] **Step 1: Write API and component tests**

Mock the project API client and protect search reset/debounce, refresh, loading/error/empty states, pagination, account disclosure, unblock confirmation, exact-IP validation, stronger blocked-IP allowlist confirmation, remove confirmation, pending controls, and cache invalidation.

- [ ] **Step 2: Run the focused test and verify RED**

Run from `web/default`: `bun test src/features/system-settings/security/registration-ip-abuse/registration-ip-abuse-section.test.tsx`

Expected: FAIL because feature files do not exist.

- [ ] **Step 3: Implement typed API calls**

Use the project `api` client with `skipBusinessError: true`, `encodeURIComponent` for IP path values, exact backend snake-case fields, and no `any`.

- [ ] **Step 4: Implement the complete responsive section**

Use TanStack Query with independent blocked/allowlist keys and parallel queries. Compose project Button, InputGroup, Table, Badge, Empty, Skeleton, Spinner, Collapsible, Tooltip, and AlertDialog components. Use Hugeicons for refresh, disclosure, add, remove, and unblock. Desktop uses a table; mobile uses compact un-nested item cards. Bound long IPv6/name cells and label icon-only controls accessibly.

- [ ] **Step 5: Register the section and verify**

Add `registration-ip-abuse` / `Registration Abuse Protection` to the Security registry, then rerun the focused test until it passes.

- [ ] **Step 6: Commit UI code**

Commit: `git add web/default/src/features/system-settings/security && git commit -m "feat(settings): manage registration IP protection"`

## Task 7: Add All Frontend Translations Through the Script Workflow

**Files:**

- Create temporarily: `web/default/scripts/add-missing-keys.mjs`
- Modify through script: `web/default/src/i18n/locales/{en,zh,zh-TW,fr,ja,ru,vi}.json`
- Remove: `web/default/scripts/add-missing-keys.mjs`

- [ ] **Step 1: Inventory every literal `t(...)` key**

Include headings, fixed-rule summary, search, columns, account fields, states, confirmations, validation, toasts, and pagination. Preserve interpolation placeholders.

- [ ] **Step 2: Apply all seven locales with the mandated script**

Create the exact `add-missing-keys.mjs` structure from `i18n-translate`, populate all languages, run `node scripts/add-missing-keys.mjs` and `bun run i18n:sync`, then remove the script.

- [ ] **Step 3: Verify translations and frontend quality**

Run:

~~~powershell
bun run i18n:sync
bun test src/features/system-settings/security/registration-ip-abuse/registration-ip-abuse-section.test.tsx
bun run typecheck
bunx oxlint -c .oxlintrc.json src/features/system-settings/security/registration-ip-abuse src/features/system-settings/security/section-registry.tsx
bunx oxfmt --check src/features/system-settings/security/registration-ip-abuse src/features/system-settings/security/section-registry.tsx
~~~

Expected: every locale has zero missing/extra keys and all commands PASS.

- [ ] **Step 4: Commit translations**

Commit: `git add web/default/src/i18n/locales && git commit -m "feat(i18n): translate registration IP controls"`

## Task 8: Complete Cross-Layer Verification

**Files:**

- Modify only files from Tasks 1-7 if verification exposes a defect.

- [ ] **Step 1: Run backend verification**

~~~powershell
go test ./model ./controller ./router -count=1
go test ./... -count=1
~~~

Expected: PASS. If the root embed fails only because `web/classic/dist` is missing, build classic with its existing package scripts first and rerun.

- [ ] **Step 2: Run frontend verification**

From `web/default`, run `bun test`, `bun run typecheck`, and `bun run build`.

- [ ] **Step 3: Browser-check desktop and mobile**

With a disposable database, verify search, disclosure, pagination, refresh, unblock, add/remove allowlist, mutation pending states, long IPv6 layout, and absence of overlaps at desktop/mobile widths.

- [ ] **Step 4: Verify the end-to-end acceptance sequence**

Register four accounts from one IP, confirm all four disabled and searchable, manually disable one, unblock the IP, confirm only other eligible accounts restored, register a new-cycle set, then allowlist and confirm later registrations bypass counting.

- [ ] **Step 5: Review and record verification fixes**

Run `git diff --check` and `git status --short`. Commit any verification-only corrections as `fix(registration): complete IP abuse verification`.
