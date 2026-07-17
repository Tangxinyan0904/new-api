package service

import (
	"errors"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupWalletQuotaNotifyTest(t *testing.T, userId int, quota int, notifyType string) *relaycommon.RelayInfo {
	t.Helper()
	truncate(t)

	setting := dto.UserSetting{
		NotifyType:            notifyType,
		QuotaWarningThreshold: 50,
	}
	user := model.User{
		Id:       userId,
		Username: "quota-notify-user",
		Password: "password",
		Email:    "quota-notify@example.com",
		Status:   common.UserStatusEnabled,
		Quota:    quota,
	}
	user.SetSetting(setting)
	require.NoError(t, model.DB.Create(&user).Error)

	return &relaycommon.RelayInfo{
		UserId:      user.Id,
		UserEmail:   user.Email,
		UserQuota:   user.Quota,
		UserSetting: setting,
	}
}

func TestSendWalletQuotaNotifySendsEmailOnceWhileBelowThreshold(t *testing.T) {
	relayInfo := setupWalletQuotaNotifyTest(t, 401, 40, dto.NotifyTypeEmail)
	sendCount := 0
	var sent dto.Notify
	notify := func(_ int, _ string, _ dto.UserSetting, data dto.Notify) error {
		sendCount++
		sent = data
		return nil
	}

	require.NoError(t, sendWalletQuotaNotify(relayInfo, 40, notify))
	require.NoError(t, sendWalletQuotaNotify(relayInfo, 35, notify))

	assert.Equal(t, 1, sendCount)
	require.Len(t, sent.Values, 4)
	assert.Equal(t, logger.FormatQuota(40), sent.Values[1])
}

func TestSendWalletQuotaNotifyReleasesFailedEmailForRetry(t *testing.T) {
	relayInfo := setupWalletQuotaNotifyTest(t, 402, 40, dto.NotifyTypeEmail)
	sendCount := 0
	notify := func(_ int, _ string, _ dto.UserSetting, _ dto.Notify) error {
		sendCount++
		if sendCount == 1 {
			return errors.New("smtp unavailable")
		}
		return nil
	}

	require.Error(t, sendWalletQuotaNotify(relayInfo, 40, notify))
	require.NoError(t, sendWalletQuotaNotify(relayInfo, 40, notify))
	assert.Equal(t, 2, sendCount)
}

func TestSendWalletQuotaNotifyRearmsEmailAtThreshold(t *testing.T) {
	relayInfo := setupWalletQuotaNotifyTest(t, 403, 50, dto.NotifyTypeEmail)
	claimed, err := model.TryClaimQuotaWarningEmail(relayInfo.UserId)
	require.NoError(t, err)
	require.True(t, claimed)

	sendCount := 0
	notify := func(_ int, _ string, _ dto.UserSetting, _ dto.Notify) error {
		sendCount++
		return nil
	}
	require.NoError(t, sendWalletQuotaNotify(relayInfo, 50, notify))
	assert.Zero(t, sendCount)

	claimed, err = model.TryClaimQuotaWarningEmail(relayInfo.UserId)
	require.NoError(t, err)
	assert.True(t, claimed)
}

func TestSendWalletQuotaNotifyLeavesNonEmailBehaviorUnchanged(t *testing.T) {
	relayInfo := setupWalletQuotaNotifyTest(t, 404, 40, dto.NotifyTypeWebhook)
	sendCount := 0
	notify := func(_ int, _ string, _ dto.UserSetting, _ dto.Notify) error {
		sendCount++
		return nil
	}

	require.NoError(t, sendWalletQuotaNotify(relayInfo, 40, notify))
	require.NoError(t, sendWalletQuotaNotify(relayInfo, 35, notify))
	assert.Equal(t, 2, sendCount)
}
