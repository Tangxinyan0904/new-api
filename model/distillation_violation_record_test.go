package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAdvanceDistillationPenaltyPersistsTransitionHistory(t *testing.T) {
	setupDistillationPenaltyFixture(t)

	firstTrigger := DistillationTrigger{
		UserID: 7, TriggeredAt: 1000, PenaltySeconds: 600, ObservationSeconds: 3600,
		RequestCount: 200, DetectionThreshold: 200, PenaltyRPM: 10,
	}
	_, err := AdvanceDistillationPenalty(firstTrigger)
	require.NoError(t, err)

	secondTrigger := firstTrigger
	secondTrigger.TriggeredAt = 1700
	secondTrigger.RequestCount = 201
	_, err = AdvanceDistillationPenalty(secondTrigger)
	require.NoError(t, err)

	var records []DistillationViolationRecord
	require.NoError(t, DB.Order("id ASC").Find(&records).Error)
	require.Len(t, records, 2)

	assert.Equal(t, 7, records[0].UserId)
	assert.Equal(t, int64(1000), records[0].CycleStartedAt)
	assert.Equal(t, int64(1000), records[0].TriggeredAt)
	assert.Equal(t, 200, records[0].RequestCount)
	assert.Equal(t, 200, records[0].DetectionThreshold)
	assert.Equal(t, 10, records[0].PenaltyRPM)
	assert.Equal(t, DistillationViolationActionTemporaryLimit, records[0].Action)
	assert.Equal(t, int64(1600), records[0].EffectiveUntil)

	assert.Equal(t, int64(1000), records[1].CycleStartedAt)
	assert.Equal(t, int64(1700), records[1].TriggeredAt)
	assert.Equal(t, 201, records[1].RequestCount)
	assert.Equal(t, DistillationViolationActionPermanentBan, records[1].Action)
	assert.Zero(t, records[1].EffectiveUntil)
}

func TestAdvanceDistillationPenaltyDoesNotRepeatTemporaryHistory(t *testing.T) {
	setupDistillationPenaltyFixture(t)

	firstTrigger := DistillationTrigger{
		UserID: 11, TriggeredAt: 1000, PenaltySeconds: 600, ObservationSeconds: 3600,
		RequestCount: 200, DetectionThreshold: 200, PenaltyRPM: 10,
	}
	_, err := AdvanceDistillationPenalty(firstTrigger)
	require.NoError(t, err)

	repeatedTrigger := firstTrigger
	repeatedTrigger.TriggeredAt = 1200
	repeatedTrigger.RequestCount = 250
	_, err = AdvanceDistillationPenalty(repeatedTrigger)
	require.NoError(t, err)

	var records []DistillationViolationRecord
	require.NoError(t, DB.Where("user_id = ?", 11).Find(&records).Error)
	require.Len(t, records, 1)
	assert.Equal(t, int64(1000), records[0].TriggeredAt)
	assert.Equal(t, DistillationViolationActionTemporaryLimit, records[0].Action)
}

func TestListUserDistillationViolationRecordsIsolatesAndPaginates(t *testing.T) {
	setupDistillationPenaltyFixture(t)
	records := []DistillationViolationRecord{
		{UserId: 41, CycleStartedAt: 1000, TriggeredAt: 1000, RequestCount: 200, DetectionThreshold: 200, PenaltyRPM: 10, Action: DistillationViolationActionTemporaryLimit, EffectiveUntil: 1600},
		{UserId: 42, CycleStartedAt: 1100, TriggeredAt: 1100, RequestCount: 200, DetectionThreshold: 200, PenaltyRPM: 10, Action: DistillationViolationActionTemporaryLimit, EffectiveUntil: 1700},
		{UserId: 41, CycleStartedAt: 1000, TriggeredAt: 1700, RequestCount: 201, DetectionThreshold: 200, PenaltyRPM: 10, Action: DistillationViolationActionPermanentBan},
	}
	require.NoError(t, DB.Create(&records).Error)

	items, total, err := ListUserDistillationViolationRecords(41, &common.PageInfo{Page: 1, PageSize: 1})
	require.NoError(t, err)
	assert.Equal(t, int64(2), total)
	require.Len(t, items, 1)
	assert.Equal(t, records[2].Id, items[0].Id)
	assert.Equal(t, DistillationViolationActionPermanentBan, items[0].Action)

	empty, total, err := ListUserDistillationViolationRecords(99, nil)
	require.NoError(t, err)
	assert.Zero(t, total)
	assert.NotNil(t, empty)
	assert.Empty(t, empty)
}

func TestListUserDistillationViolationRecordsRejectsInvalidUser(t *testing.T) {
	setupDistillationPenaltyFixture(t)

	items, total, err := ListUserDistillationViolationRecords(0, nil)
	require.Error(t, err)
	assert.Nil(t, items)
	assert.Zero(t, total)
}
