# Wallet Transfer History Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add secure user transfer history, expose the existing one-dollar transfer minimum in the wallet UI, show submitted requests consistently, and permanently consume both reward components for newly rejected requests.

**Architecture:** Keep the existing affiliate transfer request table and add a timestamp marker that distinguishes newly forfeited rejections from legacy rejected rows. Enforce all accounting transitions transactionally in the model, expose a current-user-only paginated history DTO, and add a lazy wallet dialog backed by the existing API and dialog patterns.

**Tech Stack:** Go 1.22+, Gin, GORM v2, SQLite/MySQL/PostgreSQL, React 19, TypeScript, TanStack Query, Base UI, Tailwind CSS, Bun.

---

## File Map

- Modify `model/affiliate_transfer_request.go`: add the forfeiture marker, include new rejections in consumed rebate calculations, harden create/approve/reject transactions, and add the current-user history query.
- Create `model/affiliate_transfer_request_test.go`: protect minimum transfer, legacy/new rejection accounting, terminal-transition idempotency, and current-user history ordering.
- Modify `controller/affiliate_transfer.go`: add the authenticated self-history handler.
- Modify `controller/affiliate_transfer_test.go`: verify user isolation and response-field privacy.
- Modify `router/api-router.go`: register the self-history route under `UserAuth`.
- Modify `web/default/src/features/wallet/lib/affiliate.ts`: add the pure transfer-action business-state helper.
- Create `web/default/src/features/wallet/lib/affiliate.test.ts`: protect the one-dollar boundary and submitted label.
- Modify `web/default/src/features/wallet/types.ts`: add history item/page response types.
- Modify `web/default/src/features/wallet/api.ts`: add the paginated self-history request.
- Modify `web/default/src/features/wallet/components/affiliate-rewards-card.tsx`: enforce the frontend minimum and render Refresh, Request Transfer, Transfer History in the confirmed order.
- Create `web/default/src/features/wallet/components/dialogs/transfer-history-dialog.tsx`: lazy-load and paginate current-user history.
- Modify `web/default/src/features/wallet/index.tsx`: own the history dialog state and pass the configured one-dollar quota threshold.
- Modify `web/default/src/i18n/locales/{en,zh,fr,ja,ru,vi}.json`: translate new wallet labels through the project i18n workflow.

### Task 1: Protect New Rejection Accounting With Model Tests

**Files:**
- Create: `model/affiliate_transfer_request_test.go`
- Modify: `model/affiliate_transfer_request.go`

- [ ] **Step 1: Add an explicit affiliate model fixture and failing rejection tests**

Create `model/affiliate_transfer_request_test.go` in package `model`. Use the package test database but initialize and clear every required table inside the fixture:

```go
package model

import (
    "sync"
    "testing"

    "github.com/QuantumNous/new-api/common"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func setupAffiliateTransferTest(t *testing.T) {
    t.Helper()
    require.NoError(t, DB.AutoMigrate(&AffiliateTransferRequest{}, &User{}, &TopUp{}))
    require.NoError(t, DB.Exec("DELETE FROM affiliate_transfer_requests").Error)
    require.NoError(t, DB.Exec("DELETE FROM top_ups").Error)
    require.NoError(t, DB.Exec("DELETE FROM users").Error)
}

func TestRejectAffiliateTransferRequestForfeitsNewRequest(t *testing.T) {
    setupAffiliateTransferTest(t)
    require.NoError(t, DB.Create(&User{Id: 10, Username: "owner", AffQuota: 200, Quota: 50}).Error)
    require.NoError(t, DB.Create(&User{Id: 11, Username: "invitee", InviterId: 10}).Error)
    require.NoError(t, DB.Create(&TopUp{
        UserId: 11, PaymentProvider: PaymentProviderCreem,
        Amount: 6000, Status: common.TopUpStatusSuccess, CompleteTime: 100,
    }).Error)
    request := AffiliateTransferRequest{
        UserId: 10, InviteRewardQuota: 200, RechargeRebateQuota: 300,
        TotalQuota: 500, Status: AffiliateTransferStatusPending, CreatedAt: 200,
    }
    require.NoError(t, DB.Create(&request).Error)

    require.NoError(t, RejectAffiliateTransferRequest(request.Id, 99, "policy"))

    var user User
    require.NoError(t, DB.First(&user, 10).Error)
    assert.Equal(t, 0, user.AffQuota)
    assert.Equal(t, 50, user.Quota)

    var stored AffiliateTransferRequest
    require.NoError(t, DB.First(&stored, request.Id).Error)
    assert.Equal(t, AffiliateTransferStatusRejected, stored.Status)
    assert.Positive(t, stored.RejectedQuotaForfeitedAt)

    summary, err := GetAffiliateRebateSummary(10)
    require.NoError(t, err)
    assert.Equal(t, 0, summary.RechargeRebateQuota)
}

func TestAffiliateRebateSummaryDoesNotForfeitLegacyRejection(t *testing.T) {
    setupAffiliateTransferTest(t)
    require.NoError(t, DB.Create(&User{Id: 20, Username: "owner"}).Error)
    require.NoError(t, DB.Create(&User{Id: 21, Username: "invitee", InviterId: 20}).Error)
    require.NoError(t, DB.Create(&TopUp{
        UserId: 21, PaymentProvider: PaymentProviderCreem,
        Amount: 6000, Status: common.TopUpStatusSuccess, CompleteTime: 100,
    }).Error)
    require.NoError(t, DB.Create(&AffiliateTransferRequest{
        UserId: 20, RechargeRebateQuota: 300, TotalQuota: 300,
        Status: AffiliateTransferStatusRejected, CreatedAt: 200,
    }).Error)

    summary, err := GetAffiliateRebateSummary(20)
    require.NoError(t, err)
    assert.Equal(t, 300, summary.RechargeRebateQuota)
}
```

