package model

import (
	"errors"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

var (
	ErrChannelStatusMonitorNotFound = common.NewMessage("Channel monitor not found")
	ErrChannelStatusGroupTaken      = common.NewMessage("This group already has a monitor")
)

// ChannelStatusMonitor shows one group on the channel status page. Its status
// comes from the group's call logs; nothing is probed.
type ChannelStatusMonitor struct {
	Id         int    `json:"id"`
	Group      string `json:"group" gorm:"column:group_name;type:varchar(64);not null;uniqueIndex"`
	PublicName string `json:"public_name" gorm:"type:varchar(128);not null"`
	NameColor  string `json:"name_color" gorm:"type:varchar(7)"`
	// PublicEnabled shows the monitor to every user; otherwise only staff see it.
	PublicEnabled bool  `json:"public_enabled"`
	CreatedBy     int   `json:"created_by"`
	CreatedAt     int64 `json:"created_at"`
	UpdatedAt     int64 `json:"updated_at"`
}

// ListChannelStatusMonitors returns the monitors in creation order, only the
// public ones unless includePrivate is set.
func ListChannelStatusMonitors(includePrivate bool) ([]ChannelStatusMonitor, error) {
	query := DB.Order("id ASC")
	if !includePrivate {
		query = query.Where("public_enabled = ?", true)
	}
	monitors := make([]ChannelStatusMonitor, 0)
	err := query.Find(&monitors).Error
	return monitors, err
}

// GetChannelStatusMonitor returns a monitor, or ErrChannelStatusMonitorNotFound
// when it does not exist or is private and includePrivate is not set.
func GetChannelStatusMonitor(id int, includePrivate bool) (*ChannelStatusMonitor, error) {
	var monitor ChannelStatusMonitor
	err := DB.Where("id = ?", id).First(&monitor).Error
	if errors.Is(err, gorm.ErrRecordNotFound) || (err == nil && !monitor.PublicEnabled && !includePrivate) {
		return nil, ErrChannelStatusMonitorNotFound
	}
	if err != nil {
		return nil, err
	}
	return &monitor, nil
}

// SaveChannelStatusMonitor creates the monitor, or updates it when it has an
// id. A group can have only one monitor.
func SaveChannelStatusMonitor(monitor *ChannelStatusMonitor) error {
	var taken int64
	if err := DB.Model(&ChannelStatusMonitor{}).Where("group_name = ? AND id <> ?", monitor.Group, monitor.Id).Count(&taken).Error; err != nil {
		return err
	}
	if taken > 0 {
		return ErrChannelStatusGroupTaken
	}
	monitor.UpdatedAt = common.GetTimestamp()
	if monitor.Id == 0 {
		monitor.CreatedAt = monitor.UpdatedAt
		return DB.Create(monitor).Error
	}
	result := DB.Model(&ChannelStatusMonitor{}).Where("id = ?", monitor.Id).
		Select("group_name", "public_name", "name_color", "public_enabled", "updated_at").
		Updates(monitor)
	if result.Error == nil && result.RowsAffected == 0 {
		return ErrChannelStatusMonitorNotFound
	}
	return result.Error
}

func DeleteChannelStatusMonitor(id int) error {
	result := DB.Where("id = ?", id).Delete(&ChannelStatusMonitor{})
	if result.Error == nil && result.RowsAffected == 0 {
		return ErrChannelStatusMonitorNotFound
	}
	return result.Error
}

// ChannelStatusSuccessRow counts the successful calls of one group and model.
type ChannelStatusSuccessRow struct {
	GroupName string
	ModelName string
	Count     int64
	// UseTime is the summed duration in seconds.
	UseTime int64
}

// CountChannelStatusSuccesses counts the consume logs of groups created in
// [start, end), per group and model.
func CountChannelStatusSuccesses(groups []string, start, end int64) ([]ChannelStatusSuccessRow, error) {
	rows := make([]ChannelStatusSuccessRow, 0)
	err := LOG_DB.Model(&Log{}).
		Select(logGroupCol+" AS group_name, model_name, COUNT(*) AS count, COALESCE(SUM(use_time), 0) AS use_time").
		Where("created_at >= ? AND created_at < ? AND type = ?", start, end, LogTypeConsume).
		Where(logGroupCol+" IN ?", groups).
		Group(logGroupCol + ", model_name").
		Scan(&rows).Error
	return rows, err
}

// ChannelStatusErrorRow is one failed upstream attempt.
type ChannelStatusErrorRow struct {
	Id        int
	RequestId string
	GroupName string
	ModelName string
	CreatedAt int64
	Other     string
}

// ListChannelStatusErrors returns the error logs of groups created in
// [start, end).
func ListChannelStatusErrors(groups []string, start, end int64) ([]ChannelStatusErrorRow, error) {
	rows := make([]ChannelStatusErrorRow, 0)
	err := LOG_DB.Model(&Log{}).
		Select("id, request_id, "+logGroupCol+" AS group_name, model_name, created_at, other").
		Where("created_at >= ? AND created_at < ? AND type = ?", start, end, LogTypeError).
		Where(logGroupCol+" IN ?", groups).
		Scan(&rows).Error
	return rows, err
}

// SucceededRequestIds returns which of requestIds have a consume log created
// at or after since, so a retried request that succeeded is not a failure.
func SucceededRequestIds(requestIds []string, since int64) (map[string]bool, error) {
	succeeded := make(map[string]bool)
	const batchSize = 500
	for start := 0; start < len(requestIds); start += batchSize {
		batch := requestIds[start:min(start+batchSize, len(requestIds))]
		var found []string
		err := LOG_DB.Model(&Log{}).
			Where("type = ? AND created_at >= ? AND request_id IN ?", LogTypeConsume, since, batch).
			Distinct().Pluck("request_id", &found).Error
		if err != nil {
			return nil, err
		}
		for _, id := range found {
			succeeded[id] = true
		}
	}
	return succeeded, nil
}
