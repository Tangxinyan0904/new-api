package service

import (
	"errors"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"gorm.io/gorm"
)

const channelMonitorPublicSummaryCacheTTL = 15 * time.Second

var (
	channelMonitorQueryNow      = time.Now
	channelMonitorQueryLocation = time.Local

	channelMonitorPublicSummaryCacheMu sync.Mutex
	channelMonitorPublicSummaryCache   channelMonitorPublicSummaryCacheEntry
)

type ChannelMonitorStatusCounts struct {
	Operational int `json:"operational"`
	Degraded    int `json:"degraded"`
	Failed      int `json:"failed"`
	Unknown     int `json:"unknown"`
}

type ChannelMonitorTimelinePoint struct {
	Status        string `json:"status"`
	LatencyMs     int64  `json:"latency_ms"`
	ErrorCategory string `json:"error_category"`
	CheckedAt     int64  `json:"checked_at"`
}

type ChannelMonitorModelStatus struct {
	Model              string   `json:"model"`
	IsPrimary          bool     `json:"is_primary"`
	Status             string   `json:"status"`
	LatencyMs          *int64   `json:"latency_ms"`
	ErrorCategory      string   `json:"error_category"`
	LastCheckedAt      int64    `json:"last_checked_at"`
	Availability1d     *float64 `json:"availability_1d"`
	Availability3d     *float64 `json:"availability_3d"`
	Availability7d     *float64 `json:"availability_7d"`
	AverageLatencyMs1d *float64 `json:"average_latency_ms_1d"`
	AverageLatencyMs3d *float64 `json:"average_latency_ms_3d"`
	AverageLatencyMs7d *float64 `json:"average_latency_ms_7d"`
}

type ChannelMonitorStatusItem struct {
	ID               int                           `json:"id"`
	PublicName       string                        `json:"public_name"`
	NameColor        string                        `json:"name_color"`
	Provider         string                        `json:"provider"`
	PrimaryModel     string                        `json:"primary_model"`
	Status           string                        `json:"status"`
	LatencyMs        *int64                        `json:"latency_ms"`
	ErrorCategory    string                        `json:"error_category"`
	LastCheckedAt    int64                         `json:"last_checked_at"`
	Availability     *float64                      `json:"availability"`
	AverageLatencyMs *float64                      `json:"average_latency_ms"`
	Timeline         []ChannelMonitorTimelinePoint `json:"timeline"`
}

type ChannelMonitorStatusResponse struct {
	LastUpdatedAt int64                      `json:"last_updated_at"`
	Counts        ChannelMonitorStatusCounts `json:"counts"`
	Days          int                        `json:"days"`
	Items         []ChannelMonitorStatusItem `json:"items"`
}

type ChannelMonitorStatusDetail struct {
	ID            int                           `json:"id"`
	PublicName    string                        `json:"public_name"`
	NameColor     string                        `json:"name_color"`
	Provider      string                        `json:"provider"`
	PrimaryModel  string                        `json:"primary_model"`
	Status        string                        `json:"status"`
	LastUpdatedAt int64                         `json:"last_updated_at"`
	Models        []ChannelMonitorModelStatus   `json:"models"`
	Timeline      []ChannelMonitorTimelinePoint `json:"timeline"`
}

type ChannelMonitorPublicMonitor struct {
	ID             int      `json:"id"`
	PublicName     string   `json:"public_name"`
	NameColor      string   `json:"name_color"`
	Provider       string   `json:"provider"`
	PrimaryModel   string   `json:"primary_model"`
	Status         string   `json:"status"`
	LatencyMs      *int64   `json:"latency_ms"`
	ErrorCategory  string   `json:"error_category"`
	Availability1d *float64 `json:"availability_1d"`
	LastCheckedAt  int64    `json:"last_checked_at"`
}

type ChannelMonitorPublicSummary struct {
	LastUpdatedAt int64                         `json:"last_updated_at"`
	Counts        ChannelMonitorStatusCounts    `json:"counts"`
	Monitors      []ChannelMonitorPublicMonitor `json:"monitors"`
}

