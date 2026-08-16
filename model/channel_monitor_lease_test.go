package model

import (
	"sync"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestClaimChannelMonitorAllowsOnlyOneConcurrentOwner(t *testing.T) {
	setupChannelMonitorLeaseTestDB(t)
	monitor := channelMonitorLeaseFixture()
	require.NoError(t, DB.Create(monitor).Error)

	type claimResult struct {
		monitor *ChannelMonitor
		claimed bool
		err     error
	}
	start := make(chan struct{})
	results := make(chan claimResult, 2)
	var waitGroup sync.WaitGroup
	for _, owner := range []string{"node-a", "node-b"} {
		waitGroup.Add(1)
		go func(owner string) {
			defer waitGroup.Done()
			<-start
			claimedMonitor, claimed, err := ClaimChannelMonitor(monitor.ID, owner, 100, 200, false)
			results <- claimResult{monitor: claimedMonitor, claimed: claimed, err: err}
		}(owner)
	}
	close(start)
	waitGroup.Wait()
	close(results)

	claimedCount := 0
	for result := range results {
		require.NoError(t, result.err)
		if result.claimed {
			claimedCount++
			require.NotNil(t, result.monitor)
			assert.Contains(t, []string{"node-a", "node-b"}, result.monitor.LeaseOwner)
			assert.Equal(t, int64(200), result.monitor.LeaseUntil)
		} else {
			assert.Nil(t, result.monitor)
		}
	}
	assert.Equal(t, 1, claimedCount)
}

func TestClaimChannelMonitorRecoversExpiredLeaseAndManualIgnoresSchedule(t *testing.T) {
	setupChannelMonitorLeaseTestDB(t)
	monitor := channelMonitorLeaseFixture()
	monitor.NextCheckAt = 1000
	monitor.LeaseOwner = "old-node"
	monitor.LeaseUntil = 99
	require.NoError(t, DB.Create(monitor).Error)

	claimedMonitor, claimed, err := ClaimChannelMonitor(monitor.ID, "manual-node", 100, 200, true)
	require.NoError(t, err)
	assert.True(t, claimed)
	require.NotNil(t, claimedMonitor)
	assert.Equal(t, "manual-node", claimedMonitor.LeaseOwner)

	_, claimed, err = ClaimChannelMonitor(monitor.ID, "other-node", 100, 300, true)
	require.NoError(t, err)
	assert.False(t, claimed)

	require.NoError(t, DB.Model(&ChannelMonitor{}).Where("id = ?", monitor.ID).Updates(map[string]any{
		"next_check_at": 100,
		"lease_owner":   "expired-node",
		"lease_until":   99,
	}).Error)
	claimedMonitor, claimed, err = ClaimChannelMonitor(monitor.ID, "automatic-node", 100, 220, false)
	require.NoError(t, err)
	assert.True(t, claimed)
	require.NotNil(t, claimedMonitor)
	assert.Equal(t, "automatic-node", claimedMonitor.LeaseOwner)
}

func TestFindDueChannelMonitorIDsExcludesDisabledBlockedFutureAndLeased(t *testing.T) {
	setupChannelMonitorLeaseTestDB(t)
	monitors := []*ChannelMonitor{
		channelMonitorLeaseFixture(),
		channelMonitorLeaseFixture(),
		channelMonitorLeaseFixture(),
		channelMonitorLeaseFixture(),
		channelMonitorLeaseFixture(),
	}
	monitors[1].Enabled = false
	monitors[2].ScheduleBlockedReason = "credential_decryption_failed"
	monitors[3].NextCheckAt = 101
	monitors[4].LeaseOwner = "active-node"
	monitors[4].LeaseUntil = 100
	for _, monitor := range monitors {
		require.NoError(t, DB.Create(monitor).Error)
	}

	ids, err := FindDueChannelMonitorIDs(100, 50)
	require.NoError(t, err)
	assert.Equal(t, []int{monitors[0].ID}, ids)
}

func TestClaimChannelMonitorRejectsDisabledAndBlocked(t *testing.T) {
	setupChannelMonitorLeaseTestDB(t)
	disabled := channelMonitorLeaseFixture()
	disabled.Enabled = false
	blocked := channelMonitorLeaseFixture()
	blocked.ScheduleBlockedReason = "credential_decryption_failed"
	require.NoError(t, DB.Create(disabled).Error)
	require.NoError(t, DB.Create(blocked).Error)

	for _, id := range []int{disabled.ID, blocked.ID} {
		claimedMonitor, claimed, err := ClaimChannelMonitor(id, "node", 100, 200, true)
		require.NoError(t, err)
		assert.False(t, claimed)
		assert.Nil(t, claimedMonitor)
	}
}

func setupChannelMonitorLeaseTestDB(t *testing.T) {
	t.Helper()
	previousDB := DB
	previousType := common.MainDatabaseType()
	db, err := gorm.Open(
		sqlite.Open("file:"+uuid.NewString()+"?mode=memory&cache=shared&_pragma=busy_timeout(5000)"),
		&gorm.Config{},
	)
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(4)
	sqlDB.SetMaxIdleConns(4)
	DB = db
	common.SetMainDatabaseType(common.DatabaseTypeSQLite)
	require.NoError(t, db.AutoMigrate(&ChannelMonitor{}))
	t.Cleanup(func() {
		DB = previousDB
		common.SetMainDatabaseType(previousType)
	})
}

func channelMonitorLeaseFixture() *ChannelMonitor {
	return &ChannelMonitor{
		InternalName:        "Internal",
		PublicName:          "Public",
		Provider:            ChannelMonitorProviderOpenAI,
		APIMode:             ChannelMonitorAPIModeChatCompletions,
		Endpoint:            "https://example.com",
		APIKeyEncrypted:     "ciphertext",
		PrimaryModel:        "gpt-test",
		ExtraModels:         "[]",
		IntervalSeconds:     60,
		DegradedThresholdMs: 6000,
		TimeoutSeconds:      30,
		Enabled:             true,
		NextCheckAt:         100,
	}
}
