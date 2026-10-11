package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/config"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupAffiliateRebateSettingsFixture(t *testing.T, settings operation_setting.AffiliateSetting) {
	t.Helper()
	setupAffiliateTransferRequestFixture(t)
	require.NoError(t, DB.AutoMigrate(&AffiliateRebateRateChange{}, &Option{}))
	clearAffiliateRebateRateChanges(t)
	t.Cleanup(func() { clearAffiliateRebateRateChanges(t) })

	common.OptionMapRWMutex.Lock()
	if common.OptionMap == nil {
		common.OptionMap = map[string]string{}
	}
	common.OptionMapRWMutex.Unlock()

	previous := operation_setting.GetAffiliateSetting()
	previousInviter, previousInvitee := common.QuotaForInviter, common.QuotaForInvitee
	applyAffiliateSettings(t, settings)
	t.Cleanup(func() {
		applyAffiliateSettings(t, previous)
		common.QuotaForInviter, common.QuotaForInvitee = previousInviter, previousInvitee
	})
}

func clearAffiliateRebateRateChanges(t *testing.T) {
	t.Helper()
	require.NoError(t, DB.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&AffiliateRebateRateChange{}).Error)
}

func applyAffiliateSettings(t *testing.T, settings operation_setting.AffiliateSetting) {
	t.Helper()
	values, err := config.ConfigToMap(&settings)
	require.NoError(t, err)
	require.NoError(t, config.UpdateConfigFromMap(config.GlobalConfig.Get("affiliate_setting"), values))
}

func defaultAffiliateTestSettings() operation_setting.AffiliateSetting {
	return operation_setting.AffiliateSetting{
		RebateEnabled:     true,
		RebateRatePercent: 5,
		MinTransferQuota:  1,
		DailyRequestLimit: 1,
		RequireApproval:   true,
		ForfeitOnReject:   true,
	}
}

type affiliateRebateTestAccounts struct {
	owner   User
	invitee User
}

func createAffiliateRebateTestAccounts(t *testing.T, prefix string, affQuota int) affiliateRebateTestAccounts {
	t.Helper()
	owner := User{Username: prefix + "-owner", Password: "password", AffCode: prefix + "-owner-code", AffQuota: affQuota}
	require.NoError(t, DB.Create(&owner).Error)
	invitee := User{Username: prefix + "-invitee", Password: "password", AffCode: prefix + "-invitee-code", InviterId: owner.Id}
	require.NoError(t, DB.Create(&invitee).Error)
	return affiliateRebateTestAccounts{owner: owner, invitee: invitee}
}

// createCreemTopUp records a successful top-up whose credited quota equals amount.
func createCreemTopUp(t *testing.T, userId int, tradeNo string, amount int64, completeTime int64) {
	t.Helper()
	require.NoError(t, DB.Create(&TopUp{
		UserId:          userId,
		Amount:          amount,
		TradeNo:         tradeNo,
		PaymentMethod:   PaymentMethodCreem,
		PaymentProvider: PaymentProviderCreem,
		CreateTime:      completeTime,
		CompleteTime:    completeTime,
		Status:          common.TopUpStatusSuccess,
	}).Error)
}

func createAffiliateRateChange(t *testing.T, effectiveAt int64, enabled bool, basisPoints int) {
	t.Helper()
	require.NoError(t, DB.Create(&AffiliateRebateRateChange{EffectiveAt: effectiveAt, Enabled: enabled, RateBasisPoints: basisPoints}).Error)
}

func TestAffiliateRebateUsesRateInEffectWhenTopUpCompleted(t *testing.T) {
	setupAffiliateRebateSettingsFixture(t, defaultAffiliateTestSettings())
	accounts := createAffiliateRebateTestAccounts(t, "rate-timeline", 0)

	createCreemTopUp(t, accounts.invitee.Id, "rate-timeline-legacy", 10000, 1000)
	createAffiliateRateChange(t, 2000, true, 1000)
	createCreemTopUp(t, accounts.invitee.Id, "rate-timeline-ten", 20000, 3000)
	createAffiliateRateChange(t, 4000, false, 1000)
	createCreemTopUp(t, accounts.invitee.Id, "rate-timeline-off", 50000, 5000)
	createAffiliateRateChange(t, 6000, true, 250)
	createCreemTopUp(t, accounts.invitee.Id, "rate-timeline-quarter", 40000, 7000)

	summary, err := GetAffiliateRebateSummary(accounts.owner.Id)
	require.NoError(t, err)
	// 10000*5% + 20000*10% + 50000*0 (rebate off) + 40000*2.5%
	assert.Equal(t, 500+2000+0+1000, summary.RechargeRebateQuota)
	assert.Equal(t, 120000, summary.TotalInvitedRechargeQuota)
	assert.Equal(t, summary.RechargeRebateQuota, summary.TotalPendingQuota)
}

