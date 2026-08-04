# Non-Stream-Only Distillation Enforcement Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking. The user explicitly prohibited subagents for this work.

**Goal:** Make stream requests bypass all distillation detection and punishment while preserving every existing detection, temporary-limit, observation, and permanent-punishment rule for non-stream requests.

**Architecture:** Add an immediate stream guard at both the public service entry point and the internal dependency-injected function so stream requests cannot initialize runtime storage or read durable penalties. Keep the non-stream flow otherwise unchanged, and clarify permanent-punishment wording in the backend error, administrator penalty UI, and personal violation history.

**Tech Stack:** Go 1.22+, Gin, testify, React 19, TypeScript, Bun test, i18next, Rsbuild

## Global Constraints

- Do not use subagents; execute every task inline in the current worktree.
- Stream requests must return before runtime-counter, cache, or database access.
- Detection, temporary RPM limiting, observation, and permanent punishment apply only to non-stream requests.
- A permanent distillation penalty must continue to return HTTP 403 and `distillation_banned` for non-stream requests.
- Existing fixed natural-minute counter and threshold-transition behavior must not change.
- Do not add a setting, option, model field, schema migration, or API response field.
- Preserve existing penalty and violation-history records; no data conversion is required.
- Other user-specific and group-specific rate limits remain independent and unchanged.
- All frontend wording must use i18n keys and be translated for `en`, `fr`, `ja`, `ru`, `vi`, `zh-TW`, and `zh`.
- Locale writes must go through a temporary `web/scripts/add-missing-keys.mjs`, followed by `bun run i18n:sync`; never edit locale JSON directly.
- Preserve and do not stage the user's unrelated API-key, wallet, button, theme, and locale worktree changes.
- All manual source edits use `apply_patch`.

## File Map

- `service/distillation_rate_limit.go`: public and internal stream guards plus permanent non-stream error text.
- `service/distillation_rate_limit_test.go`: service-level stream bypass, storage bypass, permanent non-stream rejection, and error-copy regressions.
- `controller/rate_limit_test.go`: administrator penalty-clear integration updated to verify stream bypass and non-stream enforcement.
- `web/src/features/system-settings/request-limits/distillation-penalties.ts`: administrator permanent phase label key.
- `web/src/features/system-settings/request-limits/distillation-penalty-list.tsx`: mobile and desktop permanent timestamp headings.
- `web/src/features/system-settings/request-limits/distillation-penalties-table.tsx`: administrator description and empty-state wording.
- `web/src/features/system-settings/request-limits/__tests__/distillation-penalties-table.test.tsx`: administrator wording regressions.
- `web/src/features/violation-records/lib/violation-display.ts`: personal violation-history permanent action label.
- `web/src/features/violation-records/lib/__tests__/violation-display.test.ts`: action-label regression.
- `web/src/features/violation-records/components/__tests__/violation-records-table.test.tsx`: rendered personal-history wording regression.
- `web/src/i18n/locales/{en,fr,ja,ru,vi,zh-TW,zh}.json`: four new non-stream-specific translation keys, written only by the temporary script.
- `web/scripts/add-missing-keys.mjs`: temporary locale updater; create for the translation task and delete before commit.

---

### Task 1: Lock In the Backend Contract With Failing Tests

**Files:**
- Modify: `service/distillation_rate_limit_test.go`
- Modify: `controller/rate_limit_test.go`
- Test: `service/distillation_rate_limit_test.go`
- Test: `controller/rate_limit_test.go`

**Interfaces:**
- Consumes: `CheckDistillationRateLimit(c *gin.Context, relayInfo *relaycommon.RelayInfo) *types.NewAPIError` and the existing internal `checkDistillationRateLimit` function.
- Produces: Regression tests requiring stream requests to bypass runtime-store construction and penalty-store reads, while non-stream permanent penalties still return 403.

- [ ] **Step 1: Add imports needed by the public-entry regression**

Add these imports without removing the existing service-test imports:

```go
import (
    "net/http"
    "net/http/httptest"

    "github.com/QuantumNous/new-api/common"
    relaycommon "github.com/QuantumNous/new-api/relay/common"
    "github.com/gin-gonic/gin"
)
```

- [ ] **Step 2: Add a failing test proving streams bypass runtime initialization**

Add this test near the existing stream test:

```go
func TestDistillationRateLimitStreamBypassesUnavailableRuntimeStore(t *testing.T) {
    previousRedisEnabled := common.RedisEnabled
    previousRDB := common.RDB
    common.RedisEnabled = true
    common.RDB = nil
    t.Cleanup(func() {
        common.RedisEnabled = previousRedisEnabled
        common.RDB = previousRDB
    })

    relayContext, _ := gin.CreateTestContext(httptest.NewRecorder())
    relayContext.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)

    err := CheckDistillationRateLimit(relayContext, &relaycommon.RelayInfo{
        UserId:   7,
        IsStream: true,
    })
    assert.Nil(t, err)
}
```

The current code must fail this test with a distillation storage error because Redis is enabled but unavailable.

- [ ] **Step 3: Replace the existing mixed stream/permanent test with explicit contracts**

Replace `TestDistillationRateLimitStreamsSkipDetectionButHonorPermanentBan` with these tests:

```go
func TestDistillationRateLimitStreamsBypassAllDistillationState(t *testing.T) {
    runtimeStore := &fakeDistillationRuntimeStore{}
    penaltyStore := &fakeDistillationPenaltyStore{
        current: &model.DistillationPenalty{
            TemporaryLimitedUntil: 1600,
            ObservationUntil:      5200,
            PermanentlyBannedAt:   1100,
        },
    }
    dependencies := distillationRateLimitDependencies{
        runtime:   runtimeStore,
        penalties: penaltyStore,
        now:       func() time.Time { return time.Unix(1200, 0) },
    }

    err := checkDistillationRateLimit(
        context.Background(),
        7,
        true,
        enabledDistillationSettings(),
        dependencies,
    )

    assert.Nil(t, err)
    assert.Empty(t, runtimeStore.requestKeys)
    assert.Zero(t, penaltyStore.getCalls)
    assert.Zero(t, penaltyStore.advanceCalls)
}

func TestDistillationRateLimitPermanentPenaltyBlocksNonStreamRequests(t *testing.T) {
    runtimeStore := &fakeDistillationRuntimeStore{}
    penaltyStore := &fakeDistillationPenaltyStore{
        current: &model.DistillationPenalty{
            PermanentlyBannedAt: 1100,
        },
    }
    dependencies := distillationRateLimitDependencies{
        runtime:   runtimeStore,
        penalties: penaltyStore,
        now:       func() time.Time { return time.Unix(1200, 0) },
    }

    err := checkDistillationRateLimit(
        context.Background(),
        7,
        false,
        enabledDistillationSettings(),
        dependencies,
    )

    require.NotNil(t, err)
    assert.Equal(t, types.ErrorCodeDistillationBanned, err.GetErrorCode())
    assert.Equal(t, http.StatusForbidden, err.StatusCode)
    assert.Contains(t, err.Error(), "non-stream")
    assert.Empty(t, runtimeStore.requestKeys)
    assert.Equal(t, 1, penaltyStore.getCalls)
}
```

- [ ] **Step 4: Update the administrator clear-penalty integration contract**

In `TestClearDistillationPenaltyIsIdempotentInvalidatesCachedBanAndRecordsAudit`,
replace the old stream-ban assumption with both request modes:

```go
streamError := service.CheckDistillationRateLimit(
    relayCtx,
    &relaycommon.RelayInfo{UserId: 51, IsStream: true},
)
assert.Nil(t, streamError)

banError := service.CheckDistillationRateLimit(
    relayCtx,
    &relaycommon.RelayInfo{UserId: 51, IsStream: false},
)
require.NotNil(t, banError)
```

After the two clear calls, verify the non-stream request is restored:

```go
afterClearError := service.CheckDistillationRateLimit(
    relayCtx,
    &relaycommon.RelayInfo{UserId: 51, IsStream: false},
)
assert.Nil(t, afterClearError)
```

- [ ] **Step 5: Run the focused tests and confirm the intended failures**

Run:

```powershell
go test ./service -run 'TestDistillationRateLimit(Stream|Permanent)' -count=1
go test ./controller -run TestClearDistillationPenaltyIsIdempotentInvalidatesCachedBanAndRecordsAudit -count=1
```

Expected: FAIL because the public stream path initializes the unavailable runtime store, the internal stream path reads the permanent penalty, the permanent error does not yet contain `non-stream`, and the controller integration still receives a stream ban.

---

### Task 2: Implement the Non-Stream-Only Backend Boundary

**Files:**
- Modify: `service/distillation_rate_limit.go`
- Test: `service/distillation_rate_limit_test.go`
- Test: `controller/rate_limit_test.go`

