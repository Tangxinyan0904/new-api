package controller

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

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