- [ ] **Step 2: Run the focused tests and verify RED**

Run:

```powershell
go test ./model -run 'TestRejectAffiliateTransferRequestForfeitsNewRequest|TestAffiliateRebateSummaryDoesNotForfeitLegacyRejection' -count=1
```

Expected: compilation fails because `RejectedQuotaForfeitedAt` does not exist, or the new-rejection assertion fails because rejected rebate is currently returned.

- [ ] **Step 3: Add the internal marker and consumed-rebate predicates**

In `AffiliateTransferRequest`, add exactly:

```go
RejectedQuotaForfeitedAt int64 `json:"-" gorm:"column:rejected_quota_forfeited_at"`
```

Change both recharge-consumption sums, in `getAffiliateRebateSummaryWithDB` and `GetAffiliateTransferRequestDetail`, to include a newly forfeited rejection:

```go
Where(
    "user_id = ? AND (status <> ? OR rejected_quota_forfeited_at > ?)",
    userId,
    AffiliateTransferStatusRejected,
    0,
)
```

Use `userId` in the summary query and `item.UserId` in the detail query. For the detail query, retain the existing creation-time/ID boundary in the same `Where` chain.

- [ ] **Step 4: Make rejection atomic and non-crediting**

Replace `RejectAffiliateTransferRequest` with a transaction that performs a guarded terminal transition and then deducts invitation rewards. All later failures must roll the status update back:

```go
func RejectAffiliateTransferRequest(requestId int, reviewerId int, reason string) error {
    return DB.Transaction(func(tx *gorm.DB) error {
        var request AffiliateTransferRequest
        if err := lockForUpdate(tx).First(&request, "id = ?", requestId).Error; err != nil {
            return err
        }
        if request.Status != AffiliateTransferStatusPending {
            return errors.New("request is not pending")
        }

        reviewedAt := common.GetTimestamp()
        result := tx.Model(&AffiliateTransferRequest{}).
            Where("id = ? AND status = ?", requestId, AffiliateTransferStatusPending).
            Updates(map[string]interface{}{
                "status": AffiliateTransferStatusRejected,
                "reviewed_at": reviewedAt,
                "reviewed_by": reviewerId,
                "reject_reason": strings.TrimSpace(reason),
                "rejected_quota_forfeited_at": reviewedAt,
            })
        if result.Error != nil {
            return result.Error
        }
        if result.RowsAffected != 1 {
            return errors.New("request is not pending")
        }

        if request.InviteRewardQuota == 0 {
            return nil
        }
        result = tx.Model(&User{}).
            Where("id = ? AND aff_quota >= ?", request.UserId, request.InviteRewardQuota).
            Update("aff_quota", gorm.Expr("aff_quota - ?", request.InviteRewardQuota))
        if result.Error != nil {
            return result.Error
        }
        if result.RowsAffected != 1 {
            return errors.New("insufficient invitation reward quota")
        }
        return nil
    })
}
```

