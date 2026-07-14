package controller

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestListSelfAffiliateTransferRequestsReturnsOnlyCurrentUserHistory(t *testing.T) {
	db := setupModelListControllerTestDB(t)
	require.NoError(t, db.AutoMigrate(&model.AffiliateTransferRequest{}))

	requests := []model.AffiliateTransferRequest{
		{
			Id:                       51,
			UserId:                   301,
			InviteRewardQuota:        100,
			RechargeRebateQuota:      200,
			TotalQuota:               300,
			Status:                   model.AffiliateTransferStatusApproved,
			CreatedAt:                1000,
			ReviewedAt:               1100,
			ReviewedBy:               1,
			RejectedQuotaForfeitedAt: 0,
		},
		{
			Id:                       52,
			UserId:                   999,
			InviteRewardQuota:        900,
			RechargeRebateQuota:      900,
			TotalQuota:               1800,
			Status:                   model.AffiliateTransferStatusRejected,
			CreatedAt:                2000,
			ReviewedAt:               2100,
			ReviewedBy:               2,
			RejectReason:             "other user",
			RejectedQuotaForfeitedAt: 2200,
		},
		{
			Id:                       53,
			UserId:                   301,
			InviteRewardQuota:        400,
			RechargeRebateQuota:      500,
			TotalQuota:               900,
			Status:                   model.AffiliateTransferStatusRejected,
			CreatedAt:                3000,
			ReviewedAt:               3100,
			ReviewedBy:               3,
			RejectReason:             "invalid request",
			RejectedQuotaForfeitedAt: 3200,
		},
	}
	require.NoError(t, db.Create(&requests).Error)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/user/affiliate/transfer-requests/self?p=1&page_size=10&user_id=999", nil)
	ctx.Set("id", 301)

	ListSelfAffiliateTransferRequests(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
	var response struct {
		Success bool `json:"success"`
		Data    struct {
			Total int                                          `json:"total"`
			Items []*model.AffiliateTransferRequestHistoryItem `json:"items"`
		} `json:"data"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	assert.True(t, response.Success)
	assert.Equal(t, 2, response.Data.Total)
	require.Len(t, response.Data.Items, 2)
	assert.Equal(t, 53, response.Data.Items[0].Id)
	assert.Equal(t, 900, response.Data.Items[0].TotalQuota)
	assert.Equal(t, model.AffiliateTransferStatusRejected, response.Data.Items[0].Status)
	assert.Equal(t, 51, response.Data.Items[1].Id)
	assert.Equal(t, 300, response.Data.Items[1].TotalQuota)

	raw := recorder.Body.String()
	assert.NotContains(t, raw, "user_id")
	assert.NotContains(t, raw, "reviewed_by")
	assert.NotContains(t, raw, "rejected_quota_forfeited_at")
}

func TestApproveAffiliateTransferRequestRecordsDetailedAudit(t *testing.T) {
	db := setupModelListControllerTestDB(t)
	require.NoError(t, db.AutoMigrate(&model.AffiliateTransferRequest{}, &model.Log{}))

	require.NoError(t, db.Create(&model.User{
		Id:       1,
		Username: "root-admin",
		Password: "password",
		Role:     common.RoleRootUser,
		Status:   common.UserStatusEnabled,
		AffCode:  "root",
	}).Error)
	require.NoError(t, db.Create(&model.User{
		Id:       302,
		Username: "rebate-user",
		Password: "password",
		Status:   common.UserStatusEnabled,
		AffCode:  "u302",
		AffQuota: 200,
	}).Error)
	require.NoError(t, db.Create(&model.AffiliateTransferRequest{
		Id:                  7,
		UserId:              302,
		InviteRewardQuota:   200,
		RechargeRebateQuota: 300,
		TotalQuota:          500,
		Status:              model.AffiliateTransferStatusPending,
		CreatedAt:           1000,
	}).Error)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/user/affiliate/transfer-requests/7/approve", nil)
	ctx.Params = gin.Params{{Key: "id", Value: "7"}}
	ctx.Set("id", 1)
	ctx.Set("username", "root-admin")
	ctx.Set("role", common.RoleRootUser)

	ApproveAffiliateTransferRequest(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)

	var log model.Log
	require.NoError(t, db.Where("type = ?", model.LogTypeManage).First(&log).Error)
	require.Contains(t, log.Content, "Approved rebate transfer request #7 for user 302")

	other, err := common.StrToMap(log.Other)
	require.NoError(t, err)
	op, ok := other["op"].(map[string]interface{})
	require.True(t, ok)
	require.Equal(t, "affiliate.transfer.approve", op["action"])
	params, ok := op["params"].(map[string]interface{})
	require.True(t, ok)
	require.EqualValues(t, 7, params["request_id"])
	require.EqualValues(t, 302, params["target_user_id"])
	require.EqualValues(t, 200, params["invite_reward_quota"])
	require.EqualValues(t, 300, params["recharge_rebate_quota"])
	require.EqualValues(t, 500, params["total_quota"])
}