type channelMonitorPublicSummaryCacheEntry struct {
	Summary   ChannelMonitorPublicSummary
	ExpiresAt time.Time
	Valid     bool
}

type channelMonitorPublishedRow struct {
	ID            int
	PublicName    string
	NameColor     string
	Provider      string
	PrimaryModel  string
	ExtraModels   string
	LastCheckedAt int64
}

type channelMonitorSafeCheckRow struct {
	ID            int64
	Model         string
	Status        string
	LatencyMs     int64
	ErrorCategory string
	CheckedAt     int64
}

type channelMonitorRawMetricRow struct {
	Model     string
	Status    string
	LatencyMs int64
	CheckedAt int64
}

type channelMonitorRollupMetricRow struct {
	Model              string
	BucketStart        int64
	TotalCount         int64
	OperationalCount   int64
	DegradedCount      int64
	LatencySumMs       int64
	LatencySampleCount int64
}

type channelMonitorMetricAccumulator struct {
	TotalCount         int64
	AvailableCount     int64
	LatencySumMs       int64
	LatencySampleCount int64
}

type channelMonitorModelMetrics struct {
	Availability1d     *float64
	Availability3d     *float64
	Availability7d     *float64
	AverageLatencyMs1d *float64
	AverageLatencyMs3d *float64
	AverageLatencyMs7d *float64
}

func GetChannelMonitorPublicSummary() (ChannelMonitorPublicSummary, error) {
	now := channelMonitorQueryNow()
	channelMonitorPublicSummaryCacheMu.Lock()
	defer channelMonitorPublicSummaryCacheMu.Unlock()
	if channelMonitorPublicSummaryCache.Valid && now.Before(channelMonitorPublicSummaryCache.ExpiresAt) {
		return cloneChannelMonitorPublicSummary(channelMonitorPublicSummaryCache.Summary), nil
	}

	monitors, err := listPublishedChannelMonitorRows()
	if err != nil {
		return ChannelMonitorPublicSummary{}, err
	}
	summary := ChannelMonitorPublicSummary{
		Monitors: make([]ChannelMonitorPublicMonitor, 0, len(monitors)),
	}
	for _, monitor := range monitors {
		statuses, _, err := loadChannelMonitorStatusData(
			monitor.ID,
			[]string{monitor.PrimaryModel},
			monitor.PrimaryModel,
			now,
			false,
		)
		if err != nil {
			return ChannelMonitorPublicSummary{}, err
		}
		primary := statuses[0]
		summary.Monitors = append(summary.Monitors, ChannelMonitorPublicMonitor{
			ID:             monitor.ID,
			PublicName:     monitor.PublicName,
			NameColor:      monitor.NameColor,
			Provider:       monitor.Provider,
			PrimaryModel:   monitor.PrimaryModel,
			Status:         primary.Status,
			LatencyMs:      cloneInt64Pointer(primary.LatencyMs),
			ErrorCategory:  primary.ErrorCategory,
			Availability1d: cloneFloat64Pointer(primary.Availability1d),
			LastCheckedAt:  channelMonitorSafeLastCheckedAt(primary.LastCheckedAt, monitor.LastCheckedAt),
		})
		addChannelMonitorStatusCount(&summary.Counts, primary.Status)
		if monitor.LastCheckedAt > summary.LastUpdatedAt {
			summary.LastUpdatedAt = monitor.LastCheckedAt
		}
	}
	channelMonitorPublicSummaryCache = channelMonitorPublicSummaryCacheEntry{
		Summary:   cloneChannelMonitorPublicSummary(summary),
		ExpiresAt: now.Add(channelMonitorPublicSummaryCacheTTL),
		Valid:     true,
	}
	return summary, nil
}

func InvalidateChannelMonitorPublicSummaryCache() {
	channelMonitorPublicSummaryCacheMu.Lock()
	channelMonitorPublicSummaryCache = channelMonitorPublicSummaryCacheEntry{}
	channelMonitorPublicSummaryCacheMu.Unlock()
}