- [ ] **Step 5: Run the focused tests and verify GREEN**

Run the command from Step 2. Expected: both tests pass.

- [ ] **Step 6: Commit the accounting behavior**

```powershell
git add model/affiliate_transfer_request.go model/affiliate_transfer_request_test.go
git commit -m "fix(wallet): forfeit newly rejected transfer rewards"
```

### Task 2: Enforce Minimum and Terminal Transition Invariants

**Files:**
- Modify: `model/affiliate_transfer_request_test.go`
- Modify: `model/affiliate_transfer_request.go`

- [ ] **Step 1: Add boundary and terminal-transition tests**

Append tests that set `AffQuota` to `int(common.QuotaPerUnit)-1` and then exactly `int(common.QuotaPerUnit)`, asserting failure followed by success. Add a terminal transition test that calls approve and reject concurrently against one pending request and asserts exactly one nil error, one terminal status, one invitation deduction, and either the full approved credit or no credit:

```go
func TestCreateAffiliateTransferRequestRequiresOneDollar(t *testing.T) {
    setupAffiliateTransferTest(t)
    minimum := int(common.QuotaPerUnit)
    user := User{Id: 30, Username: "owner", AffQuota: minimum - 1}
    require.NoError(t, DB.Create(&user).Error)

    _, err := CreateAffiliateTransferRequest(user.Id)
    require.Error(t, err)

    require.NoError(t, DB.Model(&User{}).Where("id = ?", user.Id).Update("aff_quota", minimum).Error)
    request, err := CreateAffiliateTransferRequest(user.Id)
    require.NoError(t, err)
    assert.Equal(t, minimum, request.TotalQuota)
}

func TestAffiliateTransferRequestHasOneTerminalWinner(t *testing.T) {
    setupAffiliateTransferTest(t)
    require.NoError(t, DB.Create(&User{Id: 40, Username: "owner", AffQuota: 100}).Error)
    request := AffiliateTransferRequest{
        UserId: 40, InviteRewardQuota: 100, TotalQuota: 100,
        Status: AffiliateTransferStatusPending, CreatedAt: 100,
    }
    require.NoError(t, DB.Create(&request).Error)

    errorsByOperation := make([]error, 2)
    var wait sync.WaitGroup
    wait.Add(2)
    go func() { defer wait.Done(); errorsByOperation[0] = ApproveAffiliateTransferRequest(request.Id, 1) }()
    go func() { defer wait.Done(); errorsByOperation[1] = RejectAffiliateTransferRequest(request.Id, 2, "no") }()
    wait.Wait()

    successCount := 0
    for _, err := range errorsByOperation {
        if err == nil { successCount++ }
    }
    assert.Equal(t, 1, successCount)

    var stored AffiliateTransferRequest
    var user User
    require.NoError(t, DB.First(&stored, request.Id).Error)
    require.NoError(t, DB.First(&user, 40).Error)
    assert.Contains(t, []string{AffiliateTransferStatusApproved, AffiliateTransferStatusRejected}, stored.Status)
    assert.Equal(t, 0, user.AffQuota)
    if stored.Status == AffiliateTransferStatusApproved {
        assert.Equal(t, 100, user.Quota)
    } else {
        assert.Equal(t, 0, user.Quota)
    }
}
```

- [ ] **Step 2: Run the focused invariant tests**

Run:

```powershell
go test ./model -run 'TestCreateAffiliateTransferRequestRequiresOneDollar|TestAffiliateTransferRequestHasOneTerminalWinner' -count=1
```

Expected: the minimum test passes against existing backend validation; the terminal test protects the requested accounting contract while the locking implementation is hardened.

- [ ] **Step 3: Lock creation and use valid locking/CAS for approval**

At the start of `CreateAffiliateTransferRequest`'s transaction, lock and validate the user before checking existing requests:

```go
var user User
if err := lockForUpdate(tx).Select("id").First(&user, "id = ?", userId).Error; err != nil {
    return err
}
```

In approval, replace `tx.Set("gorm:query_option", "FOR UPDATE")` with `lockForUpdate(tx)`. Perform the guarded status update before balance mutations; the transaction rolls that update back if either balance operation fails:

