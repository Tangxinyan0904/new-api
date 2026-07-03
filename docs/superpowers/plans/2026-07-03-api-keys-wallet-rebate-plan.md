# API Keys Wallet Rebate Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add API key daily usage, hide expiration by default, and implement wallet recharge rebate transfer requests with admin approval.

**Architecture:** Token daily usage is calculated server-side in the token list/search response so the frontend receives a row-level `today_quota`. Wallet rebates use a new transfer request model that keeps invitation registration rewards and invited-user recharge rebates separate while sharing one approval workflow.

**Tech Stack:** Go, Gin, GORM, SQLite/MySQL/PostgreSQL-compatible SQL, React, TanStack Router/Table/Query, TypeScript, Rsbuild.

---

## File Map

- Modify `model/token.go`: add response-only `TodayQuota` and page-level daily usage aggregation.
- Modify `controller/token.go`: attach `today_quota` before returning token list/search data.
- Modify `web/default/src/features/keys/types.ts`: add `today_quota` to the API key schema.
- Modify `web/default/src/features/keys/components/api-keys-columns.tsx`: add a `Today Usage` column after `Quota`.
- Modify `web/default/src/features/keys/components/api-keys-table.tsx`: hide `expired_time` by default and show today usage on mobile cards.
- Create `model/affiliate_transfer_request.go`: request model, summary structs, rebate math, daily limit, approval/rejection logic.
- Modify `model/main.go`: include `AffiliateTransferRequest` in auto migration.
- Create `controller/affiliate_transfer.go`: user summary/request endpoints and admin list/approve/reject endpoints.
- Modify `router/api-router.go`: wire new user/admin routes.
- Modify `web/default/src/features/wallet/types.ts`: add rebate summary and transfer request types.
- Modify `web/default/src/features/wallet/api.ts`: add rebate summary/request API calls.
- Modify `web/default/src/features/wallet/hooks/use-affiliate.ts`: load rebate summary and submit transfer requests.
- Modify `web/default/src/features/wallet/components/affiliate-rewards-card.tsx`: display masked invitees, invite reward, recharge rebate, and pending approval state.
- Modify `web/default/src/features/wallet/index.tsx`: pass rebate data into wallet components and refresh user data after request submission.
- Modify `web/default/src/features/wallet/components/dialogs/transfer-dialog.tsx`: change immediate-transfer wording to approval-request wording.
- Create `web/default/src/features/rebate-approvals/`: admin approval feature module.
- Create `web/default/src/routes/_authenticated/rebate-approvals/index.tsx`: admin route.
- Modify `web/default/src/hooks/use-sidebar-data.ts`: add Admin sidebar item `Rebate Approvals` / `返利审批`.
- Update `web/default/src/routeTree.gen.ts` if route generation changes it during typecheck/build.

## Task 1: API Key Today Usage Backend

**Files:**
- Modify: `model/token.go`
- Modify: `controller/token.go`
- Test: `controller/token_test.go`

- [ ] **Step 1: Add failing token list test**

Add this test near existing token controller tests. If helper names differ, reuse the local test helpers already present in `controller/token_test.go`.

