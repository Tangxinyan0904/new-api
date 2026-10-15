package service

import (
	"strconv"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
)

const (
	// ChannelStatusWindowSeconds is how far back the status page looks.
	ChannelStatusWindowSeconds = 3600
	// ChannelStatusBucketSeconds is the width of one timeline bucket.
	ChannelStatusBucketSeconds = 300
	channelStatusBucketCount   = ChannelStatusWindowSeconds / ChannelStatusBucketSeconds

	channelStatusOperationalRate = 95.0
	channelStatusDegradedRate    = 80.0
	channelStatusCacheTTL        = time.Minute

	ChannelStatusOperational = "operational"
	ChannelStatusDegraded    = "degraded"
	ChannelStatusFailed      = "failed"
	ChannelStatusUnknown     = "unknown"
)

var (
	channelStatusNow     = time.Now
	channelStatusCacheMu sync.Mutex
	channelStatusCache   *ChannelStatusSnapshot
)

// ChannelStatusTally counts the outcome of user requests.
type ChannelStatusTally struct {
	Success int64
	Failure int64
	// DurationSum is the summed duration of the successful requests, in seconds.
	DurationSum int64
}

func (tally *ChannelStatusTally) add(other ChannelStatusTally) {
	tally.Success += other.Success
	tally.Failure += other.Failure
	tally.DurationSum += other.DurationSum
}

// ChannelStatusGroupStats is what the logs say about one group.
type ChannelStatusGroupStats struct {
	Total   ChannelStatusTally
	Buckets [channelStatusBucketCount]ChannelStatusTally
	Models  map[string]*ChannelStatusTally
}

// ChannelStatusSnapshot holds the last hour of every monitored group. The
// window is made of five-minute buckets; the last one is still filling.
type ChannelStatusSnapshot struct {
	UpdatedAt   int64
	WindowStart int64
	Groups      map[string]*ChannelStatusGroupStats
}

func (snapshot *ChannelStatusSnapshot) group(name string) *ChannelStatusGroupStats {
	stats := snapshot.Groups[name]
	if stats == nil {
		stats = &ChannelStatusGroupStats{Models: map[string]*ChannelStatusTally{}}
		snapshot.Groups[name] = stats
	}
	return stats
}

func (snapshot *ChannelStatusSnapshot) record(group, modelName string, bucket int, tally ChannelStatusTally) {
	stats := snapshot.group(group)
	stats.Total.add(tally)
	stats.Buckets[bucket].add(tally)
	modelTally := stats.Models[modelName]
	if modelTally == nil {
		modelTally = &ChannelStatusTally{}
		stats.Models[modelName] = modelTally
	}
	modelTally.add(tally)
}

// IsChannelStatusFailure reports whether a request that ended with this
// upstream status code counts against the group. Errors caused by the
// request itself, such as 400 or 404, do not; a missing code means the
// upstream could not be reached.
func IsChannelStatusFailure(statusCode int) bool {
	switch {
	case statusCode == 0, statusCode >= 500:
		return true
	case statusCode == 401, statusCode == 403, statusCode == 408, statusCode == 429:
		return true
	}
	return false
}

