package service

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
)

const (
	channelMonitorSchedulerScanInterval   = 5 * time.Second
	channelMonitorSchedulerWorkerCount    = 5
	channelMonitorSchedulerBatchSize      = 50
	channelMonitorCredentialBlockedReason = "credential_decryption_failed"
	channelMonitorMinimumLeaseSeconds     = 60
	channelMonitorMaximumLeaseSeconds     = 2460
	channelMonitorHistoryBatchSize        = 500
	channelMonitorHistoryInterval         = time.Minute
)

var (
	ErrChannelMonitorBusy = errors.New("channel monitor is already running")

	channelMonitorSchedulerNow  = time.Now
	channelMonitorSchedulerOnce sync.Once
)

type channelMonitorSchedulerConfig struct {
	Owner            string
	WorkerCount      int
	BatchSize        int
	Client           *http.Client
	Now              func() time.Time
	AggregateHistory func(time.Time) error
}

type channelMonitorScheduler struct {
	config    channelMonitorSchedulerConfig
	jobs      chan *model.ChannelMonitor
	pending   sync.WaitGroup
	workers   sync.WaitGroup
	startOnce sync.Once
	stopOnce  sync.Once

	maintenanceMu        sync.Mutex
	lastMaintenanceRunAt time.Time
}

func newChannelMonitorScheduler(config channelMonitorSchedulerConfig) *channelMonitorScheduler {
	if strings.TrimSpace(config.Owner) == "" {
		config.Owner = "channel-monitor-runner"
	}
	if config.WorkerCount <= 0 {
		config.WorkerCount = channelMonitorSchedulerWorkerCount
	}
	if config.BatchSize <= 0 {
		config.BatchSize = channelMonitorSchedulerBatchSize
	}
	if config.Now == nil {
		config.Now = channelMonitorSchedulerNow
	}
	if config.AggregateHistory == nil {
		config.AggregateHistory = func(now time.Time) error {
			return model.AggregateChannelMonitorHistory(now, time.Local, channelMonitorHistoryBatchSize)
		}
	}
	return &channelMonitorScheduler{
		config: config,
		jobs:   make(chan *model.ChannelMonitor, config.BatchSize),
	}
}

func (scheduler *channelMonitorScheduler) start(ctx context.Context) {
	scheduler.startOnce.Do(func() {
		for workerIndex := 0; workerIndex < scheduler.config.WorkerCount; workerIndex++ {
			scheduler.workers.Add(1)
			go func() {
				defer scheduler.workers.Done()
				for monitor := range scheduler.jobs {
					_, err := runClaimedChannelMonitorWithDependencies(
						ctx,
						monitor,
						scheduler.config.Owner,
						scheduler.config.Client,
						scheduler.config.Now,
					)
					if err != nil {
						logger.LogWarn(ctx, fmt.Sprintf("channel monitor run failed: monitor_id=%d err=%v", monitor.ID, err))
					}
					scheduler.pending.Done()
				}
			}()
		}
	})
}

func (scheduler *channelMonitorScheduler) scan(ctx context.Context) error {
	runAt := scheduler.config.Now()
	now := runAt.Unix()
	ids, err := model.FindDueChannelMonitorIDs(now, scheduler.config.BatchSize)
	if err != nil {
		return err
	}
	for _, id := range ids {
		monitor, err := model.GetChannelMonitor(id)
		if err != nil {
			return err
		}
		if monitor == nil {
			continue
		}
		leaseUntil := now + channelMonitorLeaseSeconds(monitor)
		claimedMonitor, claimed, err := model.ClaimChannelMonitor(id, scheduler.config.Owner, now, leaseUntil, false)
		if err != nil {
			return err
		}
		if !claimed {
			continue
		}

		scheduler.pending.Add(1)
		select {
		case scheduler.jobs <- claimedMonitor:
		case <-ctx.Done():
			scheduler.pending.Done()
			_ = model.ReleaseChannelMonitorLease(claimedMonitor.ID, scheduler.config.Owner)
			return ctx.Err()
		}
	}
	return scheduler.aggregateHistory(runAt)
}

func (scheduler *channelMonitorScheduler) aggregateHistory(runAt time.Time) error {
	scheduler.maintenanceMu.Lock()
	defer scheduler.maintenanceMu.Unlock()
	if !scheduler.lastMaintenanceRunAt.IsZero() && runAt.Before(scheduler.lastMaintenanceRunAt.Add(channelMonitorHistoryInterval)) {
		return nil
	}
	scheduler.lastMaintenanceRunAt = runAt
	return scheduler.config.AggregateHistory(runAt)
}

func (scheduler *channelMonitorScheduler) wait() {
	scheduler.pending.Wait()
}

func (scheduler *channelMonitorScheduler) stop() {
	scheduler.stopOnce.Do(func() {
		close(scheduler.jobs)
		scheduler.workers.Wait()
	})
}

func StartChannelMonitorScheduler() {
	channelMonitorSchedulerOnce.Do(func() {
		if !common.IsMasterNode {
			return
		}
		owner := fmt.Sprintf("%s-channel-monitor-%s", common.NodeName, common.GetRandomString(8))
		scheduler := newChannelMonitorScheduler(channelMonitorSchedulerConfig{
			Owner:       owner,
			WorkerCount: channelMonitorSchedulerWorkerCount,
			BatchSize:   channelMonitorSchedulerBatchSize,
			Client:      GetHttpClient(),
			Now:         channelMonitorSchedulerNow,
		})
		ctx := context.Background()
		scheduler.start(ctx)
		go func() {
			logger.LogInfo(ctx, fmt.Sprintf("channel monitor scheduler started: owner=%s workers=%d", owner, channelMonitorSchedulerWorkerCount))
			if err := scheduler.scan(ctx); err != nil {
				logger.LogWarn(ctx, fmt.Sprintf("channel monitor scheduler scan failed: %v", err))
			}
			ticker := time.NewTicker(channelMonitorSchedulerScanInterval)
			defer ticker.Stop()
			for range ticker.C {
				if err := scheduler.scan(ctx); err != nil {
					logger.LogWarn(ctx, fmt.Sprintf("channel monitor scheduler scan failed: %v", err))
				}
			}
		}()
	})
}

