# Disable Quota Notifications and Expand API Key Group Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Stop every wallet/subscription quota-exhaustion notification while preserving other notifications, then hide the API key creation-time column by default and use its width for a readable group selector.

**Architecture:** Remove quota-notification scheduling from settlement paths and add a defense-in-depth `quota_exceed` guard at the notification service boundary. Remove the obsolete per-user email latch and its balance-recovery hooks. Keep the API key table structure intact, changing only default visibility, group-column sizing, and expanded-menu text wrapping.

**Tech Stack:** Go 1.22+, Gin, GORM v2, testify, React 19, TypeScript, TanStack Table, Base UI/shadcn components, Tailwind CSS, Bun, Playwright.

---

### Task 1: Make quota-exhaustion notifications a disabled notification contract

**Files:**
- Create: `service/user_notify_test.go`
- Modify: `service/user_notify.go:50-55`

- [ ] **Step 1: Write the failing notification-boundary tests**

Create `service/user_notify_test.go` with tests that prove all four configured channels are skipped before rate limiting, while a different notification type still reaches normal dispatch:

```go
package service

import (
	"fmt"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNotifyUserSkipsQuotaExceedForEveryChannel(t *testing.T) {
	originalRedisEnabled := common.RedisEnabled
	originalSMTPServer := common.SMTPServer
	originalSMTPAccount := common.SMTPAccount
	originalSMTPFrom := common.SMTPFrom
	common.RedisEnabled = false
	common.SMTPServer = ""
	common.SMTPAccount = ""
	common.SMTPFrom = "sender@example.com"
	t.Cleanup(func() {
		common.RedisEnabled = originalRedisEnabled
		common.SMTPServer = originalSMTPServer
		common.SMTPAccount = originalSMTPAccount
		common.SMTPFrom = originalSMTPFrom
	})

	settings := []dto.UserSetting{
		{NotifyType: dto.NotifyTypeEmail, NotificationEmail: "receiver@example.com"},
		{NotifyType: dto.NotifyTypeWebhook, WebhookUrl: "://invalid"},
		{NotifyType: dto.NotifyTypeBark, BarkUrl: "://invalid"},
		{NotifyType: dto.NotifyTypeGotify, GotifyUrl: "://invalid", GotifyToken: "token"},
	}

	for index, setting := range settings {
		t.Run(setting.NotifyType, func(t *testing.T) {
			userID := 9100 + index
			limitKey := fmt.Sprintf(
				"%d:%s:%s",
				userID,
				dto.NotifyTypeQuotaExceed,
				time.Now().Format("2006010215"),
			)
			notifyLimitStore.Delete(limitKey)

			err := NotifyUser(
				userID,
				"receiver@example.com",
				setting,
				dto.NewNotify(dto.NotifyTypeQuotaExceed, "quota", "quota", nil),
			)
			require.NoError(t, err)

			_, limited := notifyLimitStore.Load(limitKey)
			assert.False(t, limited)
		})
	}
}

func TestNotifyUserKeepsOtherNotificationTypesEnabled(t *testing.T) {
	originalRedisEnabled := common.RedisEnabled
	common.RedisEnabled = false
	t.Cleanup(func() { common.RedisEnabled = originalRedisEnabled })

	const userID = 9200
	limitKey := fmt.Sprintf(
		"%d:%s:%s",
		userID,
		dto.NotifyTypeChannelUpdate,
		time.Now().Format("2006010215"),
	)
	notifyLimitStore.Delete(limitKey)

	err := NotifyUser(
		userID,
		"",
		dto.UserSetting{NotifyType: dto.NotifyTypeWebhook, WebhookUrl: "://invalid"},
		dto.NewNotify(dto.NotifyTypeChannelUpdate, "update", "update", nil),
	)
	require.Error(t, err)

	_, limited := notifyLimitStore.Load(limitKey)
	assert.True(t, limited)
}
```

- [ ] **Step 2: Run the new test and verify the quota case fails**

Run:

```powershell
go test ./service -run 'TestNotifyUser(SkipsQuotaExceedForEveryChannel|KeepsOtherNotificationTypesEnabled)' -count=1
```

Expected: `TestNotifyUserSkipsQuotaExceedForEveryChannel` fails because current code enters rate limiting and channel dispatch; the non-quota test passes.

- [ ] **Step 3: Add the centralized quota-notification guard**

At the beginning of `NotifyUser`, before resolving `NotifyType` or calling `CheckNotificationLimit`, add:

