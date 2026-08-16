package model

import (
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestChannelMonitorModelsMigrateAndEnforceRollupKeys(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+uuid.NewString()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&ChannelMonitor{},
		&ChannelMonitorCheck{},
		&ChannelMonitorHourlyRollup{},
		&ChannelMonitorDailyRollup{},
	))

	monitor := ChannelMonitor{
		InternalName:        "primary upstream",
		PublicName:          "Primary API",
		Provider:            ChannelMonitorProviderOpenAI,
		APIMode:             ChannelMonitorAPIModeChatCompletions,
		Endpoint:            "https://example.com",
		APIKeyEncrypted:     "ciphertext",
		APIKeySuffix:        "cret",
		PrimaryModel:        "gpt-test",
		ExtraModels:         `["gpt-test-mini"]`,
		IntervalSeconds:     60,
		DegradedThresholdMs: 6000,
		TimeoutSeconds:      30,
		Enabled:             true,
		NextCheckAt:         1,
	}
	require.NoError(t, db.Create(&monitor).Error)
	assert.Positive(t, monitor.ID)
	assert.Positive(t, monitor.CreatedAt)
	assert.Positive(t, monitor.UpdatedAt)

	require.NoError(t, db.Create(&ChannelMonitorCheck{
		MonitorID: monitor.ID,
		Model:     monitor.PrimaryModel,
		Status:    ChannelMonitorStatusOperational,
		LatencyMs: 120,
		CheckedAt: 100,
	}).Error)

	hourly := ChannelMonitorHourlyRollup{
		MonitorID:          monitor.ID,
		Model:              monitor.PrimaryModel,
		BucketStart:        3600,
		TotalCount:         1,
		OperationalCount:   1,
		LatencySumMs:       120,
		LatencySampleCount: 1,
		ComputedAt:         7200,
	}
	require.NoError(t, db.Create(&hourly).Error)
	err = db.Create(&ChannelMonitorHourlyRollup{
		MonitorID:   monitor.ID,
		Model:       monitor.PrimaryModel,
		BucketStart: hourly.BucketStart,
		TotalCount:  1,
	}).Error
	require.Error(t, err)

	daily := ChannelMonitorDailyRollup{
		MonitorID:          monitor.ID,
		Model:              monitor.PrimaryModel,
		BucketStart:        86400,
		TotalCount:         1,
		DegradedCount:      1,
		LatencySumMs:       7000,
		LatencySampleCount: 1,
		ComputedAt:         172800,
	}
	require.NoError(t, db.Create(&daily).Error)
	err = db.Create(&ChannelMonitorDailyRollup{
		MonitorID:   monitor.ID,
		Model:       monitor.PrimaryModel,
		BucketStart: daily.BucketStart,
		TotalCount:  1,
	}).Error
	require.Error(t, err)
}