func RunClaimedChannelMonitor(ctx context.Context, monitor *model.ChannelMonitor, owner string) ([]model.ChannelMonitorCheck, error) {
	return runClaimedChannelMonitorWithDependencies(ctx, monitor, owner, GetHttpClient(), channelMonitorSchedulerNow)
}

func runClaimedChannelMonitorWithDependencies(
	ctx context.Context,
	monitor *model.ChannelMonitor,
	owner string,
	client *http.Client,
	now func() time.Time,
) ([]model.ChannelMonitorCheck, error) {
	if monitor == nil || monitor.ID <= 0 || strings.TrimSpace(owner) == "" {
		return nil, errors.New("invalid claimed channel monitor")
	}
	if now == nil {
		now = channelMonitorSchedulerNow
	}

	models, err := channelMonitorModels(monitor)
	if err != nil {
		return nil, err
	}
	credential, decryptErr := DecryptChannelMonitorCredential(monitor.APIKeyEncrypted)
	if decryptErr != nil {
		completedAt := now().Unix()
		checks := make([]model.ChannelMonitorCheck, 0, len(models))
		for _, modelName := range models {
			checks = append(checks, model.ChannelMonitorCheck{
				Model:         modelName,
				Status:        model.ChannelMonitorStatusFailed,
				ErrorCategory: model.ChannelMonitorErrorConfig,
				ErrorMessage:  "stored credential could not be decrypted",
				CheckedAt:     completedAt,
			})
		}
		if err := model.CommitChannelMonitorRun(monitor.ID, owner, checks, completedAt, 0, channelMonitorCredentialBlockedReason); err != nil {
			return nil, err
		}
		InvalidateChannelMonitorPublicSummaryCache()
		return checks, nil
	}

	probeResults := make([]ChannelMonitorProbeResult, 0, len(models))
	for _, modelName := range models {
		probeResults = append(probeResults, ProbeChannelMonitorModel(ctx, monitor, modelName, credential, client))
	}
	completedAt := now().Unix()
	checks := make([]model.ChannelMonitorCheck, 0, len(models))
	for index, modelName := range models {
		result := probeResults[index]
		checks = append(checks, model.ChannelMonitorCheck{
			Model:         modelName,
			Status:        result.Status,
			LatencyMs:     result.LatencyMs,
			ErrorCategory: result.ErrorCategory,
			HTTPStatus:    result.HTTPStatus,
			ErrorMessage:  result.ErrorMessage,
			CheckedAt:     completedAt,
		})
	}
	nextCheckAt := completedAt + int64(monitor.IntervalSeconds)
	if err := model.CommitChannelMonitorRun(monitor.ID, owner, checks, completedAt, nextCheckAt, ""); err != nil {
		return nil, err
	}
	InvalidateChannelMonitorPublicSummaryCache()
	return checks, nil
}

func RunChannelMonitorNow(ctx context.Context, id int) ([]model.ChannelMonitorCheck, error) {
	monitor, err := model.GetChannelMonitor(id)
	if err != nil {
		return nil, err
	}
	if monitor == nil {
		return nil, errors.New("channel monitor not found")
	}
	now := channelMonitorSchedulerNow()
	owner := fmt.Sprintf("%s-channel-monitor-manual-%s", common.NodeName, common.GetRandomString(8))
	claimedMonitor, claimed, err := model.ClaimChannelMonitor(
		id,
		owner,
		now.Unix(),
		now.Unix()+channelMonitorLeaseSeconds(monitor),
		true,
	)
	if err != nil {
		return nil, err
	}
	if !claimed {
		return nil, ErrChannelMonitorBusy
	}
	return runClaimedChannelMonitorWithDependencies(ctx, claimedMonitor, owner, GetHttpClient(), channelMonitorSchedulerNow)
}

func channelMonitorModels(monitor *model.ChannelMonitor) ([]string, error) {
	models := []string{monitor.PrimaryModel}
	if monitor.ExtraModels == "" {
		return models, nil
	}
	extraModels := make([]string, 0)
	if err := common.UnmarshalJsonStr(monitor.ExtraModels, &extraModels); err != nil {
		return nil, errors.New("decode channel monitor models failed")
	}
	return append(models, extraModels...), nil
}

func channelMonitorLeaseSeconds(monitor *model.ChannelMonitor) int64 {
	modelCount := 1
	if monitor != nil {
		if models, err := channelMonitorModels(monitor); err == nil && len(models) > 0 {
			modelCount = len(models)
		}
	}
	timeoutSeconds := channelMonitorDefaultTimeoutSeconds
	if monitor != nil && monitor.TimeoutSeconds > 0 {
		timeoutSeconds = monitor.TimeoutSeconds
	}
	leaseSeconds := int64(modelCount*timeoutSeconds + 30)
	if leaseSeconds < channelMonitorMinimumLeaseSeconds {
		return channelMonitorMinimumLeaseSeconds
	}
	if leaseSeconds > channelMonitorMaximumLeaseSeconds {
		return channelMonitorMaximumLeaseSeconds
	}
	return leaseSeconds
}
