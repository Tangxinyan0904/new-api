package model

import (
	"database/sql"
	"strconv"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"gorm.io/gorm"
)

// ChannelCostBucketSeconds is the width of one cost statistics bucket. Buckets
// start on whole UTC hours, so any whole-hour time zone can sum them into days.
const ChannelCostBucketSeconds int64 = 3600

// ChannelCostHourly is the net usage (consumed minus refunded) one channel
// handled for one group in one hour. It is rebuilt from the logs so the cost
// statistics page never scans raw logs.
type ChannelCostHourly struct {
	Id          int    `json:"id"`
	BucketStart int64  `json:"bucket_start" gorm:"uniqueIndex:idx_channel_cost_bucket,priority:1"`
	Group       string `json:"group" gorm:"column:group_name;type:varchar(64);uniqueIndex:idx_channel_cost_bucket,priority:2"`
	ChannelId   int    `json:"channel_id" gorm:"uniqueIndex:idx_channel_cost_bucket,priority:3"`
	// Quota is what users were charged. BaseQuota is the same usage at the
	// original price, before the group ratio each request was charged at.
	Quota     int64   `json:"quota"`
	BaseQuota float64 `json:"base_quota"`
}

// ChannelCostKey identifies the usage of one channel within one group.
type ChannelCostKey struct {
	Group     string
	ChannelId int
}

// ChannelCostUsage is charged and original-price usage over a range.
type ChannelCostUsage struct {
	Quota     int64   `json:"quota"`
	BaseQuota float64 `json:"base_quota"`
}

func (usage *ChannelCostUsage) add(bucket ChannelCostHourly) {
	usage.Quota += bucket.Quota
	usage.BaseQuota += bucket.BaseQuota
}

// ChannelCostTotals is the usage in the three ranges of the cost statistics
// page.
type ChannelCostTotals struct {
	Today     ChannelCostUsage
	Yesterday ChannelCostUsage
	Week      ChannelCostUsage
}

// chargedGroupRatio returns the group ratio a consume or refund log was billed
// at, read from its other field: the user group special ratio when one
// applied, otherwise the group ratio. Some logs store the ratios as strings.
func chargedGroupRatio(other string) (float64, bool) {
	var ratios struct {
		GroupRatio     any `json:"group_ratio"`
		UserGroupRatio any `json:"user_group_ratio"`
	}
	if other == "" || common.UnmarshalJsonStr(other, &ratios) != nil {
		return 0, false
	}
	// A user group ratio of -1 means no special ratio applied.
	for _, value := range []any{ratios.UserGroupRatio, ratios.GroupRatio} {
		var ratio float64
		switch v := value.(type) {
		case float64:
			ratio = v
		case string:
			parsed, err := strconv.ParseFloat(v, 64)
			if err != nil {
				continue
			}
			ratio = parsed
		default:
			continue
		}
		if ratio > 0 {
			return ratio, true
		}
	}
	return 0, false
}

// RebuildChannelCostHour recomputes the buckets that start at hourStart from
// the consume and refund logs, replacing what was stored for that hour. Logs
// that do not record their group ratio fall back to the group's current one.
func RebuildChannelCostHour(hourStart int64) error {
	rows, err := LOG_DB.Model(&Log{}).
		Select("channel_id, "+logGroupCol+", type, quota, other").
		Where("created_at >= ? AND created_at < ?", hourStart, hourStart+ChannelCostBucketSeconds).
		Where("type IN ?", []int{LogTypeConsume, LogTypeRefund}).
		Where("channel_id <> 0").
		Rows()
	if err != nil {
		return err
	}
	defer rows.Close()

	groupRatios := ratio_setting.GetGroupRatioCopy()
	sums := make(map[ChannelCostKey]*ChannelCostHourly)
	for rows.Next() {
		var channelId, logType int
		var quota int64
		var group, other sql.NullString
		if err := rows.Scan(&channelId, &group, &logType, &quota, &other); err != nil {
			return err
		}
		ratio, ok := chargedGroupRatio(other.String)
		if !ok {
			// Billing charges groups without a configured ratio at 1.
			ratio = 1
			if configured, exists := groupRatios[group.String]; exists {
				ratio = configured
			}
		}
		baseQuota := 0.0
		if ratio > 0 {
			baseQuota = float64(quota) / ratio
		}
		if logType == LogTypeRefund {
			quota, baseQuota = -quota, -baseQuota
		}
		key := ChannelCostKey{Group: group.String, ChannelId: channelId}
		bucket := sums[key]
		if bucket == nil {
			bucket = &ChannelCostHourly{BucketStart: hourStart, Group: key.Group, ChannelId: channelId}
			sums[key] = bucket
		}
		bucket.Quota += quota
		bucket.BaseQuota += baseQuota
	}
	if err := rows.Err(); err != nil {
		return err
	}
	// Release the read before writing: SQLite may share one connection.
	rows.Close()

	buckets := make([]ChannelCostHourly, 0, len(sums))
	for _, bucket := range sums {
		if bucket.Quota != 0 || bucket.BaseQuota != 0 {
			buckets = append(buckets, *bucket)
		}
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
// (yesterdayStart to todayStart) and the week (from weekStart), per channel
// and group.
func GetChannelCostTotals(weekStart, yesterdayStart, todayStart int64) (map[ChannelCostKey]*ChannelCostTotals, error) {
	var buckets []ChannelCostHourly
	if err := DB.Where("bucket_start >= ?", weekStart).Find(&buckets).Error; err != nil {
		return nil, err
	}
	totals := make(map[ChannelCostKey]*ChannelCostTotals)
	for _, bucket := range buckets {
		key := ChannelCostKey{Group: bucket.Group, ChannelId: bucket.ChannelId}
		total := totals[key]
		if total == nil {
			total = &ChannelCostTotals{}
			totals[key] = total
		}
		total.Week.add(bucket)
		if bucket.BucketStart >= todayStart {
			total.Today.add(bucket)
		} else if bucket.BucketStart >= yesterdayStart {
			total.Yesterday.add(bucket)
		}
	}
	return totals, nil
}