func GetPublishedChannelMonitorStatus(days int) (ChannelMonitorStatusResponse, error) {
	if days != 1 && days != 3 && days != 7 {
		return ChannelMonitorStatusResponse{}, errors.New("channel monitor availability days must be 1, 3, or 7")
	}
	now := channelMonitorQueryNow()
	monitors, err := listPublishedChannelMonitorRows()
	if err != nil {
		return ChannelMonitorStatusResponse{}, err
	}
	response := ChannelMonitorStatusResponse{
		Days:  days,
		Items: make([]ChannelMonitorStatusItem, 0, len(monitors)),
	}
	for _, monitor := range monitors {
		statuses, timeline, err := loadChannelMonitorStatusData(
			monitor.ID,
			[]string{monitor.PrimaryModel},
			monitor.PrimaryModel,
			now,
			true,
		)
		if err != nil {
			return ChannelMonitorStatusResponse{}, err
		}
		primary := statuses[0]
		availability, averageLatency := channelMonitorMetricsForDays(primary, days)
		response.Items = append(response.Items, ChannelMonitorStatusItem{
			ID:               monitor.ID,
			PublicName:       monitor.PublicName,
			NameColor:        monitor.NameColor,
			Provider:         monitor.Provider,
			PrimaryModel:     monitor.PrimaryModel,
			Status:           primary.Status,
			LatencyMs:        cloneInt64Pointer(primary.LatencyMs),
			ErrorCategory:    primary.ErrorCategory,
			LastCheckedAt:    channelMonitorSafeLastCheckedAt(primary.LastCheckedAt, monitor.LastCheckedAt),
			Availability:     cloneFloat64Pointer(availability),
			AverageLatencyMs: cloneFloat64Pointer(averageLatency),
			Timeline:         timeline,
		})
		addChannelMonitorStatusCount(&response.Counts, primary.Status)
		if monitor.LastCheckedAt > response.LastUpdatedAt {
			response.LastUpdatedAt = monitor.LastCheckedAt
		}
	}
	return response, nil
}

func GetPublishedChannelMonitorDetail(id int) (*ChannelMonitorStatusDetail, error) {
	if id <= 0 {
		return nil, nil
	}
	monitor, err := getPublishedChannelMonitorRow(id)
	if err != nil || monitor == nil {
		return nil, err
	}
	models, err := channelMonitorPublishedModels(*monitor)
	if err != nil {
		return nil, err
	}
	statuses, timeline, err := loadChannelMonitorStatusData(monitor.ID, models, monitor.PrimaryModel, channelMonitorQueryNow(), true)
	if err != nil {
		return nil, err
	}
	counts := ChannelMonitorStatusCounts{}
	lastUpdatedAt := monitor.LastCheckedAt
	for _, status := range statuses {
		addChannelMonitorStatusCount(&counts, status.Status)
		if status.LastCheckedAt > lastUpdatedAt {
			lastUpdatedAt = status.LastCheckedAt
		}
	}
	return &ChannelMonitorStatusDetail{
		ID:            monitor.ID,
		PublicName:    monitor.PublicName,
		NameColor:     monitor.NameColor,
		Provider:      monitor.Provider,
		PrimaryModel:  monitor.PrimaryModel,
		Status:        channelMonitorOverallStatus(counts, len(statuses)),
		LastUpdatedAt: lastUpdatedAt,
		Models:        statuses,
		Timeline:      timeline,
	}, nil
}

func listPublishedChannelMonitorRows() ([]channelMonitorPublishedRow, error) {
	rows := make([]channelMonitorPublishedRow, 0)
	err := model.DB.Model(&model.ChannelMonitor{}).
		Select("id, public_name, name_color, provider, primary_model, extra_models, last_checked_at").
		Where("enabled = ? AND public_enabled = ?", true, true).
		Order("id ASC").
		Scan(&rows).Error
	return rows, err
}

func getPublishedChannelMonitorRow(id int) (*channelMonitorPublishedRow, error) {
	var row channelMonitorPublishedRow
	err := model.DB.Model(&model.ChannelMonitor{}).
		Select("id, public_name, name_color, provider, primary_model, extra_models, last_checked_at").
		Where("id = ? AND enabled = ? AND public_enabled = ?", id, true, true).
		Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &row, nil
}

