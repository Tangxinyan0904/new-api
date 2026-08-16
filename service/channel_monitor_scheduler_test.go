package service

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestRunClaimedChannelMonitorCommitsModelsSequentially(t *testing.T) {
	setupChannelMonitorSchedulerTestDB(t)
	credential, suffix, err := EncryptChannelMonitorCredential("scheduler-secret")
	require.NoError(t, err)

	var mutex sync.Mutex
	requestedModels := make([]string, 0)
	var active int32
	var maxActive int32
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		current := atomic.AddInt32(&active, 1)
		updateChannelMonitorMaxConcurrency(&maxActive, current)
		defer atomic.AddInt32(&active, -1)

		var body map[string]any
		require.NoError(t, common.DecodeJson(request.Body, &body))
		modelName, ok := body["model"].(string)
		require.True(t, ok)
		mutex.Lock()
		requestedModels = append(requestedModels, modelName)
		mutex.Unlock()
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"choices":[{"message":{"content":"NEWAPI_MONITOR_OK"}}]}`))
	}))
	defer server.Close()

	monitor := channelMonitorSchedulerFixture(server.URL, credential, suffix)
	monitor.PrimaryModel = "model-a"
	monitor.ExtraModels = `["model-b","model-c"]`
	monitor.LeaseOwner = "scheduler-owner"
	monitor.LeaseUntil = 2000
	require.NoError(t, model.DB.Create(monitor).Error)

	checks, err := runClaimedChannelMonitorWithDependencies(
		context.Background(),
		monitor,
		"scheduler-owner",
		server.Client(),
		func() time.Time { return time.Unix(1000, 0) },
	)
	require.NoError(t, err)
	require.Len(t, checks, 3)
	assert.Equal(t, []string{"model-a", "model-b", "model-c"}, requestedModels)
	assert.Equal(t, int32(1), maxActive)
	for _, check := range checks {
		assert.Equal(t, int64(1000), check.CheckedAt)
		assert.Equal(t, model.ChannelMonitorStatusOperational, check.Status)
	}

	stored, err := model.GetChannelMonitor(monitor.ID)
	require.NoError(t, err)
	require.NotNil(t, stored)
	assert.Equal(t, int64(1000), stored.LastCheckedAt)
	assert.Equal(t, int64(1060), stored.NextCheckAt)
	assert.Empty(t, stored.LeaseOwner)
	assert.Zero(t, stored.LeaseUntil)

	var persisted []model.ChannelMonitorCheck
	require.NoError(t, model.DB.Where("monitor_id = ?", monitor.ID).Order("id ASC").Find(&persisted).Error)
	assert.Equal(t, checks, persisted)
}

func TestRunClaimedChannelMonitorBlocksUndecryptableCredentialWithoutHTTP(t *testing.T) {
	setupChannelMonitorSchedulerTestDB(t)
	monitor := channelMonitorSchedulerFixture("https://example.com", "v1:invalid", "")
	monitor.ExtraModels = `["model-b"]`
	monitor.PublicEnabled = true
	monitor.LeaseOwner = "scheduler-owner"
	monitor.LeaseUntil = 2000
	require.NoError(t, model.DB.Create(monitor).Error)
	setChannelMonitorQueryClock(t, time.Unix(1000, 0), time.UTC)
	before, err := GetChannelMonitorPublicSummary()
	require.NoError(t, err)
	require.Len(t, before.Monitors, 1)
	assert.Equal(t, model.ChannelMonitorStatusUnknown, before.Monitors[0].Status)

	var requests int32
	client := &http.Client{Transport: channelMonitorRoundTripFunc(func(*http.Request) (*http.Response, error) {
		atomic.AddInt32(&requests, 1)
		return nil, errors.New("HTTP should not be called")
	})}
	checks, err := runClaimedChannelMonitorWithDependencies(
		context.Background(),
		monitor,
		"scheduler-owner",
		client,
		func() time.Time { return time.Unix(1000, 0) },
	)
	require.NoError(t, err)
	require.Len(t, checks, 2)
	assert.Zero(t, requests)
	for _, check := range checks {
		assert.Equal(t, model.ChannelMonitorStatusFailed, check.Status)
		assert.Equal(t, model.ChannelMonitorErrorConfig, check.ErrorCategory)
		assert.Equal(t, int64(1000), check.CheckedAt)
	}

	stored, err := model.GetChannelMonitor(monitor.ID)
	require.NoError(t, err)
	require.NotNil(t, stored)
	assert.Equal(t, channelMonitorCredentialBlockedReason, stored.ScheduleBlockedReason)
	assert.Zero(t, stored.NextCheckAt)
	assert.Empty(t, stored.LeaseOwner)
	assert.Zero(t, stored.LeaseUntil)

	after, err := GetChannelMonitorPublicSummary()
	require.NoError(t, err)
	require.Len(t, after.Monitors, 1)
	assert.Equal(t, model.ChannelMonitorStatusFailed, after.Monitors[0].Status)
}

func TestChannelMonitorSchedulerUsesBoundedWorkerPool(t *testing.T) {
	setupChannelMonitorSchedulerTestDB(t)
	credential, suffix, err := EncryptChannelMonitorCredential("worker-pool-secret")
	require.NoError(t, err)

	started := make(chan struct{}, 7)
	release := make(chan struct{})
	var active int32
	var maxActive int32
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		current := atomic.AddInt32(&active, 1)
		updateChannelMonitorMaxConcurrency(&maxActive, current)
		started <- struct{}{}
		<-release
		atomic.AddInt32(&active, -1)
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"choices":[{"message":{"content":"NEWAPI_MONITOR_OK"}}]}`))
	}))
	defer server.Close()

	for index := 0; index < 7; index++ {
		monitor := channelMonitorSchedulerFixture(server.URL, credential, suffix)
		monitor.InternalName = channelMonitorWorkerName(index)
		monitor.PublicName = monitor.InternalName
		require.NoError(t, model.DB.Create(monitor).Error)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	scheduler := newChannelMonitorScheduler(channelMonitorSchedulerConfig{
		Owner:       "worker-owner",
		WorkerCount: 3,
		BatchSize:   50,
		Client:      server.Client(),
		Now:         func() time.Time { return time.Unix(100, 0) },
	})
	scheduler.start(ctx)
	t.Cleanup(scheduler.stop)
	require.NoError(t, scheduler.scan(ctx))

	waitContext, waitCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer waitCancel()
	for index := 0; index < 3; index++ {
		select {
		case <-started:
		case <-waitContext.Done():
			t.Fatal("workers did not start the expected concurrent requests")
		}
	}
	assert.Equal(t, int32(3), maxActive)
	close(release)
	scheduler.wait()

	var checkCount int64
	require.NoError(t, model.DB.Model(&model.ChannelMonitorCheck{}).Count(&checkCount).Error)
	assert.Equal(t, int64(7), checkCount)
}