**Interfaces:**
- Consumes: Tests created in Task 1 and `relaycommon.RelayInfo.IsStream`.
- Produces: Stream requests return `nil` without storage access; non-stream requests retain the existing `*types.NewAPIError` behavior.

- [ ] **Step 1: Add the public-entry stream guard**

Make the first operation in `CheckDistillationRateLimit` the stream return:

```go
func CheckDistillationRateLimit(c *gin.Context, relayInfo *relaycommon.RelayInfo) *types.NewAPIError {
    if relayInfo.IsStream {
        return nil
    }
    runtimeStore, err := currentDistillationRuntimeStore()
    if err != nil {
        return newDistillationStorageError(err)
    }
    return checkDistillationRateLimit(
        c.Request.Context(),
        relayInfo.UserId,
        relayInfo.IsStream,
        setting.GetDistillationRateLimitSettings(),
        distillationRateLimitDependencies{
            runtime:   runtimeStore,
            penalties: cachedDistillationPenaltyStore{},
            now:       time.Now,
        },
    )
}
```

- [ ] **Step 2: Add the internal stream guard before all validation and storage**

Start `checkDistillationRateLimit` with:

```go
) *types.NewAPIError {
    if isStream {
        return nil
    }
    if userID <= 0 || dependencies.runtime == nil || dependencies.penalties == nil {
        return newDistillationStorageError(errors.New("invalid distillation rate limit context"))
    }
```

Then change the later enablement guard from:

```go
if isStream || !settings.Enabled {
    return nil
}
```

to:

```go
if !settings.Enabled {
    return nil
}
```

Do not reorder any remaining non-stream penalty, settings-validation, counter, or transition logic.

- [ ] **Step 3: Clarify the permanent-punishment error**

Use this exact backend error:

```go
func newDistillationBannedError() *types.NewAPIError {
    return types.NewErrorWithStatusCode(
        errors.New("non-stream model API access is permanently suspended after repeated distillation detection; streaming requests remain available; contact an administrator to restore access"),
        types.ErrorCodeDistillationBanned,
        http.StatusForbidden,
        types.ErrOptionWithSkipRetry(),
    )
}
```

- [ ] **Step 4: Format and run focused backend tests**

Run:

```powershell
gofmt -w service/distillation_rate_limit.go service/distillation_rate_limit_test.go
go test ./service -run 'TestDistillationRateLimit(Stream|Permanent)' -count=1
```

Expected: PASS.

- [ ] **Step 5: Run the full affected backend regression set**

Run:

```powershell
go test ./service ./controller -count=1
```

Expected: PASS, including existing fixed-minute, threshold-transition, temporary RPM, second-trigger, clearing, and controller integration tests.

- [ ] **Step 6: Commit the backend behavior**

Stage only the backend files and inspect the index before committing:

```powershell
git add -- service/distillation_rate_limit.go service/distillation_rate_limit_test.go controller/rate_limit_test.go
git diff --cached --check
git diff --cached --name-only
git commit -m "fix: limit distillation enforcement to non-stream requests"
```

Expected staged names: exactly the two service files and the controller test.

---

### Task 3: Lock In the User-Visible Wording With Failing Tests

**Files:**
- Modify: `web/src/features/system-settings/request-limits/__tests__/distillation-penalties-table.test.tsx`
- Modify: `web/src/features/violation-records/lib/__tests__/violation-display.test.ts`
- Modify: `web/src/features/violation-records/components/__tests__/violation-records-table.test.tsx`

**Interfaces:**
- Consumes: Existing display helpers and table components.
- Produces: Tests requiring the exact English source key `Permanent non-stream ban` and its timestamp/description variants.

- [ ] **Step 1: Update administrator penalty expectations**

In `distillation-penalties-table.test.tsx`, change the permanent phase expectation to:

```tsx
test('uses a destructive badge for permanent non-stream bans', () => {
  expect(getDistillationPenaltyPhaseConfig('permanent')).toEqual({
    labelKey: 'Permanent non-stream ban',
    variant: 'destructive',
  })
})
```

In the rendered table test, require:

```tsx
expect(html).toContain('Permanent non-stream ban')
expect(html).toContain('Permanent non-stream ban time')
```

In the empty-state test, require:

```tsx
expect(html).toContain(
  'Temporary limits, observation periods, and permanent non-stream bans will appear here.'
)
```

- [ ] **Step 2: Update personal violation-history expectations**

In `violation-display.test.ts`, require:

```ts
expect(getViolationActionLabel('permanent_ban')).toBe(
  'Permanent non-stream ban'
)
```