```go
func ApproveAffiliateTransferRequest(requestId int, reviewerId int) error {
    return DB.Transaction(func(tx *gorm.DB) error {
        var request AffiliateTransferRequest
        if err := lockForUpdate(tx).First(&request, "id = ?", requestId).Error; err != nil {
            return err
        }
        if request.Status != AffiliateTransferStatusPending {
            return errors.New("request is not pending")
        }

        result := tx.Model(&AffiliateTransferRequest{}).
            Where("id = ? AND status = ?", requestId, AffiliateTransferStatusPending).
            Updates(map[string]interface{}{
                "status": AffiliateTransferStatusApproved,
                "reviewed_at": common.GetTimestamp(),
                "reviewed_by": reviewerId,
                "reject_reason": "",
            })
        if result.Error != nil {
            return result.Error
        }
        if result.RowsAffected != 1 {
            return errors.New("request is not pending")
        }

        if request.InviteRewardQuota > 0 {
            result = tx.Model(&User{}).
                Where("id = ? AND aff_quota >= ?", request.UserId, request.InviteRewardQuota).
                Update("aff_quota", gorm.Expr("aff_quota - ?", request.InviteRewardQuota))
            if result.Error != nil {
                return result.Error
            }
            if result.RowsAffected != 1 {
                return errors.New("insufficient invitation reward quota")
            }
        }
        return tx.Model(&User{}).
            Where("id = ?", request.UserId).
            Update("quota", gorm.Expr("quota + ?", request.TotalQuota)).Error
    })
}
```

- [ ] **Step 4: Re-run all affiliate model tests**

```powershell
go test ./model -run Affiliate -count=1
```

Expected: all affiliate tests pass, including existing summary/detail behavior.

- [ ] **Step 5: Commit transaction hardening**

```powershell
git add model/affiliate_transfer_request.go model/affiliate_transfer_request_test.go
git commit -m "fix(wallet): serialize affiliate transfer transitions"
```

### Task 3: Add the Authenticated Self-History API

**Files:**
- Modify: `model/affiliate_transfer_request.go`
- Modify: `model/affiliate_transfer_request_test.go`
- Modify: `controller/affiliate_transfer.go`
- Modify: `controller/affiliate_transfer_test.go`
- Modify: `router/api-router.go`

- [ ] **Step 1: Add failing current-user history tests**

Add records for two users and assert newest-first ordering and strict user filtering through a new model function:

```go
items, total, err := ListUserAffiliateTransferRequests(51, &common.PageInfo{Page: 1, PageSize: 10})
require.NoError(t, err)
assert.EqualValues(t, 2, total)
require.Len(t, items, 2)
assert.Greater(t, items[0].Id, items[1].Id)
for _, item := range items {
    assert.NotZero(t, item.Id)
}
```

In `controller/affiliate_transfer_test.go`, seed one request for the context user and another user, call `ListSelfAffiliateTransferRequests`, decode with `common.Unmarshal`, and assert the body omits `reviewed_by` and `rejected_quota_forfeited_at`:

```go
assert.NotContains(t, recorder.Body.String(), `"reviewed_by"`)
assert.NotContains(t, recorder.Body.String(), `"rejected_quota_forfeited_at"`)
```

- [ ] **Step 2: Run the focused tests and verify RED**

```powershell
go test ./model ./controller -run 'ListUserAffiliateTransferRequests|ListSelfAffiliateTransferRequests' -count=1
```

Expected: compilation fails because the model function and controller handler do not exist.

- [ ] **Step 3: Add the private history DTO and query**

Add a DTO without `UserId`, `ReviewedBy`, or the forfeiture marker:

```go
type AffiliateTransferRequestHistoryItem struct {
    Id                  int    `json:"id"`
    InviteRewardQuota   int    `json:"invite_reward_quota"`
    RechargeRebateQuota int    `json:"recharge_rebate_quota"`
    TotalQuota          int    `json:"total_quota"`
    Status              string `json:"status"`
    CreatedAt           int64  `json:"created_at"`
    ReviewedAt          int64  `json:"reviewed_at"`
    RejectReason        string `json:"reject_reason"`
}
```

Implement the query against `AffiliateTransferRequest` explicitly so GORM does not infer a table from the DTO:

