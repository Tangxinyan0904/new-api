package service

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/setting/config"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/setting/system_setting"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestTicketNoticesFollowEachAdministratorsSwitch(t *testing.T) {
	on, off := true, false
	for _, tc := range []struct {
		name     string
		role     int
		switched *bool
		want     bool
	}{
		{"root who never chose", common.RoleRootUser, nil, true},
		{"root who turned them off", common.RoleRootUser, &off, false},
		{"admin who never chose", common.RoleAdminUser, nil, false},
		{"admin who turned them on", common.RoleAdminUser, &on, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, receivesTicketNotices(tc.role, dto.UserSetting{TicketNotifyEnabled: tc.switched}))
		})
	}
}

// Intercept the provider request after the real notification and email routing.
type ticketMailboxRecorder struct {
	mu         sync.Mutex
	recipients []string
}

func (m *ticketMailboxRecorder) RoundTrip(r *http.Request) (*http.Response, error) {
	var mail struct{ To []string }
	if err := common.DecodeJson(r.Body, &mail); err != nil {
		return nil, err
	}
	m.mu.Lock()
	m.recipients = append(m.recipients, mail.To...)
	m.mu.Unlock()
	return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{"id":"test-message"}`))}, nil
}

func captureTicketMail(t *testing.T) *ticketMailboxRecorder {
	t.Helper()
	originalTransport, originalOptions := http.DefaultTransport, common.OptionMap
	originalRedis, originalLimit := common.RedisEnabled, constant.NotifyLimitCount
	originalWindow := constant.NotificationLimitDurationMinute
	mailbox := &ticketMailboxRecorder{}
	http.DefaultTransport = mailbox
	common.OptionMap = map[string]string{"EmailProvider": "resend", "ResendFrom": "sender@example.com", "ResendApiKey": "fixture"}
	common.RedisEnabled, constant.NotifyLimitCount = false, 100
	constant.NotificationLimitDurationMinute = 10
	t.Cleanup(func() {
		http.DefaultTransport, common.OptionMap = originalTransport, originalOptions
		common.RedisEnabled, constant.NotifyLimitCount = originalRedis, originalLimit
		constant.NotificationLimitDurationMinute = originalWindow
	})
	return mailbox
}

func TestNewTicketNotifiesDedicatedMailboxAndSubscribedStaff(t *testing.T) {
	mailbox := captureTicketMail(t)
	require.NoError(t, i18n.Init())
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { require.NoError(t, sqlDB.Close()) })
	require.NoError(t, db.AutoMigrate(&model.User{}))
	previousDB := model.DB
	model.DB = db
	t.Cleanup(func() { model.DB = previousDB })
	staff := model.User{Id: 8710, Username: "support", Email: "root@example.com", Role: common.RoleRootUser, Status: common.UserStatusEnabled}
	t.Cleanup(func() {
		notifyLimitStore.Delete("8710:ticket")
		notifyLimitStore.Delete("8711:ticket")
	})
	staff.SetSetting(dto.UserSetting{NotificationEmail: "Support <SHARED@example.com>;second@example.com"})
	require.NoError(t, db.Create(&staff).Error)

	const key = operation_setting.TicketAdminEmailOptionKey
	originalMailbox := operation_setting.GetTicketAdminEmail()
	t.Cleanup(func() { require.NoError(t, config.GlobalConfig.LoadFromDB(map[string]string{key: originalMailbox})) })
	require.NoError(t, config.GlobalConfig.LoadFromDB(map[string]string{key: "shared@example.com"}))
	ticket := model.Ticket{Id: 123, Subject: "API question", UserId: 456}
	sendTicketStaffNotices(ticket, "owner-without-email", true)
	assert.ElementsMatch(t, []string{"shared@example.com", "second@example.com"}, mailbox.recipients)

	staff.SetSetting(dto.UserSetting{NotificationEmail: "second@example.com"})
	require.NoError(t, db.Save(&staff).Error)
	mailbox.recipients = nil
	sendTicketStaffNotices(ticket, "owner-without-email", false)
	assert.Equal(t, []string{"second@example.com"}, mailbox.recipients)

	require.NoError(t, config.GlobalConfig.LoadFromDB(map[string]string{key: ""}))
	mailbox.recipients = nil
	// A subscription already at its limit cannot claim the mailbox on
	// behalf of another subscriber who can still receive the notice.
	constant.NotifyLimitCount = 1
	second := model.User{Id: 8711, Username: "otherstaff", AffCode: "otherstaff", Email: "second@example.com", Role: common.RoleRootUser, Status: common.UserStatusEnabled}
	require.NoError(t, db.Create(&second).Error)
	sendTicketStaffNotices(ticket, "owner-without-email", true)
	assert.Equal(t, []string{"second@example.com"}, mailbox.recipients)
}

func TestTicketReplyEmailsDependOnProfileMailbox(t *testing.T) {
	mailbox := captureTicketMail(t)
	t.Cleanup(func() { notifyLimitStore.Delete("8712:ticket") })
	webhook := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	t.Cleanup(webhook.Close)
	previousClient, previousWorker := httpClient, system_setting.WorkerUrl
	fetch := system_setting.GetFetchSetting()
	previousProtection := fetch.EnableSSRFProtection
	httpClient, system_setting.WorkerUrl, fetch.EnableSSRFProtection = webhook.Client(), "", false
	t.Cleanup(func() {
		httpClient, system_setting.WorkerUrl, fetch.EnableSSRFProtection = previousClient, previousWorker, previousProtection
	})
	for _, tc := range []struct {
		name, profile, preferred, channel string
		recipients                        []string
		channelError                      bool
	}{
		{"profile only", "owner@example.com", "", "", []string{"owner@example.com"}, false},
		{"overlapping email preferences", "owner@example.com", "OWNER@example.com;backup@example.com", dto.NotifyTypeEmail, []string{"owner@example.com", "backup@example.com"}, false},
		{"missing profile skips all mail", "", "backup@example.com", dto.NotifyTypeEmail, nil, false},
		{"malformed profile cannot expand into multiple recipients", "owner@example.com;other@example.com", "", "", nil, false},
		{"profile still notified when webhook fails", "owner@example.com", "", dto.NotifyTypeWebhook, []string{"owner@example.com"}, true},
		{"missing profile still attempts webhook", "", "", dto.NotifyTypeWebhook, nil, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			mailbox.recipients = nil
			err := sendTicketOwnerNotice(8712, tc.profile, dto.UserSetting{NotifyType: tc.channel, NotificationEmail: tc.preferred, WebhookUrl: webhook.URL}, dto.NewNotify(NotifyTypeTicket, "Reply", "Open your ticket", nil))
			if tc.channelError {
				assert.ErrorContains(t, err, "status code: 503")
			} else {
				require.NoError(t, err)
			}
			assert.ElementsMatch(t, tc.recipients, mailbox.recipients)
		})
	}
}