func channelMonitorPublishedModels(monitor channelMonitorPublishedRow) ([]string, error) {
	models := []string{monitor.PrimaryModel}
	if monitor.ExtraModels == "" {
		return models, nil
	}
	extraModels := make([]string, 0)
	if err := common.UnmarshalJsonStr(monitor.ExtraModels, &extraModels); err != nil {
		return nil, errors.New("decode channel monitor models failed")
	}
	seen := map[string]struct{}{monitor.PrimaryModel: {}}
	for _, modelName := range extraModels {
		if _, exists := seen[modelName]; exists {
			continue
		}
		seen[modelName] = struct{}{}
		models = append(models, modelName)
	}
	return models, nil
}

func loadChannelMonitorStatusData(
	monitorID int,
	models []string,
	primaryModel string,
	now time.Time,
	includeTimeline bool,
) ([]ChannelMonitorModelStatus, []ChannelMonitorTimelinePoint, error) {
	latest, err := loadChannelMonitorLatestChecks(monitorID, models)
	if err != nil {
		return nil, nil, err
	}
	metrics, err := loadChannelMonitorAvailability(monitorID, models, now)
	if err != nil {
		return nil, nil, err
	}
	statuses := make([]ChannelMonitorModelStatus, 0, len(models))
	for _, modelName := range models {
		status := ChannelMonitorModelStatus{
			Model:     modelName,
			IsPrimary: modelName == primaryModel,
			Status:    model.ChannelMonitorStatusUnknown,
		}
		if check, exists := latest[modelName]; exists {
			latency := check.LatencyMs
			status.Status = channelMonitorSafeStatus(check.Status)
			status.LatencyMs = &latency
			status.ErrorCategory = channelMonitorSafeErrorCategory(check.ErrorCategory)
			status.LastCheckedAt = check.CheckedAt
		}
		if value, exists := metrics[modelName]; exists {
			status.Availability1d = value.Availability1d
			status.Availability3d = value.Availability3d
			status.Availability7d = value.Availability7d
			status.AverageLatencyMs1d = value.AverageLatencyMs1d
			status.AverageLatencyMs3d = value.AverageLatencyMs3d
			status.AverageLatencyMs7d = value.AverageLatencyMs7d
		}
		statuses = append(statuses, status)
	}
	if !includeTimeline {
		return statuses, nil, nil
	}
	timeline, err := loadChannelMonitorTimeline(monitorID, primaryModel, now)
	return statuses, timeline, err
}

func loadChannelMonitorLatestChecks(monitorID int, models []string) (map[string]channelMonitorSafeCheckRow, error) {
	rows := make([]channelMonitorSafeCheckRow, 0)
	err := model.DB.Model(&model.ChannelMonitorCheck{}).
		Select("id, model, status, latency_ms, error_category, checked_at").
		Where("monitor_id = ? AND model IN ?", monitorID, models).
		Order("checked_at DESC, id DESC").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	latest := make(map[string]channelMonitorSafeCheckRow, len(models))
	for _, row := range rows {
		if _, exists := latest[row.Model]; !exists {
			latest[row.Model] = row
		}
	}
	return latest, nil
}

