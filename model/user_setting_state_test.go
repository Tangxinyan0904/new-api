package model

import (
	"sync"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupQuotaWarningEmailTestUser(t *testing.T, id int, quota int) User {
	t.Helper()
	truncateTables(t)

	user := User{
		Id:       id,
		Username: "quota-warning-user",
		Password: "password",
		Email:    "quota-warning@example.com",
		Status:   common.UserStatusEnabled,
		Quota:    quota,
	}
	user.SetSetting(dto.UserSetting{
		NotifyType:            dto.NotifyTypeEmail,
		QuotaWarningThreshold: 50,
		Language:              "en",
	})
	require.NoError(t, DB.Create(&user).Error)
	return user
}

func quotaWarningEmailSent(t *testing.T, userId int) bool {
	t.Helper()

	var user User
	require.NoError(t, DB.Select("setting").First(&user, userId).Error)
	record, err := decodeUserSettingRecord(user.Setting)
	require.NoError(t, err)
	return record.QuotaWarningEmailSent
}

func TestQuotaWarningEmailClaimAndRelease(t *testing.T) {
	user := setupQuotaWarningEmailTestUser(t, 301, 40)

	claimed, err := TryClaimQuotaWarningEmail(user.Id)
	require.NoError(t, err)
	assert.True(t, claimed)
	assert.True(t, quotaWarningEmailSent(t, user.Id))

	claimed, err = TryClaimQuotaWarningEmail(user.Id)
	require.NoError(t, err)
	assert.False(t, claimed)

	require.NoError(t, ReleaseQuotaWarningEmail(user.Id))
	assert.False(t, quotaWarningEmailSent(t, user.Id))

	claimed, err = TryClaimQuotaWarningEmail(user.Id)
	require.NoError(t, err)
	assert.True(t, claimed)
}

func TestQuotaWarningEmailConcurrentClaimHasOneWinner(t *testing.T) {
	user := setupQuotaWarningEmailTestUser(t, 302, 40)

	results := make(chan bool, 2)
	errors := make(chan error, 2)
	var wg sync.WaitGroup
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			claimed, err := TryClaimQuotaWarningEmail(user.Id)
			results <- claimed
			errors <- err
		}()
	}
	wg.Wait()
	close(results)
	close(errors)

	claimedCount := 0
	for err := range errors {
		require.NoError(t, err)
	}
	for claimed := range results {
		if claimed {
			claimedCount++
		}
	}
	assert.Equal(t, 1, claimedCount)
}

func TestUpdateUserSettingPreservesQuotaWarningEmailState(t *testing.T) {
	user := setupQuotaWarningEmailTestUser(t, 303, 40)

	claimed, err := TryClaimQuotaWarningEmail(user.Id)
	require.NoError(t, err)
	require.True(t, claimed)

	require.NoError(t, UpdateUserSetting(user.Id, dto.UserSetting{
		NotifyType:            dto.NotifyTypeEmail,
		QuotaWarningThreshold: 75,
		Language:              "zh",
	}))

	var got User
	require.NoError(t, DB.First(&got, user.Id).Error)
	assert.Equal(t, "zh", got.GetSetting().Language)
	assert.Equal(t, float64(75), got.GetSetting().QuotaWarningThreshold)
	assert.True(t, quotaWarningEmailSent(t, user.Id))
}

func TestQuotaWarningEmailMalformedSettingIsNotOverwritten(t *testing.T) {
	truncateTables(t)
	user := User{
		Id:       304,
		Username: "malformed-setting-user",
		Password: "password",
		Status:   common.UserStatusEnabled,
		Setting:  "{malformed",
	}
	require.NoError(t, DB.Create(&user).Error)

	claimed, err := TryClaimQuotaWarningEmail(user.Id)
	assert.False(t, claimed)
	require.Error(t, err)

	var raw string
	require.NoError(t, DB.Model(&User{}).Where("id = ?", user.Id).Select("setting").Scan(&raw).Error)
	assert.Equal(t, "{malformed", raw)
}

func TestQuotaWarningEmailRearmOnQuotaRecovery(t *testing.T) {
	tests := []struct {
		name       string
		userId     int
		startQuota int
		credit     int
		wantArmed  bool
	}{
		{name: "partial credit remains latched", userId: 305, startQuota: 40, credit: 9, wantArmed: false},
		{name: "exact threshold rearms", userId: 306, startQuota: 40, credit: 10, wantArmed: true},
		{name: "above threshold rearms", userId: 307, startQuota: 40, credit: 11, wantArmed: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			user := setupQuotaWarningEmailTestUser(t, tt.userId, tt.startQuota)
			claimed, err := TryClaimQuotaWarningEmail(user.Id)
			require.NoError(t, err)
			require.True(t, claimed)

			require.NoError(t, IncreaseUserQuota(user.Id, tt.credit, true))

			claimed, err = TryClaimQuotaWarningEmail(user.Id)
			require.NoError(t, err)
			assert.Equal(t, tt.wantArmed, claimed)
		})
	}
}

func TestQuotaWarningEmailExplicitRearm(t *testing.T) {
	user := setupQuotaWarningEmailTestUser(t, 308, 40)
	claimed, err := TryClaimQuotaWarningEmail(user.Id)
	require.NoError(t, err)
	require.True(t, claimed)

	require.NoError(t, RearmQuotaWarningEmail(user.Id))
	claimed, err = TryClaimQuotaWarningEmail(user.Id)
	require.NoError(t, err)
	assert.True(t, claimed)
}
