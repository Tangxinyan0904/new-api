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
	require.NoError(t, db.AutoMigrate(&model.AffiliateTransferRequest{}, &model.AffiliateRebateRateChange{}))

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

	logs := affiliateTransferAuditLogs(t, db, 0)
	require.Len(t, logs, 2)

	adminLog := logs[0]
	assert.Equal(t, 1, adminLog.UserId)
	assert.Contains(t, adminLog.Content, "Approved rebate transfer request #7 for user 302")

	require.NotNil(t, adminLog.Other.Op)
	assert.Equal(t, "affiliate.transfer.approve", adminLog.Other.Op.Action)
	adminParams := adminLog.Other.Op.Params
	assert.Equal(t, "7", auditParamJSON(t, adminParams["request_id"]))
	assert.Equal(t, "302", auditParamJSON(t, adminParams["target_user_id"]))
	assert.Equal(t, "200", auditParamJSON(t, adminParams["invite_reward_quota"]))
	assert.Equal(t, "300", auditParamJSON(t, adminParams["recharge_rebate_quota"]))
	assert.Equal(t, "500", auditParamJSON(t, adminParams["total_quota"]))

	userLog := logs[1]
	assert.Equal(t, 302, userLog.UserId)
	assert.Equal(t, "rebate-user", userLog.Username)
	assert.Contains(t, userLog.Content, "Administrator approved")

	require.NotNil(t, userLog.Other.Op)
	assert.Equal(t, "affiliate.transfer.approved_for_user", userLog.Other.Op.Action)
	userParams := userLog.Other.Op.Params
	assert.Equal(t, "7", auditParamJSON(t, userParams["request_id"]))
	assert.Equal(t, "500", auditParamJSON(t, userParams["quota"]))
	require.NotNil(t, userLog.Other.AdminInfo)
	assert.Equal(t, 1, userLog.Other.AdminInfo.AdminID)
	assert.Equal(t, "root-admin", userLog.Other.AdminInfo.AdminUsername)
}

func TestApproveAffiliateTransferRequestDoesNotRecordUserLogWhenApprovalFails(t *testing.T) {
	db := setupAffiliateTransferApprovalControllerFixture(t)
	recorder, ctx := newAffiliateTransferApprovalContext(7)

	ApproveAffiliateTransferRequest(ctx)
	require.Equal(t, http.StatusOK, recorder.Code)

	_, secondCtx := newAffiliateTransferApprovalContext(7)
	ApproveAffiliateTransferRequest(secondCtx)

	logs := affiliateTransferAuditLogs(t, db, 302)
	require.Len(t, logs, 1)
	require.NotNil(t, logs[0].Other.Op)
	assert.Equal(t, "affiliate.transfer.approved_for_user", logs[0].Other.Op.Action)
}