```go
func TestGetAllTokensIncludesTodayQuota(t *testing.T) {
    db := setupTokenControllerTestDB(t)
    model.LOG_DB = db
    userID := 1
    todayStart := time.Now().Truncate(24 * time.Hour).Unix()
    token := seedToken(t, db, userID, "daily-token", "daily1234token5678")
    other := seedToken(t, db, userID, "other-token", "other1234token5678")

    require.NoError(t, db.Create(&model.Log{UserId: userID, Type: model.LogTypeConsume, TokenId: token.Id, Quota: 120, CreatedAt: todayStart + 60}).Error)
    require.NoError(t, db.Create(&model.Log{UserId: userID, Type: model.LogTypeConsume, TokenId: token.Id, Quota: 999, CreatedAt: todayStart - 60}).Error)
    require.NoError(t, db.Create(&model.Log{UserId: userID, Type: model.LogTypeConsume, TokenId: other.Id, Quota: 50, CreatedAt: todayStart + 90}).Error)

    ctx := newTokenControllerContext(t, userID, "p=1&size=20")
    GetAllTokens(ctx)

    var payload struct {
        Success bool `json:"success"`
        Data struct {
            Items []model.Token `json:"items"`
        } `json:"data"`
    }
    require.NoError(t, json.Unmarshal(ctx.Response.Body.Bytes(), &payload))
    require.True(t, payload.Success)
    got := map[string]int{}
    for _, item := range payload.Data.Items {
        got[item.Name] = item.TodayQuota
    }
    require.Equal(t, 120, got["daily-token"])
    require.Equal(t, 50, got["other-token"])
}
```

- [ ] **Step 2: Run failing test**

Run: `go test ./controller -run TestGetAllTokensIncludesTodayQuota -count=1`

Expected: fail because `TodayQuota` is not implemented.

- [ ] **Step 3: Implement token aggregation**

In `model.Token`, add:

```go
TodayQuota int `json:"today_quota" gorm:"-"`
```

Add `time` to imports and implement:

```go
func GetTodayTokenQuotaMap(userId int, tokenIds []int, startTimestamp int64, endTimestamp int64) (map[int]int, error) {
    result := make(map[int]int, len(tokenIds))
    if len(tokenIds) == 0 {
        return result, nil
    }

    rows := []struct {
        TokenId int `gorm:"column:token_id"`
        Quota   int `gorm:"column:quota"`
    }{}

    err := LOG_DB.Table("logs").
        Select("token_id, COALESCE(sum(quota), 0) as quota").
        Where("user_id = ? AND token_id IN ? AND type = ? AND created_at >= ? AND created_at <= ?", userId, tokenIds, LogTypeConsume, startTimestamp, endTimestamp).
        Group("token_id").
        Scan(&rows).Error
    if err != nil {
        return nil, err
    }

    for _, row := range rows {
        result[row.TokenId] = row.Quota
    }
    return result, nil
}

func AttachTodayQuotaToTokens(userId int, tokens []*Token) error {
    ids := make([]int, 0, len(tokens))
    for _, token := range tokens {
        ids = append(ids, token.Id)
    }

    now := time.Now()
    start := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()).Unix()
    quotas, err := GetTodayTokenQuotaMap(userId, ids, start, now.Unix())
    if err != nil {
        return err
    }

    for _, token := range tokens {
        token.TodayQuota = quotas[token.Id]
    }
    return nil
}
```

- [ ] **Step 4: Attach usage in controllers**

In `controller/token.go`, call before returning masked tokens in both `GetAllTokens` and `SearchTokens`:

```go
if err := model.AttachTodayQuotaToTokens(userId, tokens); err != nil {
    common.ApiError(c, err)
    return
}
```

- [ ] **Step 5: Verify backend token behavior**

Run: `go test ./controller -run TestGetAllTokensIncludesTodayQuota -count=1`

Expected: pass.

## Task 2: API Key Today Usage Frontend

**Files:**
- Modify: `web/default/src/features/keys/types.ts`
- Modify: `web/default/src/features/keys/components/api-keys-columns.tsx`
- Modify: `web/default/src/features/keys/components/api-keys-table.tsx`

- [ ] **Step 1: Add schema field**

Add to `apiKeySchema`:

```ts
today_quota: z.number().optional().default(0),
```

- [ ] **Step 2: Add visible table column**

Insert after the quota column in `api-keys-columns.tsx`:

```tsx
{
  id: 'today_quota',
  accessorKey: 'today_quota',
  header: t('Today Usage'),
  cell: ({ row }) => (
    <span className='font-medium tabular-nums'>
      {formatQuota(row.original.today_quota ?? 0)}
    </span>
  ),
  size: 150,
}
```

