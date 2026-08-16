package model

import (
	"errors"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const channelMonitorHistoryDefaultBatchSize = 500

type channelMonitorRollupKey struct {
	MonitorID   int
	Model       string
	BucketStart int64
}

func AggregateChannelMonitorHistory(now time.Time, location *time.Location, batchSize int) error {
	if now.IsZero() {
		return errors.New("channel monitor aggregation time is required")
	}
	if location == nil {
		location = time.Local
	}
	if batchSize <= 0 {
		batchSize = channelMonitorHistoryDefaultBatchSize
	}
	if batchSize > 5000 {
		batchSize = 5000
	}
	for {
		processed, err := aggregateChannelMonitorChecks(now, location, batchSize)
		if err != nil {
			return err
		}
		if processed < batchSize {
			break
		}
	}
	for {
		processed, err := aggregateChannelMonitorHours(now, location, batchSize)
		if err != nil {
			return err
		}
		if processed < batchSize {
			break
		}
	}
	for {
		processed, err := deleteExpiredChannelMonitorDays(now, location, batchSize)
		if err != nil {
			return err
		}
		if processed < batchSize {
			return nil
		}
	}
}

func aggregateChannelMonitorChecks(now time.Time, location *time.Location, batchSize int) (int, error) {
	cutoff := now.Add(-time.Hour).Unix()
	processed := 0
	err := DB.Transaction(func(tx *gorm.DB) error {
		rows := make([]ChannelMonitorCheck, 0)
		if err := tx.Where("checked_at < ?", cutoff).
			Order("checked_at ASC, id ASC").
			Limit(batchSize).
			Find(&rows).Error; err != nil {
			return err
		}
		if len(rows) == 0 {
			return nil
		}
		processed = len(rows)

		rollups := make(map[channelMonitorRollupKey]*ChannelMonitorHourlyRollup)
		ids := make([]int64, 0, len(rows))
		for _, row := range rows {
			ids = append(ids, row.ID)
			key := channelMonitorRollupKey{
				MonitorID:   row.MonitorID,
				Model:       row.Model,
				BucketStart: channelMonitorNaturalHourStart(time.Unix(row.CheckedAt, 0), location).Unix(),
			}
			rollup := rollups[key]
			if rollup == nil {
				rollup = &ChannelMonitorHourlyRollup{
					MonitorID:   key.MonitorID,
					Model:       key.Model,
					BucketStart: key.BucketStart,
					ComputedAt:  now.Unix(),
				}
				rollups[key] = rollup
			}
			rollup.TotalCount++
			switch row.Status {
			case ChannelMonitorStatusOperational:
				rollup.OperationalCount++
				rollup.LatencySumMs += row.LatencyMs
				rollup.LatencySampleCount++
			case ChannelMonitorStatusDegraded:
				rollup.DegradedCount++
				rollup.LatencySumMs += row.LatencyMs
				rollup.LatencySampleCount++
			default:
				rollup.FailedCount++
			}
		}

		for _, rollup := range rollups {
			if err := upsertChannelMonitorHourlyRollup(tx, rollup); err != nil {
				return err
			}
		}
		return tx.Where("id IN ?", ids).Delete(&ChannelMonitorCheck{}).Error
	})
	return processed, err
}

func aggregateChannelMonitorHours(now time.Time, location *time.Location, batchSize int) (int, error) {
	cutoff := now.Add(-24 * time.Hour).Unix()
	processed := 0
	err := DB.Transaction(func(tx *gorm.DB) error {
		rows := make([]ChannelMonitorHourlyRollup, 0)
		if err := tx.Where("bucket_start < ?", cutoff).
			Order("bucket_start ASC, id ASC").
			Limit(batchSize).
			Find(&rows).Error; err != nil {
			return err
		}
		if len(rows) == 0 {
			return nil
		}
		processed = len(rows)

		rollups := make(map[channelMonitorRollupKey]*ChannelMonitorDailyRollup)
		ids := make([]int64, 0, len(rows))
		for _, row := range rows {
			ids = append(ids, row.ID)
			key := channelMonitorRollupKey{
				MonitorID:   row.MonitorID,
				Model:       row.Model,
				BucketStart: channelMonitorNaturalDayStart(time.Unix(row.BucketStart, 0), location).Unix(),
			}
			rollup := rollups[key]
			if rollup == nil {
				rollup = &ChannelMonitorDailyRollup{
					MonitorID:   key.MonitorID,
					Model:       key.Model,
					BucketStart: key.BucketStart,
					ComputedAt:  now.Unix(),
				}
				rollups[key] = rollup
			}
			rollup.TotalCount += row.TotalCount
			rollup.OperationalCount += row.OperationalCount
			rollup.DegradedCount += row.DegradedCount
			rollup.FailedCount += row.FailedCount
			rollup.LatencySumMs += row.LatencySumMs
			rollup.LatencySampleCount += row.LatencySampleCount
		}

		for _, rollup := range rollups {
			if err := upsertChannelMonitorDailyRollup(tx, rollup); err != nil {
				return err
			}
		}
		return tx.Where("id IN ?", ids).Delete(&ChannelMonitorHourlyRollup{}).Error
	})
	return processed, err
}

func upsertChannelMonitorHourlyRollup(tx *gorm.DB, rollup *ChannelMonitorHourlyRollup) error {
	return tx.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "monitor_id"}, {Name: "model"}, {Name: "bucket_start"}},
		DoUpdates: clause.Assignments(map[string]any{
			"total_count":          gorm.Expr("channel_monitor_hourly_rollups.total_count + ?", rollup.TotalCount),
			"operational_count":    gorm.Expr("channel_monitor_hourly_rollups.operational_count + ?", rollup.OperationalCount),
			"degraded_count":       gorm.Expr("channel_monitor_hourly_rollups.degraded_count + ?", rollup.DegradedCount),
			"failed_count":         gorm.Expr("channel_monitor_hourly_rollups.failed_count + ?", rollup.FailedCount),
			"latency_sum_ms":       gorm.Expr("channel_monitor_hourly_rollups.latency_sum_ms + ?", rollup.LatencySumMs),
			"latency_sample_count": gorm.Expr("channel_monitor_hourly_rollups.latency_sample_count + ?", rollup.LatencySampleCount),
			"computed_at":          rollup.ComputedAt,
		}),
	}).Create(rollup).Error
}