func loadChannelMonitorAvailability(monitorID int, models []string, now time.Time) (map[string]channelMonitorModelMetrics, error) {
	location := channelMonitorQueryLocation
	if location == nil {
		location = time.Local
	}
	dayStart := channelMonitorNaturalDayStart(now, location)
	cutoffs := map[int]int64{
		1: dayStart.Unix(),
		3: dayStart.AddDate(0, 0, -2).Unix(),
		7: dayStart.AddDate(0, 0, -6).Unix(),
	}
	accumulators := make(map[string]map[int]*channelMonitorMetricAccumulator, len(models))
	for _, modelName := range models {
		accumulators[modelName] = map[int]*channelMonitorMetricAccumulator{
			1: {},
			3: {},
			7: {},
		}
	}

	rawRows := make([]channelMonitorRawMetricRow, 0)
	err := model.DB.Model(&model.ChannelMonitorCheck{}).
		Select("model, status, latency_ms, checked_at").
		Where("monitor_id = ? AND model IN ? AND checked_at >= ?", monitorID, models, cutoffs[7]).
		Scan(&rawRows).Error
	if err != nil {
		return nil, err
	}
	for _, row := range rawRows {
		byWindow, exists := accumulators[row.Model]
		if !exists {
			continue
		}
		for days, cutoff := range cutoffs {
			if row.CheckedAt < cutoff {
				continue
			}
			accumulator := byWindow[days]
			accumulator.TotalCount++
			if row.Status == model.ChannelMonitorStatusOperational || row.Status == model.ChannelMonitorStatusDegraded {
				accumulator.AvailableCount++
				accumulator.LatencySumMs += row.LatencyMs
				accumulator.LatencySampleCount++
			}
		}
	}

	if err := addChannelMonitorRollupMetrics(
		model.DB.Model(&model.ChannelMonitorHourlyRollup{}), monitorID, models, cutoffs, accumulators,
	); err != nil {
		return nil, err
	}
	if err := addChannelMonitorRollupMetrics(
		model.DB.Model(&model.ChannelMonitorDailyRollup{}), monitorID, models, cutoffs, accumulators,
	); err != nil {
		return nil, err
	}

	metrics := make(map[string]channelMonitorModelMetrics, len(models))
	for _, modelName := range models {
		metrics[modelName] = channelMonitorModelMetrics{
			Availability1d:     channelMonitorAvailability(accumulators[modelName][1]),
			Availability3d:     channelMonitorAvailability(accumulators[modelName][3]),
			Availability7d:     channelMonitorAvailability(accumulators[modelName][7]),
			AverageLatencyMs1d: channelMonitorAverageLatency(accumulators[modelName][1]),
			AverageLatencyMs3d: channelMonitorAverageLatency(accumulators[modelName][3]),
			AverageLatencyMs7d: channelMonitorAverageLatency(accumulators[modelName][7]),
		}
	}
	return metrics, nil
}

func addChannelMonitorRollupMetrics(
	query *gorm.DB,
	monitorID int,
	models []string,
	cutoffs map[int]int64,
	accumulators map[string]map[int]*channelMonitorMetricAccumulator,
) error {
	rows := make([]channelMonitorRollupMetricRow, 0)
	err := query.
		Select("model, bucket_start, total_count, operational_count, degraded_count, latency_sum_ms, latency_sample_count").
		Where("monitor_id = ? AND model IN ? AND bucket_start >= ?", monitorID, models, cutoffs[7]).
		Scan(&rows).Error
	if err != nil {
		return err
	}
	for _, row := range rows {
		byWindow, exists := accumulators[row.Model]
		if !exists || row.TotalCount <= 0 {
			continue
		}
		availableCount := row.OperationalCount + row.DegradedCount
		if availableCount < 0 {
			availableCount = 0
		}
		if availableCount > row.TotalCount {
			availableCount = row.TotalCount
		}
		for days, cutoff := range cutoffs {
			if row.BucketStart < cutoff {
				continue
			}
			accumulator := byWindow[days]
			accumulator.TotalCount += row.TotalCount
			accumulator.AvailableCount += availableCount
			if row.LatencySampleCount > 0 && row.LatencySumMs >= 0 {
				accumulator.LatencySumMs += row.LatencySumMs
				accumulator.LatencySampleCount += row.LatencySampleCount
			}
		}
	}
	return nil
}

func loadChannelMonitorTimeline(monitorID int, primaryModel string, now time.Time) ([]ChannelMonitorTimelinePoint, error) {
	rows := make([]channelMonitorSafeCheckRow, 0, 60)
	err := model.DB.Model(&model.ChannelMonitorCheck{}).
		Select("id, status, latency_ms, error_category, checked_at").
		Where("monitor_id = ? AND model = ? AND checked_at >= ?", monitorID, primaryModel, now.Add(-time.Hour).Unix()).
		Order("checked_at DESC, id DESC").
		Limit(60).
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	timeline := make([]ChannelMonitorTimelinePoint, 0, len(rows))
	for _, row := range rows {
		timeline = append(timeline, ChannelMonitorTimelinePoint{
			Status:        channelMonitorSafeStatus(row.Status),
			LatencyMs:     row.LatencyMs,
			ErrorCategory: channelMonitorSafeErrorCategory(row.ErrorCategory),
			CheckedAt:     row.CheckedAt,
		})
	}
	return timeline, nil
}