- [ ] **Step 3: Hide expiration by default**

Update `initialColumnVisibility` in `api-keys-table.tsx`:

```ts
initialColumnVisibility: {
  model_limits: false,
  allow_ips: false,
  expired_time: false,
},
```

- [ ] **Step 4: Add mobile display row**

In `ApiKeysMobileList`, below the existing quota row, add:

```tsx
<div className='flex items-center justify-between gap-2 text-xs'>
  <span className='text-muted-foreground'>{t('Today Usage')}</span>
  <span className='font-medium tabular-nums'>
    {formatQuota(apiKey.today_quota ?? 0)}
  </span>
</div>
```

- [ ] **Step 5: Verify frontend key files**

Run from `web/default`:

`..\node_modules\.bin\oxlint.exe -c .oxlintrc.json src\features\keys\types.ts src\features\keys\components\api-keys-columns.tsx src\features\keys\components\api-keys-table.tsx`

Expected: exit 0.

## Task 3: Affiliate Transfer Backend Model

**Files:**
- Create: `model/affiliate_transfer_request.go`
- Modify: `model/main.go`
- Test: `model/affiliate_transfer_request_test.go`

- [ ] **Step 1: Write model tests**

Create tests covering masked invitees, rebate = credited quota * 5%, daily request limit, pending request block, approval, and rejection.

- [ ] **Step 2: Create model and constants**

Create `model/affiliate_transfer_request.go` with:

```go
const (
    AffiliateTransferStatusPending  = "pending"
    AffiliateTransferStatusApproved = "approved"
    AffiliateTransferStatusRejected = "rejected"
    AffiliateRechargeRebateRate     = 0.05
)

type AffiliateTransferRequest struct {
    Id                  int    `json:"id"`
    UserId              int    `json:"user_id" gorm:"index"`
    InviteRewardQuota   int    `json:"invite_reward_quota"`
    RechargeRebateQuota int    `json:"recharge_rebate_quota"`
    TotalQuota          int    `json:"total_quota"`
    Status              string `json:"status" gorm:"type:varchar(32);index"`
    CreatedAt           int64  `json:"created_at" gorm:"index"`
    ReviewedAt          int64  `json:"reviewed_at"`
    ReviewedBy          int    `json:"reviewed_by" gorm:"index"`
    RejectReason        string `json:"reject_reason" gorm:"type:varchar(255)"`
}
```

- [ ] **Step 3: Implement summary structs and masking**

Add structs:

```go
type AffiliateInvitedUserSummary struct {
    Id          int    `json:"id"`
    DisplayName string `json:"display_name"`
}

type AffiliateRebateSummary struct {
    InvitedUsers                []AffiliateInvitedUserSummary `json:"invited_users"`
    InvitedCount                int                           `json:"invited_count"`
    TotalInvitedRechargeQuota   int                           `json:"total_invited_recharge_quota"`
    InviteRewardQuota           int                           `json:"invite_reward_quota"`
    RechargeRebateQuota         int                           `json:"recharge_rebate_quota"`
    TotalPendingQuota           int                           `json:"total_pending_quota"`
    PendingRequest              *AffiliateTransferRequest     `json:"pending_request,omitempty"`
    SubmittedToday              bool                          `json:"submitted_today"`
}
```

Mask names with a helper that never returns the full username/display name.

- [ ] **Step 4: Implement credited quota calculation**

Implement a helper that treats successful top-ups consistently with existing completion flows:

- Creem and any provider whose `amount` already stores internal quota: use `Amount` directly.
- Epay, Waffo, Waffo Pancake, and non-Creem display-unit top-ups: multiply `Amount * common.QuotaPerUnit`.
- Stripe: use `Money * common.QuotaPerUnit`, matching the current `Recharge` path.

- [ ] **Step 5: Implement lifecycle functions**

Implement:

```go
func GetAffiliateRebateSummary(userId int) (*AffiliateRebateSummary, error)
func CreateAffiliateTransferRequest(userId int) (*AffiliateTransferRequest, error)
func ListAffiliateTransferRequests(status string, pageInfo *common.PageInfo) ([]*AffiliateTransferRequestListItem, int64, error)
func ApproveAffiliateTransferRequest(requestId int, reviewerId int) error
func RejectAffiliateTransferRequest(requestId int, reviewerId int, reason string) error
```

Approval must be transactional and idempotent for non-pending rows: only pending rows can mutate balance.

- [ ] **Step 6: Register migration**

Add `&AffiliateTransferRequest{}` to the model migration list in `model/main.go`.

- [ ] **Step 7: Verify model tests**

Run: `go test ./model -run Affiliate -count=1`

Expected: pass.

## Task 4: Affiliate Transfer Controllers and Routes

**Files:**
- Create: `controller/affiliate_transfer.go`
- Modify: `router/api-router.go`
- Test: `controller/affiliate_transfer_test.go`

- [ ] **Step 1: Add controller handlers**

Create user handlers:

```go
func GetAffiliateRebateSummary(c *gin.Context) {
    summary, err := model.GetAffiliateRebateSummary(c.GetInt("id"))
    if err != nil {
        common.ApiError(c, err)
        return
    }
    common.ApiSuccess(c, summary)
}

func CreateAffiliateTransferRequest(c *gin.Context) {
    request, err := model.CreateAffiliateTransferRequest(c.GetInt("id"))
    if err != nil {
        common.ApiError(c, err)
        return
    }
    common.ApiSuccess(c, request)
}
```

Add admin list/approve/reject handlers using `common.GetPageQuery(c)` and `strconv.Atoi(c.Param("id"))`.

- [ ] **Step 2: Wire routes**

Inside authenticated self route:

```go
selfRoute.GET("/affiliate/rebate-summary", controller.GetAffiliateRebateSummary)
selfRoute.POST("/affiliate/transfer-request", controller.CreateAffiliateTransferRequest)
```

Inside admin user route:

```go
adminRoute.GET("/affiliate/transfer-requests", controller.ListAffiliateTransferRequests)
adminRoute.POST("/affiliate/transfer-requests/:id/approve", controller.ApproveAffiliateTransferRequest)
adminRoute.POST("/affiliate/transfer-requests/:id/reject", controller.RejectAffiliateTransferRequest)
```

- [ ] **Step 3: Verify controller tests**

Run: `go test ./controller -run AffiliateTransfer -count=1`

Expected: pass.

## Task 5: Wallet Frontend Rebate UI

**Files:**
- Modify: `web/default/src/features/wallet/types.ts`
- Modify: `web/default/src/features/wallet/api.ts`
- Modify: `web/default/src/features/wallet/hooks/use-affiliate.ts`
- Modify: `web/default/src/features/wallet/components/affiliate-rewards-card.tsx`
- Modify: `web/default/src/features/wallet/index.tsx`
- Modify: `web/default/src/features/wallet/components/dialogs/transfer-dialog.tsx`

- [ ] **Step 1: Add frontend types**

Add `AffiliateInvitedUserSummary`, `AffiliateTransferRequest`, `AffiliateRebateSummary`, `AffiliateRebateSummaryResponse`, and `AffiliateTransferRequestResponse` in wallet types.

- [ ] **Step 2: Add API functions**

Add to wallet API:

```ts
export async function getAffiliateRebateSummary(): Promise<AffiliateRebateSummaryResponse> {
  const res = await api.get('/api/user/affiliate/rebate-summary')
  return res.data
}

export async function createAffiliateTransferRequest(): Promise<AffiliateTransferRequestResponse> {
  const res = await api.post('/api/user/affiliate/transfer-request')
  return res.data
}
```

- [ ] **Step 3: Update affiliate hook**

