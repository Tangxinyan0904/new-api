package model

import (
	"math"
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

type affiliateRejectionFixture struct {
	owner   User
	request AffiliateTransferRequest
}

func createAffiliateRejectionFixture(t *testing.T, db *gorm.DB, prefix string) affiliateRejectionFixture {
	t.Helper()

	owner := User{
		Username: prefix + "-owner",
		Password: "password",
		AffCode:  prefix + "-owner-code",
		AffQuota: 400,
		Quota:    50,
	}
	require.NoError(t, db.Create(&owner).Error)
	invitee := User{
		Username:  prefix + "-invitee",
		Password:  "password",
		AffCode:   prefix + "-invitee-code",
		InviterId: owner.Id,
	}
	require.NoError(t, db.Create(&invitee).Error)
	require.NoError(t, db.Create(&TopUp{
		UserId:          invitee.Id,
		Amount:          6000,
		TradeNo:         prefix + "-topup",
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
	require.NoError(t, db.Create(&request).Error)

	return affiliateRejectionFixture{owner: owner, request: request}
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

func TestAffiliateRebateCalculationsRejectSaturatedTopUpQuota(t *testing.T) {
	tests := []struct {
		name string
		load func(t *testing.T, owner User, invitee User) error
	}{
		{
			name: "summary",
			load: func(t *testing.T, owner User, _ User) error {
				_, err := GetAffiliateRebateSummary(owner.Id)
				return err
			},
		},
		{
			name: "approval detail",
			load: func(t *testing.T, owner User, _ User) error {
				request := AffiliateTransferRequest{
					UserId:              owner.Id,
					RechargeRebateQuota: 1,
					TotalQuota:          1,
					Status:              AffiliateTransferStatusPending,
					CreatedAt:           100,
				}
				require.NoError(t, DB.Create(&request).Error)
				_, err := GetAffiliateTransferRequestDetail(request.Id)
				return err
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setupAffiliateTransferRequestFixture(t)

			owner := User{Username: "saturated-affiliate-owner", Password: "password", AffCode: "saturated-affiliate-owner-code"}
			require.NoError(t, DB.Create(&owner).Error)
			invitee := User{Username: "saturated-affiliate-invitee", Password: "password", AffCode: "saturated-affiliate-invitee-code", InviterId: owner.Id}
			require.NoError(t, DB.Create(&invitee).Error)
			require.NoError(t, DB.Create(&TopUp{
				UserId:          invitee.Id,
				Amount:          math.MaxInt64,
				TradeNo:         "saturated-affiliate-topup-" + tt.name,
				PaymentMethod:   PaymentMethodBalance,
				PaymentProvider: PaymentProviderEpay,
				CompleteTime:    100,
				Status:          common.TopUpStatusSuccess,
			}).Error)

			err := tt.load(t, owner, invitee)
			require.Error(t, err)
			var clamp *common.QuotaClamp
			require.ErrorAs(t, err, &clamp)
			assert.Equal(t, common.QuotaClampOverflow, clamp.Kind)
		})
	}
}

func TestApproveAffiliateTransferRequestUpdatesCachedQuota(t *testing.T) {
	setupAffiliateTransferRequestFixture(t)
	useUserCacheMiniRedis(t)

	owner := User{
		Username:    "cached-affiliate-owner",
		Password:    "password",
		AffCode:     "cached-affiliate-owner-code",
		Quota:       50,
		Role:        common.RoleCommonUser,
		Status:      common.UserStatusEnabled,
		Group:       "default",
		AuthVersion: 1,
	}
	require.NoError(t, DB.Create(&owner).Error)
	require.NoError(t, populateUserCache(owner))
	request := AffiliateTransferRequest{
		UserId:              owner.Id,
		RechargeRebateQuota: 500,
		TotalQuota:          500,
		Status:              AffiliateTransferStatusPending,
		CreatedAt:           100,
	}
	require.NoError(t, DB.Create(&request).Error)

	require.NoError(t, ApproveAffiliateTransferRequest(request.Id, 99))

	quota, err := GetUserQuota(owner.Id, false)
	require.NoError(t, err)
	assert.Equal(t, 550, quota)
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

func TestRejectAffiliateTransferRequestOnlyForfeitsOnce(t *testing.T) {
	setupAffiliateTransferRequestFixture(t)
	fixture := createAffiliateRejectionFixture(t, DB, "sequential-rejection")

	firstErr := RejectAffiliateTransferRequest(fixture.request.Id, 91, "first rejection")
	secondErr := RejectAffiliateTransferRequest(fixture.request.Id, 92, "second rejection")

	require.NoError(t, firstErr)
	require.Error(t, secondErr)
	assert.ErrorContains(t, secondErr, "request is not pending")

	var storedOwner User
	require.NoError(t, DB.First(&storedOwner, fixture.owner.Id).Error)
	assert.Equal(t, 200, storedOwner.AffQuota)
	assert.Equal(t, 50, storedOwner.Quota)

	var storedRequest AffiliateTransferRequest
	require.NoError(t, DB.First(&storedRequest, fixture.request.Id).Error)
	assert.Equal(t, AffiliateTransferStatusRejected, storedRequest.Status)
	assert.Equal(t, 91, storedRequest.ReviewedBy)
	assert.Equal(t, "first rejection", storedRequest.RejectReason)
	assert.Positive(t, storedRequest.RejectedQuotaForfeitedAt)

	summary, err := GetAffiliateRebateSummary(fixture.owner.Id)
	require.NoError(t, err)
	assert.Equal(t, 0, summary.RechargeRebateQuota)
}

func TestRejectAffiliateTransferRequestConcurrentCallsOnlyForfeitOnce(t *testing.T) {
	concurrentDB, err := gorm.Open(sqlite.Open("file:"+uuid.NewString()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := concurrentDB.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(4)
	sqlDB.SetMaxIdleConns(4)
	require.NoError(t, concurrentDB.AutoMigrate(&AffiliateTransferRequest{}, &User{}, &TopUp{}))

	originalDB := DB
	DB = concurrentDB
	t.Cleanup(func() {
		DB = originalDB
		require.NoError(t, sqlDB.Close())
	})

	fixture := createAffiliateRejectionFixture(t, concurrentDB, "concurrent-rejection")
	type rejectionAttempt struct {
		reviewerId int
		reason     string
	}
	attempts := []rejectionAttempt{
		{reviewerId: 91, reason: "first concurrent rejection"},
		{reviewerId: 92, reason: "second concurrent rejection"},
	}
	start := make(chan struct{})
	results := make(chan error, len(attempts))
	var ready sync.WaitGroup
	ready.Add(len(attempts))
	var wg sync.WaitGroup
	wg.Add(len(attempts))
	for _, attempt := range attempts {
		go func(attempt rejectionAttempt) {
			defer wg.Done()
			ready.Done()
			<-start
			results <- RejectAffiliateTransferRequest(fixture.request.Id, attempt.reviewerId, attempt.reason)
		}(attempt)
	}
	ready.Wait()
	close(start)
	wg.Wait()
	close(results)

	successes := 0
	for result := range results {
		if result == nil {
			successes++
		}
	}
	require.Equal(t, 1, successes)

	var storedOwner User
	require.NoError(t, DB.First(&storedOwner, fixture.owner.Id).Error)
	assert.Equal(t, 200, storedOwner.AffQuota)
	assert.Equal(t, 50, storedOwner.Quota)

	var storedRequest AffiliateTransferRequest
	require.NoError(t, DB.First(&storedRequest, fixture.request.Id).Error)
	assert.Equal(t, AffiliateTransferStatusRejected, storedRequest.Status)
	assert.Contains(t, []int{91, 92}, storedRequest.ReviewedBy)
	assert.Positive(t, storedRequest.RejectedQuotaForfeitedAt)

	summary, err := GetAffiliateRebateSummary(fixture.owner.Id)
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

func TestAffiliateTransferRequestDetailIncludesAllInvitedUsers(t *testing.T) {
	setupAffiliateTransferRequestFixture(t)

	owner := User{
		Username: "audit-owner",
		Password: "password",
		AffCode:  "audit-owner-code",
	}
	require.NoError(t, DB.Create(&owner).Error)
	request := AffiliateTransferRequest{
		UserId:    owner.Id,
		Status:    AffiliateTransferStatusPending,
		CreatedAt: 200,
	}
	require.NoError(t, DB.Create(&request).Error)

	older := User{
		Username:    "audit-older",
		Password:    "password",
		DisplayName: "Older User",
		Email:       "older@example.com",
		AffCode:     "audit-older-code",
		InviterId:   owner.Id,
		CreatedAt:   100,
		LastLoginAt: 150,
	}
	newerNeverLoggedIn := User{
		Username:  "audit-newer",
		Password:  "password",
		AffCode:   "audit-newer-code",
		InviterId: owner.Id,
		CreatedAt: 300,
	}
	deletedAtSameTime := User{
		Username:    "audit-deleted",
		Password:    "password",
		DisplayName: "Deleted User",
		AffCode:     "audit-deleted-code",
		InviterId:   owner.Id,
		CreatedAt:   300,
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
	assert.Equal(t, "audit-older", detail.InvitedUsers[2].Username)
	assert.Equal(t, "Older User", detail.InvitedUsers[2].DisplayName)
	assert.Equal(t, int64(100), detail.InvitedUsers[2].CreatedAt)
	assert.Equal(t, int64(150), detail.InvitedUsers[2].LastLoginAt)
	assert.Zero(t, detail.InvitedUsers[1].LastLoginAt)
	assert.True(t, detail.InvitedUsers[0].IsDeleted)
	assert.False(t, detail.InvitedUsers[1].IsDeleted)

	payload, err := common.Marshal(detail)
	require.NoError(t, err)
	assert.NotContains(t, string(payload), "password")
	assert.NotContains(t, string(payload), "older@example.com")
}

func TestAffiliateTransferRequestDetailExcludesDeletedInviteeTopUpsFromRechargeSources(t *testing.T) {
	setupAffiliateTransferRequestFixture(t)

	owner := User{
		Username: "deleted-topup-owner",
		Password: "password",
		AffCode:  "deleted-topup-owner-code",
	}
	require.NoError(t, DB.Create(&owner).Error)
	request := AffiliateTransferRequest{
		UserId:              owner.Id,
		RechargeRebateQuota: 300,
		TotalQuota:          300,
		Status:              AffiliateTransferStatusPending,
		CreatedAt:           200,
	}
	require.NoError(t, DB.Create(&request).Error)
	deletedInvitee := User{
		Username:  "deleted-topup-invitee",
		Password:  "password",
		AffCode:   "deleted-topup-invitee-code",
		InviterId: owner.Id,
		CreatedAt: 100,
	}
	require.NoError(t, DB.Create(&deletedInvitee).Error)
	require.NoError(t, DB.Create(&TopUp{
		UserId:          deletedInvitee.Id,
		Amount:          6000,
		TradeNo:         "deleted-topup-invitee-recharge",
		PaymentMethod:   PaymentMethodCreem,
		PaymentProvider: PaymentProviderCreem,
		CompleteTime:    100,
		Status:          common.TopUpStatusSuccess,
	}).Error)
	require.NoError(t, DB.Delete(&deletedInvitee).Error)

	detail, err := GetAffiliateTransferRequestDetail(request.Id)
	require.NoError(t, err)
	require.Len(t, detail.InvitedUsers, 1)
	assert.True(t, detail.InvitedUsers[0].IsDeleted)
	assert.Zero(t, detail.TotalInvitedRechargeQuota)
	assert.Empty(t, detail.RechargeSources)
}
