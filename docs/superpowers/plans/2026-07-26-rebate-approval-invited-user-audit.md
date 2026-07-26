# Rebate Approval Invited User Audit Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Show every user invited by a rebate applicant in the admin approval detail, including registration time, last-login time, and soft-deleted status.

**Architecture:** Extend the existing rebate-request detail DTO and its single invited-user query; use `Unscoped()` plus a narrow `Select` so active and soft-deleted invitees are returned without exposing sensitive fields. Add a focused React list component between the existing source summary and recharge-source details, with all visible copy translated through the existing i18n workflow.

**Tech Stack:** Go 1.22, GORM v2, SQLite/MySQL/PostgreSQL-compatible queries, testify, React 19, TypeScript, Base UI/Tailwind, react-i18next, Bun.

---

### Task 1: Protect the backend response contract

**Files:**
- Modify: `model/affiliate_transfer_request_test.go`

- [ ] **Step 1: Write the failing model test**

Add `TestAffiliateTransferRequestDetailIncludesAllInvitedUsers` beside the existing detail tests. Create an owner and request, then create invitees that cover all required behavior:

```go
owner := User{Username: "audit-owner", Password: "password", AffCode: "audit-owner-code"}
require.NoError(t, DB.Create(&owner).Error)

request := AffiliateTransferRequest{
    UserId: owner.Id,
    Status: AffiliateTransferStatusPending,
    CreatedAt: 200,
}
require.NoError(t, DB.Create(&request).Error)

older := User{
    Username: "audit-older",
    DisplayName: "Older User",
    Password: "password",
    AffCode: "audit-older-code",
    InviterId: owner.Id,
    CreatedAt: 100,
    LastLoginAt: 150,
}
newerNeverLoggedIn := User{
    Username: "audit-newer",
    Password: "password",
    AffCode: "audit-newer-code",
    InviterId: owner.Id,
    CreatedAt: 300,
}
deletedAtSameTime := User{
    Username: "audit-deleted",
    DisplayName: "Deleted User",
    Password: "password",
    AffCode: "audit-deleted-code",
    InviterId: owner.Id,
    CreatedAt: 300,
    LastLoginAt: 350,
}
require.NoError(t, DB.Create(&older).Error)
require.NoError(t, DB.Create(&newerNeverLoggedIn).Error)
require.NoError(t, DB.Create(&deletedAtSameTime).Error)
require.NoError(t, DB.Delete(&deletedAtSameTime).Error)

detail, err := GetAffiliateTransferRequestDetail(request.Id)
require.NoError(t, err)
require.Len(t, detail.InvitedUsers, 3)
assert.Equal(t, 3, detail.InvitedCount)
assert.Equal(t, deletedAtSameTime.Id, detail.InvitedUsers[0].Id)
assert.Equal(t, newerNeverLoggedIn.Id, detail.InvitedUsers[1].Id)
assert.Equal(t, older.Id, detail.InvitedUsers[2].Id)
assert.Equal(t, int64(100), detail.InvitedUsers[2].CreatedAt)
assert.Equal(t, int64(150), detail.InvitedUsers[2].LastLoginAt)
assert.Zero(t, detail.InvitedUsers[1].LastLoginAt)
assert.True(t, detail.InvitedUsers[0].IsDeleted)
assert.False(t, detail.InvitedUsers[1].IsDeleted)
```

Do not create top-ups for these users; their presence proves the audit list is independent of recharge contribution and request creation time.

- [ ] **Step 2: Run the focused test and verify RED**

Run:

```powershell
go test ./model -run TestAffiliateTransferRequestDetailIncludesAllInvitedUsers -count=1
```

Expected: compilation fails because `AffiliateTransferRequestDetail.InvitedUsers` does not exist.

- [ ] **Step 3: Commit the failing test**

```powershell
git add model/affiliate_transfer_request_test.go
git commit -m "test: cover rebate invitee audit detail"
```

### Task 2: Return invited-user audit data from the backend

**Files:**
- Modify: `model/affiliate_transfer_request.go`
- Test: `model/affiliate_transfer_request_test.go`

- [ ] **Step 1: Add the narrow response DTO**

Define a response-only type next to the existing affiliate detail types and add it to the detail response:

```go
type AffiliateInvitedUserDetail struct {
    Id          int    `json:"id"`
    Username    string `json:"username"`
    DisplayName string `json:"display_name"`
    CreatedAt   int64  `json:"created_at"`
    LastLoginAt int64  `json:"last_login_at"`
    IsDeleted   bool   `json:"is_deleted"`
}

type AffiliateTransferRequestDetail struct {
    AffiliateTransferRequest
    Username                  string                        `json:"username"`
    DisplayName               string                        `json:"display_name"`
    InvitedUsers              []AffiliateInvitedUserDetail  `json:"invited_users"`
    InvitedCount              int                           `json:"invited_count"`
    TotalInvitedRechargeQuota int                           `json:"total_invited_recharge_quota"`
    RechargeRebateRate        float64                       `json:"recharge_rebate_rate"`
    RechargeSources           []AffiliateRechargeSourceItem `json:"recharge_sources"`
}
```

