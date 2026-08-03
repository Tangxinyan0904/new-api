package controller

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
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
	db := setupAffiliateTransferApprovalControllerFixture(t)
	recorder, ctx := newAffiliateTransferApprovalContext(7)

	ApproveAffiliateTransferRequest(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)

	var logs []model.Log
	require.NoError(t, db.Where("type = ?", model.LogTypeManage).Order("id ASC").Find(&logs).Error)
	require.Len(t, logs, 2)

	adminLog := logs[0]
	assert.Equal(t, 1, adminLog.UserId)
	assert.Contains(t, adminLog.Content, "Approved rebate transfer request #7 for user 302")

	adminOther, err := common.StrToMap(adminLog.Other)
	require.NoError(t, err)
	adminOp, ok := adminOther["op"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "affiliate.transfer.approve", adminOp["action"])
	adminParams, ok := adminOp["params"].(map[string]interface{})
	require.True(t, ok)
	assert.EqualValues(t, 7, adminParams["request_id"])
	assert.EqualValues(t, 302, adminParams["target_user_id"])
	assert.EqualValues(t, 200, adminParams["invite_reward_quota"])
	assert.EqualValues(t, 300, adminParams["recharge_rebate_quota"])
	assert.EqualValues(t, 500, adminParams["total_quota"])

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
}

func TestApproveAffiliateTransferRequestDoesNotRecordUserLogWhenApprovalFails(t *testing.T) {
	db := setupAffiliateTransferApprovalControllerFixture(t)
	recorder, ctx := newAffiliateTransferApprovalContext(7)

	ApproveAffiliateTransferRequest(ctx)
	require.Equal(t, http.StatusOK, recorder.Code)

	_, secondCtx := newAffiliateTransferApprovalContext(7)
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

func setupAffiliateTransferApprovalControllerFixture(t *testing.T) *gorm.DB {
	t.Helper()
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
	return db
}

func newAffiliateTransferApprovalContext(requestID int) (*httptest.ResponseRecorder, *gin.Context) {
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/user/affiliate/transfer-requests/"+strconv.Itoa(requestID)+"/approve", nil)
	ctx.Params = gin.Params{{Key: "id", Value: strconv.Itoa(requestID)}}
	ctx.Set("id", 1)
	ctx.Set("username", "root-admin")
	ctx.Set("role", common.RoleRootUser)
	return recorder, ctx
}
