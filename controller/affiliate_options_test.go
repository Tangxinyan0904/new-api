package controller

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/config"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type affiliateOptionsResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

func setupAffiliateOptionsControllerTest(t *testing.T) *gorm.DB {
	t.Helper()
	db := setupGeoIPOptionControllerTest(t)
	require.NoError(t, db.AutoMigrate(&model.AffiliateRebateRateChange{}, &model.AuditLog{}))

	previous := operation_setting.GetAffiliateSetting()
	previousInviter, previousInvitee := common.QuotaForInviter, common.QuotaForInvitee
	t.Cleanup(func() {
		values, err := config.ConfigToMap(&previous)
		require.NoError(t, err)
		require.NoError(t, config.UpdateConfigFromMap(config.GlobalConfig.Get("affiliate_setting"), values))
		common.QuotaForInviter, common.QuotaForInvitee = previousInviter, previousInvitee
	})
	return db
}

func putAffiliateOptions(t *testing.T, body string) affiliateOptionsResponse {
	t.Helper()
	response := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(response)
	context.Request = httptest.NewRequest(http.MethodPut, "/api/option/affiliate", strings.NewReader(body))
	context.Set("id", 1)
	context.Set("username", "root")
	context.Set("role", common.RoleRootUser)

	UpdateAffiliateOptions(context)

	require.Equal(t, http.StatusOK, response.Code)
	var payload affiliateOptionsResponse
	require.NoError(t, common.Unmarshal(response.Body.Bytes(), &payload))
	return payload
}

func TestUpdateOptionRejectsAffiliateSettingKeys(t *testing.T) {
	response := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(response)
	context.Request = httptest.NewRequest(
		http.MethodPut,
		"/api/option/",
		strings.NewReader(`{"key":"affiliate_setting.rebate_rate_percent","value":"50"}`),
	)

	UpdateOption(context)

	var payload affiliateOptionsResponse
	require.NoError(t, common.Unmarshal(response.Body.Bytes(), &payload))
	assert.False(t, payload.Success)
	assert.NotEqual(t, 50.0, operation_setting.GetAffiliateSetting().RebateRatePercent)
}

func TestUpdateAffiliateOptionsSavesSettingsAndRateHistory(t *testing.T) {
	db := setupAffiliateOptionsControllerTest(t)

	payload := putAffiliateOptions(t, `{
		"rebate_enabled": true,
		"rebate_rate_percent": 8.5,
		"min_transfer_quota": 250000,
		"daily_request_limit": 3,
		"require_approval": false,
		"forfeit_on_reject": false,
		"quota_for_inviter": 0,
		"quota_for_invitee": 0
	}`)
	require.True(t, payload.Success, payload.Message)

	saved := operation_setting.GetAffiliateSetting()
	assert.Equal(t, operation_setting.AffiliateSetting{
		RebateEnabled:     true,
		RebateRatePercent: 8.5,
		MinTransferQuota:  250000,
		DailyRequestLimit: 3,
		RequireApproval:   false,
		ForfeitOnReject:   false,
	}, saved)

	var changes []model.AffiliateRebateRateChange
	require.NoError(t, db.Find(&changes).Error)
	require.Len(t, changes, 1)
	assert.Equal(t, 850, changes[0].RateBasisPoints)

	var option model.Option
	require.NoError(t, db.First(&option, "key = ?", "affiliate_setting.rebate_rate_percent").Error)
	assert.Equal(t, "8.5", option.Value)

	var auditCount int64
	require.NoError(t, db.Table("audit_logs").Where("action = ?", "affiliate.settings.update").Count(&auditCount).Error)
	assert.EqualValues(t, 1, auditCount)
}

func TestUpdateAffiliateOptionsRejectsIncompleteOrInvalidForms(t *testing.T) {
	setupAffiliateOptionsControllerTest(t)
	before := operation_setting.GetAffiliateSetting()

	incomplete := putAffiliateOptions(t, `{"rebate_enabled": true, "rebate_rate_percent": 10}`)
	assert.False(t, incomplete.Success)

	invalid := putAffiliateOptions(t, `{
		"rebate_enabled": true,
		"rebate_rate_percent": 12.345,
		"min_transfer_quota": 0,
		"daily_request_limit": 1,
		"require_approval": true,
		"forfeit_on_reject": true,
		"quota_for_inviter": 0,
		"quota_for_invitee": 0
	}`)
	assert.False(t, invalid.Success)
	assert.Equal(t, before, operation_setting.GetAffiliateSetting())
}

func TestUpdateAffiliateOptionsRequiresComplianceForInvitationRewards(t *testing.T) {
	setupAffiliateOptionsControllerTest(t)
	require.False(t, operation_setting.IsPaymentComplianceConfirmed())

	payload := putAffiliateOptions(t, `{
		"rebate_enabled": true,
		"rebate_rate_percent": 5,
		"min_transfer_quota": 0,
		"daily_request_limit": 1,
		"require_approval": true,
		"forfeit_on_reject": true,
		"quota_for_inviter": 1000,
		"quota_for_invitee": 0
	}`)
	assert.False(t, payload.Success)
	assert.NotEqual(t, 1000, common.QuotaForInviter)
}