```go
func ListUserAffiliateTransferRequests(
    userId int,
    pageInfo *common.PageInfo,
) ([]*AffiliateTransferRequestHistoryItem, int64, error) {
    baseQuery := DB.Model(&AffiliateTransferRequest{}).
        Where("user_id = ?", userId)

    var total int64
    if err := baseQuery.Count(&total).Error; err != nil {
        return nil, 0, err
    }

    items := make([]*AffiliateTransferRequestHistoryItem, 0)
    if err := baseQuery.
        Select(
            "id", "invite_reward_quota", "recharge_rebate_quota",
            "total_quota", "status", "created_at", "reviewed_at",
            "reject_reason",
        ).
        Order("id desc").
        Limit(pageInfo.GetPageSize()).
        Offset(pageInfo.GetStartIdx()).
        Scan(&items).Error; err != nil {
        return nil, 0, err
    }
    return items, total, nil
}
```

- [ ] **Step 4: Add the handler and route**

Add:

```go
func ListSelfAffiliateTransferRequests(c *gin.Context) {
    pageInfo := common.GetPageQuery(c)
    items, total, err := model.ListUserAffiliateTransferRequests(c.GetInt("id"), pageInfo)
    if err != nil {
        common.ApiError(c, err)
        return
    }
    pageInfo.SetTotal(int(total))
    pageInfo.SetItems(items)
    common.ApiSuccess(c, pageInfo)
}
```

Register it only in the `selfRoute` block guarded by `middleware.UserAuth()`:

```go
selfRoute.GET("/affiliate/transfer-requests/self", controller.ListSelfAffiliateTransferRequests)
```

- [ ] **Step 5: Verify API tests**

Run the command from Step 2. Expected: all focused tests pass.

- [ ] **Step 6: Commit the self-history API**

```powershell
git add model/affiliate_transfer_request.go model/affiliate_transfer_request_test.go controller/affiliate_transfer.go controller/affiliate_transfer_test.go router/api-router.go
git commit -m "feat(wallet): expose personal transfer history"
```

### Task 4: Implement the Wallet Transfer Action State With TDD

**Files:**
- Modify: `web/default/src/features/wallet/lib/affiliate.ts`
- Create: `web/default/src/features/wallet/lib/affiliate.test.ts`
- Modify: `web/default/src/features/wallet/components/affiliate-rewards-card.tsx`
- Modify: `web/default/src/features/wallet/index.tsx`

- [ ] **Step 1: Add the failing business-state test**

Create table-driven tests using `node:test` and `node:assert/strict`:

```ts
import assert from 'node:assert/strict'
import { describe, test } from 'node:test'
import { getAffiliateTransferActionState } from './affiliate'

describe('getAffiliateTransferActionState', () => {
  test('requires the configured one-dollar quota boundary', () => {
    assert.deepEqual(
      getAffiliateTransferActionState({
        totalPendingQuota: 499_999,
        minimumQuota: 500_000,
        pendingRequest: false,
        submittedToday: false,
      }),
      { disabled: true, labelKey: 'Request Transfer', showMinimum: true }
    )
    assert.equal(
      getAffiliateTransferActionState({
        totalPendingQuota: 500_000,
        minimumQuota: 500_000,
        pendingRequest: false,
        submittedToday: false,
      }).disabled,
      false
    )
  })

  test('uses Submitted for pending and same-day requests', () => {
    for (const state of [
      { pendingRequest: true, submittedToday: false },
      { pendingRequest: false, submittedToday: true },
    ]) {
      assert.equal(
        getAffiliateTransferActionState({
          totalPendingQuota: 500_000,
          minimumQuota: 500_000,
          ...state,
        }).labelKey,
        'Submitted'
      )
    }
  })
})
```

- [ ] **Step 2: Run the focused frontend test and verify RED**

```powershell
Set-Location web/default
bun test src/features/wallet/lib/affiliate.test.ts
```

Expected: fail because `getAffiliateTransferActionState` is not exported.

- [ ] **Step 3: Implement the pure domain helper**

Add typed input/output interfaces and return `Submitted` whenever either submitted flag is true. Treat a non-positive minimum defensively as unavailable rather than silently allowing a request:

```ts
export function getAffiliateTransferActionState(input: {
  totalPendingQuota: number
  minimumQuota: number
  pendingRequest: boolean
  submittedToday: boolean
}): {
  disabled: boolean
  labelKey: 'Request Transfer' | 'Submitted'
  showMinimum: boolean
} {
  const submitted = input.pendingRequest || input.submittedToday
  const showMinimum = input.minimumQuota <= 0 || input.totalPendingQuota < input.minimumQuota
  return {
    disabled: submitted || showMinimum,
    labelKey: submitted ? 'Submitted' : 'Request Transfer',
    showMinimum: !submitted && showMinimum,
  }
}
```

