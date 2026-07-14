package model

import (
	"sync"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func clearAffiliateTransferRequestFixture(t *testing.T) {
	t.Helper()
	require.NoError(t, DB.Session(&gorm.Session{AllowGlobalUpdate: true}).Unscoped().Delete(&AffiliateTransferRequest{}).Error)
	require.NoError(t, DB.Session(&gorm.Session{AllowGlobalUpdate: true}).Unscoped().Delete(&TopUp{}).Error)
	require.NoError(t, DB.Session(&gorm.Session{AllowGlobalUpdate: true}).Unscoped().Delete(&User{}).Error)
}

func setupAffiliateTransferRequestFixture(t *testing.T) {
	t.Helper()
	require.NoError(t, DB.AutoMigrate(&AffiliateTransferRequest{}, &User{}, &TopUp{}))
	clearAffiliateTransferRequestFixture(t)
	t.Cleanup(func() {
		clearAffiliateTransferRequestFixture(t)
	})
}

func TestListUserAffiliateTransferRequestsScopesOrdersAndPaginates(t *testing.T) {
	setupAffiliateTransferRequestFixture(t)

	requests := []AffiliateTransferRequest{
		{
			Id:                  41,
			UserId:              101,
			InviteRewardQuota:   100,
			RechargeRebateQuota: 200,
			TotalQuota:          300,
			Status:              AffiliateTransferStatusApproved,
			CreatedAt:           1000,
		},
		{
			Id:                  42,
			UserId:              202,
			InviteRewardQuota:   900,
			RechargeRebateQuota: 900,
			TotalQuota:          1800,
			Status:              AffiliateTransferStatusRejected,
			CreatedAt:           2000,
		},
		{
			Id:                  43,
			UserId:              101,
			InviteRewardQuota:   400,
			RechargeRebateQuota: 500,
			TotalQuota:          900,
			Status:              AffiliateTransferStatusPending,
			CreatedAt:           3000,
		},
	}
	require.NoError(t, DB.Create(&requests).Error)

	items, total, err := ListUserAffiliateTransferRequests(101, &common.PageInfo{Page: 1, PageSize: 10})
	require.NoError(t, err)
	assert.EqualValues(t, 2, total)
	require.Len(t, items, 2)
	assert.Equal(t, 43, items[0].Id)
	assert.Equal(t, 900, items[0].TotalQuota)
	assert.Equal(t, AffiliateTransferStatusPending, items[0].Status)
	assert.Equal(t, 41, items[1].Id)
	assert.Equal(t, 300, items[1].TotalQuota)
	assert.Equal(t, AffiliateTransferStatusApproved, items[1].Status)

	secondPage, total, err := ListUserAffiliateTransferRequests(101, &common.PageInfo{Page: 2, PageSize: 1})
	require.NoError(t, err)
	assert.EqualValues(t, 2, total)
	require.Len(t, secondPage, 1)
	assert.Equal(t, 41, secondPage[0].Id)

	empty, total, err := ListUserAffiliateTransferRequests(303, &common.PageInfo{Page: 1, PageSize: 10})
	require.NoError(t, err)
	assert.Zero(t, total)
	assert.NotNil(t, empty)
	assert.Empty(t, empty)
}

func TestCreateAffiliateTransferRequestMinimumQuotaBoundary(t *testing.T) {
	setupAffiliateTransferRequestFixture(t)

	minimumQuota := int(common.QuotaPerUnit)
	owner := User{
		Username: "minimum-affiliate-owner",
		Password: "password",
		AffCode:  "minimum-affiliate-owner-code",
		AffQuota: minimumQuota - 1,
	}
	require.NoError(t, DB.Create(&owner).Error)

	request, err := CreateAffiliateTransferRequest(owner.Id)
	require.Error(t, err)
	assert.Nil(t, request)

	require.NoError(t, DB.Model(&owner).Update("aff_quota", minimumQuota).Error)
	request, err = CreateAffiliateTransferRequest(owner.Id)
	require.NoError(t, err)
	assert.Equal(t, minimumQuota, request.TotalQuota)
}

func TestAffiliateTransferRequestMultiConnectionConcurrentTerminalTransition(t *testing.T) {
	concurrentDB, err := gorm.Open(sqlite.Open("file:"+uuid.NewString()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := concurrentDB.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(4)
	sqlDB.SetMaxIdleConns(4)
	require.NoError(t, concurrentDB.AutoMigrate(&AffiliateTransferRequest{}, &User{}))

	originalDB := DB
	DB = concurrentDB
	t.Cleanup(func() {
		DB = originalDB
		require.NoError(t, sqlDB.Close())
	})

	owner := User{
		Username: "concurrent-affiliate-owner",
		Password: "password",
		AffCode:  "concurrent-affiliate-owner-code",
		AffQuota: 200,
		Quota:    50,
	}
	require.NoError(t, DB.Create(&owner).Error)
	request := AffiliateTransferRequest{
		UserId:              owner.Id,
		InviteRewardQuota:   200,
		RechargeRebateQuota: 300,
		TotalQuota:          500,
		Status:              AffiliateTransferStatusPending,
		CreatedAt:           common.GetTimestamp(),
	}
	require.NoError(t, DB.Create(&request).Error)

	type terminalResult struct {
		status string
		err    error
	}
	start := make(chan struct{})
	results := make(chan terminalResult, 2)
	var ready sync.WaitGroup
	ready.Add(2)
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		ready.Done()
		<-start
		results <- terminalResult{
			status: AffiliateTransferStatusApproved,
			err:    ApproveAffiliateTransferRequest(request.Id, 91),
		}
	}()
	go func() {
		defer wg.Done()
		ready.Done()
		<-start
		results <- terminalResult{
			status: AffiliateTransferStatusRejected,
			err:    RejectAffiliateTransferRequest(request.Id, 92, "rejected"),
		}
	}()
	ready.Wait()
	close(start)
	wg.Wait()
	close(results)

	successes := 0
	succeededStatus := ""
	for result := range results {
		if result.err == nil {
			successes++
			succeededStatus = result.status
		}
	}
	require.Equal(t, 1, successes)

	var storedRequest AffiliateTransferRequest
	require.NoError(t, DB.First(&storedRequest, request.Id).Error)
	assert.Equal(t, succeededStatus, storedRequest.Status)

	var storedOwner User
	require.NoError(t, DB.First(&storedOwner, owner.Id).Error)
	assert.Zero(t, storedOwner.AffQuota)
	if storedRequest.Status == AffiliateTransferStatusApproved {
		assert.Equal(t, 550, storedOwner.Quota)
	} else {
		assert.Equal(t, 50, storedOwner.Quota)
	}
}

func TestApproveAffiliateTransferRequestRollsBackWhenInviteQuotaInsufficient(t *testing.T) {
	setupAffiliateTransferRequestFixture(t)

	owner := User{
		Username: "approval-rollback-affiliate-owner",
		Password: "password",
		AffCode:  "approval-rollback-owner-code",
		AffQuota: 100,
		Quota:    50,
	}
	require.NoError(t, DB.Create(&owner).Error)
	request := AffiliateTransferRequest{
		UserId:              owner.Id,
		InviteRewardQuota:   200,
		RechargeRebateQuota: 300,
		TotalQuota:          500,
		Status:              AffiliateTransferStatusPending,
		CreatedAt:           100,
	}
	require.NoError(t, DB.Create(&request).Error)

	require.Error(t, ApproveAffiliateTransferRequest(request.Id, 99))

	var storedRequest AffiliateTransferRequest
	require.NoError(t, DB.First(&storedRequest, request.Id).Error)
	assert.Equal(t, AffiliateTransferStatusPending, storedRequest.Status)
	assert.Zero(t, storedRequest.ReviewedAt)
	assert.Zero(t, storedRequest.ReviewedBy)

	var storedOwner User
	require.NoError(t, DB.First(&storedOwner, owner.Id).Error)
	assert.Equal(t, 100, storedOwner.AffQuota)
	assert.Equal(t, 50, storedOwner.Quota)
}

func TestApproveAffiliateTransferRequestRollsBackWhenRecipientDoesNotExist(t *testing.T) {
	setupAffiliateTransferRequestFixture(t)

	request := AffiliateTransferRequest{
		UserId:              999999,
		RechargeRebateQuota: 300,
		TotalQuota:          300,
		Status:              AffiliateTransferStatusPending,
		CreatedAt:           100,
	}
	require.NoError(t, DB.Create(&request).Error)

	require.Error(t, ApproveAffiliateTransferRequest(request.Id, 99))

	var storedRequest AffiliateTransferRequest
	require.NoError(t, DB.First(&storedRequest, request.Id).Error)
	assert.Equal(t, AffiliateTransferStatusPending, storedRequest.Status)
	assert.Zero(t, storedRequest.ReviewedAt)
	assert.Zero(t, storedRequest.ReviewedBy)
}

func TestRejectAffiliateTransferRequestForfeitsNewRequest(t *testing.T) {
	setupAffiliateTransferRequestFixture(t)

	owner := User{Username: "affiliate-owner", Password: "password", AffCode: "affiliate-owner-code", AffQuota: 200, Quota: 50}
	require.NoError(t, DB.Create(&owner).Error)
	invitee := User{Username: "affiliate-invitee", Password: "password", AffCode: "affiliate-invitee-code", InviterId: owner.Id}
	require.NoError(t, DB.Create(&invitee).Error)
	require.NoError(t, DB.Create(&TopUp{
		UserId:          invitee.Id,
		Amount:          6000,
		TradeNo:         "affiliate-creem-success",
		PaymentMethod:   PaymentMethodCreem,
		PaymentProvider: PaymentProviderCreem,
		CompleteTime:    common.GetTimestamp(),
		Status:          common.TopUpStatusSuccess,
	}).Error)
	request := AffiliateTransferRequest{
		UserId:              owner.Id,
		InviteRewardQuota:   200,
		RechargeRebateQuota: 300,
		TotalQuota:          500,
		Status:              AffiliateTransferStatusPending,
		CreatedAt:           common.GetTimestamp(),
	}
	require.NoError(t, DB.Create(&request).Error)

	require.NoError(t, RejectAffiliateTransferRequest(request.Id, 99, "  invalid request  "))

	require.NoError(t, DB.First(&owner, owner.Id).Error)
	assert.Equal(t, 0, owner.AffQuota)
	assert.Equal(t, 50, owner.Quota)
	require.NoError(t, DB.First(&request, request.Id).Error)
	assert.Equal(t, AffiliateTransferStatusRejected, request.Status)
	assert.Positive(t, request.RejectedQuotaForfeitedAt)

	summary, err := GetAffiliateRebateSummary(owner.Id)
	require.NoError(t, err)
	assert.Equal(t, 0, summary.RechargeRebateQuota)
}

func TestRejectAffiliateTransferRequestRollsBackWhenInviteQuotaInsufficient(t *testing.T) {
	setupAffiliateTransferRequestFixture(t)

	owner := User{Username: "rollback-affiliate-owner", Password: "password", AffCode: "rollback-owner-code", AffQuota: 100, Quota: 50}
	require.NoError(t, DB.Create(&owner).Error)
	request := AffiliateTransferRequest{
		UserId:              owner.Id,
		InviteRewardQuota:   200,
		RechargeRebateQuota: 300,
		TotalQuota:          500,
		Status:              AffiliateTransferStatusPending,
		CreatedAt:           100,
	}
	require.NoError(t, DB.Create(&request).Error)

	require.Error(t, RejectAffiliateTransferRequest(request.Id, 99, "insufficient reward balance"))

	var storedRequest AffiliateTransferRequest
	require.NoError(t, DB.First(&storedRequest, request.Id).Error)
	assert.Equal(t, AffiliateTransferStatusPending, storedRequest.Status)
	assert.Zero(t, storedRequest.RejectedQuotaForfeitedAt)
	assert.Zero(t, storedRequest.ReviewedAt)
	assert.Zero(t, storedRequest.ReviewedBy)
	assert.Empty(t, storedRequest.RejectReason)

	var storedOwner User
	require.NoError(t, DB.First(&storedOwner, owner.Id).Error)
	assert.Equal(t, 100, storedOwner.AffQuota)
	assert.Equal(t, 50, storedOwner.Quota)
}

func TestAffiliateRebateSummaryDoesNotForfeitLegacyRejection(t *testing.T) {
	setupAffiliateTransferRequestFixture(t)

	owner := User{Username: "legacy-affiliate-owner", Password: "password", AffCode: "legacy-owner-code", AffQuota: 200, Quota: 50}
	require.NoError(t, DB.Create(&owner).Error)
	invitee := User{Username: "legacy-affiliate-invitee", Password: "password", AffCode: "legacy-invitee-code", InviterId: owner.Id}
	require.NoError(t, DB.Create(&invitee).Error)
	require.NoError(t, DB.Create(&TopUp{
		UserId:          invitee.Id,
		Amount:          6000,
		TradeNo:         "legacy-affiliate-creem-success",
		PaymentMethod:   PaymentMethodCreem,
		PaymentProvider: PaymentProviderCreem,
		CompleteTime:    common.GetTimestamp(),
		Status:          common.TopUpStatusSuccess,
	}).Error)
	legacyRequest := AffiliateTransferRequest{
		UserId:              owner.Id,
		InviteRewardQuota:   200,
		RechargeRebateQuota: 300,
		TotalQuota:          500,
		Status:              AffiliateTransferStatusRejected,
		CreatedAt:           common.GetTimestamp(),
	}
	require.NoError(t, DB.Create(&legacyRequest).Error)
	result := DB.Model(&AffiliateTransferRequest{}).
		Where("id = ?", legacyRequest.Id).
		UpdateColumn("rejected_quota_forfeited_at", nil)
	require.NoError(t, result.Error)
	require.Equal(t, int64(1), result.RowsAffected)

	summary, err := GetAffiliateRebateSummary(owner.Id)
	require.NoError(t, err)
	assert.Equal(t, 300, summary.RechargeRebateQuota)
}

func TestAffiliateTransferRequestDetailUsesForfeitureMarkerForPriorConsumption(t *testing.T) {
	tests := []struct {
		name             string
		marked           bool
		wantSecondSource bool
	}{
		{
			name:             "marked rejection consumes the earlier source",
			marked:           true,
			wantSecondSource: true,
		},
		{
			name: "legacy rejection leaves the earlier source available",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setupAffiliateTransferRequestFixture(t)

			owner := User{Username: "detail-affiliate-owner", Password: "password", AffCode: "detail-owner-code"}
			require.NoError(t, DB.Create(&owner).Error)
			firstInvitee := User{Username: "detail-first-invitee", Password: "password", DisplayName: "First Invitee", AffCode: "detail-first-code", InviterId: owner.Id}
			require.NoError(t, DB.Create(&firstInvitee).Error)
			secondInvitee := User{Username: "detail-second-user", Password: "password", DisplayName: "Second Invitee", AffCode: "detail-second-code", InviterId: owner.Id}
			require.NoError(t, DB.Create(&secondInvitee).Error)

			firstTopUp := TopUp{
				UserId:          firstInvitee.Id,
				Amount:          6000,
				TradeNo:         "detail-first-topup",
				PaymentMethod:   "first-method",
				PaymentProvider: PaymentProviderCreem,
				CompleteTime:    100,
				Status:          common.TopUpStatusSuccess,
			}
			require.NoError(t, DB.Create(&firstTopUp).Error)
			secondTopUp := TopUp{
				UserId:          secondInvitee.Id,
				Amount:          6000,
				TradeNo:         "detail-second-topup",
				PaymentMethod:   "second-method",
				PaymentProvider: PaymentProviderCreem,
				CompleteTime:    200,
				Status:          common.TopUpStatusSuccess,
			}
			require.NoError(t, DB.Create(&secondTopUp).Error)

			previousRequest := AffiliateTransferRequest{
				UserId:              owner.Id,
				RechargeRebateQuota: 300,
				TotalQuota:          300,
				Status:              AffiliateTransferStatusRejected,
				CreatedAt:           300,
			}
			if tt.marked {
				previousRequest.RejectedQuotaForfeitedAt = 225
			}
			require.NoError(t, DB.Create(&previousRequest).Error)
			if !tt.marked {
				result := DB.Model(&AffiliateTransferRequest{}).
					Where("id = ?", previousRequest.Id).
					UpdateColumn("rejected_quota_forfeited_at", nil)
				require.NoError(t, result.Error)
				require.Equal(t, int64(1), result.RowsAffected)
			}

			currentRequest := AffiliateTransferRequest{
				UserId:              owner.Id,
				RechargeRebateQuota: 300,
				TotalQuota:          300,
				Status:              AffiliateTransferStatusPending,
				CreatedAt:           300,
			}
			require.NoError(t, DB.Create(&currentRequest).Error)

			detail, err := GetAffiliateTransferRequestDetail(currentRequest.Id)
			require.NoError(t, err)
			require.Len(t, detail.RechargeSources, 1)
			assert.Equal(t, 6000, detail.TotalInvitedRechargeQuota)

			wantInviteeId := firstInvitee.Id
			wantPaymentMethod := firstTopUp.PaymentMethod
			wantCompleteTime := firstTopUp.CompleteTime
			if tt.wantSecondSource {
				wantInviteeId = secondInvitee.Id
				wantPaymentMethod = secondTopUp.PaymentMethod
				wantCompleteTime = secondTopUp.CompleteTime
			}
			source := detail.RechargeSources[0]
			assert.Equal(t, wantInviteeId, source.InvitedUserId)
			assert.Equal(t, wantPaymentMethod, source.PaymentMethod)
			assert.Equal(t, wantCompleteTime, source.CompleteTime)
			assert.Equal(t, 6000, source.CreditedQuota)
			assert.Equal(t, 300, source.RebateQuota)
		})
	}
}
