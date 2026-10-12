package model

import "gorm.io/gorm"

// ChannelCostBucketSeconds is the width of one cost statistics bucket. Buckets
// start on whole UTC hours, so any whole-hour time zone can sum them into days.
const ChannelCostBucketSeconds int64 = 3600

// ChannelCostHourly is the net quota (consumed minus refunded) one channel
// handled in one hour. It is rebuilt from the logs so the cost statistics page
// never scans raw logs.
type ChannelCostHourly struct {
	Id          int   `json:"id"`
	BucketStart int64 `json:"bucket_start" gorm:"uniqueIndex:idx_channel_cost_bucket,priority:1"`
	ChannelId   int   `json:"channel_id" gorm:"uniqueIndex:idx_channel_cost_bucket,priority:2"`
	Quota       int64 `json:"quota"`
}

// ChannelCostTotals is a channel's net quota in the three ranges of the cost
// statistics page.
type ChannelCostTotals struct {
	Today     int64
	Yesterday int64
	Week      int64
}

// RebuildChannelCostHour recomputes the bucket that starts at hourStart from
// the consume and refund logs, replacing what was stored for that hour.
func RebuildChannelCostHour(hourStart int64) error {
	var rows []struct {
		ChannelId int
		Quota     int64
	}
	err := LOG_DB.Model(&Log{}).
		Select("channel_id, SUM(CASE WHEN type = ? THEN quota ELSE -quota END) AS quota", LogTypeConsume).
		Where("created_at >= ? AND created_at < ?", hourStart, hourStart+ChannelCostBucketSeconds).
		Where("type IN ?", []int{LogTypeConsume, LogTypeRefund}).
		Group("channel_id").
		Scan(&rows).Error
	if err != nil {
		return err
	}
	buckets := make([]ChannelCostHourly, 0, len(rows))
	for _, row := range rows {
		if row.ChannelId == 0 || row.Quota == 0 {
			continue
		}
		buckets = append(buckets, ChannelCostHourly{BucketStart: hourStart, ChannelId: row.ChannelId, Quota: row.Quota})
	}
	return DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("bucket_start = ?", hourStart).Delete(&ChannelCostHourly{}).Error; err != nil {
			return err
		}
		if len(buckets) == 0 {
			return nil
		}
		return tx.Create(&buckets).Error
	})
}

// DeleteChannelCostHoursBefore drops buckets that start before cutoff.
func DeleteChannelCostHoursBefore(cutoff int64) error {
	return DB.Where("bucket_start < ?", cutoff).Delete(&ChannelCostHourly{}).Error
}

// GetChannelCostTotals sums the buckets into today (from todayStart), yesterday
// (yesterdayStart to todayStart) and the week (from weekStart), per channel.
func GetChannelCostTotals(weekStart, yesterdayStart, todayStart int64) (map[int]*ChannelCostTotals, error) {
	var buckets []ChannelCostHourly
	if err := DB.Where("bucket_start >= ?", weekStart).Find(&buckets).Error; err != nil {
		return nil, err
	}
	totals := make(map[int]*ChannelCostTotals)
	for _, bucket := range buckets {
		total := totals[bucket.ChannelId]
		if total == nil {
			total = &ChannelCostTotals{}
			totals[bucket.ChannelId] = total
		}
		total.Week += bucket.Quota
		if bucket.BucketStart >= todayStart {
			total.Today += bucket.Quota
		} else if bucket.BucketStart >= yesterdayStart {
			total.Yesterday += bucket.Quota
		}
	}
	return totals, nil
}
