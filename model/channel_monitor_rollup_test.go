package model

import (
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestAggregateChannelMonitorHistoryRespectsRetentionBoundaries(t *testing.T) {
	setupChannelMonitorRollupTestDB(t)
	location, err := time.LoadLocation("Asia/Shanghai")
	require.NoError(t, err)
	now := time.Date(2026, time.August, 16, 12, 30, 0, 0, location)
	monitor := channelMonitorRollupFixture()
	require.NoError(t, DB.Create(monitor).Error)

	rawCutoff := now.Add(-time.Hour)
	rawRows := []ChannelMonitorCheck{
		{MonitorID: monitor.ID, Model: "model-a", Status: ChannelMonitorStatusOperational, LatencyMs: 100, CheckedAt: rawCutoff.Add(-20 * time.Minute).Unix()},
		{MonitorID: monitor.ID, Model: "model-a", Status: ChannelMonitorStatusDegraded, LatencyMs: 7000, CheckedAt: rawCutoff.Add(-10 * time.Minute).Unix()},
		{MonitorID: monitor.ID, Model: "model-a", Status: ChannelMonitorStatusFailed, ErrorCategory: ChannelMonitorErrorUpstream, CheckedAt: rawCutoff.Unix()},
	}
	require.NoError(t, DB.Create(&rawRows).Error)

	hourlyCutoff := now.Add(-24 * time.Hour)
	hourlyRows := []ChannelMonitorHourlyRollup{
		{
			MonitorID: monitor.ID, Model: "model-a", BucketStart: channelMonitorTestNaturalHourStart(hourlyCutoff.Add(-time.Hour), location).Unix(),
			TotalCount: 2, OperationalCount: 1, FailedCount: 1, LatencySumMs: 200, LatencySampleCount: 1, ComputedAt: now.Add(-time.Hour).Unix(),
		},
		{
			MonitorID: monitor.ID, Model: "model-a", BucketStart: hourlyCutoff.Unix(),
			TotalCount: 1, OperationalCount: 1, LatencySumMs: 150, LatencySampleCount: 1, ComputedAt: now.Unix(),
		},
	}
	require.NoError(t, DB.Create(&hourlyRows).Error)

	retainedDayCutoff := channelMonitorTestNaturalDayStart(now, location).AddDate(0, 0, -6)
	dailyRows := []ChannelMonitorDailyRollup{
		{MonitorID: monitor.ID, Model: "model-a", BucketStart: retainedDayCutoff.AddDate(0, 0, -1).Unix(), TotalCount: 3, FailedCount: 3, ComputedAt: now.Unix()},
		{MonitorID: monitor.ID, Model: "model-a", BucketStart: retainedDayCutoff.Unix(), TotalCount: 1, OperationalCount: 1, LatencySumMs: 90, LatencySampleCount: 1, ComputedAt: now.Unix()},
	}
	require.NoError(t, DB.Create(&dailyRows).Error)

	require.NoError(t, AggregateChannelMonitorHistory(now, location, 100))

	var remainingRaw []ChannelMonitorCheck
	require.NoError(t, DB.Order("checked_at ASC").Find(&remainingRaw).Error)
	require.Len(t, remainingRaw, 1)
	assert.Equal(t, rawCutoff.Unix(), remainingRaw[0].CheckedAt)

	var remainingHourly []ChannelMonitorHourlyRollup
	require.NoError(t, DB.Order("bucket_start ASC").Find(&remainingHourly).Error)
	require.Len(t, remainingHourly, 2)
	assert.Equal(t, channelMonitorTestNaturalHourStart(time.Unix(rawRows[0].CheckedAt, 0), location).Unix(), remainingHourly[1].BucketStart)
	assert.Equal(t, int64(2), remainingHourly[1].TotalCount)
	assert.Equal(t, int64(1), remainingHourly[1].OperationalCount)
	assert.Equal(t, int64(1), remainingHourly[1].DegradedCount)
	assert.Equal(t, int64(7100), remainingHourly[1].LatencySumMs)
	assert.Equal(t, int64(2), remainingHourly[1].LatencySampleCount)

	var remainingDaily []ChannelMonitorDailyRollup
	require.NoError(t, DB.Order("bucket_start ASC").Find(&remainingDaily).Error)
	require.Len(t, remainingDaily, 2)
	assert.Equal(t, retainedDayCutoff.Unix(), remainingDaily[0].BucketStart)
	rolledDay := channelMonitorTestNaturalDayStart(hourlyCutoff.Add(-time.Hour), location).Unix()
	assert.Equal(t, rolledDay, remainingDaily[1].BucketStart)
	assert.Equal(t, int64(2), remainingDaily[1].TotalCount)
	assert.Equal(t, int64(1), remainingDaily[1].OperationalCount)
	assert.Equal(t, int64(1), remainingDaily[1].FailedCount)

	require.NoError(t, AggregateChannelMonitorHistory(now, location, 100))
	var totalHourly int64
	var totalDaily int64
	require.NoError(t, DB.Model(&ChannelMonitorHourlyRollup{}).Select("COALESCE(SUM(total_count), 0)").Scan(&totalHourly).Error)
	require.NoError(t, DB.Model(&ChannelMonitorDailyRollup{}).Select("COALESCE(SUM(total_count), 0)").Scan(&totalDaily).Error)
	assert.Equal(t, int64(3), totalHourly)
	assert.Equal(t, int64(3), totalDaily)
}

func TestAggregateChannelMonitorHistoryAddsIncrementalRowsOnce(t *testing.T) {
	setupChannelMonitorRollupTestDB(t)
	location, err := time.LoadLocation("Asia/Shanghai")
	require.NoError(t, err)
	now := time.Date(2026, time.August, 16, 12, 30, 0, 0, location)
	monitor := channelMonitorRollupFixture()
	require.NoError(t, DB.Create(monitor).Error)
	checkedAt := now.Add(-2 * time.Hour)
	rows := []ChannelMonitorCheck{
		{MonitorID: monitor.ID, Model: "model-a", Status: ChannelMonitorStatusOperational, LatencyMs: 100, CheckedAt: checkedAt.Unix()},
		{MonitorID: monitor.ID, Model: "model-a", Status: ChannelMonitorStatusFailed, CheckedAt: checkedAt.Add(time.Minute).Unix()},
	}
	require.NoError(t, DB.Create(&rows).Error)

	require.NoError(t, AggregateChannelMonitorHistory(now, location, 1))
	require.NoError(t, AggregateChannelMonitorHistory(now, location, 1))
	require.NoError(t, AggregateChannelMonitorHistory(now, location, 1))

	var rollup ChannelMonitorHourlyRollup
	require.NoError(t, DB.First(&rollup, "monitor_id = ? AND model = ?", monitor.ID, "model-a").Error)
	assert.Equal(t, int64(2), rollup.TotalCount)
	assert.Equal(t, int64(1), rollup.OperationalCount)
	assert.Equal(t, int64(1), rollup.FailedCount)
	assert.Equal(t, int64(100), rollup.LatencySumMs)
	assert.Equal(t, int64(1), rollup.LatencySampleCount)

	var rawCount int64
	require.NoError(t, DB.Model(&ChannelMonitorCheck{}).Count(&rawCount).Error)
	assert.Zero(t, rawCount)
}

func TestAggregateChannelMonitorHistoryDeletesExpiredDailyRowsInBoundedBatches(t *testing.T) {
	setupChannelMonitorRollupTestDB(t)
	location, err := time.LoadLocation("Asia/Shanghai")
	require.NoError(t, err)
	now := time.Date(2026, time.August, 16, 12, 30, 0, 0, location)
	monitor := channelMonitorRollupFixture()
	require.NoError(t, DB.Create(monitor).Error)
	cutoff := channelMonitorTestNaturalDayStart(now, location).AddDate(0, 0, -6)
	require.NoError(t, DB.Create(&[]ChannelMonitorDailyRollup{
		{MonitorID: monitor.ID, Model: "model-a", BucketStart: cutoff.AddDate(0, 0, -2).Unix(), TotalCount: 1, FailedCount: 1, ComputedAt: now.Unix()},
		{MonitorID: monitor.ID, Model: "model-a", BucketStart: cutoff.AddDate(0, 0, -1).Unix(), TotalCount: 1, FailedCount: 1, ComputedAt: now.Unix()},
	}).Error)

	require.NoError(t, AggregateChannelMonitorHistory(now, location, 1))
	var remaining int64
	require.NoError(t, DB.Model(&ChannelMonitorDailyRollup{}).Count(&remaining).Error)
	assert.Equal(t, int64(1), remaining)

	require.NoError(t, AggregateChannelMonitorHistory(now, location, 1))
	require.NoError(t, DB.Model(&ChannelMonitorDailyRollup{}).Count(&remaining).Error)
	assert.Zero(t, remaining)
}

func setupChannelMonitorRollupTestDB(t *testing.T) {
	t.Helper()
	previousDB := DB
	previousType := common.MainDatabaseType()
	db, err := gorm.Open(sqlite.Open("file:"+uuid.NewString()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	DB = db
	common.SetMainDatabaseType(common.DatabaseTypeSQLite)
	require.NoError(t, db.AutoMigrate(
		&ChannelMonitor{},
		&ChannelMonitorCheck{},
		&ChannelMonitorHourlyRollup{},
		&ChannelMonitorDailyRollup{},
	))
	t.Cleanup(func() {
		DB = previousDB
		common.SetMainDatabaseType(previousType)
	})
}

func channelMonitorRollupFixture() *ChannelMonitor {
	monitor := channelMonitorLeaseFixture()
	monitor.NextCheckAt = 0
	return monitor
}

func channelMonitorTestNaturalHourStart(value time.Time, location *time.Location) time.Time {
	local := value.In(location)
	return time.Date(local.Year(), local.Month(), local.Day(), local.Hour(), 0, 0, 0, location)
}

func channelMonitorTestNaturalDayStart(value time.Time, location *time.Location) time.Time {
	local := value.In(location)
	return time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, location)
}