- [ ] **Step 4: Wire the helper into the rewards card**

Add `minimumTransferQuota` to `AffiliateRewardsCardProps`. Combine the helper's `disabled` with compliance and in-flight state. Replace the current `Pending Approval`/`Submitted Today` branches with `t(actionState.labelKey)`. Below the action group, show `t('Minimum transfer amount is {{amount}}.', { amount: formatQuota(minimumTransferQuota) })` only when `showMinimum` is true.

In `wallet/index.tsx`, resolve the minimum with the existing system configuration and fallback:

```ts
const minimumTransferQuota =
  currency?.quotaPerUnit && currency.quotaPerUnit > 0
    ? currency.quotaPerUnit
    : DEFAULT_CURRENCY_CONFIG.quotaPerUnit
```

Pass that value to the card. Import `DEFAULT_CURRENCY_CONFIG` from `@/stores/system-config-store`.

- [ ] **Step 5: Run test, typecheck, and targeted lint**

```powershell
bun test src/features/wallet/lib/affiliate.test.ts
bun run typecheck
bunx oxlint -c .oxlintrc.json src/features/wallet/lib/affiliate.ts src/features/wallet/lib/affiliate.test.ts src/features/wallet/components/affiliate-rewards-card.tsx src/features/wallet/index.tsx
```

Expected: all commands exit zero.

- [ ] **Step 6: Commit transfer action behavior**

```powershell
Set-Location ../..
git add web/default/src/features/wallet/lib/affiliate.ts web/default/src/features/wallet/lib/affiliate.test.ts web/default/src/features/wallet/components/affiliate-rewards-card.tsx web/default/src/features/wallet/index.tsx
git commit -m "feat(wallet): show transfer minimum and submitted state"
```

### Task 5: Add the Transfer History Dialog

**Files:**
- Modify: `web/default/src/features/wallet/types.ts`
- Modify: `web/default/src/features/wallet/api.ts`
- Create: `web/default/src/features/wallet/components/dialogs/transfer-history-dialog.tsx`
- Modify: `web/default/src/features/wallet/components/affiliate-rewards-card.tsx`
- Modify: `web/default/src/features/wallet/index.tsx`

- [ ] **Step 1: Add exact API types and function**

Add:

```ts
export interface AffiliateTransferHistoryItem {
  id: number
  invite_reward_quota: number
  recharge_rebate_quota: number
  total_quota: number
  status: 'pending' | 'approved' | 'rejected'
  created_at: number
  reviewed_at: number
  reject_reason: string
}

export interface AffiliateTransferHistoryPage {
  page: number
  page_size: number
  total: number
  items: AffiliateTransferHistoryItem[]
}

export type AffiliateTransferHistoryResponse =
  ApiResponse<AffiliateTransferHistoryPage>
```

Add the API function:

```ts
export async function getAffiliateTransferHistory(
  page: number,
  pageSize: number
): Promise<AffiliateTransferHistoryResponse> {
  const params = new URLSearchParams({
    p: String(page),
    page_size: String(pageSize),
  })
  const res = await api.get(
    `/api/user/affiliate/transfer-requests/self?${params.toString()}`
  )
  return res.data
}
```

- [ ] **Step 2: Build the lazy paginated dialog**

Create `transfer-history-dialog.tsx` using `Dialog`, `useQuery`, `StatusBadge`, `Button`, `Skeleton`, `formatQuota`, and `formatTimestamp`. Keep `page` local, use a fixed page size of 10, and configure:

```ts
const query = useQuery({
  queryKey: ['affiliate-transfer-history', page, 10],
  enabled: props.open,
  staleTime: 0,
  queryFn: async () => {
    const response = await getAffiliateTransferHistory(page, 10)
    if (!response.success || !response.data) {
      throw new Error(response.message || t('Failed to load transfer history'))
    }
    return response.data
  },
})
```

Render compact repeated records with Created, Invitation Reward, Recharge Rebate, Total, Status, Reviewed, and Reject Reason. Use `Approved`, `Rejected`, and `Pending` translation keys and success/danger/warning badge variants. Show a retry button for `isError`, a neutral empty state for zero records, and previous/next buttons derived from `Math.ceil(total / page_size)`. Reset the page to 1 when the dialog is closed.

- [ ] **Step 3: Add and position the history action**

Add `onOpenTransferHistory` to `AffiliateRewardsCardProps`. Render the action group in exactly this order:

```tsx
<Button variant='outline' onClick={() => void props.onRefresh()}>...</Button>
<Button onClick={props.onTransfer} disabled={transferDisabled}>...</Button>
<Button variant='outline' onClick={props.onOpenTransferHistory}>
  <History />
  {t('Transfer History')}
</Button>
```

Keep the right edge occupied by Transfer History so the existing Refresh and Request Transfer buttons shift left. Allow wrapping on narrow screens.

In `wallet/index.tsx`, add `transferHistoryOpen`, pass the open callback to the card, and mount `TransferHistoryDialog` beside the existing dialogs.

- [ ] **Step 4: Run targeted frontend checks**

```powershell
Set-Location web/default
bun run typecheck
bunx oxlint -c .oxlintrc.json src/features/wallet/types.ts src/features/wallet/api.ts src/features/wallet/components/dialogs/transfer-history-dialog.tsx src/features/wallet/components/affiliate-rewards-card.tsx src/features/wallet/index.tsx
```

Expected: both commands exit zero.

- [ ] **Step 5: Commit history UI**

```powershell
Set-Location ../..
git add web/default/src/features/wallet/types.ts web/default/src/features/wallet/api.ts web/default/src/features/wallet/components/dialogs/transfer-history-dialog.tsx web/default/src/features/wallet/components/affiliate-rewards-card.tsx web/default/src/features/wallet/index.tsx
git commit -m "feat(wallet): add transfer history dialog"
```

### Task 6: Complete i18n and End-to-End Verification

**Files:**
- Modify: `web/default/src/i18n/locales/en.json`
- Modify: `web/default/src/i18n/locales/zh.json`
- Modify: `web/default/src/i18n/locales/fr.json`
- Modify: `web/default/src/i18n/locales/ja.json`
- Modify: `web/default/src/i18n/locales/ru.json`
- Modify: `web/default/src/i18n/locales/vi.json`
- Verify: all files from Tasks 1-5

- [ ] **Step 1: Load and follow the project i18n skill**

Read `.agents/skills/i18n-translate/SKILL.md` completely. Add every genuinely missing key used by the implementation, including these exact English source keys:

```text
Transfer History
View your affiliate transfer requests and review results.
Minimum transfer amount is {{amount}}.
No transfer records found.
Your transfer history will appear here.
Reviewed
Failed to load transfer history
```

Reuse existing keys such as `Refresh`, `Request Transfer`, `Submitted`, `Invitation Reward`, `Recharge Rebate`, `Total`, `Created`, `Reject Reason`, `Approved`, `Rejected`, `Pending`, `Retry`, `Showing`, and `of`.

- [ ] **Step 2: Synchronize and validate locales**

```powershell
Set-Location web/default
bun run i18n:sync
bun run format
bun run format:check
```

Expected: synchronization reports no missing used keys and format check exits zero. Inspect the locale diff to ensure all supported locales contain real translations rather than copied English values where a translation is expected.

- [ ] **Step 3: Run backend and frontend verification**

```powershell
Set-Location ../..
go test ./model ./controller -run Affiliate -count=1
go test ./router -count=1
Set-Location web/default
bun test src/features/wallet/lib/affiliate.test.ts
bun run typecheck
bunx oxlint -c .oxlintrc.json src/features/wallet
bun run build
```

Expected: all focused tests, router tests, typecheck, lint, and production build exit zero.

- [ ] **Step 4: Verify the real UI at desktop and mobile widths**

Start the existing local backend/frontend without replacing the user's database. Use the authenticated local session to verify:

1. Below the configured one-dollar equivalent, Request Transfer is unavailable and the minimum message is readable.
2. At or above the boundary, submission succeeds and the action changes to Submitted.
3. Action order is Refresh, Request Transfer, Transfer History; narrow mobile layouts wrap without overlap.
4. History opens lazily, shows only the signed-in user's newest-first records, paginates, and displays reject reasons.
5. Rejecting a newly submitted request removes both reward components, does not credit main balance, and the history record remains visible.

- [ ] **Step 5: Commit translations and final formatting**

```powershell
Set-Location ../..
git add web/default/src/i18n/locales web/default/src/features/wallet
git commit -m "feat(i18n): translate wallet transfer history"
git diff --check
git status --short
```

Expected: no whitespace errors and no uncommitted implementation files; the user's pre-existing `.gitignore` change remains untouched in the original worktree.