```go
func NotifyUser(userId int, userEmail string, userSetting dto.UserSetting, data dto.Notify) error {
	if data.Type == dto.NotifyTypeQuotaExceed {
		return nil
	}

	notifyType := userSetting.NotifyType
```

Do not change any other notification branch.

- [ ] **Step 4: Run the focused tests and verify they pass**

Run:

```powershell
gofmt -w service/user_notify.go service/user_notify_test.go
go test ./service -run 'TestNotifyUser(SkipsQuotaExceedForEveryChannel|KeepsOtherNotificationTypesEnabled)' -count=1
```

Expected: both tests pass; the quota tests do not create notification-limit entries.

- [ ] **Step 5: Commit the notification contract**

```powershell
git add -- service/user_notify.go service/user_notify_test.go
git commit -m "fix(notify): disable quota exhaustion notifications"
```

### Task 2: Remove quota notification scheduling and dead helper code

**Files:**
- Modify: `service/billing.go:78-85`
- Modify: `service/quota.go:150,411-575`
- Modify: `service/violation_fee.go:126`
- Modify: `relay/mjproxy_handler.go:237,544`
- Delete: `service/quota_notify_test.go`

- [ ] **Step 1: Remove settlement notification scheduling**

Delete the following block from `service/billing.go`:

```go
// 发送额度通知（订阅计费使用订阅剩余额度）
if actualQuota != 0 {
	if relayInfo.BillingSource == BillingSourceSubscription {
		checkAndSendSubscriptionQuotaNotify(relayInfo)
	} else {
		checkAndSendQuotaNotify(relayInfo, actualQuota-preConsumed, preConsumed)
	}
}
```

The settlement function should return immediately after `relayInfo.Billing.Settle(actualQuota)` succeeds.

- [ ] **Step 2: Simplify `PostConsumeQuota` and remove notification helpers**

Change the signature in `service/quota.go` to:

```go
func PostConsumeQuota(relayInfo *relaycommon.RelayInfo, quota int, preConsumedQuota int) (err error) {
```

Delete this block:

```go
if sendEmail {
	if (quota + preConsumedQuota) != 0 {
		checkAndSendQuotaNotify(relayInfo, quota, preConsumedQuota)
	}
}
```

Delete `quotaNotifyFunc`, `sendWalletQuotaNotify`, `checkAndSendQuotaNotify`, and `checkAndSendSubscriptionQuotaNotify` in full. Keep `dto` and `gopool` imports because earlier quota and asynchronous code still use them.

- [ ] **Step 3: Update every `PostConsumeQuota` caller**

Replace the five calls exactly as follows:

```go
PostConsumeQuota(relayInfo, quota, 0)
PostConsumeQuota(relayInfo, quotaDelta, relayInfo.FinalPreConsumedQuota)
PostConsumeQuota(relayInfo, feeQuota, 0)
service.PostConsumeQuota(info, priceData.Quota, 0)
service.PostConsumeQuota(relayInfo, priceData.Quota, 0)
```

The affected files are `service/quota.go`, `service/billing.go`, `service/violation_fee.go`, and the two call sites in `relay/mjproxy_handler.go`.

- [ ] **Step 4: Delete tests that assert the removed low-balance behavior**

Delete `service/quota_notify_test.go`. Its four tests require wallet notifications to be sent and therefore contradict the approved behavior. Coverage for the new contract lives at the `NotifyUser` boundary in Task 1.

- [ ] **Step 5: Verify no quota scheduling symbols remain and compile all callers**

Run:

```powershell
gofmt -w service/billing.go service/quota.go service/violation_fee.go relay/mjproxy_handler.go
rg -n "sendWalletQuotaNotify|checkAndSendQuotaNotify|checkAndSendSubscriptionQuotaNotify|quotaNotifyFunc|sendEmail bool" service relay
go test ./service ./relay/... -run '^$' -count=1
go test ./service -count=1
```

Expected: `rg` returns no matches; the compile-only relay run and service tests pass.

- [ ] **Step 6: Commit settlement cleanup**

```powershell
git add -- service/billing.go service/quota.go service/violation_fee.go relay/mjproxy_handler.go service/quota_notify_test.go
git commit -m "refactor(notify): remove quota notification scheduling"
```

### Task 3: Remove the obsolete low-balance email state