// BuildChannelStatusSnapshot reads the last hour of call logs of groups. A
// request is counted once: as a success if it has a consume log, even after
// failed retries, otherwise as a failure if its last error is a service
// failure. Requests that failed because of their own input are left out.
func BuildChannelStatusSnapshot(groups []string, now int64) (*ChannelStatusSnapshot, error) {
	windowEnd := (now/ChannelStatusBucketSeconds + 1) * ChannelStatusBucketSeconds
	snapshot := &ChannelStatusSnapshot{
		UpdatedAt:   now,
		WindowStart: windowEnd - ChannelStatusWindowSeconds,
		Groups:      map[string]*ChannelStatusGroupStats{},
	}
	if len(groups) == 0 {
		return snapshot, nil
	}
	for bucket := range channelStatusBucketCount {
		start := snapshot.WindowStart + int64(bucket)*ChannelStatusBucketSeconds
		rows, err := model.CountChannelStatusSuccesses(groups, start, start+ChannelStatusBucketSeconds)
		if err != nil {
			return nil, err
		}
		for _, row := range rows {
			snapshot.record(row.GroupName, row.ModelName, bucket, ChannelStatusTally{Success: row.Count, DurationSum: row.UseTime})
		}
	}

	errorRows, err := model.ListChannelStatusErrors(groups, snapshot.WindowStart, windowEnd)
	if err != nil {
		return nil, err
	}
	// Retries share a request id; the last error is what the user received.
	lastErrors := make(map[string]model.ChannelStatusErrorRow, len(errorRows))
	for _, row := range errorRows {
		key := row.RequestId
		if key == "" {
			key = "#" + strconv.Itoa(row.Id)
		}
		last, seen := lastErrors[key]
		if !seen || row.CreatedAt > last.CreatedAt || (row.CreatedAt == last.CreatedAt && row.Id > last.Id) {
			lastErrors[key] = row
		}
	}
	requestIds := make([]string, 0, len(lastErrors))
	for _, row := range lastErrors {
		if row.RequestId != "" {
			requestIds = append(requestIds, row.RequestId)
		}
	}
	succeeded, err := model.SucceededRequestIds(requestIds, snapshot.WindowStart)
	if err != nil {
		return nil, err
	}
	for _, row := range lastErrors {
		if succeeded[row.RequestId] {
			continue
		}
		var other struct {
			StatusCode int `json:"status_code"`
		}
		_ = common.UnmarshalJsonStr(row.Other, &other)
		if !IsChannelStatusFailure(other.StatusCode) {
			continue
		}
		bucket := int((row.CreatedAt - snapshot.WindowStart) / ChannelStatusBucketSeconds)
		snapshot.record(row.GroupName, row.ModelName, min(max(bucket, 0), channelStatusBucketCount-1), ChannelStatusTally{Failure: 1})
	}
	return snapshot, nil
}

// GetChannelStatusSnapshot returns the snapshot of all monitored groups,
// rebuilt at most once a minute.
func GetChannelStatusSnapshot() (*ChannelStatusSnapshot, error) {
	channelStatusCacheMu.Lock()
	defer channelStatusCacheMu.Unlock()
	now := channelStatusNow().Unix()
	if channelStatusCache != nil && now < channelStatusCache.UpdatedAt+int64(channelStatusCacheTTL/time.Second) {
		return channelStatusCache, nil
	}
	monitors, err := model.ListChannelStatusMonitors(true)
	if err != nil {
		return nil, err
	}
	groups := make([]string, 0, len(monitors))
	for _, monitor := range monitors {
		groups = append(groups, monitor.Group)
	}
	snapshot, err := BuildChannelStatusSnapshot(groups, now)
	if err != nil {
		return nil, err
	}
	channelStatusCache = snapshot
	return snapshot, nil
}

// InvalidateChannelStatusSnapshot drops the cached snapshot, so a new or
// changed monitor shows its data at once.
func InvalidateChannelStatusSnapshot() {
	channelStatusCacheMu.Lock()
	channelStatusCache = nil
	channelStatusCacheMu.Unlock()
}

// ChannelStatusFigures summarises a tally for the status page.
type ChannelStatusFigures struct {
	Status string `json:"status"`
	// SuccessRate is a percentage; it is null when there were no requests.
	SuccessRate  *float64 `json:"success_rate"`
	SuccessCount int64    `json:"success_count"`
	FailureCount int64    `json:"failure_count"`
	// AverageDurationMs is the mean duration of successful requests.
	AverageDurationMs *float64 `json:"average_duration_ms"`
}

// ChannelStatusFiguresOf rates a tally: 95% or more of requests succeeding is
// operational, 80% or more degraded, anything lower failed.
func ChannelStatusFiguresOf(tally ChannelStatusTally) ChannelStatusFigures {
	figures := ChannelStatusFigures{Status: ChannelStatusUnknown, SuccessCount: tally.Success, FailureCount: tally.Failure}
	total := tally.Success + tally.Failure
	if total == 0 {
		return figures
	}
	rate := float64(tally.Success) * 100 / float64(total)
	figures.SuccessRate = &rate
	switch {
	case rate >= channelStatusOperationalRate:
		figures.Status = ChannelStatusOperational
	case rate >= channelStatusDegradedRate:
		figures.Status = ChannelStatusDegraded
	default:
		figures.Status = ChannelStatusFailed
	}
	if tally.Success > 0 {
		duration := float64(tally.DurationSum) * 1000 / float64(tally.Success)
		figures.AverageDurationMs = &duration
	}
	return figures
}
