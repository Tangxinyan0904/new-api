package service

import (
	"fmt"
	"maps"
	"slices"
	"strconv"
	"strings"
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
	// channelStatusClosedDelaySeconds is how long after a bucket ends its
	// cache hits are kept, allowing for clock skew between nodes.
	channelStatusClosedDelaySeconds = 60

	ChannelStatusOperational = "operational"
	ChannelStatusDegraded    = "degraded"
	ChannelStatusFailed      = "failed"
	ChannelStatusUnknown     = "unknown"
)

var (
	channelStatusNow     = time.Now
	channelStatusCacheMu sync.Mutex
	channelStatusCache   *ChannelStatusSnapshot
	// channelStatusClosedCacheHits keeps the cache hits of ended buckets by
	// bucket start, so a rebuild reads the cache logs of open buckets only.
	// Guarded by channelStatusCacheMu.
	channelStatusClosedCacheHits = map[int64]channelStatusBucketCacheHits{}
)

// channelStatusBucketCacheHits counts, per group, the requests of one bucket
// that reached the group's cache hit threshold. Thresholds identifies the
// thresholds they were counted with.
type channelStatusBucketCacheHits struct {
	Thresholds string
	Hits       map[string]int64
}

// ChannelStatusCacheTally counts the successful requests that sent input
// tokens and how many of them reached the group's cache hit threshold.
type ChannelStatusCacheTally struct {
	Requests int64
	Hits     int64
}

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
	Cache   ChannelStatusCacheTally
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
// Successful requests of a group with a cache hit threshold (percent) are
// also counted by whether their cache hit rate reached it. Callers hold
// channelStatusCacheMu.
func BuildChannelStatusSnapshot(groups []string, cacheHitThresholds map[string]int, now int64) (*ChannelStatusSnapshot, error) {
	windowEnd := (now/ChannelStatusBucketSeconds + 1) * ChannelStatusBucketSeconds
	snapshot := &ChannelStatusSnapshot{
		UpdatedAt:   now,
		WindowStart: windowEnd - ChannelStatusWindowSeconds,
		Groups:      map[string]*ChannelStatusGroupStats{},
	}
	if len(groups) == 0 {
		return snapshot, nil
	}
	cacheGroups := make([]string, 0, len(cacheHitThresholds))
	var thresholds strings.Builder
	for _, group := range slices.Sorted(maps.Keys(cacheHitThresholds)) {
		if cacheHitThresholds[group] > 0 {
			cacheGroups = append(cacheGroups, group)
			fmt.Fprintf(&thresholds, "%s=%d;", group, cacheHitThresholds[group])
		}
	}
	for bucket := range channelStatusBucketCount {
		start := snapshot.WindowStart + int64(bucket)*ChannelStatusBucketSeconds
		end := start + ChannelStatusBucketSeconds
		rows, err := model.CountChannelStatusSuccesses(groups, start, end)
		if err != nil {
			return nil, err
		}
		for _, row := range rows {
			snapshot.record(row.GroupName, row.ModelName, bucket, ChannelStatusTally{Success: row.Count, DurationSum: row.UseTime})
			snapshot.group(row.GroupName).Cache.Requests += row.InputCount
		}
		if len(cacheGroups) == 0 {
			continue
		}
		counted, closed := channelStatusClosedCacheHits[start]
		if !closed || counted.Thresholds != thresholds.String() {
			counted = channelStatusBucketCacheHits{Thresholds: thresholds.String(), Hits: map[string]int64{}}
			cacheRows, err := model.ListChannelStatusCacheRows(cacheGroups, start, end)
			if err != nil {
				return nil, err
			}
			for _, row := range cacheRows {
				if model.LogCacheHitRate(row.PromptTokens, row.Other) >= float64(cacheHitThresholds[row.GroupName]) {
					counted.Hits[row.GroupName]++
				}
			}
			if end+channelStatusClosedDelaySeconds <= now {
				channelStatusClosedCacheHits[start] = counted
			}
		}
		for group, hits := range counted.Hits {
			snapshot.group(group).Cache.Hits += hits
		}
	}
	maps.DeleteFunc(channelStatusClosedCacheHits, func(start int64, _ channelStatusBucketCacheHits) bool {
		return start < snapshot.WindowStart
	})

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
	cacheHitThresholds := make(map[string]int, len(monitors))
	for _, monitor := range monitors {
		groups = append(groups, monitor.Group)
		cacheHitThresholds[monitor.Group] = monitor.CacheHitThreshold
	}
	snapshot, err := BuildChannelStatusSnapshot(groups, cacheHitThresholds, now)
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

// ChannelStatusCacheFigures shows how many requests reached a monitor's cache
// hit threshold.
type ChannelStatusCacheFigures struct {
	// CacheHitThreshold is in percent; 0 means the monitor shows no cache
	// figures.
	CacheHitThreshold int `json:"cache_hit_threshold"`
	// CacheHitShare is the percentage of successful requests with input
	// tokens that reached the threshold; null when off or without requests.
	CacheHitShare     *float64 `json:"cache_hit_share"`
	CacheHitCount     int64    `json:"cache_hit_count"`
	CacheRequestCount int64    `json:"cache_request_count"`
}

// ChannelStatusCacheFiguresOf summarises a cache tally counted against
// threshold.
func ChannelStatusCacheFiguresOf(tally ChannelStatusCacheTally, threshold int) ChannelStatusCacheFigures {
	figures := ChannelStatusCacheFigures{CacheHitThreshold: threshold}
	if threshold <= 0 {
		return figures
	}
	figures.CacheHitCount = tally.Hits
	figures.CacheRequestCount = tally.Requests
	if tally.Requests > 0 {
		share := float64(tally.Hits) * 100 / float64(tally.Requests)
		figures.CacheHitShare = &share
	}
	return figures
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
