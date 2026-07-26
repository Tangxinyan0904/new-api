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

func setupDistillationPenaltyFixture(t *testing.T) *gorm.DB {
	t.Helper()

	originalDB := DB
	originalMainDatabaseType := common.MainDatabaseType()
	db, err := gorm.Open(sqlite.Open("file:"+uuid.NewString()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(4)
	sqlDB.SetMaxIdleConns(4)

	DB = db
	common.SetMainDatabaseType(common.DatabaseTypeSQLite)
	require.NoError(t, db.AutoMigrate(&DistillationPenalty{}, &User{}))

	t.Cleanup(func() {
		DB = originalDB
		common.SetMainDatabaseType(originalMainDatabaseType)
		require.NoError(t, sqlDB.Close())
	})
	return db
}

func TestAdvanceDistillationPenaltyCreatesTemporaryThenBansInObservation(t *testing.T) {
	setupDistillationPenaltyFixture(t)

	first, err := AdvanceDistillationPenalty(7, 1000, 600, 3600)
	require.NoError(t, err)
	assert.Equal(t, DistillationPenaltyPhaseTemporary, first.Phase(1000))
	assert.Equal(t, int64(1000), first.FirstTriggeredAt)
	assert.Equal(t, int64(1600), first.TemporaryLimitedUntil)
	assert.Equal(t, int64(5200), first.ObservationUntil)

	duringTemporary, err := AdvanceDistillationPenalty(7, 1200, 600, 3600)
	require.NoError(t, err)
	assert.Zero(t, duringTemporary.PermanentlyBannedAt)
	assert.Equal(t, int64(1000), duringTemporary.FirstTriggeredAt)

	second, err := AdvanceDistillationPenalty(7, 1700, 600, 3600)
	require.NoError(t, err)
	assert.Equal(t, DistillationPenaltyPhasePermanent, second.Phase(1700))
	assert.Equal(t, int64(1700), second.PermanentlyBannedAt)
}

func TestAdvanceDistillationPenaltyObservationExpiryStartsNewFirstStrike(t *testing.T) {
	setupDistillationPenaltyFixture(t)

	_, err := AdvanceDistillationPenalty(8, 1000, 600, 3600)
	require.NoError(t, err)

	renewed, err := AdvanceDistillationPenalty(8, 5200, 600, 3600)
	require.NoError(t, err)
	assert.Equal(t, DistillationPenaltyPhaseTemporary, renewed.Phase(5200))
	assert.Equal(t, int64(5200), renewed.FirstTriggeredAt)
	assert.Equal(t, int64(5800), renewed.TemporaryLimitedUntil)
	assert.Equal(t, int64(9400), renewed.ObservationUntil)
	assert.Zero(t, renewed.PermanentlyBannedAt)

	var count int64
	require.NoError(t, DB.Model(&DistillationPenalty{}).Where("user_id = ?", 8).Count(&count).Error)
	assert.Equal(t, int64(1), count)
}

func TestGetDistillationPenaltyDeletesExpiredObservation(t *testing.T) {
	setupDistillationPenaltyFixture(t)
	require.NoError(t, DB.Create(&DistillationPenalty{
		UserId:                9,
		FirstTriggeredAt:      100,
		TemporaryLimitedUntil: 200,
		ObservationUntil:      300,
	}).Error)

	penalty, err := GetDistillationPenalty(9, 300)
	require.NoError(t, err)
	assert.Nil(t, penalty)

	var count int64
	require.NoError(t, DB.Model(&DistillationPenalty{}).Where("user_id = ?", 9).Count(&count).Error)
	assert.Zero(t, count)
}

func TestClearDistillationPenaltyIsIdempotent(t *testing.T) {
	setupDistillationPenaltyFixture(t)
	_, err := AdvanceDistillationPenalty(10, 1000, 600, 3600)
	require.NoError(t, err)

	require.NoError(t, ClearDistillationPenalty(10))
	require.NoError(t, ClearDistillationPenalty(10))

	penalty, err := GetDistillationPenalty(10, 1000)
	require.NoError(t, err)
	assert.Nil(t, penalty)
}

func TestListDistillationPenaltiesFiltersExpiredRowsAndSearchesUsers(t *testing.T) {
	setupDistillationPenaltyFixture(t)
	users := []User{
		{Id: 21, Username: "alice", Password: "password", AffCode: "alice-code"},
		{Id: 22, Username: "bob", Password: "password", AffCode: "bob-code"},
		{Id: 23, Username: "expired", Password: "password", AffCode: "expired-code"},
	}
	require.NoError(t, DB.Create(&users).Error)
	require.NoError(t, DB.Create(&[]DistillationPenalty{
		{UserId: 21, FirstTriggeredAt: 1000, TemporaryLimitedUntil: 1600, ObservationUntil: 5200},
		{UserId: 22, FirstTriggeredAt: 900, TemporaryLimitedUntil: 1500, ObservationUntil: 5100, PermanentlyBannedAt: 1700},
		{UserId: 23, FirstTriggeredAt: 100, TemporaryLimitedUntil: 200, ObservationUntil: 300},
	}).Error)

	items, total, err := ListDistillationPenalties("ali", &common.PageInfo{Page: 1, PageSize: 10}, 1200)
	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
	require.Len(t, items, 1)
	assert.Equal(t, 21, items[0].UserId)
	assert.Equal(t, "alice", items[0].Username)
	assert.Equal(t, DistillationPenaltyPhaseTemporary, items[0].Phase)

	items, total, err = ListDistillationPenalties("22", &common.PageInfo{Page: 1, PageSize: 10}, 1200)
	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
	require.Len(t, items, 1)
	assert.Equal(t, DistillationPenaltyPhasePermanent, items[0].Phase)
}

func TestAdvanceDistillationPenaltyConcurrentSecondTriggerWritesOnePermanentState(t *testing.T) {
	setupDistillationPenaltyFixture(t)
	_, err := AdvanceDistillationPenalty(30, 1000, 600, 3600)
	require.NoError(t, err)

	start := make(chan struct{})
	results := make(chan error, 2)
	var ready sync.WaitGroup
	ready.Add(2)
	var workers sync.WaitGroup
	workers.Add(2)
	for range 2 {
		go func() {
			defer workers.Done()
			ready.Done()
			<-start
			_, transitionErr := AdvanceDistillationPenalty(30, 1700, 600, 3600)
			results <- transitionErr
		}()
	}
	ready.Wait()
	close(start)
	workers.Wait()
	close(results)

	successes := 0
	for result := range results {
		if result == nil {
			successes++
		}
	}
	assert.Positive(t, successes)

	penalty, err := GetDistillationPenalty(30, 1700)
	require.NoError(t, err)
	require.NotNil(t, penalty)
	assert.Equal(t, int64(1700), penalty.PermanentlyBannedAt)

	var count int64
	require.NoError(t, DB.Model(&DistillationPenalty{}).Where("user_id = ?", 30).Count(&count).Error)
	assert.Equal(t, int64(1), count)
}
