package model

import (
	"errors"
	"strings"
	"time"

	"gorm.io/gorm"
)

var ErrChannelMonitorLeaseLost = errors.New("channel monitor lease lost")

const (
	ChannelMonitorProviderOpenAI    = "openai"
	ChannelMonitorProviderAnthropic = "anthropic"
	ChannelMonitorProviderGemini    = "gemini"

	ChannelMonitorAPIModeChatCompletions = "chat_completions"
	ChannelMonitorAPIModeResponses       = "responses"
	ChannelMonitorAPIModeMessages        = "messages"
	ChannelMonitorAPIModeGenerateContent = "generate_content"

	ChannelMonitorStatusOperational = "operational"
	ChannelMonitorStatusDegraded    = "degraded"
	ChannelMonitorStatusFailed      = "failed"
	ChannelMonitorStatusUnknown     = "unknown"

	ChannelMonitorErrorNetwork         = "network"
	ChannelMonitorErrorTLS             = "tls"
	ChannelMonitorErrorAuth            = "auth"
	ChannelMonitorErrorRateLimit       = "rate_limit"
	ChannelMonitorErrorTimeout         = "timeout"
	ChannelMonitorErrorUpstream        = "upstream"
	ChannelMonitorErrorInvalidResponse = "invalid_response"
	ChannelMonitorErrorConfig          = "config"
)

type ChannelMonitor struct {
	ID                    int    `json:"id" gorm:"primaryKey"`
	InternalName          string `json:"internal_name" gorm:"size:128;not null"`
	PublicName            string `json:"public_name" gorm:"size:128;not null"`
	NameColor             string `json:"name_color" gorm:"size:7"`
	Provider              string `json:"provider" gorm:"size:32;not null;index"`
	APIMode               string `json:"api_mode" gorm:"size:32;not null"`
	Endpoint              string `json:"endpoint" gorm:"type:text;not null"`
	APIKeyEncrypted       string `json:"-" gorm:"type:text;not null"`
	APIKeySuffix          string `json:"-" gorm:"size:16"`
	PrimaryModel          string `json:"primary_model" gorm:"size:256;not null"`
	ExtraModels           string `json:"extra_models" gorm:"type:text;not null"`
	IntervalSeconds       int    `json:"interval_seconds" gorm:"not null"`
	DegradedThresholdMs   int    `json:"degraded_threshold_ms" gorm:"not null"`
	TimeoutSeconds        int    `json:"timeout_seconds" gorm:"not null"`
	Enabled               bool   `json:"enabled" gorm:"not null;index:idx_channel_monitor_due,priority:1;index:idx_channel_monitor_public,priority:2"`
	PublicEnabled         bool   `json:"public_enabled" gorm:"not null;index:idx_channel_monitor_public,priority:1"`
	LastCheckedAt         int64  `json:"last_checked_at" gorm:"bigint;index"`
	NextCheckAt           int64  `json:"next_check_at" gorm:"bigint;index:idx_channel_monitor_due,priority:2"`
	ScheduleBlockedReason string `json:"schedule_blocked_reason" gorm:"type:text"`
	LeaseOwner            string `json:"lease_owner" gorm:"size:128"`
	LeaseUntil            int64  `json:"lease_until" gorm:"bigint;index:idx_channel_monitor_due,priority:3"`
	CreatedBy             int    `json:"created_by" gorm:"index"`
	CreatedAt             int64  `json:"created_at" gorm:"bigint;index"`
	UpdatedAt             int64  `json:"updated_at" gorm:"bigint;index"`
}

func (monitor *ChannelMonitor) BeforeCreate(_ *gorm.DB) error {
	now := time.Now().Unix()
	if monitor.CreatedAt == 0 {
		monitor.CreatedAt = now
	}
	if monitor.UpdatedAt == 0 {
		monitor.UpdatedAt = now
	}
	return nil
}

func (monitor *ChannelMonitor) BeforeUpdate(_ *gorm.DB) error {
	monitor.UpdatedAt = time.Now().Unix()
	return nil
}