- [ ] **Step 2: Enrich the existing invited-user query**

In `GetAffiliateTransferRequestDetail`, keep one query and make it include soft-deleted rows while selecting only audit-safe columns:

```go
var invitedUsers []User
if err := DB.Unscoped().
    Select("id", "username", "display_name", "created_at", "last_login_at", "deleted_at").
    Where("inviter_id = ?", item.UserId).
    Order("created_at DESC, id DESC").
    Find(&invitedUsers).Error; err != nil {
    return nil, err
}

invitedUserDetails := make([]AffiliateInvitedUserDetail, 0, len(invitedUsers))
invitedNames := make(map[int]string, len(invitedUsers))
invitedIds := make([]int, 0, len(invitedUsers))
for _, invited := range invitedUsers {
    invitedIds = append(invitedIds, invited.Id)
    name := invited.DisplayName
    if name == "" {
        name = invited.Username
    }
    invitedNames[invited.Id] = name
    invitedUserDetails = append(invitedUserDetails, AffiliateInvitedUserDetail{
        Id: invited.Id,
        Username: invited.Username,
        DisplayName: invited.DisplayName,
        CreatedAt: invited.CreatedAt,
        LastLoginAt: invited.LastLoginAt,
        IsDeleted: invited.DeletedAt.Valid,
    })
}
```

Assign `InvitedUsers: invitedUserDetails` in the returned detail. Because the slice is initialized with `make`, a request with no invitees serializes as `[]`, not `null`.

- [ ] **Step 3: Run focused and package tests and verify GREEN**

Run:

```powershell
go test ./model -run 'TestAffiliateTransferRequestDetail' -count=1
go test ./model ./controller -run 'Affiliate' -count=1
```

Expected: all selected tests pass.

- [ ] **Step 4: Format and commit the backend change**

```powershell
gofmt -w model/affiliate_transfer_request.go model/affiliate_transfer_request_test.go
git add model/affiliate_transfer_request.go model/affiliate_transfer_request_test.go
git commit -m "feat: expose invited users in rebate detail"
```

### Task 3: Protect the invited-user presentation behavior

**Files:**
- Create: `web/default/src/features/rebate-approvals/lib/invited-user-display.ts`
- Create: `web/default/src/features/rebate-approvals/lib/invited-user-display.test.ts`

- [ ] **Step 1: Write failing display-model tests**

Add deterministic tests for the two user-visible fallback rules:

```ts
import { describe, expect, it } from 'vitest'
import { formatTimestamp } from '@/lib/format'
import {
  getInvitedUserDisplayName,
  getInvitedUserLastLogin,
} from './invited-user-display'

describe('invited user display', () => {
  it('falls back from display name to username to a neutral placeholder', () => {
    expect(getInvitedUserDisplayName({ display_name: 'Visible', username: 'user' })).toBe('Visible')
    expect(getInvitedUserDisplayName({ display_name: '', username: 'user' })).toBe('user')
    expect(getInvitedUserDisplayName({ display_name: '', username: '' })).toBe('***')
  })

  it('shows a dash when the user has never logged in', () => {
    expect(getInvitedUserLastLogin(0)).toBe('-')
    expect(getInvitedUserLastLogin(1_700_000_000)).toBe(
      formatTimestamp(1_700_000_000)
    )
  })
})
```

- [ ] **Step 2: Run the test and verify RED**

Run from `web/default/`:

```powershell
& 'C:\Users\Administrator\.cherrystudio\bin\bun.exe' test src/features/rebate-approvals/lib/invited-user-display.test.ts
```

Expected: the test fails because `invited-user-display.ts` does not exist.

- [ ] **Step 3: Implement the minimal display helpers**

```ts
import { formatTimestamp } from '@/lib/format'

export function getInvitedUserDisplayName(user: {
  display_name: string
  username: string
}): string {
  return user.display_name || user.username || '***'
}

export function getInvitedUserLastLogin(lastLoginAt: number): string {
  return lastLoginAt === 0 ? '-' : formatTimestamp(lastLoginAt)
}
```

- [ ] **Step 4: Run the test and verify GREEN**

Run the same Bun test command. Expected: both tests pass.

- [ ] **Step 5: Commit the display-model contract**

```powershell
git add web/default/src/features/rebate-approvals/lib/invited-user-display.ts web/default/src/features/rebate-approvals/lib/invited-user-display.test.ts
git commit -m "test: cover rebate invitee presentation"
```

### Task 4: Add the invited-user audit list to the detail dialog

**Files:**
- Modify: `web/default/src/features/rebate-approvals/types.ts`
- Create: `web/default/src/features/rebate-approvals/components/rebate-invited-user-list.tsx`
- Modify: `web/default/src/features/rebate-approvals/components/rebate-approval-detail-dialog.tsx`
- Modify through i18n script: `web/default/src/i18n/locales/*.json`

- [ ] **Step 1: Extend the frontend API types**