**Files:**
- Modify: `model/user_update_test.go`
- Modify: `model/user.go:102-134,1083-1091`
- Modify: `model/affiliate_transfer_request.go:377-429`
- Modify: `model/checkin.go:117-123`
- Modify: `model/redemption.go:181-186`
- Modify: `model/topup.go:154-159,385-391,459-466,520-528,581-590`
- Modify: `model/utils.go:108-115`
- Delete: `model/user_setting_state.go`
- Delete: `model/user_setting_state_test.go`

- [ ] **Step 1: Add a failing compatibility test for the deprecated JSON key**

Append this test to `model/user_update_test.go`:

```go
func TestUpdateUserSettingDropsDeprecatedQuotaWarningEmailState(t *testing.T) {
	setupUserUpdateTestState(t)

	user := User{
		Id:       3,
		Username: "deprecated-setting-user",
		Password: "password",
		Status:   common.UserStatusEnabled,
		Setting:  `{"notify_type":"email","quota_warning_email_sent":true}`,
	}
	require.NoError(t, DB.Create(&user).Error)

	require.NoError(t, UpdateUserSetting(user.Id, dto.UserSetting{
		NotifyType: dto.NotifyTypeWebhook,
		Language:   "zh",
	}))

	var got User
	require.NoError(t, DB.Select("setting").First(&got, user.Id).Error)
	var rawSetting map[string]interface{}
	require.NoError(t, common.Unmarshal([]byte(got.Setting), &rawSetting))
	assert.NotContains(t, rawSetting, "quota_warning_email_sent")
	assert.Equal(t, dto.NotifyTypeWebhook, got.GetSetting().NotifyType)
	assert.Equal(t, "zh", got.GetSetting().Language)
}
```

- [ ] **Step 2: Run the test and verify it fails**

Run:

```powershell
go test ./model -run TestUpdateUserSettingDropsDeprecatedQuotaWarningEmailState -count=1
```

Expected: the test fails because current `UpdateUserSetting` deliberately preserves `quota_warning_email_sent`.

- [ ] **Step 3: Restore direct user-setting persistence**

Replace `UpdateUserSetting` with direct DTO serialization and a single-column GORM update:

```go
func UpdateUserSetting(userId int, setting dto.UserSetting) error {
	if userId == 0 {
		return errors.New("id 为空！")
	}
	settingBytes, err := common.Marshal(setting)
	if err != nil {
		return err
	}
	settingValue := string(settingBytes)
	if err = DB.Model(&User{}).Where("id = ?", userId).Update("setting", settingValue).Error; err != nil {
		return err
	}
	return updateUserSettingCache(userId, settingValue)
}
```

This keeps the update cross-database through GORM and removes the obsolete transaction/row lock.

- [ ] **Step 4: Remove state files and every recovery hook**

Delete `model/user_setting_state.go` and `model/user_setting_state_test.go`.

Remove each `rearmQuotaWarningEmailAfterCredit(...)` call from:

```text
model/affiliate_transfer_request.go
model/checkin.go
model/redemption.go
model/topup.go
model/user.go
model/utils.go
```

In `ApproveAffiliateTransferRequest`, restore `return DB.Transaction(...)` and remove the temporary outer `userId`/`err` variables that existed only to rearm email state. In `IncreaseUserQuota`, restore `return increaseUserQuota(id, quota)` for the non-batch path. Do not alter any quota arithmetic or transaction body.

- [ ] **Step 5: Verify the deprecated state is gone and model behavior passes**

Run:

```powershell
gofmt -w model/user.go model/user_update_test.go model/affiliate_transfer_request.go model/checkin.go model/redemption.go model/topup.go model/utils.go
rg -n "quota_warning_email_sent|QuotaWarningEmail|rearmQuotaWarningEmail" model service
go test ./model -run 'TestUpdateUserSetting(DropsDeprecatedQuotaWarningEmailState|OnlyUpdatesSetting)' -count=1
go test ./model -count=1
```

Expected: `rg` returns no matches; the compatibility test and full model package pass.

- [ ] **Step 6: Commit state cleanup**

```powershell
git add -- model/user.go model/user_update_test.go model/affiliate_transfer_request.go model/checkin.go model/redemption.go model/topup.go model/utils.go model/user_setting_state.go model/user_setting_state_test.go
git commit -m "refactor(notify): remove quota email state"
```

### Task 4: Hide creation time and expand the API key group selector