type ChannelMonitorCheck struct {
	ID            int64  `json:"id" gorm:"primaryKey"`
	MonitorID     int    `json:"monitor_id" gorm:"not null;index:idx_channel_monitor_check_model_time,priority:1"`
	Model         string `json:"model" gorm:"size:256;not null;index:idx_channel_monitor_check_model_time,priority:2"`
	Status        string `json:"status" gorm:"size:32;not null;index"`
	LatencyMs     int64  `json:"latency_ms" gorm:"bigint"`
	ErrorCategory string `json:"error_category" gorm:"size:32"`
	HTTPStatus    int    `json:"http_status"`
	ErrorMessage  string `json:"error_message" gorm:"type:text"`
	CheckedAt     int64  `json:"checked_at" gorm:"bigint;not null;index:idx_channel_monitor_check_model_time,priority:3;index:idx_channel_monitor_check_cleanup"`
}

type ChannelMonitorHourlyRollup struct {
	ID                 int64  `json:"id" gorm:"primaryKey"`
	MonitorID          int    `json:"monitor_id" gorm:"not null;uniqueIndex:idx_channel_monitor_hour_bucket,priority:1"`
	Model              string `json:"model" gorm:"size:256;not null;uniqueIndex:idx_channel_monitor_hour_bucket,priority:2"`
	BucketStart        int64  `json:"bucket_start" gorm:"bigint;not null;uniqueIndex:idx_channel_monitor_hour_bucket,priority:3;index"`
	TotalCount         int64  `json:"total_count" gorm:"bigint;not null"`
	OperationalCount   int64  `json:"operational_count" gorm:"bigint;not null"`
	DegradedCount      int64  `json:"degraded_count" gorm:"bigint;not null"`
	FailedCount        int64  `json:"failed_count" gorm:"bigint;not null"`
	LatencySumMs       int64  `json:"latency_sum_ms" gorm:"bigint;not null"`
	LatencySampleCount int64  `json:"latency_sample_count" gorm:"bigint;not null"`
	ComputedAt         int64  `json:"computed_at" gorm:"bigint;not null"`
}

type ChannelMonitorDailyRollup struct {
	ID                 int64  `json:"id" gorm:"primaryKey"`
	MonitorID          int    `json:"monitor_id" gorm:"not null;uniqueIndex:idx_channel_monitor_day_bucket,priority:1"`
	Model              string `json:"model" gorm:"size:256;not null;uniqueIndex:idx_channel_monitor_day_bucket,priority:2"`
	BucketStart        int64  `json:"bucket_start" gorm:"bigint;not null;uniqueIndex:idx_channel_monitor_day_bucket,priority:3;index"`
	TotalCount         int64  `json:"total_count" gorm:"bigint;not null"`
	OperationalCount   int64  `json:"operational_count" gorm:"bigint;not null"`
	DegradedCount      int64  `json:"degraded_count" gorm:"bigint;not null"`
	FailedCount        int64  `json:"failed_count" gorm:"bigint;not null"`
	LatencySumMs       int64  `json:"latency_sum_ms" gorm:"bigint;not null"`
	LatencySampleCount int64  `json:"latency_sample_count" gorm:"bigint;not null"`
	ComputedAt         int64  `json:"computed_at" gorm:"bigint;not null"`
}

type ChannelMonitorListFilter struct {
	Page          int
	PageSize      int
	Search        string
	Provider      string
	Enabled       *bool
	PublicEnabled *bool
}

type ChannelMonitorHistoryFilter struct {
	MonitorID     int
	Page          int
	PageSize      int
	Model         string
	Status        string
	ErrorCategory string
	StartTime     int64
	EndTime       int64
}