```ts
export interface RebateApprovalInvitedUser {
  id: number
  username: string
  display_name: string
  created_at: number
  last_login_at: number
  is_deleted: boolean
}

export interface RebateApprovalDetail extends RebateApprovalRequest {
  invited_users: RebateApprovalInvitedUser[]
  invited_count: number
  total_invited_recharge_quota: number
  recharge_rebate_rate: number
  recharge_sources: RebateApprovalRechargeSource[]
}
```

- [ ] **Step 2: Create the focused list component**

Create `RebateInvitedUserList` that accepts `users: RebateApprovalInvitedUser[]`, calls its own `useTranslation()`, and renders:

```tsx
<div className='min-w-0 space-y-2'>
  <Label className='text-xs font-semibold'>{t('Invited User Details')}</Label>
  {props.users.length === 0 ? (
    <div className='text-muted-foreground bg-muted/30 rounded-md border p-3 text-sm'>
      {t('No invited users found.')}
    </div>
  ) : (
    <div className='space-y-2'>
      {props.users.map((user) => (
        <div key={user.id} className='bg-background min-w-0 rounded-md border p-3'>
          <div className='mb-2 flex min-w-0 items-center justify-between gap-2'>
            <div className='min-w-0 truncate text-sm font-medium'>
              {getInvitedUserDisplayName(user)}
            </div>
            {user.is_deleted && <Badge variant='destructive'>{t('Deleted')}</Badge>}
          </div>
          <div className='grid gap-1.5 text-xs sm:grid-cols-2'>
            <AuditField label={t('User ID')} value={user.id} mono />
            <AuditField label={t('Created At')} value={formatTimestamp(user.created_at)} mono />
            <AuditField label={t('Last Login')} value={getInvitedUserLastLogin(user.last_login_at)} mono />
          </div>
        </div>
      ))}
    </div>
  )}
</div>
```

Keep the compact field renderer local to this component because it expresses the component's row layout and has no cross-feature caller.

- [ ] **Step 3: Insert the component in the confirmed location**

In `rebate-approval-detail-dialog.tsx`, render:

```tsx
<RebateInvitedUserList users={detail.invited_users ?? []} />
```

Place it after `Recharge Rebate Sources` and before `Source Details`. Preserve all existing recharge-source behavior.

- [ ] **Step 4: Add and synchronize i18n keys**

Use the `i18n-translate` skill workflow and a temporary script to add translated values for only the missing keys:

- `Invited User Details`
- `No invited users found.`

Reuse existing keys for `User ID`, `Created At`, `Last Login`, and `Deleted`. Run from `web/default/`:

```powershell
& 'C:\Users\Administrator\.cherrystudio\bin\bun.exe' run i18n:sync
```

Expected: locale files remain valid, ordered JSON and the sync report shows no missing key introduced by this feature. Remove the temporary script before staging.

- [ ] **Step 5: Run focused frontend verification**

Run from `web/default/`:

```powershell
& 'C:\Users\Administrator\.cherrystudio\bin\bun.exe' test src/features/rebate-approvals
& 'C:\Users\Administrator\.cherrystudio\bin\bun.exe' run typecheck
```

Expected: tests and type checking pass.

- [ ] **Step 6: Format and commit the frontend change**

```powershell
& 'C:\Users\Administrator\.cherrystudio\bin\bun.exe' run format
git add web/default/src/features/rebate-approvals web/default/src/i18n/locales
git commit -m "feat: show invited users in rebate approval detail"
```

### Task 5: Full regression verification and review

**Files:**
- Verify all files changed by Tasks 1-4

- [ ] **Step 1: Run backend regression tests**

```powershell
go test ./model ./controller -run 'Affiliate' -count=1
```

Expected: all affiliate model and controller tests pass.

- [ ] **Step 2: Run frontend tests and static checks**

From `web/default/`:

```powershell
& 'C:\Users\Administrator\.cherrystudio\bin\bun.exe' test src/features/rebate-approvals
& 'C:\Users\Administrator\.cherrystudio\bin\bun.exe' run typecheck
& 'C:\Users\Administrator\.cherrystudio\bin\bun.exe' run lint
& 'C:\Users\Administrator\.cherrystudio\bin\bun.exe' run format:check
& 'C:\Users\Administrator\.cherrystudio\bin\bun.exe' run build
```

Expected: every command exits successfully.

- [ ] **Step 3: Inspect the final diff for scope and data safety**

```powershell
git status --short
git diff HEAD~3 -- model/affiliate_transfer_request.go model/affiliate_transfer_request_test.go web/default/src/features/rebate-approvals web/default/src/i18n/locales
```

Confirm that the backend selects only the six approved invited-user columns, the frontend shows no new/old or recharge labels, and no schema migration or unrelated files are included.

- [ ] **Step 4: Create a final verification commit only if formatting changed files**

```powershell
git add model/affiliate_transfer_request.go model/affiliate_transfer_request_test.go web/default/src/features/rebate-approvals web/default/src/i18n/locales
git commit -m "chore: finalize rebate invitee audit detail"
```

Skip this commit when `git status --short` is empty.