Fetch summary, expose `rebateSummary`, `requestTransfer`, and `requestingTransfer`. Keep referral link loading behavior intact.

- [ ] **Step 4: Update referral card**

Display:

- `Invitation Reward`: `formatQuota(summary.invite_reward_quota)`
- `Recharge Rebate`: `formatQuota(summary.recharge_rebate_quota)`
- `Invited Recharge`: `formatQuota(summary.total_invited_recharge_quota)`
- masked invitees as badges or compact text
- total pending amount
- button label `审批中` when `summary.pending_request` exists

- [ ] **Step 5: Update transfer dialog**

Change title to `Submit Transfer Request`, body to explain admin approval, and confirm button to `Submit Request`.

- [ ] **Step 6: Verify wallet frontend**

Run from `web/default`: `..\node_modules\.bin\oxlint.exe -c .oxlintrc.json src\features\wallet`

Expected: exit 0.

## Task 6: Admin Rebate Approval Frontend

**Files:**
- Create: `web/default/src/features/rebate-approvals/types.ts`
- Create: `web/default/src/features/rebate-approvals/api.ts`
- Create: `web/default/src/features/rebate-approvals/index.tsx`
- Create: `web/default/src/features/rebate-approvals/components/rebate-approvals-table.tsx`
- Create: `web/default/src/features/rebate-approvals/components/rebate-approvals-columns.tsx`
- Create: `web/default/src/routes/_authenticated/rebate-approvals/index.tsx`
- Modify: `web/default/src/hooks/use-sidebar-data.ts`

- [ ] **Step 1: Create admin API/types**

Define list item fields matching backend response and functions `getRebateTransferRequests`, `approveRebateTransferRequest`, `rejectRebateTransferRequest`.

- [ ] **Step 2: Create approval table**

Use existing `DataTablePage`, `useTableUrlState`, and `useDataTable` patterns. Columns: applicant, invite reward, recharge rebate, total, status, created, reviewed, actions.

- [ ] **Step 3: Create route**

Route file:

```tsx
import { createFileRoute, redirect } from '@tanstack/react-router'
import { RebateApprovals } from '@/features/rebate-approvals'
import { ROLE } from '@/lib/roles'
import { useAuthStore } from '@/stores/auth-store'

export const Route = createFileRoute('/_authenticated/rebate-approvals/')({
  beforeLoad: () => {
    const role = useAuthStore.getState().auth.user?.role ?? ROLE.GUEST
    if (role < ROLE.ADMIN) throw redirect({ to: '/dashboard/overview' })
  },
  component: RebateApprovals,
})
```

- [ ] **Step 4: Add sidebar item**

Import `ReceiptText` from `lucide-react` and add:

```ts
{
  title: t('Rebate Approvals'),
  url: '/rebate-approvals',
  icon: ReceiptText,
},
```

- [ ] **Step 5: Verify route/type generation**

Run from `web/default`: `..\node_modules\.bin\tsgo.exe -b tsconfig.json`

Expected: exit 0. If routeTree is stale, run Rsbuild build/dev and include updated generated route file.

## Task 7: Full Verification

**Files:** all touched files.

- [ ] **Step 1: Backend tests**

Run: `go test ./model ./controller -count=1`

Expected: pass.

- [ ] **Step 2: Frontend lint**

Run from `web/default`:

`..\node_modules\.bin\oxlint.exe -c .oxlintrc.json src\features\keys src\features\wallet src\features\rebate-approvals src\routes\_authenticated\rebate-approvals src\hooks\use-sidebar-data.ts`

Expected: pass.

- [ ] **Step 3: Frontend typecheck**

Run from `web/default`: `..\node_modules\.bin\tsgo.exe -b tsconfig.json`

Expected: pass.

- [ ] **Step 4: Manual smoke test**

Start backend and frontend. Verify API key today usage, hidden expiration column, wallet rebate card, transfer request pending state, admin approval list, approve behavior, and reject behavior.