func ListChannelMonitors(filter ChannelMonitorListFilter) ([]*ChannelMonitor, int64, error) {
	if filter.Page < 1 {
		filter.Page = 1
	}
	if filter.PageSize < 1 {
		filter.PageSize = 10
	}
	if filter.PageSize > 100 {
		filter.PageSize = 100
	}

	query := DB.Model(&ChannelMonitor{})
	if search := strings.ToLower(strings.TrimSpace(filter.Search)); search != "" {
		pattern := "%" + search + "%"
		query = query.Where(
			"LOWER(internal_name) LIKE ? OR LOWER(public_name) LIKE ? OR LOWER(primary_model) LIKE ?",
			pattern,
			pattern,
			pattern,
		)
	}
	if provider := strings.TrimSpace(filter.Provider); provider != "" {
		query = query.Where("provider = ?", provider)
	}
	if filter.Enabled != nil {
		query = query.Where("enabled = ?", *filter.Enabled)
	}
	if filter.PublicEnabled != nil {
		query = query.Where("public_enabled = ?", *filter.PublicEnabled)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	items := make([]*ChannelMonitor, 0)
	err := query.
		Order("id DESC").
		Limit(filter.PageSize).
		Offset((filter.Page - 1) * filter.PageSize).
		Find(&items).Error
	if err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func GetChannelMonitor(id int) (*ChannelMonitor, error) {
	if id <= 0 {
		return nil, nil
	}
	var monitor ChannelMonitor
	err := DB.First(&monitor, "id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &monitor, nil
}

func CreateChannelMonitor(monitor *ChannelMonitor) error {
	if monitor == nil {
		return errors.New("channel monitor is required")
	}
	return DB.Create(monitor).Error
}

func UpdateChannelMonitor(monitor *ChannelMonitor, updateSchedule bool) error {
	if monitor == nil || monitor.ID <= 0 {
		return errors.New("valid channel monitor is required")
	}
	monitor.UpdatedAt = time.Now().Unix()
	updates := map[string]any{
		"internal_name":         monitor.InternalName,
		"public_name":           monitor.PublicName,
		"name_color":            monitor.NameColor,
		"provider":              monitor.Provider,
		"api_mode":              monitor.APIMode,
		"endpoint":              monitor.Endpoint,
		"api_key_encrypted":     monitor.APIKeyEncrypted,
		"api_key_suffix":        monitor.APIKeySuffix,
		"primary_model":         monitor.PrimaryModel,
		"extra_models":          monitor.ExtraModels,
		"interval_seconds":      monitor.IntervalSeconds,
		"degraded_threshold_ms": monitor.DegradedThresholdMs,
		"timeout_seconds":       monitor.TimeoutSeconds,
		"enabled":               monitor.Enabled,
		"public_enabled":        monitor.PublicEnabled,
		"updated_at":            monitor.UpdatedAt,
	}
	if updateSchedule {
		updates["next_check_at"] = monitor.NextCheckAt
		updates["schedule_blocked_reason"] = monitor.ScheduleBlockedReason
	}
	return DB.Model(&ChannelMonitor{}).Where("id = ?", monitor.ID).Updates(updates).Error
}

func DeleteChannelMonitor(id int) error {
	if id <= 0 {
		return errors.New("valid channel monitor ID is required")
	}
	return DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("monitor_id = ?", id).Delete(&ChannelMonitorCheck{}).Error; err != nil {
			return err
		}
		if err := tx.Where("monitor_id = ?", id).Delete(&ChannelMonitorHourlyRollup{}).Error; err != nil {
			return err
		}
		if err := tx.Where("monitor_id = ?", id).Delete(&ChannelMonitorDailyRollup{}).Error; err != nil {
			return err
		}
		return tx.Delete(&ChannelMonitor{}, "id = ?", id).Error
	})
}

func ListChannelMonitorHistory(filter ChannelMonitorHistoryFilter) ([]ChannelMonitorCheck, int64, error) {
	if filter.MonitorID <= 0 {
		return nil, 0, errors.New("valid channel monitor ID is required")
	}
	if filter.Page < 1 {
		filter.Page = 1
	}
	if filter.PageSize < 1 {
		filter.PageSize = 10
	}
	if filter.PageSize > 100 {
		filter.PageSize = 100
	}

	query := DB.Model(&ChannelMonitorCheck{}).Where("monitor_id = ?", filter.MonitorID)
	if value := strings.TrimSpace(filter.Model); value != "" {
		query = query.Where("model = ?", value)
	}
	if value := strings.TrimSpace(filter.Status); value != "" {
		query = query.Where("status = ?", value)
	}
	if value := strings.TrimSpace(filter.ErrorCategory); value != "" {
		query = query.Where("error_category = ?", value)
	}
	if filter.StartTime > 0 {
		query = query.Where("checked_at >= ?", filter.StartTime)
	}
	if filter.EndTime > 0 {
		query = query.Where("checked_at <= ?", filter.EndTime)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	items := make([]ChannelMonitorCheck, 0)
	err := query.
		Order("checked_at DESC, id DESC").
		Limit(filter.PageSize).
		Offset((filter.Page - 1) * filter.PageSize).
		Find(&items).Error
	return items, total, err
}

func GetLatestPrimaryChannelMonitorChecks(monitors []*ChannelMonitor) (map[int]ChannelMonitorCheck, error) {
	monitorIDs := make([]int, 0, len(monitors))
	for _, monitor := range monitors {
		if monitor != nil && monitor.ID > 0 {
			monitorIDs = append(monitorIDs, monitor.ID)
		}
	}
	latest := make(map[int]ChannelMonitorCheck, len(monitorIDs))
	if len(monitorIDs) == 0 {
		return latest, nil
	}

	checks := make([]ChannelMonitorCheck, 0)
	err := DB.Model(&ChannelMonitorCheck{}).
		Select("channel_monitor_checks.id, channel_monitor_checks.monitor_id, channel_monitor_checks.model, channel_monitor_checks.status, channel_monitor_checks.latency_ms, channel_monitor_checks.error_category, channel_monitor_checks.http_status, channel_monitor_checks.error_message, channel_monitor_checks.checked_at").
		Joins("JOIN channel_monitors ON channel_monitors.id = channel_monitor_checks.monitor_id AND channel_monitors.primary_model = channel_monitor_checks.model").
		Where("channel_monitor_checks.monitor_id IN ?", monitorIDs).
		Order("channel_monitor_checks.checked_at DESC, channel_monitor_checks.id DESC").
		Find(&checks).Error
	if err != nil {
		return nil, err
	}
	for _, check := range checks {
		if _, exists := latest[check.MonitorID]; !exists {
			latest[check.MonitorID] = check
		}
	}
	return latest, nil
}

func FindDueChannelMonitorIDs(now int64, limit int) ([]int, error) {
	if limit < 1 {
		limit = 50
	}
	if limit > 100 {
		limit = 100
	}
	ids := make([]int, 0)
	err := DB.Model(&ChannelMonitor{}).
		Where("enabled = ?", true).
		Where("schedule_blocked_reason = ?", "").
		Where("next_check_at > 0 AND next_check_at <= ?", now).
		Where("lease_until = 0 OR lease_until < ?", now).
		Order("next_check_at ASC, id ASC").
		Limit(limit).
		Pluck("id", &ids).Error
	return ids, err
}

func ClaimChannelMonitor(id int, owner string, now int64, leaseUntil int64, manual bool) (*ChannelMonitor, bool, error) {
	owner = strings.TrimSpace(owner)
	if id <= 0 || owner == "" || leaseUntil <= now {
		return nil, false, errors.New("invalid channel monitor lease claim")
	}

	query := DB.Model(&ChannelMonitor{}).
		Where("id = ?", id).
		Where("enabled = ?", true).
		Where("schedule_blocked_reason = ?", "").
		Where("lease_until = 0 OR lease_until < ?", now)
	if !manual {
		query = query.Where("next_check_at > 0 AND next_check_at <= ?", now)
	}
	result := query.Updates(map[string]any{
		"lease_owner": owner,
		"lease_until": leaseUntil,
		"updated_at":  now,
	})
	if result.Error != nil {
		return nil, false, result.Error
	}
	if result.RowsAffected == 0 {
		return nil, false, nil
	}

	monitor, err := GetChannelMonitor(id)
	if err != nil {
		return nil, false, err
	}
	if monitor == nil {
		return nil, false, errors.New("claimed channel monitor no longer exists")
	}
	return monitor, true, nil
}

func CommitChannelMonitorRun(id int, owner string, checks []ChannelMonitorCheck, completedAt int64, nextCheckAt int64, blockedReason string) error {
	if id <= 0 || strings.TrimSpace(owner) == "" || completedAt <= 0 {
		return errors.New("invalid channel monitor run commit")
	}
	return DB.Transaction(func(tx *gorm.DB) error {
		for index := range checks {
			checks[index].MonitorID = id
			checks[index].CheckedAt = completedAt
		}
		if len(checks) > 0 {
			if err := tx.Create(&checks).Error; err != nil {
				return err
			}
		}

		result := tx.Model(&ChannelMonitor{}).
			Where("id = ? AND lease_owner = ?", id, owner).
			Updates(map[string]any{
				"last_checked_at":         completedAt,
				"next_check_at":           nextCheckAt,
				"schedule_blocked_reason": blockedReason,
				"lease_owner":             "",
				"lease_until":             0,
				"updated_at":              completedAt,
			})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return ErrChannelMonitorLeaseLost
		}
		return nil
	})
}

func ReleaseChannelMonitorLease(id int, owner string) error {
	if id <= 0 || strings.TrimSpace(owner) == "" {
		return errors.New("invalid channel monitor lease release")
	}
	return DB.Model(&ChannelMonitor{}).
		Where("id = ? AND lease_owner = ?", id, owner).
		Updates(map[string]any{
			"lease_owner": "",
			"lease_until": 0,
		}).Error
}
