# Affiliate Approval User Log Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a user-visible management log after a rebate transfer is approved while preserving the existing administrator audit log.

**Architecture:** Keep the balance transaction unchanged. After it commits, write a second structured operation log owned by the credited user, retain administrator identity under the existing admin-only metadata field, and localize the structured action in web/default with the raw quota formatted in the user's current display currency.

**Tech Stack:** Go 1.22+, Gin, GORM log model, React 19, TypeScript, i18next, Bun tests, testify.

---

## File Structure

- Modify `controller/affiliate_transfer.go`: write the credited user's structured management log after approval succeeds.
- Modify `controller/audit.go`: register the English fallback template for the new stable action.
- Modify `controller/affiliate_transfer_test.go`: protect log ownership, action data, administrator metadata, and failure behavior.
- Modify `web/default/src/features/usage-logs/lib/format.ts`: map and format the new structured action.
- Modify `web/default/src/features/usage-logs/lib/format.test.ts`: protect localized amount rendering.
- Create then remove `web/default/scripts/add-missing-keys.mjs`: apply all seven locale values through the required script workflow.
- Modify `web/default/src/i18n/locales/{en,zh,zh-TW,fr,ja,ru,vi}.json`: generated locale updates.

## Task 1: Record the Credited User's Management Log

**Files:**

- Modify: `controller/affiliate_transfer_test.go`
- Modify: `controller/audit.go`
- Modify: `controller/affiliate_transfer.go`

- [ ] **Step 1: Extend the approval controller test before production code**

Replace the single-log assertions in `TestApproveAffiliateTransferRequestRecordsDetailedAudit` with assertions over both management logs:

~~~go
var logs []model.Log
require.NoError(t, db.Where("type = ?", model.LogTypeManage).Order("id asc").Find(&logs).Error)
require.Len(t, logs, 2)

adminLog := logs[0]
assert.Equal(t, 1, adminLog.UserId)
assert.Contains(t, adminLog.Content, "Approved rebate transfer request #7 for user 302")

adminOther, err := common.StrToMap(adminLog.Other)
require.NoError(t, err)
adminOp, ok := adminOther["op"].(map[string]interface{})
require.True(t, ok)
assert.Equal(t, "affiliate.transfer.approve", adminOp["action"])

userLog := logs[1]
assert.Equal(t, 302, userLog.UserId)
assert.Equal(t, "rebate-user", userLog.Username)
assert.Contains(t, userLog.Content, "Administrator approved")

userOther, err := common.StrToMap(userLog.Other)
require.NoError(t, err)
userOp, ok := userOther["op"].(map[string]interface{})
require.True(t, ok)
assert.Equal(t, "affiliate.transfer.approved_for_user", userOp["action"])
userParams, ok := userOp["params"].(map[string]interface{})
require.True(t, ok)
assert.EqualValues(t, 7, userParams["request_id"])
assert.EqualValues(t, 500, userParams["quota"])
userAdminInfo, ok := userOther["admin_info"].(map[string]interface{})
require.True(t, ok)
assert.EqualValues(t, 1, userAdminInfo["admin_id"])
assert.Equal(t, "root-admin", userAdminInfo["admin_username"])
~~~

Add a regression test that calls the approval handler a second time for the same request and asserts that the failed duplicate attempt does not add another user-owned action:

~~~go
func TestApproveAffiliateTransferRequestDoesNotRecordUserLogWhenApprovalFails(t *testing.T) {
    db := setupAffiliateTransferApprovalControllerFixture(t)
    recorder, ctx := newAffiliateTransferApprovalContext(7)

    ApproveAffiliateTransferRequest(ctx)
    require.Equal(t, http.StatusOK, recorder.Code)

    secondRecorder, secondCtx := newAffiliateTransferApprovalContext(7)
    ApproveAffiliateTransferRequest(secondCtx)

    var logs []model.Log
    require.NoError(t, db.Where("user_id = ? AND type = ?", 302, model.LogTypeManage).Find(&logs).Error)
    require.Len(t, logs, 1)
    other, err := common.StrToMap(logs[0].Other)
    require.NoError(t, err)
    op, ok := other["op"].(map[string]interface{})
    require.True(t, ok)
    assert.Equal(t, "affiliate.transfer.approved_for_user", op["action"])
}
~~~

