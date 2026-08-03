package service

import (
	"fmt"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNotifyUserSkipsQuotaExceedForEveryChannel(t *testing.T) {
	originalRedisEnabled := common.RedisEnabled
	originalSMTPServer := common.SMTPServer
	originalSMTPAccount := common.SMTPAccount
	originalSMTPFrom := common.SMTPFrom
	common.RedisEnabled = false
	common.SMTPServer = ""
	common.SMTPAccount = ""
	common.SMTPFrom = "sender@example.com"
	t.Cleanup(func() {
		common.RedisEnabled = originalRedisEnabled
		common.SMTPServer = originalSMTPServer
		common.SMTPAccount = originalSMTPAccount
		common.SMTPFrom = originalSMTPFrom
	})

	settings := []dto.UserSetting{
		{NotifyType: dto.NotifyTypeEmail, NotificationEmail: "receiver@example.com"},
		{NotifyType: dto.NotifyTypeWebhook, WebhookUrl: "://invalid"},
		{NotifyType: dto.NotifyTypeBark, BarkUrl: "://invalid"},
		{NotifyType: dto.NotifyTypeGotify, GotifyUrl: "://invalid", GotifyToken: "token"},
	}

	for index, setting := range settings {
		t.Run(setting.NotifyType, func(t *testing.T) {
			userID := 9100 + index
			limitKey := fmt.Sprintf(
				"%d:%s:%s",
				userID,
				dto.NotifyTypeQuotaExceed,
				time.Now().Format("2006010215"),
			)
			notifyLimitStore.Delete(limitKey)

			err := NotifyUser(
				userID,
				"receiver@example.com",
				setting,
				dto.NewNotify(dto.NotifyTypeQuotaExceed, "quota", "quota", nil),
			)
			require.NoError(t, err)

			_, limited := notifyLimitStore.Load(limitKey)
			assert.False(t, limited)
		})
	}
}

func TestNotifyUserKeepsOtherNotificationTypesEnabled(t *testing.T) {
	originalRedisEnabled := common.RedisEnabled
	common.RedisEnabled = false
	t.Cleanup(func() { common.RedisEnabled = originalRedisEnabled })

	const userID = 9200
	limitKey := fmt.Sprintf(
		"%d:%s:%s",
		userID,
		dto.NotifyTypeChannelUpdate,
		time.Now().Format("2006010215"),
	)
	notifyLimitStore.Delete(limitKey)

	err := NotifyUser(
		userID,
		"",
		dto.UserSetting{NotifyType: dto.NotifyTypeWebhook, WebhookUrl: "://invalid"},
		dto.NewNotify(dto.NotifyTypeChannelUpdate, "update", "update", nil),
	)
	require.Error(t, err)

	_, limited := notifyLimitStore.Load(limitKey)
	assert.True(t, limited)
}