func TestAffiliateRebateWithoutRateHistoryKeepsLegacyAggregateRounding(t *testing.T) {
	setupAffiliateRebateSettingsFixture(t, defaultAffiliateTestSettings())
	accounts := createAffiliateRebateTestAccounts(t, "legacy-rounding", 0)

	createCreemTopUp(t, accounts.invitee.Id, "legacy-rounding-a", 30, 1000)
	createCreemTopUp(t, accounts.invitee.Id, "legacy-rounding-b", 30, 1001)

	summary, err := GetAffiliateRebateSummary(accounts.owner.Id)
	require.NoError(t, err)
	// floor((30+30)*5%) = 3, matching the pre-settings calculation.
	assert.Equal(t, 3, summary.RechargeRebateQuota)
}

func TestDisabledAffiliateRebateFreezesEarnedRechargeRebate(t *testing.T) {
	settings := defaultAffiliateTestSettings()
	settings.RebateEnabled = false
	setupAffiliateRebateSettingsFixture(t, settings)
	accounts := createAffiliateRebateTestAccounts(t, "frozen-rebate", 300)
	createCreemTopUp(t, accounts.invitee.Id, "frozen-rebate-topup", 20000, 1000)

	summary, err := GetAffiliateRebateSummary(accounts.owner.Id)
	require.NoError(t, err)
	assert.False(t, summary.RebateEnabled)
	assert.Zero(t, summary.RechargeRebateQuota)
	assert.Zero(t, summary.TotalInvitedRechargeQuota)
	assert.Equal(t, 300, summary.TotalPendingQuota)

	request, err := CreateAffiliateTransferRequest(accounts.owner.Id)
	require.NoError(t, err)
	assert.Equal(t, 300, request.InviteRewardQuota)
	assert.Zero(t, request.RechargeRebateQuota)
	require.NoError(t, ApproveAffiliateTransferRequest(request.Id, 1))

	settings.RebateEnabled = true
	applyAffiliateSettings(t, settings)
	summary, err = GetAffiliateRebateSummary(accounts.owner.Id)
	require.NoError(t, err)
	assert.Equal(t, 1000, summary.RechargeRebateQuota, "frozen rebate becomes transferable again once re-enabled")
}

func TestCreateAffiliateTransferRequestWithoutApprovalCreditsImmediately(t *testing.T) {
	settings := defaultAffiliateTestSettings()
	settings.RequireApproval = false
	setupAffiliateRebateSettingsFixture(t, settings)
	accounts := createAffiliateRebateTestAccounts(t, "auto-approve", 400)
	createCreemTopUp(t, accounts.invitee.Id, "auto-approve-topup", 20000, 1000)

	request, err := CreateAffiliateTransferRequest(accounts.owner.Id)
	require.NoError(t, err)
	assert.Equal(t, AffiliateTransferStatusApproved, request.Status)
	assert.Equal(t, 1400, request.TotalQuota)

	var owner User
	require.NoError(t, DB.First(&owner, accounts.owner.Id).Error)
	assert.Equal(t, 1400, owner.Quota)
	assert.Zero(t, owner.AffQuota)

	summary, err := GetAffiliateRebateSummary(accounts.owner.Id)
	require.NoError(t, err)
	assert.Nil(t, summary.PendingRequest)
	assert.Zero(t, summary.TotalPendingQuota)
}

func TestCreateAffiliateTransferRequestHonorsConfiguredDailyLimit(t *testing.T) {
	settings := defaultAffiliateTestSettings()
	settings.RequireApproval = false
	settings.DailyRequestLimit = 2
	setupAffiliateRebateSettingsFixture(t, settings)
	accounts := createAffiliateRebateTestAccounts(t, "daily-limit", 100)

	_, err := CreateAffiliateTransferRequest(accounts.owner.Id)
	require.NoError(t, err)
	require.NoError(t, DB.Model(&User{}).Where("id = ?", accounts.owner.Id).Update("aff_quota", 100).Error)
	_, err = CreateAffiliateTransferRequest(accounts.owner.Id)
	require.NoError(t, err)
	require.NoError(t, DB.Model(&User{}).Where("id = ?", accounts.owner.Id).Update("aff_quota", 100).Error)

	summary, err := GetAffiliateRebateSummary(accounts.owner.Id)
	require.NoError(t, err)
	assert.Equal(t, 2, summary.TodayRequestCount)
	assert.True(t, summary.SubmittedToday)
	_, err = CreateAffiliateTransferRequest(accounts.owner.Id)
	require.ErrorIs(t, err, ErrAffiliateTransferDailyLimit)

	settings.DailyRequestLimit = 0
	applyAffiliateSettings(t, settings)
	_, err = CreateAffiliateTransferRequest(accounts.owner.Id)
	require.NoError(t, err, "a zero daily limit means unlimited requests")
}