Extract only durable test-fixture concepts (`setupAffiliateTransferApprovalControllerFixture` and `newAffiliateTransferApprovalContext`) from the existing setup so both tests exercise the real controller and database.

- [ ] **Step 2: Run the focused controller test and verify RED**

Run:

~~~powershell
go test ./controller -run 'TestApproveAffiliateTransferRequestRecordsDetailedAudit|TestApproveAffiliateTransferRequestDoesNotRecordUserLogWhenApprovalFails' -count=1
~~~

Expected: the first test fails because only the existing administrator log is present.

- [ ] **Step 3: Register the fallback action template**

Add this entry to `auditContentTemplates` in `controller/audit.go`:

~~~go
"affiliate.transfer.approved_for_user": "Administrator approved ${amount} balance",
~~~

The fallback receives `amount` already formatted through `logger.LogQuota`; the structured `quota` parameter remains the authoritative raw value for web/default.

- [ ] **Step 4: Write the user-owned log only after approval commits**

Add the logger import to `controller/affiliate_transfer.go`, then append this block after `model.ApproveAffiliateTransferRequest` succeeds and after the existing administrator audit is recorded:

~~~go
userLogParams := map[string]interface{}{
    "request_id": detail.Id,
    "quota":      detail.TotalQuota,
    "amount":     logger.LogQuota(detail.TotalQuota),
}
model.RecordOperationAuditLog(
    detail.UserId,
    auditContentEN("affiliate.transfer.approved_for_user", userLogParams),
    c.ClientIP(),
    "affiliate.transfer.approved_for_user",
    userLogParams,
    auditOperatorInfo(c),
    nil,
)
~~~

Do not move the call into `ApproveAffiliateTransferRequest` in the model: the log database may be separate from the main database and cannot participate atomically in the balance transaction.

- [ ] **Step 5: Run controller and model approval tests**

Run:

~~~powershell
go test ./controller ./model -run 'AffiliateTransfer|ApproveAffiliate' -count=1
~~~

Expected: PASS.

- [ ] **Step 6: Commit the backend log behavior**

~~~powershell
git add controller/affiliate_transfer.go controller/affiliate_transfer_test.go controller/audit.go
git commit -m "feat(rebate): log approved balance for users"
~~~

## Task 2: Localize and Currency-Format the New Action

**Files:**

- Modify: `web/default/src/features/usage-logs/lib/format.test.ts`
- Modify: `web/default/src/features/usage-logs/lib/format.ts`
- Create temporarily: `web/default/scripts/add-missing-keys.mjs`
- Modify through script: `web/default/src/i18n/locales/en.json`
- Modify through script: `web/default/src/i18n/locales/zh.json`
- Modify through script: `web/default/src/i18n/locales/zh-TW.json`
- Modify through script: `web/default/src/i18n/locales/fr.json`
- Modify through script: `web/default/src/i18n/locales/ja.json`
- Modify through script: `web/default/src/i18n/locales/ru.json`
- Modify through script: `web/default/src/i18n/locales/vi.json`

- [ ] **Step 1: Write the failing frontend formatting test**

Add this test to `format.test.ts`:

~~~ts
import { formatLogQuota } from '@/lib/format'

test('renders the user-facing affiliate approval with formatted balance', () => {
  const content = renderAuditContent(
    {
      op: {
        action: 'affiliate.transfer.approved_for_user',
        params: { request_id: 7, quota: 500 },
      },
    },
    (key, params) =>
      key.replace('{{amount}}', String(params?.amount ?? ''))
  )

  expect(content).toBe(
    `Administrator approved ${formatLogQuota(500)} balance`
  )
})
~~~

- [ ] **Step 2: Run the focused Bun test and verify RED**

From `web/default`, run:

~~~powershell
bun test src/features/usage-logs/lib/format.test.ts
~~~

Expected: FAIL because the action is not present in `AUDIT_TEMPLATES`.

- [ ] **Step 3: Format the raw quota at render time**