In `violation-records-table.test.tsx`, require:

```ts
assert.match(host.textContent || '', /Permanent non-stream ban/)
```

- [ ] **Step 3: Run the three focused frontend tests and confirm failure**

Run from `web/`:

```powershell
bun test src/features/system-settings/request-limits/__tests__/distillation-penalties-table.test.tsx src/features/violation-records/lib/__tests__/violation-display.test.ts src/features/violation-records/components/__tests__/violation-records-table.test.tsx
```

Expected: FAIL because production helpers and components still emit `Permanent ban` and `Permanent ban time`.

---

### Task 4: Implement Wording and Seven-Locale Translations

**Files:**
- Modify: `web/src/features/system-settings/request-limits/distillation-penalties.ts`
- Modify: `web/src/features/system-settings/request-limits/distillation-penalty-list.tsx`
- Modify: `web/src/features/system-settings/request-limits/distillation-penalties-table.tsx`
- Modify: `web/src/features/violation-records/lib/violation-display.ts`
- Create temporarily, then delete: `web/scripts/add-missing-keys.mjs`
- Modify via script: `web/src/i18n/locales/{en,fr,ja,ru,vi,zh-TW,zh}.json`
- Test: the three Task 3 test files

**Interfaces:**
- Consumes: Existing `t(...)` calls and display-helper return values.
- Produces: Four English source keys with complete translations in all seven locales; no API or type changes.

- [ ] **Step 1: Replace permanent labels in production code**

Use `Permanent non-stream ban` in both display helpers:

```ts
return { labelKey: 'Permanent non-stream ban', variant: 'destructive' }
```

```ts
case 'permanent_ban':
  return 'Permanent non-stream ban'
```

In both mobile and desktop locations in `distillation-penalty-list.tsx`, use:

```tsx
{t('Permanent non-stream ban time')}
```

In `distillation-penalties-table.tsx`, use these exact source strings:

```tsx
{t(
  'Review temporary limits, observation periods, and permanent non-stream bans.'
)}
```

```tsx
{t(
  'Temporary limits, observation periods, and permanent non-stream bans will appear here.'
)}
```

- [ ] **Step 2: Create the temporary locale updater**

Create `web/scripts/add-missing-keys.mjs` with this exact content:

```javascript
import fs from 'node:fs/promises'
import path from 'node:path'

const LOCALES_DIR = path.resolve('src/i18n/locales')

function stableStringify(value) {
  return JSON.stringify(value, null, 2) + '\n'
}

const newKeys = {
  en: {
    'Permanent non-stream ban': 'Permanent non-stream ban',
    'Permanent non-stream ban time': 'Permanent non-stream ban time',
    'Review temporary limits, observation periods, and permanent non-stream bans.':
      'Review temporary limits, observation periods, and permanent non-stream bans.',
    'Temporary limits, observation periods, and permanent non-stream bans will appear here.':
      'Temporary limits, observation periods, and permanent non-stream bans will appear here.',
  },
  fr: {
    'Permanent non-stream ban':
      'Interdiction permanente des requêtes non streaming',
    'Permanent non-stream ban time':
      "Date de l'interdiction permanente des requêtes non streaming",
    'Review temporary limits, observation periods, and permanent non-stream bans.':
      "Consultez les limitations temporaires, les périodes d'observation et les interdictions permanentes des requêtes non streaming.",
    'Temporary limits, observation periods, and permanent non-stream bans will appear here.':
      "Les limitations temporaires, les périodes d'observation et les interdictions permanentes des requêtes non streaming apparaîtront ici.",
  },
  ja: {
    'Permanent non-stream ban': '非ストリーミングリクエストを永久禁止',
    'Permanent non-stream ban time':
      '非ストリーミングリクエストの永久禁止日時',
    'Review temporary limits, observation periods, and permanent non-stream bans.':
      '一時的な制限、観察期間、非ストリーミングリクエストの永久禁止を確認します。',
    'Temporary limits, observation periods, and permanent non-stream bans will appear here.':
      '一時的な制限、観察期間、非ストリーミングリクエストの永久禁止がここに表示されます。',
  },
  ru: {
    'Permanent non-stream ban':
      'Постоянная блокировка нестриминговых запросов',
    'Permanent non-stream ban time':
      'Время постоянной блокировки нестриминговых запросов',
    'Review temporary limits, observation periods, and permanent non-stream bans.':
      'Просмотр временных ограничений, периодов наблюдения и постоянных блокировок нестриминговых запросов.',
    'Temporary limits, observation periods, and permanent non-stream bans will appear here.':
      'Здесь отображаются временные ограничения, периоды наблюдения и постоянные блокировки нестриминговых запросов.',
  },
  vi: {
    'Permanent non-stream ban':
      'Chặn vĩnh viễn yêu cầu không phát trực tuyến',
    'Permanent non-stream ban time':
      'Thời điểm chặn vĩnh viễn yêu cầu không phát trực tuyến',
    'Review temporary limits, observation periods, and permanent non-stream bans.':
      'Xem các giới hạn tạm thời, giai đoạn theo dõi và lệnh chặn vĩnh viễn đối với yêu cầu không phát trực tuyến.',
    'Temporary limits, observation periods, and permanent non-stream bans will appear here.':
      'Các giới hạn tạm thời, giai đoạn theo dõi và lệnh chặn vĩnh viễn đối với yêu cầu không phát trực tuyến sẽ xuất hiện tại đây.',
  },
  'zh-TW': {
    'Permanent non-stream ban': '永久禁止非串流請求',
    'Permanent non-stream ban time': '永久禁止非串流請求時間',
    'Review temporary limits, observation periods, and permanent non-stream bans.':
      '查看臨時限速、觀察期和永久禁止非串流請求記錄。',
    'Temporary limits, observation periods, and permanent non-stream bans will appear here.':
      '臨時限速、觀察期和永久禁止非串流請求記錄將顯示於此。',
  },
  zh: {
    'Permanent non-stream ban': '永久禁止非流式请求',
    'Permanent non-stream ban time': '永久禁止非流式请求时间',
    'Review temporary limits, observation periods, and permanent non-stream bans.':
      '查看临时限速、观察期和永久禁止非流式请求记录。',
    'Temporary limits, observation periods, and permanent non-stream bans will appear here.':
      '临时限速、观察期和永久禁止非流式请求记录将显示在此处。',
  },
}

for (const [locale, translations] of Object.entries(newKeys)) {
  const filePath = path.join(LOCALES_DIR, `${locale}.json`)
  const json = JSON.parse(await fs.readFile(filePath, 'utf8'))
  Object.assign(json.translation, translations)
  json.translation = Object.fromEntries(
    Object.entries(json.translation).sort(([left], [right]) =>
      left.localeCompare(right)
    )
  )
  await fs.writeFile(filePath, stableStringify(json), 'utf8')
  console.log(`${locale}: ${Object.keys(translations).length} translations applied`)
}
```

- [ ] **Step 3: Apply and normalize translations**

Run from `web/`:

```powershell
node scripts/add-missing-keys.mjs
bun run i18n:sync
```

Read `src/i18n/locales/_reports/_sync-report.json` and require
`missingCount: 0`, `extrasCount: 0`, and `untranslatedCount: 0` for every
locale. Keep the temporary updater only until Step 5 has built clean locale
blobs for the index; it must not be committed.

- [ ] **Step 4: Format and run the focused frontend tests**

Run from `web/`:

```powershell
bunx oxfmt --write src/features/system-settings/request-limits/distillation-penalties.ts src/features/system-settings/request-limits/distillation-penalty-list.tsx src/features/system-settings/request-limits/distillation-penalties-table.tsx src/features/system-settings/request-limits/__tests__/distillation-penalties-table.test.tsx src/features/violation-records/lib/violation-display.ts src/features/violation-records/lib/__tests__/violation-display.test.ts src/features/violation-records/components/__tests__/violation-records-table.test.tsx
bun test src/features/system-settings/request-limits/__tests__/distillation-penalties-table.test.tsx src/features/violation-records/lib/__tests__/violation-display.test.ts src/features/violation-records/components/__tests__/violation-records-table.test.tsx
```

Expected: PASS.

- [ ] **Step 5: Stage only this task's frontend changes**

Stage the seven clean TypeScript/TSX files directly:

```powershell
git add -- web/src/features/system-settings/request-limits/distillation-penalties.ts web/src/features/system-settings/request-limits/distillation-penalty-list.tsx web/src/features/system-settings/request-limits/distillation-penalties-table.tsx web/src/features/system-settings/request-limits/__tests__/distillation-penalties-table.test.tsx web/src/features/violation-records/lib/violation-display.ts web/src/features/violation-records/lib/__tests__/violation-display.test.ts web/src/features/violation-records/components/__tests__/violation-records-table.test.tsx
```