func TestApproveAllAffiliateTransferRequestsRecordsEveryApproval(t *testing.T) {
	db := setupAffiliateTransferApproveAllControllerFixture(t, false)
	recorder, ctx := newAffiliateTransferApproveAllContext()

	ApproveAllAffiliateTransferRequests(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
	var response struct {
		Success bool `json:"success"`
		Data    struct {
			ApprovedCount int `json:"approved_count"`
		} `json:"data"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	assert.True(t, response.Success)
	assert.Equal(t, 2, response.Data.ApprovedCount)

	logs := affiliateTransferAuditLogs(t, db, 0)
	require.Len(t, logs, 4)

	adminRequestIds := make([]int, 0, 2)
	userRequestIds := make([]int, 0, 2)
	for _, log := range logs {
		require.NotNil(t, log.Other.Op)
		requestId, err := strconv.Atoi(auditParamJSON(t, log.Other.Op.Params["request_id"]))
		require.NoError(t, err)
		switch log.Other.Op.Action {
		case "affiliate.transfer.approve":
			assert.Equal(t, 1, log.UserId)
			adminRequestIds = append(adminRequestIds, requestId)
		case "affiliate.transfer.approved_for_user":
			assert.Contains(t, []int{302, 303}, log.UserId)
			userRequestIds = append(userRequestIds, requestId)
		default:
			t.Fatalf("unexpected affiliate transfer log action: %v", log.Other.Op.Action)
		}
	}
	assert.ElementsMatch(t, []int{7, 8}, adminRequestIds)
	assert.ElementsMatch(t, []int{7, 8}, userRequestIds)
}

func TestApproveAllAffiliateTransferRequestsRollsBackWithoutLogs(t *testing.T) {
	db := setupAffiliateTransferApproveAllControllerFixture(t, true)
	recorder, ctx := newAffiliateTransferApproveAllContext()

	ApproveAllAffiliateTransferRequests(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
	var response struct {
		Success bool `json:"success"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	assert.False(t, response.Success)

	var requests []model.AffiliateTransferRequest
	require.NoError(t, db.Order("id ASC").Find(&requests).Error)
	require.Len(t, requests, 2)
	assert.Equal(t, model.AffiliateTransferStatusPending, requests[0].Status)
	assert.Equal(t, model.AffiliateTransferStatusPending, requests[1].Status)

	var firstUser model.User
	require.NoError(t, db.First(&firstUser, 302).Error)
	assert.Equal(t, 10, firstUser.Quota)
	assert.Equal(t, 200, firstUser.AffQuota)

	assert.Empty(t, affiliateTransferAuditLogs(t, db, 0))
}

// auditParamJSON renders one stored audit parameter as JSON text.
func auditParamJSON(t *testing.T, value any) string {
	t.Helper()
	raw, err := common.Marshal(value)
	require.NoError(t, err)
	return string(raw)
}

// affiliateTransferAuditLogs returns affiliate transfer audit rows in insertion
// order, optionally limited to one log owner.
func affiliateTransferAuditLogs(t *testing.T, db *gorm.DB, userID int) []model.AuditLog {
	t.Helper()
	query := db.Table("audit_logs").Where("action LIKE ?", "affiliate.transfer.%")
	if userID > 0 {
		query = query.Where("user_id = ?", userID)
	}
	var logs []model.AuditLog
	require.NoError(t, query.Order("id ASC").Find(&logs).Error)
	return logs
}

func setupAffiliateTransferApprovalControllerFixture(t *testing.T) *gorm.DB {
	t.Helper()
	db := setupModelListControllerTestDB(t)
	require.NoError(t, db.AutoMigrate(&model.AffiliateTransferRequest{}, &model.AffiliateRebateRateChange{}, &model.Log{}, &model.AuditLog{}))

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

func setupAffiliateTransferApproveAllControllerFixture(t *testing.T, invalidSecond bool) *gorm.DB {
	t.Helper()
	db := setupModelListControllerTestDB(t)
	require.NoError(t, db.AutoMigrate(&model.AffiliateTransferRequest{}, &model.AffiliateRebateRateChange{}, &model.Log{}, &model.AuditLog{}))

	users := []model.User{
		{
			Id:       1,
			Username: "root-admin",
			Password: "password",
			Role:     common.RoleRootUser,
			Status:   common.UserStatusEnabled,
			AffCode:  "root",
		},
		{
			Id:       302,
			Username: "rebate-user-a",
			Password: "password",
			Status:   common.UserStatusEnabled,
			AffCode:  "u302",
			Quota:    10,
			AffQuota: 200,
		},
		{
			Id:       303,
			Username: "rebate-user-b",
			Password: "password",
			Status:   common.UserStatusEnabled,
			AffCode:  "u303",
			Quota:    20,
			AffQuota: 400,
		},
	}
	if invalidSecond {
		users[2].AffQuota = 399
	}
	require.NoError(t, db.Create(&users).Error)

	requests := []model.AffiliateTransferRequest{
		{
			Id:                  7,
			UserId:              302,
			InviteRewardQuota:   200,
			RechargeRebateQuota: 300,
			TotalQuota:          500,
			Status:              model.AffiliateTransferStatusPending,
			CreatedAt:           1000,
		},
		{
			Id:                  8,
			UserId:              303,
			InviteRewardQuota:   400,
			RechargeRebateQuota: 200,
			TotalQuota:          600,
			Status:              model.AffiliateTransferStatusPending,
			CreatedAt:           1001,
		},
	}
	require.NoError(t, db.Create(&requests).Error)
	return db
}

func newAffiliateTransferApproveAllContext() (*httptest.ResponseRecorder, *gin.Context) {
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/user/affiliate/transfer-requests/approve-all", nil)
	ctx.Set("id", 1)
	ctx.Set("username", "root-admin")
	ctx.Set("role", common.RoleRootUser)
	return recorder, ctx
}