func channelMonitorNaturalDayStart(value time.Time, location *time.Location) time.Time {
	local := value.In(location)
	return time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, location)
}

func channelMonitorAvailability(accumulator *channelMonitorMetricAccumulator) *float64 {
	if accumulator == nil || accumulator.TotalCount <= 0 {
		return nil
	}
	value := float64(accumulator.AvailableCount) * 100 / float64(accumulator.TotalCount)
	return &value
}

func channelMonitorAverageLatency(accumulator *channelMonitorMetricAccumulator) *float64 {
	if accumulator == nil || accumulator.LatencySampleCount <= 0 {
		return nil
	}
	value := float64(accumulator.LatencySumMs) / float64(accumulator.LatencySampleCount)
	return &value
}

func channelMonitorMetricsForDays(status ChannelMonitorModelStatus, days int) (*float64, *float64) {
	switch days {
	case 3:
		return status.Availability3d, status.AverageLatencyMs3d
	case 7:
		return status.Availability7d, status.AverageLatencyMs7d
	default:
		return status.Availability1d, status.AverageLatencyMs1d
	}
}

func channelMonitorSafeStatus(status string) string {
	switch status {
	case model.ChannelMonitorStatusOperational, model.ChannelMonitorStatusDegraded, model.ChannelMonitorStatusFailed:
		return status
	default:
		return model.ChannelMonitorStatusFailed
	}
}

func channelMonitorSafeErrorCategory(category string) string {
	switch category {
	case model.ChannelMonitorErrorNetwork,
		model.ChannelMonitorErrorTLS,
		model.ChannelMonitorErrorAuth,
		model.ChannelMonitorErrorRateLimit,
		model.ChannelMonitorErrorTimeout,
		model.ChannelMonitorErrorUpstream,
		model.ChannelMonitorErrorInvalidResponse,
		model.ChannelMonitorErrorConfig:
		return category
	default:
		return ""
	}
}

func addChannelMonitorStatusCount(counts *ChannelMonitorStatusCounts, status string) {
	switch status {
	case model.ChannelMonitorStatusOperational:
		counts.Operational++
	case model.ChannelMonitorStatusDegraded:
		counts.Degraded++
	case model.ChannelMonitorStatusFailed:
		counts.Failed++
	default:
		counts.Unknown++
	}
}

func channelMonitorOverallStatus(counts ChannelMonitorStatusCounts, total int) string {
	if total == 0 || counts.Unknown == total {
		return model.ChannelMonitorStatusUnknown
	}
	if counts.Failed > 0 {
		return model.ChannelMonitorStatusFailed
	}
	if counts.Degraded > 0 || counts.Unknown > 0 {
		return model.ChannelMonitorStatusDegraded
	}
	return model.ChannelMonitorStatusOperational
}

func channelMonitorSafeLastCheckedAt(latestCheckAt int64, monitorLastCheckedAt int64) int64 {
	if latestCheckAt > 0 {
		return latestCheckAt
	}
	return monitorLastCheckedAt
}

func cloneChannelMonitorPublicSummary(summary ChannelMonitorPublicSummary) ChannelMonitorPublicSummary {
	cloned := summary
	cloned.Monitors = make([]ChannelMonitorPublicMonitor, len(summary.Monitors))
	for index, monitor := range summary.Monitors {
		cloned.Monitors[index] = monitor
		cloned.Monitors[index].LatencyMs = cloneInt64Pointer(monitor.LatencyMs)
		cloned.Monitors[index].Availability1d = cloneFloat64Pointer(monitor.Availability1d)
	}
	return cloned
}

func cloneInt64Pointer(value *int64) *int64 {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}

func cloneFloat64Pointer(value *float64) *float64 {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}
