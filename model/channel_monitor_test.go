package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
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

func TestChannelMonitorCRUDListsFiltersAndDeletesHistory(t *testing.T) {
	previousDB := DB
	previousType := common.MainDatabaseType()
	db, err := gorm.Open(sqlite.Open("file:"+uuid.NewString()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	DB = db
	common.SetMainDatabaseType(common.DatabaseTypeSQLite)
	t.Cleanup(func() {
		DB = previousDB
		common.SetMainDatabaseType(previousType)
	})
	require.NoError(t, db.AutoMigrate(
		&ChannelMonitor{},
		&ChannelMonitorCheck{},
		&ChannelMonitorHourlyRollup{},
		&ChannelMonitorDailyRollup{},
	))

	monitors := []*ChannelMonitor{
		channelMonitorFixture("Alpha Internal", "Public Alpha", ChannelMonitorProviderOpenAI, "gpt-alpha", true, true),
		channelMonitorFixture("Beta Internal", "Public Beta", ChannelMonitorProviderAnthropic, "claude-beta", true, false),
		channelMonitorFixture("Gamma Internal", "Public Gamma", ChannelMonitorProviderGemini, "gemini-gamma", false, true),
	}
	monitors[0].NameColor = "#1A2B3C"
	for _, monitor := range monitors {
		require.NoError(t, CreateChannelMonitor(monitor))
	}

	enabled := true
	public := true
	items, total, err := ListChannelMonitors(ChannelMonitorListFilter{
		Page:          1,
		PageSize:      10,
		Search:        "alpha",
		Provider:      ChannelMonitorProviderOpenAI,
		Enabled:       &enabled,
		PublicEnabled: &public,
	})
	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
	require.Len(t, items, 1)
	assert.Equal(t, monitors[0].ID, items[0].ID)

	items, total, err = ListChannelMonitors(ChannelMonitorListFilter{Page: 2, PageSize: 1})
	require.NoError(t, err)
	assert.Equal(t, int64(3), total)
	require.Len(t, items, 1)
	assert.Equal(t, monitors[1].ID, items[0].ID)

	stored, err := GetChannelMonitor(monitors[0].ID)
	require.NoError(t, err)
	require.NotNil(t, stored)
	assert.Equal(t, "Public Alpha", stored.PublicName)
	assert.Equal(t, "#1A2B3C", stored.NameColor)

	stored.PublicName = "Updated Alpha"
	stored.NameColor = "#ABCDEF"
	require.NoError(t, UpdateChannelMonitor(stored, false))
	updated, err := GetChannelMonitor(stored.ID)
	require.NoError(t, err)
	require.NotNil(t, updated)
	assert.Equal(t, "Updated Alpha", updated.PublicName)
	assert.Equal(t, "#ABCDEF", updated.NameColor)

	require.NoError(t, db.Create(&ChannelMonitorCheck{MonitorID: stored.ID, Model: stored.PrimaryModel, Status: ChannelMonitorStatusOperational, CheckedAt: 100}).Error)
	require.NoError(t, db.Create(&ChannelMonitorHourlyRollup{MonitorID: stored.ID, Model: stored.PrimaryModel, BucketStart: 3600, TotalCount: 1}).Error)
	require.NoError(t, db.Create(&ChannelMonitorDailyRollup{MonitorID: stored.ID, Model: stored.PrimaryModel, BucketStart: 86400, TotalCount: 1}).Error)

	require.NoError(t, DeleteChannelMonitor(stored.ID))
	deleted, err := GetChannelMonitor(stored.ID)
	require.NoError(t, err)
	assert.Nil(t, deleted)

	for _, value := range []any{&ChannelMonitorCheck{}, &ChannelMonitorHourlyRollup{}, &ChannelMonitorDailyRollup{}} {
		var count int64
		require.NoError(t, db.Model(value).Where("monitor_id = ?", stored.ID).Count(&count).Error)
		assert.Zero(t, count)
	}
}

func TestListChannelMonitorsReturnsEmptySlice(t *testing.T) {
	previousDB := DB
	db, err := gorm.Open(sqlite.Open("file:"+uuid.NewString()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	DB = db
	t.Cleanup(func() { DB = previousDB })
	require.NoError(t, db.AutoMigrate(&ChannelMonitor{}))

	items, total, err := ListChannelMonitors(ChannelMonitorListFilter{Page: 1, PageSize: 10})
	require.NoError(t, err)
	assert.NotNil(t, items)
	assert.Empty(t, items)
	assert.Zero(t, total)
}

func TestUpdateChannelMonitorPreservesRuntimeStateChangedAfterRead(t *testing.T) {
	previousDB := DB
	previousType := common.MainDatabaseType()
	db, err := gorm.Open(sqlite.Open("file:"+uuid.NewString()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	DB = db
	common.SetMainDatabaseType(common.DatabaseTypeSQLite)
	t.Cleanup(func() {
		DB = previousDB
		common.SetMainDatabaseType(previousType)
	})
	require.NoError(t, db.AutoMigrate(&ChannelMonitor{}))

	monitor := channelMonitorFixture("Concurrent Internal", "Concurrent Public", ChannelMonitorProviderOpenAI, "gpt-concurrent", true, true)
	monitor.LastCheckedAt = 100
	monitor.NextCheckAt = 200
	require.NoError(t, CreateChannelMonitor(monitor))

	stale, err := GetChannelMonitor(monitor.ID)
	require.NoError(t, err)
	require.NotNil(t, stale)
	require.NoError(t, db.Model(&ChannelMonitor{}).Where("id = ?", monitor.ID).Updates(map[string]any{
		"last_checked_at": 300,
		"next_check_at":   400,
		"lease_owner":     "active-worker",
		"lease_until":     500,
	}).Error)

	stale.PublicName = "Updated Concurrent Public"
	require.NoError(t, UpdateChannelMonitor(stale, false))

	updated, err := GetChannelMonitor(monitor.ID)
	require.NoError(t, err)
	require.NotNil(t, updated)
	assert.Equal(t, "Updated Concurrent Public", updated.PublicName)
	assert.Equal(t, int64(300), updated.LastCheckedAt)
	assert.Equal(t, int64(400), updated.NextCheckAt)
	assert.Equal(t, "active-worker", updated.LeaseOwner)
	assert.Equal(t, int64(500), updated.LeaseUntil)

	updated.NextCheckAt = 600
	require.NoError(t, UpdateChannelMonitor(updated, true))
	scheduled, err := GetChannelMonitor(monitor.ID)
	require.NoError(t, err)
	require.NotNil(t, scheduled)
	assert.Equal(t, int64(600), scheduled.NextCheckAt)
	assert.Equal(t, "active-worker", scheduled.LeaseOwner)
	assert.Equal(t, int64(500), scheduled.LeaseUntil)
}

func channelMonitorFixture(internalName string, publicName string, provider string, primaryModel string, enabled bool, publicEnabled bool) *ChannelMonitor {
	return &ChannelMonitor{
		InternalName:        internalName,
		PublicName:          publicName,
		Provider:            provider,
		APIMode:             ChannelMonitorAPIModeChatCompletions,
		Endpoint:            "https://example.com",
		APIKeyEncrypted:     "ciphertext",
		APIKeySuffix:        "text",
		PrimaryModel:        primaryModel,
		ExtraModels:         "[]",
		IntervalSeconds:     60,
		DegradedThresholdMs: 6000,
		TimeoutSeconds:      30,
		Enabled:             enabled,
		PublicEnabled:       publicEnabled,
		NextCheckAt:         100,
	}
}