func TestCreateAffiliateTransferRequestHonorsConfiguredMinimum(t *testing.T) {
	settings := defaultAffiliateTestSettings()
	settings.MinTransferQuota = 500
	setupAffiliateRebateSettingsFixture(t, settings)
	accounts := createAffiliateRebateTestAccounts(t, "minimum", 499)

	_, err := CreateAffiliateTransferRequest(accounts.owner.Id)
	require.ErrorIs(t, err, ErrAffiliateTransferBelowMinimum)

	require.NoError(t, DB.Model(&User{}).Where("id = ?", accounts.owner.Id).Update("aff_quota", 500).Error)
	request, err := CreateAffiliateTransferRequest(accounts.owner.Id)
	require.NoError(t, err)
	assert.Equal(t, 500, request.TotalQuota)
}

func TestRejectAffiliateTransferRequestWithoutForfeitKeepsRewardsAvailable(t *testing.T) {
	settings := defaultAffiliateTestSettings()
	settings.ForfeitOnReject = false
	settings.DailyRequestLimit = 0
	setupAffiliateRebateSettingsFixture(t, settings)
	accounts := createAffiliateRebateTestAccounts(t, "no-forfeit", 200)
	createCreemTopUp(t, accounts.invitee.Id, "no-forfeit-topup", 6000, 1000)

	request, err := CreateAffiliateTransferRequest(accounts.owner.Id)
	require.NoError(t, err)
	require.Equal(t, 500, request.TotalQuota)
	require.NoError(t, RejectAffiliateTransferRequest(request.Id, 1, "retry later"))

	var owner User
	require.NoError(t, DB.First(&owner, accounts.owner.Id).Error)
	assert.Equal(t, 200, owner.AffQuota)
	summary, err := GetAffiliateRebateSummary(accounts.owner.Id)
	require.NoError(t, err)
	assert.Equal(t, 300, summary.RechargeRebateQuota)
	assert.Equal(t, 500, summary.TotalPendingQuota)
}

func TestUpdateAffiliateOptionsRecordsRateHistoryOnlyOnChange(t *testing.T) {
	setupAffiliateRebateSettingsFixture(t, defaultAffiliateTestSettings())
	update := AffiliateOptionsUpdate{Setting: defaultAffiliateTestSettings(), QuotaForInviter: 10, QuotaForInvitee: 20}

	require.NoError(t, UpdateAffiliateOptions(update, 7))
	var count int64
	require.NoError(t, DB.Model(&AffiliateRebateRateChange{}).Count(&count).Error)
	assert.Zero(t, count, "saving the legacy 5% rate does not start a new rate period")

	update.Setting.RebateRatePercent = 7.5
	require.NoError(t, UpdateAffiliateOptions(update, 7))
	require.NoError(t, UpdateAffiliateOptions(update, 7))
	var changes []AffiliateRebateRateChange
	require.NoError(t, DB.Find(&changes).Error)
	require.Len(t, changes, 1)
	assert.True(t, changes[0].Enabled)
	assert.Equal(t, 750, changes[0].RateBasisPoints)
	assert.Equal(t, 7, changes[0].ChangedBy)
	assert.Equal(t, 7.5, operation_setting.GetAffiliateSetting().RebateRatePercent)
	assert.Equal(t, 10, common.QuotaForInviter)
	assert.Equal(t, 20, common.QuotaForInvitee)

	update.Setting.RebateEnabled = false
	require.NoError(t, UpdateAffiliateOptions(update, 7))
	require.NoError(t, DB.Order("id asc").Find(&changes).Error)
	require.Len(t, changes, 2)
	assert.False(t, changes[1].Enabled)
}

func TestUpdateAffiliateOptionsRejectsInvalidValues(t *testing.T) {
	setupAffiliateRebateSettingsFixture(t, defaultAffiliateTestSettings())
	tests := []struct {
		name   string
		mutate func(update *AffiliateOptionsUpdate)
	}{
		{"rate with three decimals", func(u *AffiliateOptionsUpdate) { u.Setting.RebateRatePercent = 5.125 }},
		{"rate above 100 percent", func(u *AffiliateOptionsUpdate) { u.Setting.RebateRatePercent = 100.01 }},
		{"negative rate", func(u *AffiliateOptionsUpdate) { u.Setting.RebateRatePercent = -1 }},
		{"negative minimum", func(u *AffiliateOptionsUpdate) { u.Setting.MinTransferQuota = -1 }},
		{"daily limit above maximum", func(u *AffiliateOptionsUpdate) { u.Setting.DailyRequestLimit = 101 }},
		{"negative inviter reward", func(u *AffiliateOptionsUpdate) { u.QuotaForInviter = -1 }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			update := AffiliateOptionsUpdate{Setting: defaultAffiliateTestSettings()}
			tt.mutate(&update)
			require.Error(t, UpdateAffiliateOptions(update, 1))
		})
	}
	var count int64
	require.NoError(t, DB.Model(&AffiliateRebateRateChange{}).Count(&count).Error)
	assert.Zero(t, count)
}
