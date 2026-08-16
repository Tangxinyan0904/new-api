package service

import (
	"net/http"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestChannelMonitorAvailabilityCombinesRawHourlyAndDailyNaturalDays(t *testing.T) {
	setupChannelMonitorSchedulerTestDB(t)
	location, err := time.LoadLocation("Asia/Shanghai")
	require.NoError(t, err)
	now := time.Date(2026, time.August, 16, 12, 30, 0, 0, location)
	setChannelMonitorQueryClock(t, now, location)

	monitor := channelMonitorQueryFixture("Internal", "Public", true, true)
	monitor.PrimaryModel = "model-a"
	monitor.ExtraModels = `["model-empty"]`
	require.NoError(t, model.DB.Create(monitor).Error)

	today := channelMonitorQueryTestDayStart(now, location)
	require.NoError(t, model.DB.Create(&[]model.ChannelMonitorCheck{
		{MonitorID: monitor.ID, Model: "model-a", Status: model.ChannelMonitorStatusOperational, LatencyMs: 100, CheckedAt: now.Add(-10 * time.Minute).Unix()},
	}).Error)
	require.NoError(t, model.DB.Create(&[]model.ChannelMonitorHourlyRollup{
		{MonitorID: monitor.ID, Model: "model-a", BucketStart: now.Add(-2 * time.Hour).Unix(), TotalCount: 1, DegradedCount: 1, LatencySumMs: 7000, LatencySampleCount: 1, ComputedAt: now.Unix()},
	}).Error)
	require.NoError(t, model.DB.Create(&[]model.ChannelMonitorDailyRollup{
		{MonitorID: monitor.ID, Model: "model-a", BucketStart: today.Unix(), TotalCount: 1, FailedCount: 1, ComputedAt: now.Unix()},
		{MonitorID: monitor.ID, Model: "model-a", BucketStart: today.AddDate(0, 0, -1).Unix(), TotalCount: 2, OperationalCount: 2, LatencySumMs: 400, LatencySampleCount: 2, ComputedAt: now.Unix()},
		{MonitorID: monitor.ID, Model: "model-a", BucketStart: today.AddDate(0, 0, -3).Unix(), TotalCount: 1, FailedCount: 1, ComputedAt: now.Unix()},
		{MonitorID: monitor.ID, Model: "model-a", BucketStart: today.AddDate(0, 0, -6).Unix(), TotalCount: 1, DegradedCount: 1, LatencySumMs: 6000, LatencySampleCount: 1, ComputedAt: now.Unix()},
	}).Error)

	detail, err := GetPublishedChannelMonitorDetail(monitor.ID)
	require.NoError(t, err)
	require.NotNil(t, detail)
	require.Len(t, detail.Models, 2)
	primary := detail.Models[0]
	assert.Equal(t, "model-a", primary.Model)
	assert.True(t, primary.IsPrimary)
	require.NotNil(t, primary.Availability1d)
	require.NotNil(t, primary.Availability3d)
	require.NotNil(t, primary.Availability7d)
	require.NotNil(t, primary.AverageLatencyMs1d)
	require.NotNil(t, primary.AverageLatencyMs3d)
	require.NotNil(t, primary.AverageLatencyMs7d)
	assert.InDelta(t, 66.6666667, *primary.Availability1d, 0.0001)
	assert.InDelta(t, 80.0, *primary.Availability3d, 0.0001)
	assert.InDelta(t, 71.4285714, *primary.Availability7d, 0.0001)
	assert.InDelta(t, 3550.0, *primary.AverageLatencyMs1d, 0.0001)
	assert.InDelta(t, 1875.0, *primary.AverageLatencyMs3d, 0.0001)
	assert.InDelta(t, 2700.0, *primary.AverageLatencyMs7d, 0.0001)

	empty := detail.Models[1]
	assert.Equal(t, "model-empty", empty.Model)
	assert.False(t, empty.IsPrimary)
	assert.Nil(t, empty.Availability1d)
	assert.Nil(t, empty.Availability3d)
	assert.Nil(t, empty.Availability7d)
	assert.Nil(t, empty.AverageLatencyMs1d)
}

func TestChannelMonitorPublicAndSignedInQueriesFilterAndIsolateSensitiveFields(t *testing.T) {
	setupChannelMonitorSchedulerTestDB(t)
	location, err := time.LoadLocation("Asia/Shanghai")
	require.NoError(t, err)
	now := time.Date(2026, time.August, 16, 12, 30, 0, 0, location)
	setChannelMonitorQueryClock(t, now, location)

	published := channelMonitorQueryFixture("SENSITIVE_INTERNAL_NAME", "Published", true, true)
	published.Endpoint = "https://sensitive-endpoint.example.com"
	published.APIKeyEncrypted = "SENSITIVE_CIPHERTEXT"
	published.LeaseOwner = "SENSITIVE_LEASE_OWNER"
	unpublished := channelMonitorQueryFixture("Unpublished Internal", "Unpublished", true, false)
	disabled := channelMonitorQueryFixture("Disabled Internal", "Disabled", false, true)
	unknown := channelMonitorQueryFixture("Unknown Internal", "Unknown", true, true)
	for _, monitor := range []*model.ChannelMonitor{published, unpublished, disabled, unknown} {
		require.NoError(t, model.DB.Create(monitor).Error)
	}
	require.NoError(t, model.DB.Create(&model.ChannelMonitorCheck{
		MonitorID:     published.ID,
		Model:         published.PrimaryModel,
		Status:        model.ChannelMonitorStatusFailed,
		ErrorCategory: model.ChannelMonitorErrorAuth,
		HTTPStatus:    http.StatusUnauthorized,
		ErrorMessage:  "SENSITIVE_RAW_ERROR",
		CheckedAt:     now.Add(-time.Minute).Unix(),
	}).Error)

	publicSummary, err := GetChannelMonitorPublicSummary()
	require.NoError(t, err)
	assert.Equal(t, model.ChannelMonitorStatusFailed, publicSummary.OverallStatus)
	assert.Equal(t, 1, publicSummary.Counts.Failed)
	assert.Equal(t, 1, publicSummary.Counts.Unknown)
	require.Len(t, publicSummary.Monitors, 2)
	assert.Equal(t, "Published", publicSummary.Monitors[0].PublicName)
	assert.Equal(t, model.ChannelMonitorStatusFailed, publicSummary.Monitors[0].Status)
	assert.Equal(t, model.ChannelMonitorErrorAuth, publicSummary.Monitors[0].ErrorCategory)
	assert.Equal(t, "Unknown", publicSummary.Monitors[1].PublicName)
	assert.Equal(t, model.ChannelMonitorStatusUnknown, publicSummary.Monitors[1].Status)

	signedIn, err := GetPublishedChannelMonitorStatus(1)
	require.NoError(t, err)
	require.Len(t, signedIn.Items, 2)
	assert.Equal(t, model.ChannelMonitorStatusFailed, signedIn.OverallStatus)
	assert.Equal(t, model.ChannelMonitorStatusFailed, signedIn.Items[0].Status)
	assert.Equal(t, model.ChannelMonitorStatusUnknown, signedIn.Items[1].Status)

	publicJSON, err := common.Marshal(publicSummary)
	require.NoError(t, err)
	signedInJSON, err := common.Marshal(signedIn)
	require.NoError(t, err)
	for _, payload := range []string{string(publicJSON), string(signedInJSON)} {
		assert.NotContains(t, payload, "SENSITIVE_INTERNAL_NAME")
		assert.NotContains(t, payload, "sensitive-endpoint")
		assert.NotContains(t, payload, "SENSITIVE_CIPHERTEXT")
		assert.NotContains(t, payload, "SENSITIVE_LEASE_OWNER")
		assert.NotContains(t, payload, "SENSITIVE_RAW_ERROR")
		assert.NotContains(t, payload, "http_status")
		assert.NotContains(t, payload, "error_message")
		assert.NotContains(t, payload, "internal_name")
		assert.NotContains(t, payload, "endpoint")
		assert.NotContains(t, payload, "lease")
	}
}

func TestInvalidateChannelMonitorPublicSummaryCacheRefreshesLatestResult(t *testing.T) {
	setupChannelMonitorSchedulerTestDB(t)
	location, err := time.LoadLocation("Asia/Shanghai")
	require.NoError(t, err)
	now := time.Date(2026, time.August, 16, 12, 30, 0, 0, location)
	setChannelMonitorQueryClock(t, now, location)

	monitor := channelMonitorQueryFixture("Internal", "Public", true, true)
	require.NoError(t, model.DB.Create(monitor).Error)
	require.NoError(t, model.DB.Create(&model.ChannelMonitorCheck{
		MonitorID: monitor.ID, Model: monitor.PrimaryModel, Status: model.ChannelMonitorStatusOperational, LatencyMs: 100, CheckedAt: now.Add(-2 * time.Minute).Unix(),
	}).Error)

	first, err := GetChannelMonitorPublicSummary()
	require.NoError(t, err)
	assert.Equal(t, model.ChannelMonitorStatusOperational, first.Monitors[0].Status)

	require.NoError(t, model.DB.Create(&model.ChannelMonitorCheck{
		MonitorID: monitor.ID, Model: monitor.PrimaryModel, Status: model.ChannelMonitorStatusFailed, ErrorCategory: model.ChannelMonitorErrorUpstream, CheckedAt: now.Add(-time.Minute).Unix(),
	}).Error)
	cached, err := GetChannelMonitorPublicSummary()
	require.NoError(t, err)
	assert.Equal(t, model.ChannelMonitorStatusOperational, cached.Monitors[0].Status)

	InvalidateChannelMonitorPublicSummaryCache()
	refreshed, err := GetChannelMonitorPublicSummary()
	require.NoError(t, err)
	assert.Equal(t, model.ChannelMonitorStatusFailed, refreshed.Monitors[0].Status)
}

func channelMonitorQueryFixture(internalName string, publicName string, enabled bool, publicEnabled bool) *model.ChannelMonitor {
	monitor := channelMonitorSchedulerFixture("https://example.com", "ciphertext", "text")
	monitor.InternalName = internalName
	monitor.PublicName = publicName
	monitor.Enabled = enabled
	monitor.PublicEnabled = publicEnabled
	monitor.NextCheckAt = 0
	return monitor
}

func setChannelMonitorQueryClock(t *testing.T, now time.Time, location *time.Location) {
	t.Helper()
	previousNow := channelMonitorQueryNow
	previousLocation := channelMonitorQueryLocation
	channelMonitorQueryNow = func() time.Time { return now }
	channelMonitorQueryLocation = location
	InvalidateChannelMonitorPublicSummaryCache()
	t.Cleanup(func() {
		channelMonitorQueryNow = previousNow
		channelMonitorQueryLocation = previousLocation
		InvalidateChannelMonitorPublicSummaryCache()
	})
}

func channelMonitorQueryTestDayStart(value time.Time, location *time.Location) time.Time {
	local := value.In(location)
	return time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, location)
}