**Files:**
- Modify: `web/default/src/features/keys/components/api-keys-table.tsx:283-293`
- Modify: `web/default/src/features/keys/components/api-keys-columns.tsx:221-229`
- Modify: `web/default/src/features/keys/components/api-key-group-combobox.tsx:201-225`

- [ ] **Step 1: Reproduce the current desktop layout before editing**

Start the existing local backend and `web/default` development server, open the authenticated API key management page, and record these DOM facts with Playwright/browser inspection:

```text
Created column header count = 1
Group column computed width is approximately 220px
Long group option text has scrollWidth > clientWidth when the menu is open
```

These observations are the failing baseline for the requested layout.

- [ ] **Step 2: Hide creation time by default**

Update `initialColumnVisibility` in `api-keys-table.tsx`:

```tsx
initialColumnVisibility: {
  model_limits: false,
  allow_ips: false,
  created_time: false,
  expired_time: false,
},
```

Keep `columnVisibilityStorageKey` unchanged so explicit user choices remain persistent and the column remains available in the visibility menu.

- [ ] **Step 3: Reallocate the creation-time width to the group column**

In the `group` column definition in `api-keys-columns.tsx`, change:

```tsx
size: 400,
```

Do not change the sizes of other columns or the mobile metadata.

- [ ] **Step 4: Allow complete names in the expanded menu**

In `api-key-group-combobox.tsx`, replace the option label and description classes with wrapping classes:

```tsx
<span className='block whitespace-normal break-words font-medium'>
  {option.label}
</span>
{option.desc && (
  <span className='text-muted-foreground block whitespace-normal break-words text-xs'>
    {option.desc}
  </span>
)}
```

Keep the compact trigger itself single-line and keep the existing `min-w-64` popup fallback. The 400px anchor width will become the normal desktop popup width.

- [ ] **Step 5: Run frontend static verification**

From `web/default` run:

```powershell
npm exec --yes bun -- x oxlint -c .oxlintrc.json src/features/keys/components/api-keys-table.tsx src/features/keys/components/api-keys-columns.tsx src/features/keys/components/api-key-group-combobox.tsx
npm exec --yes bun -- x oxfmt --check src/features/keys/components/api-keys-table.tsx src/features/keys/components/api-keys-columns.tsx src/features/keys/components/api-key-group-combobox.tsx
npm exec --yes bun -- run typecheck
npm exec --yes bun -- test
npm exec --yes bun -- run build
```

Expected: Lint and format checks return exit code 0, all frontend tests pass, type checking passes, and the production build succeeds.

- [ ] **Step 6: Verify the requested UI behavior in a real browser**

Using the same authenticated page and a long group fixture, verify:

```text
Created column header count = 0 with fresh/default column visibility
Group column computed width is approximately 400px
Open popup width follows the group button width
Each option has scrollWidth <= clientWidth, or wraps to additional lines without clipping
No text overlaps the ratio badge or chevron
390x844 mobile API key cards retain their prior layout and have no horizontal overflow
```

Also use the column visibility menu to re-enable “Created” and confirm the column still renders.

- [ ] **Step 7: Commit the API key layout change**

```powershell
git add -- web/default/src/features/keys/components/api-keys-table.tsx web/default/src/features/keys/components/api-keys-columns.tsx web/default/src/features/keys/components/api-key-group-combobox.tsx
git commit -m "fix(keys): expand API key group selector"
```

### Task 5: Final cross-module verification

**Files:**
- Verify only; do not modify unrelated files.

- [ ] **Step 1: Run backend tests**

```powershell
go test ./model ./service -count=1
go test ./relay/... -run '^$' -count=1
```

Expected: all selected packages pass or compile successfully with no failures.

- [ ] **Step 2: Run frontend verification again from a clean source state**

From `web/default`:

```powershell
npm exec --yes bun -- test
npm exec --yes bun -- run typecheck
npm exec --yes bun -- run build
```

Expected: all tests pass, type checking reports no errors, and Rsbuild exits successfully.

- [ ] **Step 3: Check repository scope and whitespace**

```powershell
git diff --check 4c548bbf..HEAD
git status --short --branch
git log --oneline -5
```

Expected: no whitespace errors; only the user's pre-existing `.gitignore` modification remains unstaged; the implementation commits are visible above the design and plan commits.

- [ ] **Step 4: Report the local result without pushing**

Summarize the disabled notification behavior, API key layout change, exact test counts/results, browser verification, and local commit IDs. Do not push until the user explicitly requests it.
