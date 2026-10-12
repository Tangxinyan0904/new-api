package service

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	"github.com/bytedance/gopkg/util/gopool"
)

const (
	channelCostRollupInterval = 5 * time.Minute
	// channelCostBackfillHours covers today, yesterday and the 7-day range in
	// any time zone. It is rebuilt once when the master node starts.
	channelCostBackfillHours = 8*24 + 24
	// channelCostRefreshHours is rebuilt on every tick: the current hour and
	// the hours that may have finished since the previous tick.
	channelCostRefreshHours = 3
	channelCostRetention    = 31 * 24 * time.Hour
)

var (
	channelCostRollupOnce    sync.Once
	channelCostRollupRunning atomic.Bool
)

// StartChannelCostRollupTask keeps the hourly channel cost buckets in step
// with the logs. Only the master node writes them.
func StartChannelCostRollupTask() {
	channelCostRollupOnce.Do(func() {
		if !common.IsMasterNode {
			return
		}
		gopool.Go(func() {
			logger.LogInfo(context.Background(), common.LogText("channel cost rollup task started: tick=%s", channelCostRollupInterval))
			rebuildRecentChannelCostHours(channelCostBackfillHours)
			ticker := time.NewTicker(channelCostRollupInterval)
			defer ticker.Stop()
			for range ticker.C {
				rebuildRecentChannelCostHours(channelCostRefreshHours)
			}
		})
	})
}

// rebuildRecentChannelCostHours rebuilds the latest hours buckets, oldest
// first, and drops buckets past the retention period.
func rebuildRecentChannelCostHours(hours int) {
	if !channelCostRollupRunning.CompareAndSwap(false, true) {
		return
	}
	defer channelCostRollupRunning.Store(false)

	now := time.Now().Unix()
	currentHour := now - now%model.ChannelCostBucketSeconds
	for i := hours - 1; i >= 0; i-- {
		hourStart := currentHour - int64(i)*model.ChannelCostBucketSeconds
		if err := model.RebuildChannelCostHour(hourStart); err != nil {
			common.SysError(common.LogText("failed to rebuild channel cost hour %d: %s", hourStart, err.Error()))
		}
	}
	if err := model.DeleteChannelCostHoursBefore(now - int64(channelCostRetention/time.Second)); err != nil {
		common.SysError(common.LogText("failed to prune channel cost hours: %s", err.Error()))
	}
}