func TestRunChannelMonitorNowRejectsActiveLease(t *testing.T) {
	setupChannelMonitorSchedulerTestDB(t)
	previousNow := channelMonitorSchedulerNow
	channelMonitorSchedulerNow = func() time.Time { return time.Unix(100, 0) }
	t.Cleanup(func() { channelMonitorSchedulerNow = previousNow })

	monitor := channelMonitorSchedulerFixture("https://example.com", "ciphertext", "text")
	monitor.LeaseOwner = "active-owner"
	monitor.LeaseUntil = 200
	require.NoError(t, model.DB.Create(monitor).Error)

	_, err := RunChannelMonitorNow(context.Background(), monitor.ID)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrChannelMonitorBusy)
}

func TestChannelMonitorSchedulerAggregatesHistoryAtMostOncePerMinute(t *testing.T) {
	setupChannelMonitorSchedulerTestDB(t)
	now := time.Unix(100, 0)
	maintenanceCalls := 0
	scheduler := newChannelMonitorScheduler(channelMonitorSchedulerConfig{
		Owner:       "maintenance-owner",
		WorkerCount: 1,
		BatchSize:   10,
		Now:         func() time.Time { return now },
		AggregateHistory: func(runAt time.Time) error {
			maintenanceCalls++
			assert.Equal(t, now, runAt)
			return nil
		},
	})

	require.NoError(t, scheduler.scan(context.Background()))
	require.NoError(t, scheduler.scan(context.Background()))
	assert.Equal(t, 1, maintenanceCalls)

	now = now.Add(59 * time.Second)
	require.NoError(t, scheduler.scan(context.Background()))
	assert.Equal(t, 1, maintenanceCalls)

	now = now.Add(time.Second)
	require.NoError(t, scheduler.scan(context.Background()))
	assert.Equal(t, 2, maintenanceCalls)
}

func setupChannelMonitorSchedulerTestDB(t *testing.T) {
	t.Helper()
	previousDB := model.DB
	previousType := common.MainDatabaseType()
	previousSecret := common.CryptoSecret
	db, err := gorm.Open(
		sqlite.Open("file:"+uuid.NewString()+"?mode=memory&cache=shared&_pragma=busy_timeout(5000)"),
		&gorm.Config{},
	)
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(8)
	sqlDB.SetMaxIdleConns(8)
	model.DB = db
	common.SetMainDatabaseType(common.DatabaseTypeSQLite)
	common.CryptoSecret = "scheduler-test-secret"
	InvalidateChannelMonitorPublicSummaryCache()
	require.NoError(t, db.AutoMigrate(
		&model.ChannelMonitor{},
		&model.ChannelMonitorCheck{},
		&model.ChannelMonitorHourlyRollup{},
		&model.ChannelMonitorDailyRollup{},
	))
	t.Cleanup(func() {
		InvalidateChannelMonitorPublicSummaryCache()
		model.DB = previousDB
		common.SetMainDatabaseType(previousType)
		common.CryptoSecret = previousSecret
	})
}

func channelMonitorSchedulerFixture(endpoint string, credential string, suffix string) *model.ChannelMonitor {
	return &model.ChannelMonitor{
		InternalName:        "Internal",
		PublicName:          "Public",
		Provider:            model.ChannelMonitorProviderOpenAI,
		APIMode:             model.ChannelMonitorAPIModeChatCompletions,
		Endpoint:            endpoint,
		APIKeyEncrypted:     credential,
		APIKeySuffix:        suffix,
		PrimaryModel:        "model-a",
		ExtraModels:         "[]",
		IntervalSeconds:     60,
		DegradedThresholdMs: 6000,
		TimeoutSeconds:      30,
		Enabled:             true,
		NextCheckAt:         100,
	}
}

func updateChannelMonitorMaxConcurrency(maximum *int32, current int32) {
	for {
		observed := atomic.LoadInt32(maximum)
		if current <= observed || atomic.CompareAndSwapInt32(maximum, observed, current) {
			return
		}
	}
}

func channelMonitorWorkerName(index int) string {
	return "worker-" + string(rune('a'+index))
}