func upsertChannelMonitorDailyRollup(tx *gorm.DB, rollup *ChannelMonitorDailyRollup) error {
	return tx.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "monitor_id"}, {Name: "model"}, {Name: "bucket_start"}},
		DoUpdates: clause.Assignments(map[string]any{
			"total_count":          gorm.Expr("channel_monitor_daily_rollups.total_count + ?", rollup.TotalCount),
			"operational_count":    gorm.Expr("channel_monitor_daily_rollups.operational_count + ?", rollup.OperationalCount),
			"degraded_count":       gorm.Expr("channel_monitor_daily_rollups.degraded_count + ?", rollup.DegradedCount),
			"failed_count":         gorm.Expr("channel_monitor_daily_rollups.failed_count + ?", rollup.FailedCount),
			"latency_sum_ms":       gorm.Expr("channel_monitor_daily_rollups.latency_sum_ms + ?", rollup.LatencySumMs),
			"latency_sample_count": gorm.Expr("channel_monitor_daily_rollups.latency_sample_count + ?", rollup.LatencySampleCount),
			"computed_at":          rollup.ComputedAt,
		}),
	}).Create(rollup).Error
}

func deleteExpiredChannelMonitorDays(now time.Time, location *time.Location, batchSize int) (int, error) {
	cutoff := channelMonitorNaturalDayStart(now, location).AddDate(0, 0, -6).Unix()
	processed := 0
	err := DB.Transaction(func(tx *gorm.DB) error {
		ids := make([]int64, 0, batchSize)
		if err := tx.Model(&ChannelMonitorDailyRollup{}).
			Where("bucket_start < ?", cutoff).
			Order("bucket_start ASC, id ASC").
			Limit(batchSize).
			Pluck("id", &ids).Error; err != nil {
			return err
		}
		if len(ids) == 0 {
			return nil
		}
		processed = len(ids)
		return tx.Where("id IN ?", ids).Delete(&ChannelMonitorDailyRollup{}).Error
	})
	return processed, err
}

func channelMonitorNaturalHourStart(value time.Time, location *time.Location) time.Time {
	local := value.In(location)
	return time.Date(local.Year(), local.Month(), local.Day(), local.Hour(), 0, 0, 0, location)
}

func channelMonitorNaturalDayStart(value time.Time, location *time.Location) time.Time {
	local := value.In(location)
	return time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, location)
}