The locale worktree files already contain unrelated user changes. Build clean
locale blobs from `HEAD`, apply only `newKeys` to those clean copies, and write
those blobs directly to the index:

```powershell
$localeStageRoot = Join-Path ([System.IO.Path]::GetTempPath()) ("new-api-distillation-locales-" + [guid]::NewGuid().ToString("N"))
New-Item -ItemType Directory -Path $localeStageRoot | Out-Null
git archive HEAD web/src/i18n/locales web/scripts/sync-i18n.mjs | tar -xf - -C $localeStageRoot
Copy-Item -LiteralPath "web/scripts/add-missing-keys.mjs" -Destination (Join-Path $localeStageRoot "web/scripts/add-missing-keys.mjs")

Push-Location (Join-Path $localeStageRoot "web")
node scripts/add-missing-keys.mjs
node scripts/sync-i18n.mjs
Pop-Location

foreach ($locale in @('en', 'fr', 'ja', 'ru', 'vi', 'zh-TW', 'zh')) {
  $repoPath = "web/src/i18n/locales/$locale.json"
  $stagedFile = Join-Path $localeStageRoot $repoPath
  $blob = git hash-object -w -- $stagedFile
  git update-index --cacheinfo "100644,$blob,$repoPath"
  if ($LASTEXITCODE -ne 0) { throw "Failed to stage $repoPath" }
}

$resolvedStageRoot = (Resolve-Path -LiteralPath $localeStageRoot).Path
$resolvedTempRoot = [System.IO.Path]::GetFullPath([System.IO.Path]::GetTempPath())
if (-not $resolvedStageRoot.StartsWith($resolvedTempRoot, [System.StringComparison]::OrdinalIgnoreCase)) {
  throw "Refusing to remove locale staging directory outside the temp root"
}
Remove-Item -LiteralPath $resolvedStageRoot -Recurse -Force
```

Delete `web/scripts/add-missing-keys.mjs` with `apply_patch`, then inspect the
index:

```powershell
git diff --cached --check
git diff --cached --name-only
git diff --cached -- web/src/i18n/locales
```

Inspect `git diff --cached` and confirm it contains no API-key, wallet, button,
theme, or special-ratio changes.

- [ ] **Step 6: Commit the frontend wording**

```powershell
git commit -m "fix(web): clarify non-stream distillation penalties"
```

---

### Task 5: Full Verification and Scope Audit

**Files:**
- Verify: all files changed in Tasks 1-4
- Preserve: all unrelated dirty worktree files

**Interfaces:**
- Consumes: Completed backend and frontend commits.
- Produces: Evidence that the behavior, translations, build, and staged scope are correct.

- [ ] **Step 1: Run the complete backend test suite**

Run from the repository root:

```powershell
go test ./... -count=1
go vet ./model ./service ./controller ./router
```

Expected: PASS.

- [ ] **Step 2: Run frontend tests, type checking, lint, and build**

Run from `web/`:

```powershell
bun test src/features/system-settings/request-limits/__tests__/distillation-penalties-table.test.tsx src/features/violation-records/lib/__tests__/violation-display.test.ts src/features/violation-records/components/__tests__/violation-records-table.test.tsx
bun run typecheck
bunx oxlint -c .oxlintrc.json src/features/system-settings/request-limits/distillation-penalties.ts src/features/system-settings/request-limits/distillation-penalty-list.tsx src/features/system-settings/request-limits/distillation-penalties-table.tsx src/features/system-settings/request-limits/__tests__/distillation-penalties-table.test.tsx src/features/violation-records/lib/violation-display.ts src/features/violation-records/lib/__tests__/violation-display.test.ts src/features/violation-records/components/__tests__/violation-records-table.test.tsx
bun run build
```

Expected: PASS.

- [ ] **Step 3: Re-run i18n reporting without leaving generated changes**

Run from `web/`:

```powershell
bun run i18n:sync
```

Confirm every locale in `_sync-report.json` has zero missing, extras, and
untranslated entries. Verify `scripts/add-missing-keys.mjs` is absent.

- [ ] **Step 4: Audit commits and preserved worktree changes**

Run from the repository root:

```powershell
git diff --check
git status --short --branch
git show --stat --oneline HEAD~2..HEAD
```

Confirm the implementation commits contain only the backend service/test,
distillation wording/test, and four translation-key additions. Confirm the
user's pre-existing API-key, wallet, button, theme, and unrelated locale
changes remain unstaged and unmodified in intent.