Import `formatLogQuota` from `@/lib/format`, add the template:

~~~ts
'affiliate.transfer.approved_for_user':
  'Administrator approved {{amount}} balance',
~~~

Then normalize parameters immediately before the final `t(...)` call:

~~~ts
const params = { ...(op.params ?? {}) } as Record<string, unknown>
if (
  op.action === 'affiliate.transfer.approved_for_user' &&
  typeof params.quota === 'number' &&
  Number.isFinite(params.quota)
) {
  params.amount = formatLogQuota(params.quota)
}
return t(template, params)
~~~

This keeps historical logs responsive to the user's current display currency instead of freezing the backend's display setting.

- [ ] **Step 4: Add all locale values through the required script**

Create `web/default/scripts/add-missing-keys.mjs` using the exact script structure required by the repository's `i18n-translate` skill. Populate `newKeys` with:

~~~js
const newKeys = {
  en: {
    'Administrator approved {{amount}} balance':
      'Administrator approved {{amount}} balance',
  },
  zh: {
    'Administrator approved {{amount}} balance':
      '管理员批准 {{amount}} 余额',
  },
  'zh-TW': {
    'Administrator approved {{amount}} balance':
      '管理員批准 {{amount}} 餘額',
  },
  fr: {
    'Administrator approved {{amount}} balance':
      "L'administrateur a approuvé un crédit de {{amount}}",
  },
  ja: {
    'Administrator approved {{amount}} balance':
      '管理者が残高 {{amount}} を承認しました',
  },
  ru: {
    'Administrator approved {{amount}} balance':
      'Администратор одобрил зачисление {{amount}}',
  },
  vi: {
    'Administrator approved {{amount}} balance':
      'Quản trị viên đã duyệt số dư {{amount}}',
  },
}
~~~

Run from `web/default`:

~~~powershell
node scripts/add-missing-keys.mjs
bun run i18n:sync
Remove-Item -LiteralPath scripts/add-missing-keys.mjs
~~~

Expected: every locale reports zero missing and zero extra keys. Review `_reports/_sync-report.json` and require `missingCount: 0`, `extrasCount: 0`, and `untranslatedCount: 0` for all seven locales.

- [ ] **Step 5: Run focused frontend verification**

From `web/default`, run:

~~~powershell
bun test src/features/usage-logs/lib/format.test.ts
bun run typecheck
bunx oxlint -c .oxlintrc.json src/features/usage-logs/lib/format.ts src/features/usage-logs/lib/format.test.ts
bunx oxfmt --check src/features/usage-logs/lib/format.ts src/features/usage-logs/lib/format.test.ts
~~~

Expected: all commands PASS.

- [ ] **Step 6: Commit localized presentation**

From the repository root:

~~~powershell
git add web/default/src/features/usage-logs/lib/format.ts web/default/src/features/usage-logs/lib/format.test.ts web/default/src/i18n/locales
git commit -m "feat(logs): localize approved rebate balance"
~~~

## Task 3: Verify the Approval-Log Delivery

**Files:**

- No production file is introduced in this task.
- Modify only files from Tasks 1-2 if verification exposes a defect.

- [ ] **Step 1: Run affected backend packages**

~~~powershell
go test ./controller ./model -count=1
~~~

Expected: PASS.

- [ ] **Step 2: Run all frontend tests and build**

From `web/default`:

~~~powershell
bun test
bun run typecheck
bun run build
~~~

Expected: PASS.

- [ ] **Step 3: Confirm the exact user-visible contract**

Use a test database, approve one pending rebate transfer, call `/api/log/self` as the credited user, and verify:

1. one type-3 log is returned for `affiliate.transfer.approved_for_user`;
2. `other.admin_info` is absent from the user response;
3. the rendered content is `管理员批准 <formatted amount> 余额` under Chinese locale;
4. the administrator still sees the original `affiliate.transfer.approve` audit log.

- [ ] **Step 4: Record any verification-only fix**

If the checks required a code correction, commit only the affected files:

~~~powershell
git add controller web/default/src/features/usage-logs web/default/src/i18n/locales
git commit -m "fix(rebate): complete approval log verification"
~~~
