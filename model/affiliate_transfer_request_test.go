package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func clearAffiliateTransferRequestFixture(t *testing.T) {
	t.Helper()
	db := DB.Session(&gorm.Session{AllowGlobalUpdate: true}).Unscoped()
	require.NoError(t, db.Delete(&AffiliateTransferRequest{}).Error)
	require.NoError(t, db.Delete(&TopUp{}).Error)
	require.NoError(t, db.Delete(&User{}).Error)
}

func setupAffiliateTransferRequestFixture(t *testing.T) {
	t.Helper()
	require.NoError(t, DB.AutoMigrate(&AffiliateTransferRequest{}, &User{}, &TopUp{}))
	clearAffiliateTransferRequestFixture(t)
	t.Cleanup(func() {
		clearAffiliateTransferRequestFixture(t)
	})
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
	require.NoError(t, DB.Create(&AffiliateTransferRequest{
		UserId:              owner.Id,
		InviteRewardQuota:   200,
		RechargeRebateQuota: 300,
		TotalQuota:          500,
		Status:              AffiliateTransferStatusRejected,
		CreatedAt:           common.GetTimestamp(),
	}).Error)

	summary, err := GetAffiliateRebateSummary(owner.Id)
	require.NoError(t, err)
	assert.Equal(t, 300, summary.RechargeRebateQuota)
}
